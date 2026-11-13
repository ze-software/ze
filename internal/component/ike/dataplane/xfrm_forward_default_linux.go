//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- ICMP error messages carried by a Child SA.
// Related: engine/unmatched.go -- final-disposition apply, rollback and release.

package dataplane

import (
	"errors"
	"fmt"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/privilege"
)

// Linux include/uapi/linux/xfrm.h, struct xfrm_userpolicy_default. A zero byte
// leaves that direction unchanged; FWD is shared by both address families and all
// if_ids in one network namespace. Explicit FWD catch-alls suppress xfrm_in_fwd_icmp.
const (
	xfrmMsgSetDefault = 0x27
	xfrmMsgGetDefault = 0x28
)

type xfrmDefaultAction uint8

const (
	xfrmDefaultUnspecified xfrmDefaultAction = iota
	xfrmDefaultBlock
	xfrmDefaultAccept
)

type xfrmPolicyDefaults struct {
	in, fwd, out xfrmDefaultAction
}

// A nil lease is unclaimed. The original value survives every reload; last is the
// value Ze most recently installed, used to refuse an unrelated writer's takeover.
// The backend mu protects both fields and every native default-policy request.
type xfrmForwardDefaultLease struct {
	original, last xfrmDefaultAction
	// A matching held value alone is not healthy after restoration failed.
	restoreErr error
}

// Tests replace these seams only without parallel execution and restore them.
var (
	xfrmDefaultsRead             = readXFRMDefaults
	xfrmDefaultWrite             = writeXFRMForwardDefault
	errXFRMForwardDefaultChanged = errors.New("another policy manager changed the namespace's XFRM forwarding default")
	errXFRMForwardDefaultInput   = errors.New("Ze requested an invalid forwarding-default operation")
)

// Native helpers keep their causes; the public backend boundary supplies the
// problem, consequence and next command once, preserving errors.Is evidence.
func xfrmForwardDefaultRefusal(cause error, problem string) error {
	// refuse-by-default: guards ApplyForwardDefault -- successful native operations need no refusal envelope.
	if cause == nil {
		return nil
	}
	why := "Ze cannot confirm the forwarding disposition, so it refuses to change unknown state. Inspect the kernel warnings before retrying the IPsec configuration."
	fix := "sudo dmesg --level=err,warn"
	switch {
	case errors.Is(cause, errXFRMForwardDefaultInput):
		why = "Ze built an invalid final-disposition request. This is a Ze defect; collect the installed version when reporting it before retrying the configuration."
		fix = "ze show version"
	case errors.Is(cause, errXFRMForwardDefaultChanged):
		why = "Another XFRM policy manager changed the value Ze had installed. Ze leaves that value untouched and retains its claim. Read the current defaults, then reconcile the other manager with the IPsec configuration before retrying."
		fix = "ip xfrm policy getdefault"
	case errors.Is(cause, unix.EPERM), errors.Is(cause, unix.EACCES):
		why = "The IKE process cannot read or set the namespace forwarding policy without CAP_NET_ADMIN. The command below reinstalls the systemd service's capabilities and starts it; a container must also grant NET_ADMIN to that process."
		fix = privilege.DaemonCapabilitiesFix
	case errors.Is(cause, unix.EINVAL), errors.Is(cause, unix.EPROTONOSUPPORT), errors.Is(cause, unix.ENOPROTOOPT), errors.Is(cause, unix.EOPNOTSUPP):
		why = "The running kernel does not provide the XFRM default-policy operation IPsec forwarding needs. Install or select Ze's supported runtime kernel with CONFIG_XFRM_USER before rebooting into it; the command below reboots the host."
		fix = "sudo reboot"
	}
	return opererr.Wrap(cause, problem, why, fix)
}

func readXFRMDefaults() (xfrmPolicyDefaults, error) {
	req := nl.NewNetlinkRequest(xfrmMsgGetDefault, unix.NLM_F_REQUEST)
	req.AddRawData([]byte{0, 0, 0})
	messages, err := req.Execute(unix.NETLINK_XFRM, xfrmMsgGetDefault)
	if err != nil {
		return xfrmPolicyDefaults{}, fmt.Errorf("read the namespace XFRM policy defaults: %w", err)
	}
	if len(messages) != 1 || len(messages[0]) < 3 {
		return xfrmPolicyDefaults{}, fmt.Errorf("XFRM GETDEFAULT returned %d messages, want one three-byte policy payload", len(messages))
	}
	for direction, action := range messages[0][:3] {
		if xfrmDefaultAction(action) != xfrmDefaultBlock && xfrmDefaultAction(action) != xfrmDefaultAccept {
			return xfrmPolicyDefaults{}, fmt.Errorf("XFRM default direction %d has unknown disposition %d", direction, action)
		}
	}
	return xfrmPolicyDefaults{
		in: xfrmDefaultAction(messages[0][0]), fwd: xfrmDefaultAction(messages[0][1]),
		out: xfrmDefaultAction(messages[0][2]),
	}, nil
}

func writeXFRMForwardDefault(action xfrmDefaultAction) error {
	if action != xfrmDefaultBlock && action != xfrmDefaultAccept {
		return fmt.Errorf("XFRM forward default disposition %d is neither block nor accept", action)
	}
	req := nl.NewNetlinkRequest(xfrmMsgSetDefault, unix.NLM_F_ACK)
	req.AddRawData([]byte{byte(xfrmDefaultUnspecified), byte(action), byte(xfrmDefaultUnspecified)})
	if _, err := req.Execute(unix.NETLINK_XFRM, 0); err != nil {
		return fmt.Errorf("set the namespace XFRM forward default: %w", err)
	}
	return nil
}

// UsesForwardDefault describes the backend's strategy, not a kernel probe. An old
// or unreadable kernel is refused by ApplyForwardDefault before any policy install.
func (b *xfrmBackend) UsesForwardDefault() bool { return true }

// changeForwardDefault requires the current value to remain ours. Linux has no
// compare-and-swap default-policy message, so external writers MUST serialize their
// changes with Ze; a value observed to have changed is never overwritten here.
// Readback, rather than an ACK alone, decides whether the requested value landed.
func changeForwardDefault(before, after xfrmDefaultAction) (xfrmDefaultAction, error) {
	current, err := xfrmDefaultsRead()
	if err != nil {
		return xfrmDefaultUnspecified, err
	}
	if current.fwd != before {
		return current.fwd, fmt.Errorf("%w (expected %d, found %d)", errXFRMForwardDefaultChanged, before, current.fwd)
	}
	if before == after {
		return before, nil
	}
	writeErr := xfrmDefaultWrite(after)
	current, err = xfrmDefaultsRead()
	if err != nil {
		return xfrmDefaultUnspecified, errors.Join(writeErr, err)
	}
	if current.fwd == after {
		return after, nil
	}
	if writeErr == nil {
		writeErr = fmt.Errorf("XFRM forward default readback is %d after setting %d", current.fwd, after)
	}
	return current.fwd, writeErr
}

// ApplyForwardDefault claims the namespace FWD disposition once. Its undo belongs
// to this apply transaction and MUST run before any later apply if another final
// entry refuses installation. Failed restoration keeps the lease for a later retry.
func (b *xfrmBackend) ApplyForwardDefault(action SPAction) (undo func() error, err error) {
	defer func() {
		if err != nil {
			err = xfrmForwardDefaultRefusal(err, "Ze cannot apply the final forwarding disposition configured by vpn ipsec unmatched")
		}
	}()
	want := xfrmDefaultUnspecified
	switch action {
	case SPActionDiscard:
		want = xfrmDefaultBlock
	case SPActionBypass:
		want = xfrmDefaultAccept
	case SPActionProtect:
		return nil, fmt.Errorf("%w: protect names no transform", errXFRMForwardDefaultInput)
	default:
		return nil, fmt.Errorf("%w: unknown action %d", errXFRMForwardDefaultInput, action)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	current, err := xfrmDefaultsRead()
	if err != nil {
		return nil, err
	}
	var previous *xfrmForwardDefaultLease
	if b.forwardDefault != nil {
		copyOfLease := *b.forwardDefault
		previous = &copyOfLease
		if current.fwd != previous.last {
			return nil, fmt.Errorf("%w (expected %d, found %d)", errXFRMForwardDefaultChanged, previous.last, current.fwd)
		}
	} else {
		b.forwardDefault = &xfrmForwardDefaultLease{original: current.fwd, last: current.fwd}
	}
	before := current.fwd
	// Retain the attempted value if readback becomes unavailable: release must first
	// prove the namespace still holds it, rather than silently overwrite an unknown.
	b.forwardDefault.last = want
	observed, err := changeForwardDefault(before, want)
	if observed == before || observed == want {
		b.forwardDefault.last = observed
	}
	if err != nil {
		return nil, errors.Join(err, b.restoreForwardDefaultLocked(previous, before))
	}
	b.forwardDefault.restoreErr = nil
	return func() (undoErr error) {
		defer func() {
			if undoErr != nil {
				undoErr = xfrmForwardDefaultRefusal(undoErr, "Ze cannot restore the forwarding disposition from before the refused IPsec apply")
			}
		}()
		b.mu.Lock()
		defer b.mu.Unlock()
		return b.restoreForwardDefaultLocked(previous, before)
	}, nil
}

func (b *xfrmBackend) restoreForwardDefaultLocked(previous *xfrmForwardDefaultLease, disposition xfrmDefaultAction) error {
	if b.forwardDefault == nil {
		return fmt.Errorf("%w: rollback has no namespace claim", errXFRMForwardDefaultInput)
	}
	observed, err := changeForwardDefault(b.forwardDefault.last, disposition)
	if err != nil {
		b.forwardDefault.restoreErr = err
		return fmt.Errorf("restore the namespace XFRM forward default; Ze retains its lease until restoration succeeds: %w", err)
	}
	if previous != nil {
		copyOfLease := *previous
		copyOfLease.last = observed
		b.forwardDefault = &copyOfLease
	} else {
		b.forwardDefault = nil
	}
	return nil
}

// ReleaseForwardDefault is called only after every other Ze SPD entry was removed.
// Close intentionally does not call it: failed policy removal must retain protection.
func (b *xfrmBackend) ReleaseForwardDefault() (err error) {
	defer func() {
		if err != nil {
			err = xfrmForwardDefaultRefusal(err, "Ze cannot restore the namespace forwarding default after removing its IPsec policies")
		}
	}()
	b.mu.Lock()
	defer b.mu.Unlock()
	// refuse-by-default: guards ApplyForwardDefault -- no successful claim means no default belongs to Ze to restore.
	if b.forwardDefault == nil {
		return nil
	}
	return b.restoreForwardDefaultLocked(nil, b.forwardDefault.original)
}

// CheckForwardDefault reads kernel support and the held value under the mutation
// lock. It neither claims the namespace nor reasserts or restores any disposition.
func (b *xfrmBackend) CheckForwardDefault() (err error) {
	defer func() {
		if err != nil {
			err = xfrmForwardDefaultRefusal(err, "Ze cannot confirm the forwarding default its IPsec policies need")
		}
	}()
	b.mu.Lock()
	defer b.mu.Unlock()
	current, err := xfrmDefaultsRead()
	if err != nil {
		return err
	}
	if b.forwardDefault != nil && current.fwd != b.forwardDefault.last {
		return fmt.Errorf("%w (expected %d, found %d)", errXFRMForwardDefaultChanged, b.forwardDefault.last, current.fwd)
	}
	if b.forwardDefault != nil && b.forwardDefault.restoreErr != nil {
		return fmt.Errorf("restoration of the namespace forwarding default remains incomplete: %w", b.forwardDefault.restoreErr)
	}
	return nil
}
