// Design: docs/research/l2tpv2-ze-integration.md -- provider configuration refusals.

package pppconfig

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
)

// TestLCPProviderRejectsUnknownSetting sends a misspelled leaf through each
// transport boundary. Its refusal must name the unknown input, suggest the
// closest declared setting and provide the complete transport repair command.
func TestLCPProviderRejectsUnknownSetting(t *testing.T) {
	for _, transport := range []string{"l2tp", "pppoe"} {
		t.Run(transport, func(t *testing.T) {
			_, err := ParseLCP(transport, map[string]any{"restart-timre": "500"})
			operatorError, isOperator := errors.AsType[*opererr.Error](err)
			if !isOperator {
				t.Fatalf("unknown setting refusal = %v, want opererr.Error", err)
			}
			if !strings.Contains(err.Error(), "restart-timre") {
				t.Fatalf("refusal omits the unknown setting: %v", err)
			}
			if !strings.Contains(err.Error(), "restart-timer") {
				t.Fatalf("refusal omits the closest declared setting: %v", err)
			}
			want := "set " + transport + " lcp restart-timer 3000"
			if operatorError.Fix() != want {
				t.Fatalf("repair = %q, want %q", operatorError.Fix(), want)
			}
			if !strings.HasSuffix(err.Error(), "\nfix: "+want) {
				t.Fatalf("repair is not the final line: %v", err)
			}
		})
	}
}

// An unrelated name has no safe spelling repair. The refusal must still name
// the input and every supported setting, with a valid transport-root repair.
func TestLCPProviderRejectsUnrelatedSetting(t *testing.T) {
	for _, transport := range []string{"l2tp", "pppoe"} {
		t.Run(transport, func(t *testing.T) {
			_, err := ParseLCP(transport, map[string]any{"xyz": "500"})
			operatorError, isOperator := errors.AsType[*opererr.Error](err)
			if !isOperator {
				t.Fatalf("unknown setting refusal = %v, want opererr.Error", err)
			}
			for _, text := range []string{"xyz", "restart-timer", "max-terminate", "max-configure", "max-failure"} {
				if !strings.Contains(err.Error(), text) {
					t.Fatalf("refusal omits %q: %v", text, err)
				}
			}
			want := "delete " + transport + " lcp"
			if operatorError.Fix() != want {
				t.Fatalf("repair = %q, want %q", operatorError.Fix(), want)
			}
			if !strings.HasSuffix(err.Error(), "\nfix: "+want) {
				t.Fatalf("repair is not the final line: %v", err)
			}
		})
	}
}

// TestLCPProviderAcceptsKnownSettings is the useful-output contrast: a reader
// that refuses every provider map must not pass the unknown-setting check.
func TestLCPProviderAcceptsKnownSettings(t *testing.T) {
	for _, transport := range []string{"l2tp", "pppoe"} {
		t.Run(transport, func(t *testing.T) {
			c, err := ParseLCP(transport, map[string]any{"restart-timer": "500", "max-configure": "3"})
			if err != nil {
				t.Fatal(err)
			}
			if c.RestartTimer() != 500*time.Millisecond {
				t.Fatalf("timer = %s, want 500ms", c.RestartTimer())
			}
			if c.MaxConfigure() != 3 {
				t.Fatalf("request budget = %d, want 3", c.MaxConfigure())
			}
		})
	}
}
