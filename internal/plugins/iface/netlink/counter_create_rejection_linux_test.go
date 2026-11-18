// Design: docs/features/interfaces.md -- definitive creation rejection receipts

//go:build linux

package ifacenetlink

import (
	"errors"
	"syscall"
	"testing"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// A completed original negative ACK grants rejection; ordinary errno does not.
func TestCounterCreateNegativeACKGrantsRejection(t *testing.T) {
	for _, code := range []unix.Errno{unix.EBUSY, unix.EEXIST} {
		t.Run(code.Error(), func(t *testing.T) {
			var source counterSource
			dump := counterCreateRejectionDump()
			if err := counterCreateDecode(t, &source, &dump, []syscall.NetlinkMessage{counterCreateNativeNACK(code)}, nl.PidKernel); err != nil {
				t.Fatal(err)
			}
			link, read, attempted, err := counterCreateFinish(t, &source, &dump, nil)
			if !errors.Is(err, code) {
				t.Fatalf("negative ACK lost native cause: %v", err)
			}
			if !counterCreateHasRejection(err) {
				t.Fatalf("completed original negative ACK has no rejection receipt: %v", err)
			}
			if !attempted {
				t.Fatal("negative ACK erased the original send attempt")
			}
			if link != nil {
				t.Fatal("negative ACK invented a created link")
			}
			if read != nil {
				t.Fatal("negative ACK invented a counter reader")
			}
		})
	}
}

// Identical errno from send or receive uncertainty never authorizes a retry.
func TestCounterCreateUncertainErrnoHasNoRejection(t *testing.T) {
	for _, code := range []unix.Errno{unix.EBUSY, unix.EEXIST} {
		t.Run(code.Error(), func(t *testing.T) {
			var source counterSource
			dump := counterCreateRejectionDump()
			_, _, attempted, err := counterCreateFinish(t, &source, &dump, code)
			if !errors.Is(err, code) {
				t.Fatalf("uncertain send lost original errno: %v", err)
			}
			if !attempted {
				t.Fatal("uncertain send erased its attempt")
			}
			if counterCreateHasRejection(err) {
				t.Fatal("ordinary uncertain errno granted native rejection")
			}
		})
	}
}

// Uncapped kernel replies include the complete original request after errno.
func TestCounterCreateCompleteUncappedNACKGrantsRejection(t *testing.T) {
	var source counterSource
	dump := counterCreateRejectionDump()
	message := counterCreateNativeNACK(unix.EBUSY)
	message.Header.Flags &^= unix.NLM_F_CAPPED
	message.Data = append(message.Data, make([]byte, int(dump.creationHeader.Len)-unix.NLMSG_HDRLEN)...)
	if err := counterCreateDecode(t, &source, &dump, []syscall.NetlinkMessage{message}, nl.PidKernel); err != nil {
		t.Fatal(err)
	}
	_, _, _, err := counterCreateFinish(t, &source, &dump, nil)
	if !errors.Is(err, unix.EBUSY) {
		t.Fatalf("uncapped native reply lost its cause: %v", err)
	}
	if !counterCreateHasRejection(err) {
		t.Fatal("complete valid uncapped native reply has no rejection receipt")
	}
}

// Clearing CAPPED does not make a header-only response a complete native reply.
func TestCounterCreateTruncatedUncappedNACKHasNoRejection(t *testing.T) {
	var source counterSource
	dump := counterCreateRejectionDump()
	message := counterCreateNativeNACK(unix.EBUSY)
	message.Header.Flags &^= unix.NLM_F_CAPPED
	cause := counterCreateDecode(t, &source, &dump, []syscall.NetlinkMessage{message}, nl.PidKernel)
	_, _, _, err := counterCreateFinish(t, &source, &dump, cause)
	if !errors.Is(err, unix.EBUSY) {
		t.Fatalf("truncated native reply lost its cause: %v", err)
	}
	if counterCreateHasRejection(err) {
		t.Fatal("truncated uncapped native reply granted definitive rejection")
	}
}

// A transport failure dominates even a previously decoded negative ACK.
func TestCounterCreateTransportFailureRevokesRejection(t *testing.T) {
	var source counterSource
	dump := counterCreateRejectionDump()
	if err := counterCreateDecode(t, &source, &dump, []syscall.NetlinkMessage{counterCreateNativeNACK(unix.EBUSY)}, nl.PidKernel); err != nil {
		t.Fatal(err)
	}
	_, _, _, err := counterCreateFinish(t, &source, &dump, unix.EBUSY)
	if !errors.Is(err, unix.EBUSY) {
		t.Fatalf("transport failure lost original errno: %v", err)
	}
	if counterCreateHasRejection(err) {
		t.Fatal("transport failure retained a definitive rejection marker")
	}
}

// Parsing must consume the complete batch; an early negative ACK is insufficient.
func TestCounterCreateLateInvalidBatchHasNoRejection(t *testing.T) {
	var source counterSource
	dump := counterCreateRejectionDump()
	messages := []syscall.NetlinkMessage{
		counterCreateNativeNACK(unix.EBUSY),
		{Header: syscall.NlMsghdr{Type: unix.NLMSG_OVERRUN}},
	}
	cause := counterCreateDecode(t, &source, &dump, messages, nl.PidKernel)
	if !errors.Is(cause, unix.ENOBUFS) {
		t.Fatalf("late invalid message was not observed: %v", cause)
	}
	_, _, _, err := counterCreateFinish(t, &source, &dump, cause)
	if !errors.Is(err, unix.EBUSY) {
		t.Fatalf("late invalid batch lost negative ACK evidence: %v", err)
	}
	if !errors.Is(err, unix.ENOBUFS) {
		t.Fatalf("late invalid batch lost parser cause: %v", err)
	}
	if counterCreateHasRejection(err) {
		t.Fatal("incomplete valid batch granted rejection")
	}
}

// A partial creation echo preserves custody and refuses safe-retry authority.
func TestCounterCreateEchoWithNACKHasNoRejection(t *testing.T) {
	var source counterSource
	dump := counterCreateRejectionDump()
	messages := []syscall.NetlinkMessage{counterCreateEcho(), counterCreateNativeNACK(unix.EEXIST)}
	if err := counterCreateDecode(t, &source, &dump, messages, nl.PidKernel); err != nil {
		t.Fatal(err)
	}
	link, read, attempted, err := counterCreateFinish(t, &source, &dump, nil)
	if !errors.Is(err, unix.EEXIST) {
		t.Fatalf("partial creation lost native cause: %v", err)
	}
	if counterCreateHasRejection(err) {
		t.Fatal("an actual creation echo was treated as a safe rejection")
	}
	if link == nil {
		t.Fatal("partial creation discarded its original link")
	}
	if read == nil {
		t.Fatal("partial creation discarded its refused reader")
	}
	if !attempted {
		t.Fatal("partial creation erased its send attempt")
	}
	sample, readErr := read()
	if !errors.Is(readErr, unix.EEXIST) {
		t.Fatalf("partial reader lost original refusal: %v", readErr)
	}
	if sample != nil {
		t.Fatalf("partial reader became productive: sample=%v", sample)
	}
}

// Outer identity alone cannot grant rejection for a malformed embedded request.
func TestCounterCreateNACKRequiresOriginalRequestHeader(t *testing.T) {
	cases := map[string]func(*syscall.NetlinkMessage){
		"short-original-header": func(message *syscall.NetlinkMessage) { message.Data = message.Data[:4] },
		"wrong-original-length": func(message *syscall.NetlinkMessage) { nl.NativeEndian().PutUint32(message.Data[4:8], 64) },
		"wrong-original-type": func(message *syscall.NetlinkMessage) {
			nl.NativeEndian().PutUint16(message.Data[8:10], unix.RTM_DELLINK)
		},
		"wrong-original-sequence": func(message *syscall.NetlinkMessage) { nl.NativeEndian().PutUint32(message.Data[12:16], 152) },
		"wrong-original-pid":      func(message *syscall.NetlinkMessage) { nl.NativeEndian().PutUint32(message.Data[16:20], 17) },
		"missing-exclusive": func(message *syscall.NetlinkMessage) {
			nl.NativeEndian().PutUint16(message.Data[10:12], unix.NLM_F_REQUEST|unix.NLM_F_CREATE|unix.NLM_F_ACK|unix.NLM_F_ECHO)
		},
		"positive-code": func(message *syscall.NetlinkMessage) {
			nl.NativeEndian().PutUint32(message.Data[:4], uint32(unix.EBUSY))
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			var source counterSource
			dump := counterCreateRejectionDump()
			message := counterCreateNativeNACK(unix.EBUSY)
			change(&message)
			cause := counterCreateDecode(t, &source, &dump, []syscall.NetlinkMessage{message}, nl.PidKernel)
			_, _, _, err := counterCreateFinish(t, &source, &dump, cause)
			if err == nil {
				t.Fatal("unproven creation refusal disappeared")
			}
			if counterCreateHasRejection(err) {
				t.Fatal("unmatched original request granted native rejection")
			}
		})
	}
}

// Contradictory completion messages cannot supply one definitive outcome.
func TestCounterCreateMultipleACKsHaveNoRejection(t *testing.T) {
	var source counterSource
	dump := counterCreateRejectionDump()
	messages := []syscall.NetlinkMessage{counterCreateNativeNACK(unix.EBUSY), counterCreateACK(0)}
	cause := counterCreateDecode(t, &source, &dump, messages, nl.PidKernel)
	_, _, _, err := counterCreateFinish(t, &source, &dump, cause)
	if err == nil {
		t.Fatal("contradictory completion erased original refusal")
	}
	if counterCreateHasRejection(err) {
		t.Fatal("multiple completion messages granted native rejection")
	}
}

// A success ACK can leave the real receive loop waiting for its echo. A later
// batch's negative ACK must not erase that contradictory completion history.
func TestCounterCreateSplitACKsHaveNoRejection(t *testing.T) {
	cases := map[string][]syscall.NetlinkMessage{
		"success-then-negative": {counterCreateACK(0), counterCreateNativeNACK(unix.EBUSY)},
		"negative-then-success": {counterCreateNativeNACK(unix.EBUSY), counterCreateACK(0)},
	}
	for name, messages := range cases {
		t.Run(name, func(t *testing.T) {
			var source counterSource
			dump := counterCreateRejectionDump()
			for index := range messages {
				if err := counterCreateDecode(t, &source, &dump, messages[index:index+1], nl.PidKernel); err != nil {
					t.Fatal(err)
				}
			}
			_, _, _, err := counterCreateFinish(t, &source, &dump, nil)
			if !errors.Is(err, unix.EBUSY) {
				t.Fatalf("split acknowledgment history lost its native refusal: %v", err)
			}
			if counterCreateHasRejection(err) {
				t.Fatal("later batch erased contradictory prior acknowledgment")
			}
		})
	}
}

func counterCreateHasRejection(cause error) bool {
	var receipt interface{ NativeLinkCreationRejected() bool }
	if !errors.As(cause, &receipt) {
		return false
	}
	return receipt.NativeLinkCreationRejected()
}

func counterCreateRejectionDump() counterDump {
	dump := counterCreateDump()
	request := nl.NewNetlinkRequest(unix.RTM_NEWLINK,
		unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK|unix.NLM_F_ECHO)
	request.Seq = dump.sequence
	request.AddData(nl.NewIfInfomsg(unix.AF_UNSPEC))
	request.Serialize()
	dump.creationHeader = request.NlMsghdr
	return dump
}

func counterCreateNativeNACK(code unix.Errno) syscall.NetlinkMessage {
	message := counterCreateACKFor(151, unix.RTM_NEWLINK,
		unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK|unix.NLM_F_ECHO, code)
	// newCounterCreateRequest constructs a new request with original PID zero.
	// The outer response PID still names the actual socket's port 191.
	nl.NativeEndian().PutUint32(message.Data[16:20], 0)
	return message
}
