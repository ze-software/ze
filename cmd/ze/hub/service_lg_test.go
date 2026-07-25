//go:build ze_lg

package hub

// VALIDATES: the ze_lg-gated looking-glass factory -- parseASNForDecorator's
// numeric parsing/bounds and buildLGService's not-configured skip path.
// PREVENTS: an ASN decorator misparse (overflow/garbage) and a no-op build
// being treated as a failure.

import (
	"context"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

func TestParseASNForDecorator(t *testing.T) {
	cases := []struct {
		in   string
		want uint32
	}{
		{"0", 0},
		{"1", 1},
		{"65535", 65535},
		{"4294967295", 4294967295}, // max uint32, last valid
		{"4294967296", 0},          // one past max -> 0
		{"99999999999", 0},         // far past max -> 0
		{"", 0},                    // empty -> 0
		{"abc", 0},                 // non-numeric -> 0
		{"12x", 0},                 // trailing garbage -> 0
	}
	for _, c := range cases {
		if got := parseASNForDecorator(c.in); got != c.want {
			t.Errorf("parseASNForDecorator(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestBuildLGService_NotConfigured(t *testing.T) {
	// No listen addresses -> the factory skips (nil service, nil error).
	svc, err := buildLGService(ServiceDeps{Dispatch: func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc != nil {
		t.Fatalf("want nil service when not configured, got %#v", svc)
	}
}
