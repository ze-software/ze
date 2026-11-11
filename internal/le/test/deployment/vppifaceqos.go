// Design: docs/architecture/testing/interop.md -- real VPP VLAN QoS reload proof
// Related: internal/plugins/iface/vpp/ifacevpp.go -- UpdateVLANQoSMap

package testdeployment

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// This VLAN ID belongs only to the QoS scenario inside its private container.
const qosVLANNameSuffix = ".100"

func vlanQoSConfig(mappings string) string {
	var text textbuf.Buffer
	return text.Str(vppBase).Str(`interface {
    backend vpp;
    dummy qos0 {
        unit q100 {
            vlan-id 100;
            `).Str(mappings).Str(`
        }
    }
}
`).String()
}

// proveVLANQoSReload keeps one daemon alive through both SIGHUPs. Only VPP's
// installed row and mark supply the verdict; a successful reload log cannot.
func (v *vppIface) proveVLANQoSReload(container string, one *vppScenario) (ScenarioResult, error) {
	result := ScenarioResult{Feature: one.feature, Outcome: OutcomeFail, Detail: one.missingDetail}
	// No earlier scenario creates a QoS map, and this private container has no
	// other map owner. No stage deletes a map, so the sole first allocation
	// stays at pool index 0 (qos_egress_map_find_or_create in the pinned VPP).
	maps, ok := v.vppctl(container, "show qos egress map")
	if !ok {
		return result, errors.New("VPP QoS map preflight query failed")
	}
	if strings.TrimSpace(maps) != "" {
		return result, errors.New("VPP QoS proof requires a fresh empty map pool")
	}
	if err := v.stageConfig(container, one.file); err != nil {
		return result, err
	}
	seen := newCollector()
	spec := v.daemonExec(container, daemonRel(v.Goarch), one.file)
	daemon, err := startWatchedExec(v.ctx, v.Docker, spec, "ze> ", seen, v.Progress)
	if err != nil {
		return result, err
	}
	// The private container and exact argv identify the command this proof
	// started, as in startEvidenceDaemon. Never signal the local docker client.
	pattern := "^" + regexp.QuoteMeta(strings.Join(spec.Command, " ")) + "$"
	daemon.signal = func(signal syscall.Signal) error {
		_, signalErr := v.Docker.ExecAttached(context.WithoutCancel(v.ctx), interoplab.ContainerExec{
			Container: container,
			Command:   []string{"pkill", "-" + strconv.Itoa(int(signal)), "-f", pattern},
			Timeout:   stopGrace,
		})
		return signalErr
	}
	defer stopVPPProcess(daemon, seen)

	var expected [256]byte
	expected[3], expected[5] = 3, 5
	name, index, evidence, err := v.awaitVLANQoS(container, daemon, "", 0, &expected, true)
	if err != nil {
		result.LogTail = seen.tailLines()
		return result, err
	}
	result.Evidence = append(result.Evidence, "initial: "+evidence)
	stages := []struct {
		mappings string
		marked   bool
	}{
		{mappings: "egress-qos-map 3 { pcp 3; }", marked: true},
		{mappings: "", marked: false},
	}
	expected[5] = 0
	for _, stage := range stages {
		if err := v.reloadVLANQoS(container, one.file, stage.mappings, daemon); err != nil {
			return result, err
		}
		_, _, evidence, err = v.awaitVLANQoS(container, daemon, name, index, &expected, stage.marked)
		if err != nil {
			result.LogTail = seen.tailLines()
			return result, err
		}
		result.Evidence = append(result.Evidence, evidence)
	}
	result.Outcome = OutcomePass
	result.Detail = one.provenDetail
	return result, nil
}

func (v *vppIface) reloadVLANQoS(container, configFile, mappings string, daemon *running) error {
	if daemon.exited() {
		return errors.New("QoS daemon exited before reload")
	}
	_, err := v.Docker.ExecAttached(v.ctx, interoplab.ContainerExec{
		Container:   container,
		Command:     []string{"tee", filepath.Join(zeRunDir, configFile, "ze.conf")},
		Interactive: true, Stdin: strings.NewReader(vlanQoSConfig(mappings)),
		Stdout: io.Discard, Timeout: vppctlTimeout,
	})
	if err != nil {
		return fmt.Errorf("replace QoS candidate: %w", err)
	}
	return daemon.signal(syscall.SIGHUP)
}

// awaitVLANQoS requires the same existing VLAN at every stage, so deleting the
// interface cannot masquerade as clearing its mark. Every query must succeed.
func (v *vppIface) awaitVLANQoS(container string, daemon *running, name string, index uint32, expected *[256]byte, marked bool) (string, uint32, string, error) {
	deadline := time.Now().Add(v.ScenarioWait)
	last := "VLAN not observed"
	for {
		if daemon.exited() {
			return name, index, last, errors.New("QoS daemon exited during its lifecycle proof")
		}
		interfaces, ok := v.vppctl(container, "show interface")
		if !ok {
			return name, index, interfaces, errors.New("VPP interface query failed during QoS proof")
		}
		found, foundIndex, err := qosVLANInterface(interfaces)
		if err == nil {
			if name == "" {
				name, index = found, foundIndex
			}
			if found != name {
				return name, index, interfaces, errors.New("QoS VLAN name changed during reload")
			}
			if foundIndex != index {
				return name, index, interfaces, errors.New("QoS VLAN index changed during reload")
			}
			matches, evidence, queryErr := v.vlanQoSMatches(container, name, index, expected, marked)
			if queryErr != nil {
				return name, index, evidence, queryErr
			}
			last = evidence
			if matches && !daemon.exited() {
				return name, index, evidence, nil
			}
		} else {
			last = err.Error()
		}
		if !time.Now().Before(deadline) {
			return name, index, last, fmt.Errorf("VPP QoS state did not reach candidate: %s", last)
		}
		select {
		case <-v.ctx.Done():
			return name, index, last, v.ctx.Err()
		case <-time.After(vppQueryPoll):
		}
	}
}

func (v *vppIface) vlanQoSMatches(container, name string, index uint32, expected *[256]byte, marked bool) (bool, string, error) {
	mark, ok := v.vppctl(container, "show qos mark "+name)
	if !ok {
		return false, mark, errors.New("VPP QoS mark query failed")
	}
	if !marked {
		return strings.TrimSpace(mark) == "", "disabled mark: " + mark, nil
	}
	// The interface is visible before CreateVLAN finishes its map and mark
	// requests. Successful empty mark output is pending startup, not a bad row.
	if strings.TrimSpace(mark) == "" {
		return false, "VLAN exists but has no mark yet", nil
	}
	row, ok := v.vppctl(container, "show qos egress map")
	if !ok {
		return false, row, errors.New("VPP QoS egress map query failed")
	}
	if err := qosOnlyMap(row, index); err != nil {
		return false, row, err
	}
	actual, err := qosVLANRow(row)
	if err != nil {
		return false, row, err
	}
	return actual == *expected && qosVLANMarked(mark, name), row + mark, nil
}

// qosVLANInterface reads VPP's Name/Idx table, never Ze's config or cache.
func qosVLANInterface(output string) (string, uint32, error) {
	name := ""
	var index uint32
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if !strings.HasSuffix(fields[0], qosVLANNameSuffix) {
			continue
		}
		if name != "" {
			return "", 0, errors.New("VPP reported multiple QoS VLAN candidates")
		}
		parsed, err := strconv.ParseUint(fields[1], 10, 32)
		if err != nil {
			return "", 0, fmt.Errorf("VPP VLAN interface index: %w", err)
		}
		name, index = fields[0], uint32(parsed)
	}
	if name == "" {
		return "", 0, errors.New("VPP has no QoS VLAN interface")
	}
	return name, index, nil
}

// FDio src/vnet/qos/qos_egress_map.c formats each source as source:[256 values].
// Missing, duplicate or truncated VLAN rows are errors, not all-zero maps.
func qosVLANRow(output string) ([256]byte, error) {
	var row [256]byte
	found := false
	for line := range strings.SplitSeq(output, "\n") {
		source, values, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		if !strings.EqualFold(source, "vlan") {
			continue
		}
		if found {
			return row, errors.New("VPP reported duplicate VLAN QoS rows")
		}
		found = true
		values = strings.TrimSpace(values)
		if !strings.HasPrefix(values, "[") {
			return row, errors.New("VPP VLAN QoS row has no brackets")
		}
		if !strings.HasSuffix(values, "]") {
			return row, errors.New("VPP VLAN QoS row has no brackets")
		}
		entries := strings.Split(values[1:len(values)-1], ",")
		if len(entries) != len(row) {
			return row, errors.New("VPP VLAN QoS row does not contain 256 entries")
		}
		for i, value := range entries {
			parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 8)
			if err != nil {
				return row, fmt.Errorf("VPP VLAN QoS value: %w", err)
			}
			row[i] = byte(parsed)
		}
	}
	if !found {
		return row, errors.New("VPP reported no VLAN QoS row")
	}
	return row, nil
}

func qosVLANMarked(output, name string) bool {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		return false
	}
	if strings.TrimSpace(lines[0]) != name+":" {
		return false
	}
	fields := strings.Fields(lines[1])
	if len(fields) != 2 {
		return false
	}
	if !strings.EqualFold(fields[0], "vlan:") {
		return false
	}
	value, ok := strings.CutPrefix(fields[1], "map:")
	if !ok {
		return false
	}
	poolIndex, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return false
	}
	return poolIndex == 0
}

// qosOnlyMap ties the external map ID to the sole first allocation whose
// internal pool index show qos mark prints. An extra map invalidates that proof.
func qosOnlyMap(output string, expected uint32) error {
	found := false
	for line := range strings.SplitSeq(output, "\n") {
		value, ok := strings.CutPrefix(strings.TrimSpace(line), "Map-ID:")
		if !ok {
			continue
		}
		if found {
			return errors.New("VPP QoS proof found more than one map")
		}
		found = true
		id, err := strconv.ParseUint(strings.TrimSpace(value), 10, 32)
		if err != nil {
			return fmt.Errorf("VPP QoS map ID: %w", err)
		}
		if uint32(id) != expected {
			return errors.New("VPP QoS map ID differs from the VLAN index")
		}
	}
	if !found {
		return errors.New("VPP QoS proof found no map ID")
	}
	return nil
}
