// Design: docs/architecture/testing/interop.md -- both directions of an atomic CoA.
// Related: check_coa_disconnect.go -- the download TBF observation.
// Upstream: https://github.com/iproute2/iproute2/blob/main/tc/m_police.c --
// print_police prints rate only in text mode, not JSON.
package radiussubscriber

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

func (s *scenarioLab) uploadRate(ctx context.Context) (uint64, error) {
	output, err := s.lab.Query(ctx, peerZe, []string{"tc", "filter", "show", "dev", "ppp0", "ingress"}, nil)
	if err != nil {
		return 0, err
	}
	return policerRate(output)
}

// policerRate requires one policing action and reads its rate, rather than
// treating an absent or unreadable policer as zero. The fixture rates are whole
// decimal Mbit values; a changed representation is an error, not a guessed unit.
func policerRate(output string) (uint64, error) {
	var rate uint64
	count := 0
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		for i, field := range fields {
			if field != "police" {
				continue
			}
			if len(fields)-i < 4 {
				return 0, fmt.Errorf("truncated police action: %s", line)
			}
			if fields[i+2] != "rate" {
				return 0, fmt.Errorf("police action has no rate: %s", line)
			}
			number, ok := strings.CutSuffix(fields[i+3], "Mbit")
			if !ok {
				return 0, fmt.Errorf("unexpected police rate unit: %s", line)
			}
			mbit, err := strconv.ParseUint(number, 10, 32)
			if err != nil {
				return 0, fmt.Errorf("police rate: %w", err)
			}
			if mbit == 0 {
				return 0, fmt.Errorf("police rate is zero: %s", line)
			}
			rate = mbit * 1_000_000
			count++
		}
	}
	if count != 1 {
		return 0, fmt.Errorf("kernel has %d police actions, want one: %s", count, output)
	}
	return rate, nil
}

func (s *scenarioLab) waitUploadRate(ctx context.Context, want uint64) error {
	rate, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: s.timeout, Interval: time.Second, Description: "the acknowledged upload policer rate",
	}, s.uploadRate, func(rate uint64) bool { return rate == want })
	if err != nil {
		return fmt.Errorf("kernel upload rate %d, want %d: %w", rate, want, err)
	}
	return nil
}
