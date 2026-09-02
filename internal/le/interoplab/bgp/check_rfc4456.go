// Design: docs/architecture/testing/interop.md -- RFC evidence executed by the native lab action.
// RFC: rfc/short/rfc4456.md -- Section 9, the CLUSTER_LIST length tie-break.
// Related: check_rfc.go -- the other RFC carriers of this package.
// Related: check_special.go -- the registry that dispatches this checker.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// The bgp-cluster-list-length-bird lab, named where the checker and the
// scenario directory must agree.
const (
	clusterListScenario = "bgp-cluster-list-length-bird"

	// The one prefix the origin announces. Every assertion below reads it.
	clusterListPrefix = "10.66.0.0/24"

	// BIRD's two sessions, as bird.conf names them. BIRD prints the producing
	// protocol inside the brackets of a route line, so the name is what says
	// which of the two candidate paths BIRD selected.
	clusterListDirectProtocol    = "origin_direct"
	clusterListReflectedProtocol = "reflected_path"

	// The step ze names when the CLUSTER_LIST length decided, spelled as
	// BestStep.String reports it (internal/component/bgp/plugins/rib/bestpath.go).
	clusterListDecidingStep = "cluster-list-length"

	// The two fields of ze's `show bgp rib best ... reason` answer this checker
	// reads (bestReasonTerminal, rib_pipeline_best.go).
	clusterListWinnerField = "winner-peer"
	clusterListStepField   = "step"

	// BIRD's spelling of the reflected path's CLUSTER_LIST attribute.
	clusterListAttributeLabel = "BGP.cluster_list: "

	clusterListBudget = 90 * time.Second
)

// RFC requirement: RFC4456-9-1 positive -- "a BGP Speaker SHOULD prefer a route
// with the shorter CLUSTER_LIST length. The CLUSTER_LIST length is zero if a
// route does not carry the CLUSTER_LIST attribute" (RFC 4456 Section 9), and
// the rule sits between Steps f) and g) of RFC 4271 Section 9.1.2.2. Ze and
// BIRD hold the SAME two paths for 10.66.0.0/24, from the same two speakers,
// tied through every step up to and including f). BIRD decides the same
// question in bgp_rte_better (proto/bgp/attrs.c), which compares CLUSTER_LIST
// lengths in that exact slot with no configuration gate, so its selection is an
// independent answer rather than a restatement of ze's. The checker requires
// the two speakers to agree, requires the agreed answer to be the shorter list,
// and requires ze to have reached it AT the CLUSTER_LIST step: an earlier step
// separating the two paths would make the whole scenario prove nothing, so that
// last requirement is asserted rather than assumed. Step g), the peer address,
// would select the other path, which is what gives the scenario a red phase.
func checkClusterListLengthTieBreak(ctx context.Context, check *interoplab.CheckContext) error {
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, clusterListScenario, assertion, cause)
	}
	if !check.Network.IPv4.IsValid() {
		return fail(1, errors.New("cluster-list scenario has no selected IPv4 network"))
	}
	zeAddress := networkHostAddress(check.Network, 2)
	reflectorAddress := networkHostAddress(check.Network, 3)
	birdAddress := networkHostAddress(check.Network, 4)
	originAddress := networkHostAddress(check.Network, 5)

	// Assertions 1 to 5. Both candidate paths travel over these five sessions:
	// the origin reaches ze and BIRD directly, and reaches them a second time
	// through the reflector. A missing session leaves one speaker with one
	// candidate, and a one-candidate decision process exercises no tie-break.
	sessions := []operation{
		{kind: opFRRSession, argument: originAddress},
		{kind: opFRRSession, argument: zeAddress},
		{kind: opFRRSession, argument: birdAddress},
		{kind: opBIRDSession, argument: clusterListDirectProtocol},
		{kind: opBIRDSession, argument: clusterListReflectedProtocol},
	}
	for index := range sessions {
		if err := runOperation(ctx, check.Network, check.Lab, &sessions[index]); err != nil {
			return fail(index+1, err)
		}
	}

	// Assertion 6. GoBGP has no static route surface in its configuration file,
	// so the prefix under test enters its RIB here. Everything downstream is one
	// path with one set of attributes, which is what makes the two copies ze and
	// BIRD compare differ in the CLUSTER_LIST and in nothing else.
	if _, err := check.Lab.Exec(ctx, peerGoBGP, []string{
		cmdGoBGP, gobgpGlobal, gobgpRIB, gobgpAdd, clusterListPrefix,
		"-a", gobgpFamilyIPv4, gobgpNextHop, originAddress,
	}, nil); err != nil {
		return fail(6, err)
	}

	// Assertion 7. BIRD's own table, waited on until both candidates are in it.
	// The predicate is the two-candidate shape rather than the verdict, so a
	// wrong verdict is reported as a disagreement below instead of expiring as
	// a timeout that names nothing.
	birdRoutes, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     clusterListBudget,
		Interval:    2 * time.Second,
		Description: "BIRD candidate paths for " + clusterListPrefix,
	}, func(probeCtx context.Context) (string, error) {
		return check.Lab.Query(probeCtx, peerBIRD, []string{
			cmdBirdc, "show route for " + clusterListPrefix + " all",
		}, nil)
	}, func(output string) bool {
		return strings.Contains(output, clusterListDirectProtocol) &&
			strings.Contains(output, clusterListReflectedProtocol)
	})
	if err != nil {
		return fail(7, err)
	}

	// Assertion 8. The premise of the whole scenario, read out of the peer that
	// is about to judge it: exactly one of the two paths carries a CLUSTER_LIST,
	// and it holds the reflector's single CLUSTER_ID. Two lists, or none, and
	// the tie-break has no input to act on.
	lists := strings.Count(birdRoutes, clusterListAttributeLabel)
	if lists != 1 {
		return fail(8, fmt.Errorf(
			"BIRD reports %d CLUSTER_LIST attributes for %s, expected exactly one: the tie-break needs one path with a list and one without\n%s",
			lists, clusterListPrefix, birdRoutes))
	}
	if !strings.Contains(birdRoutes, clusterListAttributeLabel+reflectorAddress) {
		return fail(8, fmt.Errorf(
			"BIRD reports no CLUSTER_LIST carrying the reflector's CLUSTER_ID %s for %s\n%s",
			reflectorAddress, clusterListPrefix, birdRoutes))
	}

	// Assertion 9. BIRD's verdict.
	primary, err := birdPrimaryProtocol(birdRoutes)
	if err != nil {
		return fail(9, fmt.Errorf("%w\n%s", err, birdRoutes))
	}
	var birdWinner string
	switch primary {
	case clusterListDirectProtocol:
		birdWinner = originAddress
	case clusterListReflectedProtocol:
		birdWinner = reflectorAddress
	default:
		return fail(9, fmt.Errorf("BIRD selected a route from protocol %q, which is neither session of this lab\n%s", primary, birdRoutes))
	}

	// Assertion 10. Ze's verdict, and the step it reached it at. The predicate
	// waits for a narrated comparison to exist, which happens only once ze holds
	// both candidates: a single-candidate answer carries no step at all.
	zeAnswer, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     clusterListBudget,
		Interval:    2 * time.Second,
		Description: "ze best-path decision for " + clusterListPrefix,
	}, func(probeCtx context.Context) (string, error) {
		command := zeCommand("show bgp rib best prefix " + clusterListPrefix + " reason")
		return check.Lab.Query(probeCtx, "ze", command, queryEnvironment("ze", command))
	}, func(output string) bool {
		winner, step, decodeErr := clusterListDecision(output)
		return decodeErr == nil && winner != "" && step != ""
	})
	if err != nil {
		return fail(10, err)
	}
	zeWinner, zeStep, err := clusterListDecision(zeAnswer)
	if err != nil {
		return fail(10, fmt.Errorf("%w\n%s", err, zeAnswer))
	}

	// Assertion 11. The interop claim: two implementations, the same inputs, the
	// same route.
	if zeWinner != birdWinner {
		return fail(11, fmt.Errorf(
			"ze and BIRD disagree on %s: ze selected the path from %s at step %q, BIRD selected the path from %s (protocol %s)",
			clusterListPrefix, zeWinner, zeStep, birdWinner, primary))
	}

	// Assertion 12. Agreement on the WRONG route is still a violation, so the
	// agreed answer is held to RFC 4456 Section 9 rather than to the other
	// speaker.
	if zeWinner != originAddress {
		return fail(12, fmt.Errorf(
			"both speakers selected the path from %s for %s; RFC 4456 Section 9 requires the shorter CLUSTER_LIST, which is the path from %s",
			zeWinner, clusterListPrefix, originAddress))
	}

	// Assertion 13. What stops the scenario from passing vacuously. If any step
	// before the CLUSTER_LIST comparison separated the two paths, ze would name
	// that step here, and the right answer would have been reached for a reason
	// that has nothing to do with RFC 4456 Section 9.
	if zeStep != clusterListDecidingStep {
		return fail(13, fmt.Errorf(
			"ze decided %s at step %q rather than %q: an earlier step of RFC 4271 Section 9.1.2.2 separated the two paths, so this scenario did not exercise RFC 4456 Section 9",
			clusterListPrefix, zeStep, clusterListDecidingStep))
	}
	return nil
}

// birdPrimaryProtocol answers the BIRD protocol that produced the route BIRD
// selected, read out of `show route ... all`.
//
// BIRD writes one line per route as "<net> <dest> [<protocol> <time>]<flag><info>"
// (rt_show_rte, nest/rt-show.c) and the flag is " *" on the selected route
// alone. An answer with no such line is an ERROR rather than an empty name: a
// query that failed, a net BIRD does not hold, and a net whose every route was
// filtered all produce output with no asterisk in it, and none of them is a
// verdict.
//
// A next-hop line and an attribute line both start with a tab (rt_show_rte and
// rta_show), and a route line never does, so the tab is what separates a route
// from an attribute whose own value carries brackets and an asterisk.
func birdPrimaryProtocol(output string) (string, error) {
	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(line, "\t") {
			continue
		}
		open := strings.IndexByte(line, '[')
		if open < 0 {
			continue
		}
		shut := strings.IndexByte(line[open:], ']')
		if shut < 0 {
			continue
		}
		shut += open
		if !strings.HasPrefix(line[shut+1:], " *") {
			continue
		}
		name, _, _ := strings.Cut(strings.TrimSpace(line[open+1:shut]), " ")
		if name == "" {
			continue
		}
		return name, nil
	}
	return "", errors.New("BIRD marked no route as selected: no route line carries the asterisk BIRD writes beside the primary route")
}

// clusterListDecision reads the winning peer and the deciding step out of ze's
// `show bgp rib best ... reason` answer.
//
// An answer that carries neither field is not an empty decision, so both
// lookups are required. Ze narrates one step per pairwise comparison, and this
// lab has exactly two candidates, so the single step is the one that decided.
func clusterListDecision(output string) (string, string, error) {
	if strings.TrimSpace(output) == "" {
		return "", "", errors.New("ze answered nothing for its best-path decision")
	}
	var document any
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return "", "", fmt.Errorf("ze best-path answer is not JSON: %w", err)
	}
	fields := map[string]string{}
	for _, key := range []string{clusterListWinnerField, clusterListStepField} {
		value, ok := findJSONField(document, key)
		if !ok {
			return "", "", fmt.Errorf("ze best-path answer carries no %q", key)
		}
		text, ok := value.(string)
		if !ok {
			return "", "", fmt.Errorf("ze best-path answer field %q is %v, expected a string", key, value)
		}
		fields[key] = text
	}
	return fields[clusterListWinnerField], fields[clusterListStepField], nil
}
