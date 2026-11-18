// Design: docs/functional-tests.md -- BGP diagnostics through real commands and sockets.
// Related: bgp_best_route_transit.go -- useful ordinary selected and API wire helpers.
package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	bgpPolicyDiagnosticsLearnedText            = "no routes learned from other peers"
	bgpPolicyDiagnosticsLocalText              = "Local announcements remain permitted"
	bgpPolicyDiagnosticsRFC                    = "RFC 8212"
	bgpPolicyDiagnosticsPeerC                  = "peer C:"
	bgpPolicyDiagnosticsExternalControlAddress = "127.0.0.6"
)

func init() {
	Register("plugin/bgp-policy-diagnostics", bgpPolicyDiagnosticsDriver, StopsDaemon)
}

// One question crosses three product entry points: do validation, supplied-file
// doctor and the live Established warning distinguish C from the real RR D and
// external RS E while their useful output proves the topology is operating?
func bgpPolicyDiagnosticsDriver(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("BGP policy diagnostics requires <internal|forked> <BGP listen address> <active config file>")
	}
	switch args[0] {
	case sysribPlacementInternal, sysribPlacementForked:
	default:
		return fmt.Errorf("BGP policy diagnostics: unknown placement %q", args[0])
	}
	return Observe(ctx, "bgp-policy-diagnostics", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := bgpPolicyDiagnosticsConfigs(ctx, args[2]); err != nil {
			return err
		}
		if err := bgpPolicyDiagnosticsWire(ctx, plugin, args[1]); err != nil {
			return err
		}
		_, err := fmt.Fprintf(os.Stderr, "OK: BGP policy diagnostics %s commands, role wire and API survivor\n", args[0])
		return err
	})
}

// bgpPolicyDiagnosticsConfigs reads this placement's actual saved source, then
// changes C through the parser into three owned supplied-file variants. The
// original supplied file is checked again after the two explicit policies, so
// a cached report from either direction cannot satisfy the assertions.
func bgpPolicyDiagnosticsConfigs(ctx context.Context, active string) error {
	text, err := os.ReadFile(active) //nolint:gosec // The .ci names its own active configuration file.
	if err != nil {
		return err
	}
	schema, err := config.YANGSchema()
	if err != nil {
		return err
	}
	tree, err := config.NewParser(schema).Parse(string(text))
	if err != nil {
		return err
	}
	if err := bgpPolicyDiagnosticsReports(ctx, string(text), true); err != nil {
		return fmt.Errorf("original absent policy: %w", err)
	}
	for _, final := range []string{actionAccept, actionReject} {
		next := tree.Clone()
		bgp := next.GetContainer(rfc8212RootBGP)
		if bgp == nil {
			return errors.New("BGP diagnostics fixture omitted bgp")
		}
		peer := bgp.GetList("peer")["C"]
		if peer == nil {
			return errors.New("BGP diagnostics fixture omitted C")
		}
		filter := peer.GetContainer("filter")
		if filter == nil {
			filter = config.NewTree()
			peer.SetContainer("filter", filter)
		}
		export := config.NewTree()
		export.Set("final", final)
		filter.SetContainer(directionExport, export)
		// This is a separate saved configuration, not a live reload. C's live
		// session remains deliberately unconfigured for the wire assertion.
		file, err := os.CreateTemp(".", "bgp-diagnostic-"+final+"-*.conf")
		if err != nil {
			return err
		}
		path := file.Name()
		serialized := config.Serialize(next, schema)
		written, writeErr := file.WriteString(serialized)
		closeErr := file.Close()
		if err := errors.Join(writeErr, closeErr); err != nil {
			return errors.Join(err, os.Remove(path))
		}
		if written != len(serialized) {
			return errors.Join(errors.New("incomplete saved BGP diagnostics configuration"), os.Remove(path))
		}
		saved, readErr := os.ReadFile(path) //nolint:gosec // This function created the private test-owned file above.
		var reportErr error
		if readErr == nil {
			reportErr = bgpPolicyDiagnosticsReports(ctx, string(saved), false)
		}
		removeErr := os.Remove(path)
		if err := errors.Join(readErr, reportErr, removeErr); err != nil {
			return fmt.Errorf("saved final %s: %w", final, err)
		}
	}
	return bgpPolicyDiagnosticsReports(ctx, string(text), true)
}

// The dash carries the saved bytes to each real CLI. Doctor deliberately uses
// its supplied-config route: '-' cannot be borrowed from a running daemon.
// Each child MUST be waited for; CommandContext MUST terminate it on timeout.
func bgpPolicyDiagnosticsCommand(ctx context.Context, text string, args ...string) (stdout string, code int, err error) {
	work, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	command := exec.CommandContext(work, "ze", args...) //nolint:gosec // The functional fixture owns every command word.
	command.Stdin = strings.NewReader(text)
	var out, stderr bytes.Buffer
	command.Stdout = &out
	command.Stderr = &stderr
	err = command.Run()
	if err == nil {
		return out.String(), 0, nil
	}
	if work.Err() != nil {
		return "", 0, fmt.Errorf("ze %v: %w", args, work.Err())
	}
	if exited, ok := errors.AsType[*exec.ExitError](err); ok {
		return out.String(), exited.ExitCode(), nil
	}
	return "", 0, fmt.Errorf("start or join ze %v: %w; stderr=%s", args, err, stderr.String())
}

func bgpPolicyDiagnosticsReports(ctx context.Context, text string, absent bool) error {
	output, code, err := bgpPolicyDiagnosticsCommand(ctx, text, "config", "validate", "-")
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("ze config validate exited %d: %s", code, output)
	}
	if err := bgpPolicyDiagnosticsValidate(output, absent); err != nil {
		return err
	}
	output, code, err = bgpPolicyDiagnosticsCommand(ctx, text, "doctor", "--json", "config", "-")
	if err != nil {
		return err
	}
	// Other platform checks can legitimately make the box not ready. A valid
	// report still has to contain E's external finding after C is cleared.
	switch code {
	case 0, 1:
	default:
		return fmt.Errorf("ze doctor exited %d: %s", code, output)
	}
	var report diagnostic.DoctorResult
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		return fmt.Errorf("decode actual ze doctor report: %w", err)
	}
	return bgpPolicyDiagnosticsDoctor(report, absent)
}

func bgpPolicyDiagnosticsValidate(output string, absent bool) error {
	counts := map[string]int{"C": 0, "E": 0}
	for line := range strings.Lines(output) {
		if !strings.Contains(line, "warning: peer ") {
			continue
		}
		var name string
		switch {
		case strings.Contains(line, "warning: peer C:"):
			name = "C"
			if !strings.Contains(line, bgpPolicyDiagnosticsLearnedText) {
				return fmt.Errorf("C validation omitted learned-route rejection: %s", line)
			}
			if !strings.Contains(line, bgpPolicyDiagnosticsLocalText) {
				return fmt.Errorf("C validation omitted local-route permission: %s", line)
			}
			if strings.Contains(line, bgpPolicyDiagnosticsRFC) {
				return fmt.Errorf("C validation borrowed external wording: %s", line)
			}
		case strings.Contains(line, "warning: peer E:"):
			name = "E"
			if !strings.Contains(line, bgpPolicyDiagnosticsRFC) {
				return fmt.Errorf("E validation omitted RFC8212: %s", line)
			}
			if !strings.Contains(line, "no export policy") {
				return fmt.Errorf("E validation omitted export-policy absence: %s", line)
			}
		default:
			return fmt.Errorf("configured or role-served peer raised a policy warning: %s", line)
		}
		if !strings.Contains(line, "set bgp peer "+name+" filter export final accept") {
			return fmt.Errorf("%s validation warning omitted its exact editor fix: %s", name, line)
		}
		counts[name]++
	}
	want := 0
	if absent {
		want = 1
	}
	if counts["C"] != want {
		return fmt.Errorf("validation C warnings=%d, want %d; output=%s", counts["C"], want, output)
	}
	if counts["E"] != 1 {
		return fmt.Errorf("validation E warnings=%d, want 1; output=%s", counts["E"], output)
	}
	return nil
}

func bgpPolicyDiagnosticsDoctor(report diagnostic.DoctorResult, absent bool) error {
	if report.SchemaVersion != diagnostic.SchemaVersion {
		return fmt.Errorf("doctor returned a different schema: %+v", report)
	}
	if report.Diagnostics == nil {
		return errors.New("doctor returned no diagnostics list")
	}
	counts := map[string]int{"C": 0, "E": 0}
	for index := range report.Diagnostics {
		row := &report.Diagnostics[index]
		var name string
		switch row.Code {
		case diagnostic.CodeDoctorBGPLearnedExportNoPolicy:
			name = "C"
			if !strings.HasPrefix(row.Message, bgpPolicyDiagnosticsPeerC) {
				return fmt.Errorf("internal doctor policy finding names another peer: %+v", row)
			}
			if !strings.Contains(row.Message, bgpPolicyDiagnosticsLearnedText) {
				return fmt.Errorf("peer C doctor finding omitted learned-route rejection: %+v", row)
			}
			if !strings.Contains(row.Message, bgpPolicyDiagnosticsLocalText) {
				return fmt.Errorf("peer C doctor finding omitted local-route permission: %+v", row)
			}
			if strings.Contains(row.Message, bgpPolicyDiagnosticsRFC) {
				return fmt.Errorf("peer C doctor finding borrowed external wording: %+v", row)
			}
		case diagnostic.CodeDoctorBGPRFC8212NoPolicy:
			name = "E"
			if !strings.HasPrefix(row.Message, "peer E:") {
				return fmt.Errorf("external doctor policy finding names another peer: %+v", row)
			}
			if !strings.Contains(row.Message, bgpPolicyDiagnosticsRFC) {
				return fmt.Errorf("peer E doctor finding omitted RFC8212: %+v", row)
			}
			if !strings.Contains(row.Message, "no export policy") {
				return fmt.Errorf("peer E doctor finding omitted export-policy absence: %+v", row)
			}
		default:
			if strings.Contains(row.Message, bgpPolicyDiagnosticsLearnedText) {
				return fmt.Errorf("learned policy finding used an unexpected doctor code: %+v", row)
			}
			if strings.HasPrefix(row.Message, "peer E:") {
				if strings.Contains(row.Message, bgpPolicyDiagnosticsRFC) {
					return fmt.Errorf("external policy finding used an unexpected doctor code: %+v", row)
				}
			}
			continue
		}
		if row.Severity != diagnostic.SeverityWarning {
			return fmt.Errorf("%s doctor finding lacks warning severity: %+v", name, row)
		}
		if row.Fix != "set bgp peer "+name+" filter export final accept" {
			return fmt.Errorf("%s doctor finding lacks its exact editor fix: %+v", name, row)
		}
		counts[name]++
	}
	want := 0
	if absent {
		want = 1
	}
	if counts["C"] != want {
		return fmt.Errorf("doctor C findings=%d, want %d", counts["C"], want)
	}
	if counts["E"] != 1 {
		return fmt.Errorf("doctor E findings=%d, want 1", counts["E"])
	}
	return nil
}

// All six sockets MUST remain open through every downstream assertion; the
// owning defer MUST close them on success and every partial-setup failure.
func bgpPolicyDiagnosticsWire(ctx context.Context, plugin *sdk.Plugin, address string) error {
	var sockets []net.Conn
	defer func() {
		for _, socket := range sockets {
			_ = socket.Close()
		}
	}()
	for _, peer := range []struct {
		address string
		asn     uint16
	}{{"127.0.0.2", 65001}, {"127.0.0.3", 65000}, {"127.0.0.4", 65000}, {"127.0.0.5", 65000}, {bgpPolicyDiagnosticsExternalControlAddress, 65002}, {"127.0.0.7", 65000}} {
		socket, err := atomicAggregatePolicyConnect(ctx, address, netip.MustParseAddr(peer.address), peer.asn)
		if err != nil {
			return err
		}
		sockets = append(sockets, socket)
	}
	if err := p12WaitPeersEOR(ctx, plugin, "A", "B", "C", "D", "E", "S"); err != nil {
		return err
	}
	for index, socket := range sockets {
		if err := bgpBestRouteExpect(socket, nil, nil, nil); err != nil {
			return fmt.Errorf("peer %d initial EOR: %w", index, err)
		}
	}
	if err := rfc8212ReloadTopology(ctx, plugin); err != nil {
		return err
	}
	a, b, c, d, e, sourceInternal := sockets[0], sockets[1], sockets[2], sockets[3], sockets[4], sockets[5]
	api := func(prefix string, native []byte) error {
		if err := liveStagesRoute(ctx, plugin, "127.0.0.4 update text nhop 192.0.2.1 nlri add "+prefix); err != nil {
			return err
		}
		return bgpBestRouteExpect(c, native, nil, nil)
	}
	if err := api("10.253.0.0/24", []byte{24, 10, 253, 0}); err != nil {
		return fmt.Errorf("peer C local baseline: %w", err)
	}
	external := []byte{24, 10, 254, 0}
	if err := bgpBestRouteSend(a, external, false, false); err != nil {
		return err
	}
	if err := rfc8212ReloadBest(ctx, plugin, "10.254.0.0/24", "127.0.0.2"); err != nil {
		return err
	}
	if err := bgpPolicyDiagnosticsRoute(b, external, []byte{2, 1, 0xfd, 0xe9}, false); err != nil {
		return fmt.Errorf("peer B useful ordinary output: %w", err)
	}
	if err := bgpPolicyDiagnosticsRoute(d, external, []byte{2, 1, 0xfd, 0xe9}, false); err != nil {
		return fmt.Errorf("peer D external-source non-reflection control: %w", err)
	}
	internal := []byte{24, 10, 254, 1}
	if err := bgpBestRouteSend(sourceInternal, internal, true, false); err != nil {
		return err
	}
	if err := rfc8212ReloadBest(ctx, plugin, "10.254.1.0/24", "127.0.0.7"); err != nil {
		return err
	}
	if err := bgpPolicyDiagnosticsRoute(d, internal, nil, true); err != nil {
		return fmt.Errorf("peer D genuine reflected output without export policy: %w", err)
	}
	if err := quiesce(ctx, plugin); err != nil {
		return err
	}
	// Same-socket fences decide both negative assertions. A route incorrectly
	// emitted before either fence is the next UPDATE and cannot hide behind time.
	if err := api("10.253.1.0/24", []byte{24, 10, 253, 1}); err != nil {
		return fmt.Errorf("peer C leaked learned output or lost its API survivor: %w", err)
	}
	if err := fixture10Call(ctx, plugin, "send bgp "+bgpPolicyDiagnosticsExternalControlAddress+" update text nlri eor").requireDone("E external absence fence"); err != nil {
		return err
	}
	if err := bgpBestRouteExpect(e, nil, nil, nil); err != nil {
		return fmt.Errorf("E leaked route despite absent export policy: %w", err)
	}
	return policyFinalSessions(ctx, plugin, 6)
}

func bgpPolicyDiagnosticsRoute(conn net.Conn, prefix, path []byte, reflected bool) error {
	update, err := atomicAggregatePolicyRead(conn)
	if err != nil {
		return err
	}
	return bgpPolicyDiagnosticsRouteValue(update, prefix, path, reflected)
}

func bgpPolicyDiagnosticsRouteValue(update *message.Update, prefix, path []byte, reflected bool) error {
	if update == nil {
		return errors.New("wire reader returned no UPDATE")
	}
	if !bytes.Equal(update.NLRI, prefix) {
		return fmt.Errorf("wire announcement=%x, want %x", update.NLRI, prefix)
	}
	if len(update.WithdrawnRoutes) != 0 {
		return fmt.Errorf("wire route also withdraws %x", update.WithdrawnRoutes)
	}
	for _, expected := range []struct {
		code  attribute.AttributeCode
		value []byte
	}{{attribute.AttrOrigin, []byte{0}}, {attribute.AttrASPath, path}, {attribute.AttrNextHop, []byte{192, 0, 2, 1}}} {
		_, _, got, found := attribute.AttrFind(update.PathAttributes, expected.code)
		if !found {
			return fmt.Errorf("wire attribute %d is absent", expected.code)
		}
		if !bytes.Equal(got, expected.value) {
			return fmt.Errorf("wire attribute %d=%x, want %x", expected.code, got, expected.value)
		}
	}
	for _, expected := range []struct {
		code  attribute.AttributeCode
		value []byte
	}{{attribute.AttrOriginatorID, []byte{127, 0, 0, 7}}, {attribute.AttrClusterList, []byte{10, 0, 0, 1}}} {
		_, _, got, found := attribute.AttrFind(update.PathAttributes, expected.code)
		if found != reflected {
			return fmt.Errorf("reflection attribute %d present=%t, want reflected=%t", expected.code, found, reflected)
		}
		if reflected {
			if !bytes.Equal(got, expected.value) {
				return fmt.Errorf("reflection attribute %d=%x, want %x", expected.code, got, expected.value)
			}
		}
	}
	return nil
}
