// Design: docs/features/interfaces.md -- original native creation rejection
// Related: counter_create_receive_linux.go -- complete ordered response batch
// Related: counter_create_linux.go -- final creation outcome

//go:build linux

package ifacenetlink

import (
	"syscall"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// counterCreateNegativeACK is evidence from the complete validated reply batch.
// It deliberately has no public rejection marker: a later transport/parser
// failure must keep that evidence without granting definitive retry authority.
// Zero is not a negative acknowledgment and grants no authority.
type counterCreateNegativeACK syscall.Errno

func (rejection counterCreateNegativeACK) Error() string {
	return syscall.Errno(rejection).Error()
}

func (rejection counterCreateNegativeACK) Unwrap() error {
	return syscall.Errno(rejection)
}

// counterCreateNativeRejection is minted only at successful batch completion
// without any creation echo or transport/parser failure. The private value's
// zero form is invalid; its marker returns false. It preserves the native cause.
type counterCreateNativeRejection syscall.Errno

func (rejection counterCreateNativeRejection) Error() string {
	return syscall.Errno(rejection).Error()
}

func (rejection counterCreateNativeRejection) Unwrap() error {
	return syscall.Errno(rejection)
}

func (rejection counterCreateNativeRejection) NativeLinkCreationRejected() bool {
	return rejection != 0
}

// counterCreateNegativeReply reads the kernel's original request header, rather
// than inferring rejection from an errno supplied by Send or Receive. The caller
// must first validate kernel sender, outer request identity and the WHOLE batch.
// The operation captures the actual owned clone's serialized header before Send.
// A missing or changed header grants
// no receipt, while the ordinary error path still retains its native cause.
func counterCreateNegativeReply(dump *counterDump, message syscall.NetlinkMessage) counterCreateNegativeACK {
	if message.Header.Type != unix.NLMSG_ERROR {
		return 0
	}
	if message.Header.Seq != dump.sequence {
		return 0
	}
	if message.Header.Pid != dump.portID {
		return 0
	}
	if len(message.Data) < 4+unix.NLMSG_HDRLEN {
		return 0
	}
	code := int32(nl.NativeEndian().Uint32(message.Data[:4]))
	if code >= 0 {
		return 0
	}
	// Linux reserves errno values1..4095. Avoid negating an invalid int32 min.
	if code < -4095 {
		return 0
	}
	expected := dump.creationHeader
	if expected.Type != unix.RTM_NEWLINK {
		return 0
	}
	const flags = unix.NLM_F_REQUEST | unix.NLM_F_CREATE | unix.NLM_F_EXCL |
		unix.NLM_F_ACK | unix.NLM_F_ECHO
	if expected.Flags != flags {
		return 0
	}
	if expected.Len < unix.NLMSG_HDRLEN+unix.SizeofIfInfomsg {
		return 0
	}
	// netlink_ack retains the complete original request unless CAPPED says
	// only its header follows errno. A truncated uncapped response is no proof.
	if message.Header.Flags&unix.NLM_F_CAPPED == 0 {
		if uint64(len(message.Data)) < 4+uint64(expected.Len) {
			return 0
		}
	}
	original := message.Data[4 : 4+unix.NLMSG_HDRLEN]
	if nl.NativeEndian().Uint32(original[:4]) != expected.Len {
		return 0
	}
	if nl.NativeEndian().Uint16(original[4:6]) != expected.Type {
		return 0
	}
	if nl.NativeEndian().Uint16(original[6:8]) != expected.Flags {
		return 0
	}
	if nl.NativeEndian().Uint32(original[8:12]) != expected.Seq {
		return 0
	}
	if nl.NativeEndian().Uint32(original[12:16]) != expected.Pid {
		return 0
	}
	return counterCreateNegativeACK(-code)
}

// completedCounterCreateRejection projects only the one proven rejection case.
// Nil means no such receipt; callers keep the ordinary uncertain error path.
func completedCounterCreateRejection(dump *counterDump, cause error) error {
	if cause != nil {
		return nil
	}
	if !dump.single {
		return nil
	}
	if !dump.done {
		return nil
	}
	if dump.interrupted {
		return nil
	}
	if len(dump.links) != 0 {
		return nil
	}
	rejection, confirmed := dump.failure.(counterCreateNegativeACK)
	if !confirmed {
		return nil
	}
	if rejection == 0 {
		return nil
	}
	return counterCreateNativeRejection(rejection)
}
