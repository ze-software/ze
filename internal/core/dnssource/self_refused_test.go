package dnssource

import (
	"errors"
	"strings"
	"testing"
)

// TestSelfRefusedOnSystemAndForward: a self source written under the
// router's list (system dns-source) and under the LAN resolver's upstream list
// (service dns-resolver forward dns-source) is refused through ParseContainer,
// the reader both consumers call, and the refusal carries three parts the
// operator reads: what (the path and the class), why (the consumer's reason)
// and the fix on a line of its own. The refusal comes ahead of the mode
// refusal, so a list of self plus another source still names self.
//
// Method: one row per consumer, each with the test's own refusal text (this
// package names no consumer), checked part by part through What, the
// Refusal fields and the rendered Error.
//
// VALIDATES: AC-11, the self refusal's what, why and fix parts for both
// consumers that cannot use self (spec-home-cpe-23b).
// PREVENTS: self accepted on an upstream list, where Ze would query itself;
// a refusal that drops the why or the fix; the mode refusal hiding self.
func TestSelfRefusedOnSystemAndForward(t *testing.T) {
	consumers := []struct {
		name    string
		path    string
		refusal Refusal
	}{
		{
			name: "system",
			path: "system dns-source",
			refusal: Refusal{Class: ClassSelf, Why: "Ze cannot be its own upstream",
				Fix: "delete system dns-source source self"},
		},
		{
			name: "forward",
			path: "service dns-resolver forward dns-source",
			refusal: Refusal{Class: ClassSelf, Why: "the resolver cannot be its own upstream",
				Fix: "delete service dns-resolver forward dns-source source self"},
		},
	}
	bodies := []struct {
		name   string
		source any
	}{
		{name: "self alone", source: "self"},
		{name: "self beside received, no mode", source: []any{"received", "self"}},
	}
	for _, consumer := range consumers {
		builtin := StatedDefault(consumer.path, FamilyAll, AudienceZe, ModeFirstUsable, ClassConfigured, ClassReceived)
		for _, body := range bodies {
			parent := map[string]any{ContainerKey: map[string]any{"source": body.source}}
			_, err := ParseContainer(parent, builtin, FamilyAll, AudienceZe, consumer.refusal)
			var refused *ClassRefusedError
			if !errors.As(err, &refused) {
				t.Fatalf("%s, %s: err = %v, want *ClassRefusedError", consumer.name, body.name, err)
			}
			if want := consumer.path + " names self"; refused.What() != want {
				t.Errorf("%s, %s: what = %q, want %q", consumer.name, body.name, refused.What(), want)
			}
			if refused.Refusal.Why != consumer.refusal.Why {
				t.Errorf("%s, %s: why = %q, want %q", consumer.name, body.name, refused.Refusal.Why, consumer.refusal.Why)
			}
			lines := strings.Split(err.Error(), "\n")
			if len(lines) != 2 {
				t.Fatalf("%s, %s: error %q has %d lines, want what and why, then the fix",
					consumer.name, body.name, err.Error(), len(lines))
			}
			if want := consumer.path + " names self: " + consumer.refusal.Why; lines[0] != want {
				t.Errorf("%s, %s: first line = %q, want %q", consumer.name, body.name, lines[0], want)
			}
			if lines[1] != consumer.refusal.Fix {
				t.Errorf("%s, %s: fix line = %q, want %q", consumer.name, body.name, lines[1], consumer.refusal.Fix)
			}
		}
	}
}
