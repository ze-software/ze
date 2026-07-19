// VALIDATES: spec-fixit-plugin-event-subscription Gap A/B SDK surface (D-2).
//   - SetStartupSubscriptionsIn threads an explicit namespace into the
//     startup SubscribeEventsInput (AC-1 reachability from a plugin).
//   - SetEnvelope opts a plugin into enveloped delivery without disturbing the
//     other startup-subscription fields (AC-2 reachability).
//   - SetStartupSubscriptions is unchanged: it yields an empty Namespace and a
//     false Envelope, so existing/out-of-tree callers are byte-identical (AC-7).
package sdk

import "testing"

func TestSetStartupSubscriptionsIn(t *testing.T) {
	p := &Plugin{}
	p.SetStartupSubscriptionsIn("vpn-ipsec", []string{"sa-up"}, []string{"*"}, "full")

	sub := p.startupSubscription
	if sub == nil {
		t.Fatal("startupSubscription is nil")
	}
	if sub.Namespace != "vpn-ipsec" {
		t.Fatalf("Namespace = %q, want vpn-ipsec", sub.Namespace)
	}
	if len(sub.Events) != 1 || sub.Events[0] != "sa-up" {
		t.Fatalf("Events = %v, want [sa-up]", sub.Events)
	}
	if len(sub.Peers) != 1 || sub.Peers[0] != "*" {
		t.Fatalf("Peers = %v, want [*]", sub.Peers)
	}
	if sub.Format != "full" {
		t.Fatalf("Format = %q, want full", sub.Format)
	}
}

func TestSetStartupSubscriptionsUnchanged(t *testing.T) {
	p := &Plugin{}
	p.SetStartupSubscriptions([]string{"update"}, nil, "parsed")

	sub := p.startupSubscription
	if sub == nil {
		t.Fatal("startupSubscription is nil")
	}
	if sub.Namespace != "" {
		t.Fatalf("Namespace = %q, want empty (legacy caller must not set it)", sub.Namespace)
	}
	if sub.Envelope {
		t.Fatal("Envelope = true, want false by default")
	}
}

func TestSetEnvelope(t *testing.T) {
	p := &Plugin{}
	p.SetStartupSubscriptions([]string{"update"}, nil, "parsed")
	p.SetEnvelope(true)

	sub := p.startupSubscription
	if !sub.Envelope {
		t.Fatal("Envelope = false, want true after SetEnvelope(true)")
	}
	// Other fields preserved.
	if len(sub.Events) != 1 || sub.Events[0] != "update" || sub.Format != "parsed" {
		t.Fatalf("SetEnvelope disturbed other fields: %+v", sub)
	}
}

// SetEnvelope before any SetStartupSubscriptions must lazily allocate the input,
// mirroring SetEncoding, so the opt-in is never silently dropped.
func TestSetEnvelopeBeforeSubscriptions(t *testing.T) {
	p := &Plugin{}
	p.SetEnvelope(true)
	if p.startupSubscription == nil || !p.startupSubscription.Envelope {
		t.Fatal("SetEnvelope before SetStartupSubscriptions did not take effect")
	}
}
