// Design: docs/architecture/testing/interop.md -- fail-closed peer assertions.
// Related: checkers.go -- scenario-specific ordered operations.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

type operationKind uint8

const (
	opUnspecified operationKind = iota
	opFRRSession
	opBIRDSession
	opGoBGPSession
	opFRRRoute
	opFRRRouteAbsent
	opBIRDRoute
	opBIRDRouteAbsent
	opGoBGPRoute
	opFRRCommunity
	opFRRNoAS
	opWaitContains
	opRequireContains
	opRequireAbsent
	opExec
	opSignal
	opStart
	opWaitLogFields
	opWaitLogContains
	opWaitAbsent
	opRequireJSONFields
	opWaitJSONFields
	opWaitContainsAny
	opDelayRequireContains
)

type operation struct {
	kind     operationKind
	peer     string
	argument string
	family   string
	command  []string
	contains []string
	absent   []string
	proof    []string
	fields   map[string]string
	minimum  map[string]int
	delay    time.Duration
	timeout  time.Duration
}

func checkScenario(ctx context.Context, check *interoplab.CheckContext, name string) error {
	operations, ok := scenarioOperations[name]
	if !ok || len(operations) == 0 {
		return fmt.Errorf("scenario %s has no typed assertions", name)
	}
	operations = append(append([]operation(nil), operations...), scenarioExtras[name]...)
	for index := range operations {
		if err := runOperation(ctx, check.Network, check.Lab, &operations[index]); err != nil {
			return checkerFailure(ctx, check.Lab, name, index+1, err)
		}
	}
	return nil
}
func checkerFailure(ctx context.Context, lab interoplab.CheckerLab, name string, assertion int, cause error) error {
	var diagnostics textbuf.Buffer
	for _, peer := range []string{"ze", peerFRR, peerBIRD, peerGoBGP, peerInject, peerSpeaker, peerSpeaker2} {
		logs, err := lab.Logs(ctx, peer, 80)
		if err != nil || !logs.Available || strings.TrimSpace(logs.Text) == "" {
			continue
		}
		if peer == "ze" {
			for line := range strings.SplitSeq(logs.Text, "\n") {
				if strings.Contains(line, "ZE-OBSERVER-FAIL") {
					return fmt.Errorf("scenario %s assertion %d: %w; process plugin failed: %s", name, assertion, cause, strings.TrimSpace(line))
				}
			}
		}
		fmt.Fprintf(&diagnostics, "\n--- %s logs ---\n%s", peer, strings.TrimSpace(logs.Text))
	}
	if diagnostics.Len() != 0 {
		return fmt.Errorf("scenario %s assertion %d: %w%s", name, assertion, cause, diagnostics.String())
	}
	return fmt.Errorf("scenario %s assertion %d: %w", name, assertion, cause)
}

func runOperation(ctx context.Context, network interoplab.Network, lab interoplab.CheckerLab, current *operation) error {
	rewriteOperation(network, current)
	var command textbuf.Buffer
	switch current.kind {
	case opUnspecified:
		return errors.New("checker operation is unspecified")
	case opFRRSession:
		return waitContains(
			ctx,
			lab,
			peerFRR,
			[]string{
				cmdVtysh,
				"-c",
				command.Str("show bgp neighbor ").Str(current.argument).String(),
			},
			current.timeout,
			"BGP state = Established",
		)
	case opBIRDSession:
		return waitContains(ctx, lab, peerBIRD, []string{cmdBirdc, "show protocols"}, current.timeout, current.argument, "Established")
	case opGoBGPSession:
		return waitContainsFold(ctx, lab, peerGoBGP, []string{cmdGoBGP, gobgpNeighbor, current.argument}, current.timeout, peerStateEstablished)
	case opFRRRoute:
		return waitFRRRoute(ctx, lab, current.argument, current.family, current.timeout, true)
	case opFRRRouteAbsent:
		return waitFRRRoute(ctx, lab, current.argument, current.family, current.timeout, false)
	case opBIRDRoute:
		return waitContains(
			ctx,
			lab,
			peerBIRD,
			[]string{
				cmdBirdc,
				command.Str("show route for ").Str(current.argument).String(),
			},
			current.timeout,
			current.argument,
		)
	case opBIRDRouteAbsent:
		output, err := lab.Query(
			ctx,
			peerBIRD,
			[]string{
				cmdBirdc,
				command.Str("show route for ").Str(current.argument).String(),
			},
			nil,
		)
		if err != nil {
			return err
		}
		return requireAbsentWithProof(output, []string{current.argument}, current.proof)
	case opGoBGPRoute:
		afi := strings.Fields(current.family)
		family := gobgpFamilyIPv4
		if len(afi) > 0 {
			family = afi[0]
		}
		return waitContains(ctx, lab, peerGoBGP, []string{cmdGoBGP, gobgpGlobal, gobgpRIB, "-a", family, current.argument, "-j"}, current.timeout, current.argument)
	case opFRRCommunity:
		output, err := lab.Query(
			ctx,
			peerFRR,
			[]string{
				cmdVtysh,
				"-c",
				command.Str("show bgp ipv4 unicast ").Str(current.argument).String(),
			},
			nil,
		)
		if err != nil {
			return err
		}
		comma := strings.ReplaceAll(current.contains[0], ":", ",")
		comma = command.Reset().Byte('(').Str(comma).Byte(')').String()
		if strings.Contains(output, current.contains[0]) || strings.Contains(output, comma) {
			return nil
		}
		return fmt.Errorf("FRR route %s is missing community %s", current.argument, current.contains[0])
	case opFRRNoAS:
		return requireASAbsent(
			ctx,
			lab,
			peerFRR,
			[]string{
				cmdVtysh,
				"-c",
				command.Str("show bgp ipv4 unicast ").Str(current.argument).Str(" json").String(),
			},
			current.argument,
			current.absent[0],
		)
	case opWaitContains:
		return waitContains(ctx, lab, current.peer, current.command, current.timeout, current.contains...)
	case opRequireContains:
		output, err := lab.Query(ctx, current.peer, current.command, queryEnvironment(current.peer, current.command))
		if err != nil {
			return err
		}
		return requireContains(output, current.contains...)
	case opRequireAbsent:
		output, err := lab.Query(ctx, current.peer, current.command, queryEnvironment(current.peer, current.command))
		if err != nil {
			return err
		}
		return requireAbsentWithProof(output, current.absent, current.proof)
	case opRequireJSONFields:
		output, err := lab.Query(ctx, current.peer, current.command, queryEnvironment(current.peer, current.command))
		if err != nil {
			return err
		}
		return requireJSONFields(output, current.fields, current.minimum)
	case opWaitAbsent:
		return waitAbsent(ctx, lab, current.peer, current.command, current.timeout, current.absent, current.proof)
	case opWaitJSONFields:
		return waitJSONFields(ctx, lab, current.peer, current.command, current.timeout, current.fields, current.minimum)
	case opExec:
		_, err := lab.Exec(ctx, current.peer, current.command, queryEnvironment(current.peer, current.command))
		return err
	case opSignal:
		return lab.Signal(ctx, current.peer, current.argument)
	case opStart:
		return lab.Start(ctx, current.peer)
	case opWaitLogFields:
		return waitLogFields(ctx, lab, current.peer, current.timeout, current.fields, current.minimum)
	case opWaitLogContains:
		return waitLogContains(ctx, lab, current.peer, current.timeout, current.contains...)
	case opWaitContainsAny:
		return waitContainsAny(ctx, lab, current.peer, current.command, current.timeout, current.contains...)
	case opDelayRequireContains:
		return delayRequireContains(ctx, lab, current.peer, current.command, current.delay, current.contains...)
	}
	return errors.New("checker operation is unspecified")
}

func rewriteOperation(network interoplab.Network, current *operation) {
	current.command = append([]string(nil), current.command...)
	current.contains = append([]string(nil), current.contains...)
	current.absent = append([]string(nil), current.absent...)
	current.proof = append([]string(nil), current.proof...)
	var rendered textbuf.Buffer
	ipv4Prefix := ""
	if network.IPv4.IsValid() {
		octets := network.IPv4.Addr().As4()
		addr := netip.AddrFrom4([4]byte{octets[0], octets[1], octets[2], 0})
		ipv4Prefix = strings.TrimSuffix(rendered.Addr(addr).String(), "0")
	}
	ipv6Prefix := ""
	if network.IPv6.IsValid() {
		ipv6Prefix = rendered.Reset().Str(
			strings.TrimSuffix(network.IPv6.Addr().String(), "::"),
		).Str("::").String()
	}
	rewrite := func(value string) string {
		if ipv4Prefix != "" {
			value = strings.ReplaceAll(value, baseIPv4Prefix, ipv4Prefix)
		}
		if ipv6Prefix != "" {
			value = strings.ReplaceAll(value, baseIPv6Prefix, ipv6Prefix)
		}
		return value
	}
	current.argument = rewrite(current.argument)
	for index := range current.command {
		current.command[index] = rewrite(current.command[index])
	}
	for index := range current.contains {
		current.contains[index] = rewrite(current.contains[index])
	}
	for index := range current.absent {
		current.absent[index] = rewrite(current.absent[index])
	}
	for index := range current.proof {
		current.proof[index] = rewrite(current.proof[index])
	}
}

func waitContains(ctx context.Context, lab interoplab.CheckerLab, peer string, command []string, timeout time.Duration, needles ...string) error {
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	var description textbuf.Buffer
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     timeout,
		Interval:    2 * time.Second,
		Description: description.Str(peer).Str(" output").String(),
	}, func(probeCtx context.Context) (string, error) {
		return lab.Query(probeCtx, peer, command, queryEnvironment(peer, command))
	}, func(output string) bool {
		return containsAll(output, needles)
	})
	return err
}

func waitContainsFold(ctx context.Context, lab interoplab.CheckerLab, peer string, command []string, timeout time.Duration, needles ...string) error {
	lower := make([]string, len(needles))
	for index, needle := range needles {
		lower[index] = strings.ToLower(needle)
	}
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	var description textbuf.Buffer
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     timeout,
		Interval:    2 * time.Second,
		Description: description.Str(peer).Str(" output").String(),
	}, func(probeCtx context.Context) (string, error) {
		return lab.Query(probeCtx, peer, command, queryEnvironment(peer, command))
	}, func(output string) bool {
		return containsAll(strings.ToLower(output), lower)
	})
	return err
}

func waitContainsAny(ctx context.Context, lab interoplab.CheckerLab, peer string, command []string, timeout time.Duration, needles ...string) error {
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	var description textbuf.Buffer
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     timeout,
		Interval:    2 * time.Second,
		Description: description.Str(peer).Str(" output alternatives").String(),
	}, func(probeCtx context.Context) (string, error) {
		return lab.Query(probeCtx, peer, command, queryEnvironment(peer, command))
	}, func(output string) bool {
		for _, needle := range needles {
			if strings.Contains(output, needle) {
				return true
			}
		}
		return false
	})
	return err
}

func delayRequireContains(ctx context.Context, lab interoplab.CheckerLab, peer string, command []string, delay time.Duration, needles ...string) error {
	if delay <= 0 {
		return errors.New("delayed assertion requires a positive delay")
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	output, err := lab.Query(ctx, peer, command, queryEnvironment(peer, command))
	if err != nil {
		return err
	}
	return requireContains(output, needles...)
}

func waitAbsent(ctx context.Context, lab interoplab.CheckerLab, peer string, command []string, timeout time.Duration, absent, proof []string) error {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	var description textbuf.Buffer
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     timeout,
		Interval:    time.Second,
		Description: description.Str(peer).Str(" negative state").String(),
	}, func(probeCtx context.Context) (string, error) {
		return lab.Query(probeCtx, peer, command, queryEnvironment(peer, command))
	}, func(output string) bool {
		return requireAbsentWithProof(output, absent, proof) == nil
	})
	return err
}
func queryEnvironment(peer string, command []string) []interoplab.EnvironmentVariable {
	if peer == "ze" && len(command) >= 2 && command[0] == "ze" && command[1] == zeCLICommand {
		return []interoplab.EnvironmentVariable{{Name: "ZE_SSH_PASSWORD", Value: "testpass"}}
	}
	return nil
}

func waitJSONFields(ctx context.Context, lab interoplab.CheckerLab, peer string, command []string, timeout time.Duration, fields map[string]string, minimum map[string]int) error {
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	var description textbuf.Buffer
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     timeout,
		Interval:    2 * time.Second,
		Description: description.Str(peer).Str(" JSON state").String(),
	}, func(probeCtx context.Context) (string, error) {
		return lab.Query(probeCtx, peer, command, queryEnvironment(peer, command))
	}, func(output string) bool {
		return requireJSONFields(output, fields, minimum) == nil
	})
	return err
}

func waitFRRRoute(ctx context.Context, lab interoplab.CheckerLab, prefix, family string, timeout time.Duration, present bool) error {
	if family == "" {
		family = "ipv4 unicast"
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	var text textbuf.Buffer
	command := []string{
		cmdVtysh,
		"-c",
		text.Str("show bgp ").Str(family).Byte(' ').Str(prefix).Str(" json").String(),
	}
	description := text.Reset().Str("FRR route ").Str(prefix).String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     timeout,
		Interval:    2 * time.Second,
		Description: description,
	}, func(probeCtx context.Context) (string, error) {
		return lab.Query(probeCtx, peerFRR, command, nil)
	}, func(output string) bool {
		hasRoute, measured := jsonHasRoute(output)
		return measured && hasRoute == present
	})
	return err
}

func jsonHasRoute(output string) (bool, bool) {
	var value any
	if err := json.Unmarshal([]byte(output), &value); err != nil {
		return false, false
	}
	return nestedRoute(value), true
}

func nestedRoute(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		if _, ok := typed["paths"]; ok {
			return true
		}
		if _, ok := typed["prefix"]; ok {
			return true
		}
		for _, child := range typed {
			if nestedRoute(child) {
				return true
			}
		}
	case []any:
		return slices.ContainsFunc(typed, nestedRoute)
	}
	return false
}
func requireJSONFields(output string, fields map[string]string, minimum map[string]int) error {
	var document any
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return fmt.Errorf("peer output is not JSON: %w", err)
	}
	for key, expected := range fields {
		value, ok := findJSONField(document, key)
		if !ok {
			return fmt.Errorf("peer JSON is missing %q", key)
		}
		if fmt.Sprint(value) != expected {
			return fmt.Errorf("peer JSON field %q is %v, expected %s", key, value, expected)
		}
	}
	for key, threshold := range minimum {
		value, ok := findJSONField(document, key)
		if !ok {
			return fmt.Errorf("peer JSON is missing %q", key)
		}
		number, ok := value.(float64)
		if !ok || number < float64(threshold) {
			return fmt.Errorf("peer JSON field %q is %v, expected at least %d", key, value, threshold)
		}
	}
	return nil
}

func findJSONField(value any, key string) (any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		if answer, ok := typed[key]; ok {
			return answer, true
		}
		for _, child := range typed {
			if answer, ok := findJSONField(child, key); ok {
				return answer, true
			}
		}
	case []any:
		for _, child := range typed {
			if answer, ok := findJSONField(child, key); ok {
				return answer, true
			}
		}
	}
	return nil, false
}

func requireContains(output string, needles ...string) error {
	for _, needle := range needles {
		if !strings.Contains(output, needle) {
			return fmt.Errorf("peer output is missing %q", needle)
		}
	}
	return nil
}

func containsAll(output string, needles []string) bool {
	return requireContains(output, needles...) == nil
}

func requireAbsentWithProof(output string, absent, proof []string) error {
	if len(proof) == 0 {
		return errors.New("absence assertion names no positive state proving the mechanism ran")
	}
	if err := requireContains(output, proof...); err != nil {
		return fmt.Errorf("absence assertion was not exercised: %w", err)
	}
	for _, needle := range absent {
		if strings.Contains(output, needle) {
			return fmt.Errorf("peer output unexpectedly contains %q", needle)
		}
	}
	return nil
}

func requireASAbsent(ctx context.Context, lab interoplab.CheckerLab, peer string, command []string, prefix, asn string) error {
	output, err := lab.Query(ctx, peer, command, nil)
	if err != nil {
		return err
	}
	if !strings.Contains(output, prefix) {
		return fmt.Errorf("route %s was not present to prove AS_PATH filtering ran", prefix)
	}
	var expression textbuf.Buffer
	pattern := regexp.MustCompile(
		expression.Str(`(^|[ {}\[\],])`).Str(regexp.QuoteMeta(asn)).
			Str(`($|[ {}\[\],])`).String(),
	)
	if pattern.FindStringIndex(output) != nil {
		return fmt.Errorf("route %s AS_PATH contains AS %s", prefix, asn)
	}
	return nil
}
func waitLogContains(ctx context.Context, lab interoplab.CheckerLab, peer string, timeout time.Duration, needles ...string) error {
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	var description textbuf.Buffer
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     timeout,
		Interval:    2 * time.Second,
		Description: description.Str(peer).Str(" log evidence").String(),
	}, func(probeCtx context.Context) (string, error) {
		logs, logErr := lab.Logs(probeCtx, peer, 80)
		if logErr != nil {
			return "", logErr
		}
		if !logs.Available {
			return "", errors.New("peer logs were not read")
		}
		return logs.Text, nil
	}, func(output string) bool {
		return containsAll(output, needles)
	})
	return err
}

func waitLogFields(ctx context.Context, lab interoplab.CheckerLab, peer string, timeout time.Duration, fields map[string]string, minimum map[string]int) error {
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	var description textbuf.Buffer
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     timeout,
		Interval:    2 * time.Second,
		Description: description.Str(peer).Str(" verdict").String(),
	}, func(probeCtx context.Context) (string, error) {
		logs, logErr := lab.Logs(probeCtx, peer, 80)
		if logErr != nil {
			return "", logErr
		}
		if !logs.Available {
			return "", errors.New("peer logs were not read")
		}
		return logs.Text, nil
	}, func(output string) bool {
		for key, want := range fields {
			got, present := logField(output, key)
			if !present || got != want {
				return false
			}
		}
		for key, want := range minimum {
			got, present := logField(output, key)
			if !present {
				return false
			}
			number, parseErr := strconv.Atoi(got)
			if parseErr != nil || number < want {
				return false
			}
		}
		return true
	})
	return err
}

func logField(output, key string) (string, bool) {
	var field textbuf.Buffer
	token := field.Str(key).Byte(':').String()
	for line := range strings.SplitSeq(output, "\n") {
		_, value, found := strings.Cut(line, token)
		if found {
			return strings.TrimSpace(value), true
		}
	}
	return "", false
}
