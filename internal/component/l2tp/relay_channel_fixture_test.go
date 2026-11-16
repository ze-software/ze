// Design: docs/architecture/l2tp/followup-l2tp-call.md -- relay request fixtures
// Related: relay_sink_test.go, relay_realm_sink_test.go -- control-plane-only calls

package l2tp

// relayDescriptorForTest preserves the control-plane fixtures' borrowed socket
// identity. Those tests never create or bridge native PPP channels, so no real
// descriptor is owned or closed by this fixture.
type relayDescriptorForTest int

func (descriptor relayDescriptorForTest) WithDescriptor(use func(int) error) error {
	return use(int(descriptor))
}
