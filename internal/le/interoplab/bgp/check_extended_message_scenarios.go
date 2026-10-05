// Design: docs/architecture/testing/interop.md -- independently runnable RFC carriers.
// Related: check_extended_message.go -- shared assertions and numbered failures.
// The recorder reads each checker's literal local name and fail(1) citation. Keep
// these entry points separate so every proof selects exactly one runtime scenario.
package bgp

import (
	"context"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// RFC requirement: RFC8654-4-1 positive -- with Ze's local capability 6 present,
// a FRR-produced UPDATE carrying 4800 community octets is accepted even though
// the source's remote capability is absent, then decoded on FRR's sink session.
// This proves one >4096 UPDATE, not the 65535-octet upper boundary.
// RFC requirement: RFC8654-4-6 positive -- FRR decodes Ze's >4096 UPDATE with all
// 400 large communities when only the sink's remote capability 6 is present.
// MUTATION: Restore bilateral ExtendedMessageRecv or ExtendedMessageSend gating;
// the named FRR sink loses the learned large-community path.
func checkExtendedAsymmetricFRR(ctx context.Context, check *interoplab.CheckContext) error {
	const (
		name = "bgp-extended-message-asymmetric-frr"
	)
	fail := func(assertion int, err error) error {
		return checkerFailure(ctx, check.Lab, name, assertion, err)
	}
	// RFC 8654 Sections 4 and 6: original local and remote OPENs are independent.
	if err := checkExtendedMessages(ctx, check, extendedMessageCases[name]); err != nil {
		return fail(1, err)
	}
	return nil
}

func checkExtendedBilateralFRR(ctx context.Context, check *interoplab.CheckContext) error {
	// RFC 8654 Sections 4 and 6: control with both advertised permissions.
	return checkExtendedMessages(ctx, check, extendedMessageCases["bgp-extended-message-bilateral-frr"])
}

// RFC requirement: RFC8654-4-6 negative -- when the sink advertises no capability
// 6, no >4096 UPDATE reaches it, while a subsequent small control route does.
// MUTATION: Make ExtendedMessageSend use the local advertisement; the send-denied
// sink receives an oversized UPDATE and its complete capture rejects the sender.
func checkExtendedSendDeniedFRR(ctx context.Context, check *interoplab.CheckContext) error {
	const (
		name = "bgp-extended-message-send-denied-frr"
	)
	fail := func(assertion int, err error) error {
		return checkerFailure(ctx, check.Lab, name, assertion, err)
	}
	// RFC 8654 Section 4: a local advertisement cannot grant send permission.
	if err := checkExtendedMessages(ctx, check, extendedMessageCases[name]); err != nil {
		return fail(1, err)
	}
	return nil
}

// RFC requirement: RFC8654-5-1 negative -- with Ze's local capability 6 absent
// and the remote capability present, a real FRR >4096 UPDATE elicits exact Bad
// Message Length and ends the original session. No routing-cleanup claim is made.
// MUTATION: Use remote advertisement for receive admission; no length error follows.
func checkExtendedRemoteOnlyRejectFRR(ctx context.Context, check *interoplab.CheckContext) error {
	const (
		name = "bgp-extended-message-remote-only-reject-frr"
	)
	fail := func(assertion int, err error) error {
		return checkerFailure(ctx, check.Lab, name, assertion, err)
	}
	// RFC 8654 Sections 4 and 5: remote advertisement is not receive permission.
	if err := checkExtendedMessages(ctx, check, extendedMessageCases[name]); err != nil {
		return fail(1, err)
	}
	return nil
}

// RFC requirement: RFC8654-5-1 negative -- with both capability advertisements
// absent at Ze, a real FRR >4096 UPDATE elicits exact Bad Message Length and ends
// the original session. The relay-granted FRR view cannot authorize Ze reception.
// MUTATION: Permit extended reception unconditionally; the exact error disappears.
func checkExtendedNeitherRejectFRR(ctx context.Context, check *interoplab.CheckContext) error {
	const (
		name = "bgp-extended-message-neither-reject-frr"
	)
	fail := func(assertion int, err error) error {
		return checkerFailure(ctx, check.Lab, name, assertion, err)
	}
	// RFC 8654 Sections 4 and 5: no local advertisement means no receive permission.
	if err := checkExtendedMessages(ctx, check, extendedMessageCases[name]); err != nil {
		return fail(1, err)
	}
	return nil
}
