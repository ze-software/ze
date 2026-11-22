// Design: docs/architecture/testing/interop.md -- live ESP form changes on one Child SA.
// Related: checkers.go -- starting-form prerequisites; helpers.go -- directed counters.
// Related: peerform/action.go -- the sanitized peer-local helper receipt.

package ipsec

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const (
	espFormPeerHelper         = "/usr/bin/ipsec-peerform"
	espFormSequenceMax uint64 = 1<<32 - 1
)

// espFormState retains only identity metadata. No algorithm or key enters a
// comparison or diagnostic. Its zero value identifies no usable ESP state.
type espFormState struct {
	key      readbackSAKey
	reqid    uint32
	selector string
}

// espFormProgress advances only after a complete helper receipt matches the
// selected state and the replay floor. Its zero value cannot authorize a probe.
type espFormProgress struct {
	state    espFormState
	sequence uint64
}

func runESPFormPhases(ctx context.Context, lab *scenarioLab, zeBefore, swanBefore map[readbackSAKey]espFormState) error {
	var selected espFormState
	for key, state := range swanBefore {
		if key.source == swanIP {
			selected = state
		}
	}
	if selected.key.spi == 0 {
		return fmt.Errorf("strongSwan has no selected outbound ESP state")
	}
	progress := espFormProgress{state: selected}
	dropsBefore, err := rawESPDrops(ctx, lab)
	if err != nil {
		return err
	}
	mismatchBefore, err := zeXFRMStateMismatch(ctx, lab)
	if err != nil {
		return err
	}
	before, err := lab.espCounters(ctx, espFormChangeDirections)
	if err != nil {
		return err
	}
	for phase, form := range []string{"bare", "udp", "bare"} {
		identity := []string{swanIP, zeIP, fmt.Sprintf("%#x", selected.key.spi)}
		if phase != 0 {
			argv := append([]string{espFormPeerHelper, form}, identity...)
			if form == "udp" {
				argv = append(argv, "4500", "4500")
			}
			answer, err := lab.exec(ctx, swanPeer, argv...)
			if err != nil {
				return fmt.Errorf("change strongSwan ESP form to %s: %w", form, err)
			}
			if err := progress.readESPFormReceipt(answer, form, form, false); err != nil {
				return fmt.Errorf("strongSwan %s migration receipt: %w", form, err)
			}
		}
		argv := append([]string{espFormPeerHelper, "probe", containerInterface}, identity...)
		argv = append(argv, "4301", strconv.Itoa(phase+1))
		answer, err := lab.exec(ctx, swanPeer, argv...)
		if err != nil {
			return fmt.Errorf("probe strongSwan %s ESP delivery: %w", form, err)
		}
		if err := progress.readESPFormReceipt(answer, "probe", form, true); err != nil {
			return fmt.Errorf("strongSwan %s wire receipt: %w", form, err)
		}
		after, err := lab.espCounters(ctx, espFormChangeDirections)
		if err != nil {
			return err
		}
		for _, want := range espFormChangeDirections {
			if err := assertESPAdvanced(before[want.peer], after[want.peer], want); err != nil {
				return fmt.Errorf("ESP %s phase %d: %w", form, phase+1, err)
			}
		}
		before = after
		if err := requireESPFormIdentities(ctx, lab, zeBefore, swanBefore); err != nil {
			return err
		}
		mismatchAfter, err := zeXFRMStateMismatch(ctx, lab)
		if err != nil {
			return err
		}
		if mismatchAfter < mismatchBefore {
			return fmt.Errorf("XfrmInStateMismatch decreased during the ESP form change")
		}
		mismatchBefore = mismatchAfter
		dropsAfter, err := rawESPDrops(ctx, lab)
		if err != nil {
			return err
		}
		if dropsAfter > dropsBefore {
			return fmt.Errorf("kernel dropped %d packets on Ze raw ESP socket", dropsAfter-dropsBefore)
		}
	}
	if err := waitDuration(ctx, 2*time.Second); err != nil {
		return err
	}
	if err := requireESPFormIdentities(ctx, lab, zeBefore, swanBefore); err != nil {
		return err
	}
	logs, err := lab.logs(ctx, swanPeer)
	if err != nil {
		return err
	}
	for _, refused := range []string{"received DELETE for ESP CHILD_SA", "REKEY_SA"} {
		if strings.Contains(logs, refused) {
			return fmt.Errorf("strongSwan reports %s during the ESP form change", refused)
		}
	}
	return nil
}

func (progress *espFormProgress) readESPFormReceipt(answer, action, form string, advance bool) error {
	var receipt struct {
		Action   string  `json:"action"`
		Source   string  `json:"source"`
		Target   string  `json:"target"`
		SPI      *uint32 `json:"spi"`
		ReqID    *uint32 `json:"reqid"`
		Form     string  `json:"form"`
		Sequence *uint64 `json:"sequence"`
	}
	decoder := json.NewDecoder(strings.NewReader(answer))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return fmt.Errorf("decode peer helper output: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("peer helper output contains trailing data")
	}
	if receipt.Action != action {
		return fmt.Errorf("peer helper action = %q, want %q", receipt.Action, action)
	}
	if receipt.Source != progress.state.key.source {
		return fmt.Errorf("peer helper reported another source")
	}
	if receipt.Target != progress.state.key.target {
		return fmt.Errorf("peer helper reported another target")
	}
	if receipt.SPI == nil {
		return fmt.Errorf("peer helper omitted the SPI")
	}
	if *receipt.SPI != progress.state.key.spi {
		return fmt.Errorf("peer helper reported another SPI")
	}
	if receipt.ReqID == nil {
		return fmt.Errorf("peer helper omitted the reqid")
	}
	if *receipt.ReqID != progress.state.reqid {
		return fmt.Errorf("peer helper changed the reqid")
	}
	if receipt.Form != form {
		return fmt.Errorf("peer helper observed ESP form %q, want %q", receipt.Form, form)
	}
	if receipt.Sequence == nil {
		return fmt.Errorf("peer helper omitted the replay sequence")
	}
	if *receipt.Sequence > espFormSequenceMax {
		return fmt.Errorf("peer helper reported a sequence outside non-ESN ESP")
	}
	if advance {
		if *receipt.Sequence <= progress.sequence {
			return fmt.Errorf("wire ESP sequence did not advance beyond %d", progress.sequence)
		}
	} else {
		if *receipt.Sequence < progress.sequence {
			return fmt.Errorf("atomic migration rolled back the replay sequence")
		}
	}
	progress.sequence = *receipt.Sequence
	return nil
}

func requireESPFormIdentities(ctx context.Context, lab *scenarioLab, zeBefore, swanBefore map[readbackSAKey]espFormState) error {
	for _, peer := range []string{zePeer, swanPeer} {
		after, err := readESPFormStates(ctx, lab, peer)
		if err != nil {
			return err
		}
		before := zeBefore
		if peer == swanPeer {
			before = swanBefore
		}
		if len(before) != len(after) {
			return fmt.Errorf("%s changed its directed ESP state count", peer)
		}
		for key, original := range before {
			current, ok := after[key]
			if !ok {
				return fmt.Errorf("%s replaced a directed ESP SPI during the form change", peer)
			}
			if current != original {
				return fmt.Errorf("%s changed the ESP reqid or selector during the form change", peer)
			}
		}
	}
	return nil
}

func readESPFormStates(ctx context.Context, lab *scenarioLab, peer string) (map[readbackSAKey]espFormState, error) {
	answer, err := lab.xfrmState(ctx, peer)
	if err != nil {
		return nil, err
	}
	states, err := parseESPFormStates(answer)
	if err != nil {
		return nil, fmt.Errorf("%s ESP identity: %w", peer, err)
	}
	return states, nil
}

// parseESPFormStates reuses the lab's address, ESP SPI and reqid readers. This
// projection retains only the two directed tunnel identities and selectors.
func parseESPFormStates(answer string) (map[readbackSAKey]espFormState, error) {
	states := make(map[readbackSAKey]espFormState)
	var state espFormState
	var esp, reqidSeen, selectorSeen, modeSeen bool
	flush := func() error {
		if !esp {
			return nil
		}
		if state.key.source == "" {
			return fmt.Errorf("ESP state has no source header")
		}
		if state.key.target == "" {
			return fmt.Errorf("ESP state has no target header")
		}
		if !reqidSeen {
			return fmt.Errorf("ESP state has no reqid")
		}
		if !modeSeen {
			return fmt.Errorf("ESP state has no tunnel mode")
		}
		if _, duplicate := states[state.key]; duplicate {
			return fmt.Errorf("duplicate directed ESP identity")
		}
		states[state.key] = state
		return nil
	}
	for line := range strings.SplitSeq(answer, "\n") {
		if match := srcDstPattern.FindStringSubmatch(line); match != nil {
			if err := flush(); err != nil {
				return nil, err
			}
			state = espFormState{key: readbackSAKey{source: match[1], target: match[2]}}
			esp, reqidSeen, selectorSeen, modeSeen = false, false, false, false
			continue
		}
		if match := espSPIPattern.FindStringSubmatch(line); match != nil {
			spi, err := strconv.ParseUint(match[1], 0, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid ESP SPI: %w", err)
			}
			if spi == 0 {
				return nil, fmt.Errorf("ESP SPI is zero")
			}
			state.key.spi, esp = uint32(spi), true
		}
		if match := reqidPattern.FindStringSubmatch(line); match != nil {
			if reqidSeen {
				return nil, fmt.Errorf("duplicate ESP reqid")
			}
			reqid, err := strconv.ParseUint(match[1], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid ESP reqid: %w", err)
			}
			state.reqid, reqidSeen = uint32(reqid), true
		}
		fields := strings.Fields(line)
		for index := 0; index+1 < len(fields); index++ {
			if fields[index] == "mode" {
				if modeSeen {
					return nil, fmt.Errorf("duplicate ESP mode")
				}
				if fields[index+1] != "tunnel" {
					return nil, fmt.Errorf("ESP form helper requires tunnel mode")
				}
				modeSeen = true
			}
			if fields[index] == xfrmWordIfID {
				ifID, err := strconv.ParseUint(fields[index+1], 0, 32)
				if err != nil {
					return nil, fmt.Errorf("invalid ESP interface ID: %w", err)
				}
				state.key.ifID = uint32(ifID)
			}
		}
		if strings.HasPrefix(strings.TrimSpace(line), "sel ") {
			if selectorSeen {
				return nil, fmt.Errorf("duplicate ESP selector")
			}
			state.selector, selectorSeen = strings.Join(fields, " "), true
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	if len(states) != 2 {
		return nil, fmt.Errorf("expected two directed ESP states, found %d", len(states))
	}
	outbound, inbound := 0, 0
	for key := range states {
		if key.source == swanIP {
			if key.target != zeIP {
				return nil, fmt.Errorf("strongSwan ESP state names another target")
			}
			outbound++
			continue
		}
		if key.source != zeIP {
			return nil, fmt.Errorf("ESP state names another source")
		}
		if key.target != swanIP {
			return nil, fmt.Errorf("ze ESP state names another target")
		}
		inbound++
	}
	if outbound != 1 {
		return nil, fmt.Errorf("expected one strongSwan-to-Ze ESP state")
	}
	if inbound != 1 {
		return nil, fmt.Errorf("expected one Ze-to-strongSwan ESP state")
	}
	return states, nil
}
