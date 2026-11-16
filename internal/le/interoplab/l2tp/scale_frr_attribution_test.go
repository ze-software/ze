// Design: docs/labs/l2tp-interop.md -- attribute absent subscriber routes with BGP up.
// Related: scale_observe.go -- requireScaleFRR checks the independent peer.
package l2tp

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

type scaleFRRAttributionLab struct {
	recordingLab
	before, after  string
	routes         string
	routeArguments []string
	routeReads     int
}

func (l *scaleFRRAttributionLab) Exec(_ context.Context, peer string, arguments []string, _ []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	l.calls = append(l.calls, recordedCall{operation: "exec", peer: peer, arguments: append([]string(nil), arguments...)})
	if peer != peerFRR {
		return interoplab.CommandResult{}, fmt.Errorf("unexpected FRR attribution peer %s", peer)
	}
	neighbor := []string{commandVTYSH, "-c", "show bgp neighbor " + zeAddress}
	if slices.Equal(arguments, neighbor) {
		if l.routeReads == 0 {
			return interoplab.CommandResult{Stdout: l.before}, nil
		}
		return interoplab.CommandResult{Stdout: l.after}, nil
	}
	if !slices.Equal(arguments, l.routeArguments) {
		return interoplab.CommandResult{}, fmt.Errorf("unexpected FRR attribution command %v", arguments)
	}
	l.routeReads++
	return interoplab.CommandResult{Stdout: l.routes}, nil
}

func scaleFRRAttributionFixture(t *testing.T) (scalePopulation, *scaleFRRAttributionLab) {
	t.Helper()
	population := scaleFullTestPopulation(t)
	users := scaleUsers(scaleLACCount)
	arguments := make([]string, 1, 1+2*len(users))
	arguments[0] = commandVTYSH
	routes := make([]string, 0, len(users))
	for _, user := range users {
		session, exists := population.byUser[user]
		if !exists {
			t.Fatalf("attribution fixture lacks %s", user)
		}
		prefix := session.address.String() + "/32"
		arguments = append(arguments, "-c", "show bgp ipv4 unicast "+prefix+" json")
		routes = append(routes, fmt.Sprintf(`{"prefix":%q,"pathCount":1,"paths":[{"valid":true,"peer":{"peerId":%q}}]}`, prefix, zeAddress))
	}
	return population, &scaleFRRAttributionLab{
		before: "BGP state = Established, up for 00:02:03\n", after: "BGP state = Established, up for 00:02:04\n",
		routes: strings.Join(routes, "\n"), routeArguments: arguments,
	}
}

// TestScaleFRRChecksSessionBeforeRoutes proves an absent route does not bypass
// the contemporaneous positive BGP-session measurement.
func TestScaleFRRChecksSessionBeforeRoutes(t *testing.T) {
	population, lab := scaleFRRAttributionFixture(t)
	lab.routes = "{}\n"
	if err := requireScaleFRR(t.Context(), lab, population, population); err == nil {
		t.Fatal("missing subscriber route passed")
	}
	if len(lab.calls) == 0 {
		t.Fatal("FRR state was not read")
	}
	want := []string{commandVTYSH, "-c", "show bgp neighbor " + zeAddress}
	if !slices.Equal(lab.calls[0].arguments, want) {
		t.Fatalf("first FRR read %v, want current neighbor state", lab.calls[0].arguments)
	}
	if lab.routeReads != 1 {
		t.Fatalf("route reads=%d, want one after the established-session check", lab.routeReads)
	}
}

// TestScaleFRRRejectsDroppedSessionBeforeRoutes proves route absence is not
// blamed on subscriber identity when the current BGP session is down.
func TestScaleFRRRejectsDroppedSessionBeforeRoutes(t *testing.T) {
	population, lab := scaleFRRAttributionFixture(t)
	lab.before = "BGP state = Active\n"
	lab.routes = "{}\n"
	err := requireScaleFRR(t.Context(), lab, population, population)
	if err == nil {
		t.Fatal("a dropped BGP session passed")
	}
	if !strings.Contains(err.Error(), strings.TrimSpace(lab.before)) {
		t.Fatalf("current dropped-session evidence lost: %v", err)
	}
	if len(lab.calls) != 1 {
		t.Fatalf("reads=%d, want only the refused neighbor check", len(lab.calls))
	}
	if lab.routeReads != 0 {
		t.Fatal("subscriber routes were interpreted while BGP was down")
	}
}

// TestScaleFRRRetainsEstablishedReceipt proves a missing-prefix error retains
// the actual positive state line even when a long neighbor answer follows it.
func TestScaleFRRRetainsEstablishedReceipt(t *testing.T) {
	population, lab := scaleFRRAttributionFixture(t)
	const state = "BGP state = Established, up for 00:12:34"
	lab.before = state + "\n" + strings.Repeat("additional neighbor detail\n", 80)
	lab.routes = "{}\n"
	err := requireScaleFRR(t.Context(), lab, population, population)
	if err == nil {
		t.Fatal("missing subscriber route passed")
	}
	for _, evidence := range []string{state, "FRR subscriber scale-1-01", "present=false, want true", "{}"} {
		if !strings.Contains(err.Error(), evidence) {
			t.Fatalf("missing-route failure lost %q: %v", evidence, err)
		}
	}
}

// TestScaleFRRRetainsPostCensusSessionCheck proves valid routes cannot mask
// the BGP session going down while the complete census was read.
func TestScaleFRRRetainsPostCensusSessionCheck(t *testing.T) {
	population, lab := scaleFRRAttributionFixture(t)
	lab.after = "BGP state = Active\n"
	err := requireScaleFRR(t.Context(), lab, population, population)
	if err == nil {
		t.Fatal("valid subscriber routes hid the post-census BGP failure")
	}
	if !strings.Contains(err.Error(), strings.TrimSpace(lab.after)) {
		t.Fatalf("post-census session evidence lost: %v", err)
	}
	if lab.routeReads != 1 {
		t.Fatalf("route reads=%d, want one complete census", lab.routeReads)
	}
	last := lab.calls[len(lab.calls)-1]
	want := []string{commandVTYSH, "-c", "show bgp neighbor " + zeAddress}
	if !slices.Equal(last.arguments, want) {
		t.Fatalf("last FRR read %v, want post-census neighbor state", last.arguments)
	}
}
