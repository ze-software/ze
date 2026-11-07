// Design: docs/guide/policy-routing.md -- tcp-mss, the clamp a forwarded SYN meets
// Related: netfilter_fixture_policy.go -- policyTableOutput, the wait for the ze_pr rule
// Related: ../msslab/msslab_linux.go -- the router that forwards the probe SYNs

package fixture

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/test/msslab"
)

// policyTCPMSSClamp drives test/policy/policy-tcp-mss-clamp.ci: the daemon has
// applied a policy whose rule clamps SYNs arriving on zemssin to 1400. The
// fixture builds the router those names belong to, forwards three SYNs per
// family through it, and prints what the next hop received. A forwarded MSS
// that differs from the clamp's answer is an error, so the verdict does not
// rest on the .ci expect lines alone.
func policyTCPMSSClamp(ctx context.Context, _ []string) error {
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	if _, err := policyTableOutput(ctx, func(rules []string) bool {
		for _, rule := range rules {
			if strings.Contains(rule, "maxseg size set 1400") {
				return true
			}
		}
		return false
	}, 100); err != nil {
		return fmt.Errorf("the ze_pr table never carried the tcp-mss 1400 rule: %w", err)
	}

	lab, err := msslab.Open("zemssin", "zemssout")
	if err != nil {
		return err
	}
	probeErr := policyTCPMSSProbe(lab)
	closeErr := lab.Close()
	stopErr := signalProcess(pid, syscall.SIGTERM)
	return errors.Join(probeErr, closeErr, stopErr)
}

func policyTCPMSSProbe(lab *msslab.Lab) error {
	cases := []struct {
		sent msslab.Option
		want msslab.Option
	}{
		{msslab.Option{Present: true, Value: 1460}, msslab.Option{Present: true, Value: 1400}},
		{msslab.Option{Present: true, Value: 1300}, msslab.Option{Present: true, Value: 1300}},
		{msslab.Option{}, msslab.Option{}},
	}
	var errs []error
	for _, family := range []msslab.Family{msslab.FamilyIPv4, msslab.FamilyIPv6} {
		for _, c := range cases {
			seen, err := lab.Forward(family, c.sent, 3*time.Second)
			if err != nil {
				return err
			}
			fmt.Println("mss " + family.String() + " sent=" + mssText(c.sent) + " forwarded=" + mssText(seen.MSS) +
				" header=" + strconv.Itoa(seen.HeaderOctets))
			if seen.MSS != c.want {
				errs = append(errs, fmt.Errorf("%s SYN sent with MSS %s was forwarded with %s, want %s",
					family, mssText(c.sent), mssText(seen.MSS), mssText(c.want)))
			}
			if !c.sent.Present && seen.HeaderOctets != 20 {
				errs = append(errs, fmt.Errorf("%s SYN sent with no option was forwarded with a %d-octet TCP header, want 20",
					family, seen.HeaderOctets))
			}
		}
	}
	return errors.Join(errs...)
}

// mssAbsentText is how a printed line names a SYN with no MSS option.
const mssAbsentText = "none"

func mssText(option msslab.Option) string {
	if !option.Present {
		return mssAbsentText
	}
	return strconv.Itoa(int(option.Value))
}
