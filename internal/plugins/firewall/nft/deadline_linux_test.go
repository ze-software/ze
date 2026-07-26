//go:build linux

package firewallnft

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/env"
)

// TestNetlinkTimeoutBounds covers the spec's mandatory boundary table for the
// nft netlink deadline: range 1..60s, default 10s.
//
// VALIDATES: fixit-firewall-concurrency-deadlock D-2 -- the deadline is always
// bounded and can never be configured away.
//
// PREVENTS: reintroducing an unbounded kernel call by configuration. Zero is
// the interesting case: "no deadline" is precisely the defect D-2 removes, so
// it must clamp to the floor rather than disable the bound.
func TestNetlinkTimeoutBounds(t *testing.T) {
	for _, tt := range []struct {
		name string
		set  string
		want time.Duration
	}{
		{"unset uses the default", "", 10 * time.Second},
		{"last valid high", "60s", 60 * time.Second},
		{"invalid above clamps down", "61s", 60 * time.Second},
		{"last valid low", "1s", 1 * time.Second},
		{"zero clamps up, never disables the bound", "0s", 1 * time.Second},
		{"negative clamps up", "-5s", 1 * time.Second},
		{"unparseable falls back to the default", "banana", 10 * time.Second},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ze.firewall.nft.netlink-timeout", tt.set)
			env.ResetCache()
			t.Cleanup(env.ResetCache)

			got := netlinkTimeout()
			assert.Equal(t, tt.want, got)
			assert.GreaterOrEqual(t, got, minNetlinkTimeout,
				"an unbounded or sub-floor deadline is the defect being fixed")
			assert.LessOrEqual(t, got, maxNetlinkTimeout)
		})
	}
}

// TestAsKernelTimeoutTagsDeadlineOnly pins the sentinel that lets a caller tell
// a wedged kernel from a rejected ruleset.
//
// VALIDATES: D-2's typed timeout error.
//
// PREVENTS: the ddos-local rollback re-applying into a kernel that is already
// known to be wedged (R-8). The registry's desired state is already correct
// when Apply times out, so the retry only doubles the time an attack goes
// unmitigated -- the caller can only skip it if the timeout is distinguishable.
func TestAsKernelTimeoutTagsDeadlineOnly(t *testing.T) {
	t.Run("deadline exceeded is tagged", func(t *testing.T) {
		err := asKernelTimeout(fmt.Errorf("receive: %w", os.ErrDeadlineExceeded))
		require.Error(t, err)
		assert.ErrorIs(t, err, firewall.ErrKernelTimeout, "a wedged kernel must be identifiable")
		assert.ErrorIs(t, err, os.ErrDeadlineExceeded, "the cause must survive wrapping")
	})

	t.Run("other errors pass through untagged", func(t *testing.T) {
		cause := errors.New("EINVAL: bad rule")
		err := asKernelTimeout(cause)
		assert.ErrorIs(t, err, cause)
		assert.NotErrorIs(t, err, firewall.ErrKernelTimeout,
			"a rejected ruleset must not read as a wedged kernel")
	})

	t.Run("nil stays nil", func(t *testing.T) {
		assert.NoError(t, asKernelTimeout(nil))
	})
}
