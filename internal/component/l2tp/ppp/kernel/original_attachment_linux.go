//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- positive PPP dependency.
// Related: original_pair_abi_linux.go -- mandatory original-file owner receipts.

package kernel

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
)

const (
	pppSupportQuery                   = 0xc010742a // PPPIOCGSUPPORT: _IOWR('t', 42, struct ppp_support).
	pppSupportABI                     = 1
	pppSupportConnect          uint32 = 1 << 6
	pppSupportBridge           uint32 = 1 << 7
	pppSupportCapture          uint32 = 1 << 8
	pppSupportSession                 = pppSupportCapture | pppSupportConnect
	pppSupportExactAddressPair uint32 = 1 << 10
	// Root0011c declares bits0..8 and10. Bit9 remains reserved.
	pppSupportKnown uint32 = ((1 << 9) - 1) | pppSupportExactAddressPair
)

// This is a raw native DTO, never a resource grant. Input is ABI1 and all zero
// payload words. A successful reply still owes complete validation.
type pppSupportReply struct{ abi, features, reserved0, reserved1 uint32 }

var (
	_ [16 - int(unsafe.Sizeof(pppSupportReply{}))]byte
	_ [int(unsafe.Sizeof(pppSupportReply{})) - 16]byte
	_ [4 - int(unsafe.Alignof(pppSupportReply{}))]byte
	_ [int(unsafe.Alignof(pppSupportReply{})) - 4]byte
)

type originalAttachmentStatus uint8

const (
	originalAttachmentUnspecified originalAttachmentStatus = iota
	originalAttachmentSupported
	originalAttachmentUnavailable
	originalAttachmentUnknown
)

// Zero is invalid. Only the live probe constructs production results. Supported
// declares compiled ABI only: it MUST NOT authorize an original resource.
type originalAttachmentResult struct {
	status originalAttachmentStatus
	cause  error
}

type originalAttachmentProbeOps struct {
	open    func() (*os.File, error)
	inspect func(*os.File, *pppSupportReply) error
	close   func(*os.File) error
}

// inspectOriginalAttachment owns one fresh, unbound /dev/ppp file, querying and
// closing that exact file. It creates no unit/channel and changes no mode,
// attachment or global health state. Safe for concurrent use.
func inspectOriginalAttachment(required uint32) originalAttachmentResult {
	return inspectOriginalAttachmentWith(required, originalAttachmentProbeOps{
		open:    func() (*os.File, error) { return os.OpenFile("/dev/ppp", os.O_RDWR, 0) },
		inspect: inspectOriginalAttachmentFile, close: (*os.File).Close,
	})
}

func inspectOriginalAttachmentWith(required uint32, ops originalAttachmentProbeOps) originalAttachmentResult {
	file, err := ops.open()
	if err != nil {
		return originalAttachmentResult{status: originalAttachmentUnknown, cause: err}
	}
	reply := pppSupportReply{abi: pppSupportABI}
	queryErr := ops.inspect(file, &reply)
	inspectionErr := queryErr
	if inspectionErr == nil {
		inspectionErr = validateOriginalSupportReply(reply)
	}
	var missingErr error
	if inspectionErr == nil {
		if missing := required &^ reply.features; missing != 0 {
			missingErr = fmt.Errorf("PPP support declaration lacks required feature bits %#x (declared %#x)", missing, reply.features)
		}
	}
	if closeErr := ops.close(file); closeErr != nil {
		return originalAttachmentResult{status: originalAttachmentUnknown, cause: errors.Join(inspectionErr, missingErr, closeErr)}
	}
	if queryErr == unix.ENOTTY {
		return originalAttachmentResult{status: originalAttachmentUnavailable, cause: queryErr}
	}
	if inspectionErr != nil {
		return originalAttachmentResult{status: originalAttachmentUnknown, cause: inspectionErr}
	}
	if missingErr != nil {
		return originalAttachmentResult{status: originalAttachmentUnavailable, cause: missingErr}
	}
	return originalAttachmentResult{status: originalAttachmentSupported}
}

func validateOriginalSupportReply(reply pppSupportReply) error {
	if reply.abi != pppSupportABI {
		return fmt.Errorf("PPP support reply ABI %d, expected %d", reply.abi, pppSupportABI)
	}
	if reply.reserved0 != 0 {
		return errors.New("PPP support reply has nonzero reserved0")
	}
	if reply.reserved1 != 0 {
		return errors.New("PPP support reply has nonzero reserved1")
	}
	if reply.features&^pppSupportKnown != 0 {
		return fmt.Errorf("PPP support reply has undeclared feature bits %#x", reply.features&^pppSupportKnown)
	}
	return nil
}

func inspectOriginalAttachmentFile(file *os.File, reply *pppSupportReply) error {
	connection, err := file.SyscallConn()
	if err != nil {
		return err
	}
	var queryErr error
	err = connection.Control(func(fd uintptr) { queryErr = pppSupportIoctl(int(fd), reply) })
	if err != nil {
		return errors.Join(err, queryErr)
	}
	return queryErr
}

func pppSupportIoctl(fd int, reply *pppSupportReply) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), pppSupportQuery,
		uintptr(unsafe.Pointer(reply))) //nolint:gosec // Native ioctl requires the exact fixed-layout reply pointer.
	if errno != 0 {
		return errno
	}
	return nil
}

// CheckOriginalPPPSupport refuses unavailable/Unknown compiled support before
// CPE allocation. It grants no permission or ownership. Each constructor MUST
// also obtain positive original unit51/channel46/transport44 receipts and retain
// actual operation refusals. Safe for concurrent use in the caller's namespace.
func CheckOriginalPPPSupport() error { return checkOriginalAttachment() }

func checkOriginalAttachment() error { return checkOriginalAttachmentRequirements(pppSupportSession) }

func checkOriginalAttachmentRequirements(required uint32) error {
	result := inspectOriginalAttachment(required)
	switch result.status {
	case originalAttachmentSupported:
		return nil
	case originalAttachmentUnavailable:
		return originalAttachmentUnavailableRefusal(result.cause)
	case originalAttachmentUnknown:
		return opererr.Wrap(result.cause, "Ze cannot inspect the running kernel's PPP support declaration",
			"Typed PPP admission is refused because support is Unknown. Inspect the daemon's device access and native capability grant; an inspection failure does not establish kernel absence.", "ze show doctor")
	case originalAttachmentUnspecified:
		panic("BUG: original PPP support probe returned no classification")
	default:
		panic("BUG: original PPP support probe returned an invalid classification")
	}
}

// A successful declaration does not promise future operations will succeed.
// Preserve the exact late syscall error and refuse; never attach numerically.
func originalAttachmentRefusal(cause error) error {
	return opererr.Wrap(cause, "Ze cannot use the original PPP channel and unit files",
		"The native operation refused this exact original pair. Check the retained cause for device access, namespace, syscall policy and resource lifetime; a compiled declaration grants no operation permission. Inspect support again before selecting a kernel rebuild.", "ze show doctor")
}

func originalAttachmentUnavailableRefusal(cause error) error {
	return opererr.Wrap(cause, "The running kernel lacks required original-resource PPP support",
		"Build the matching Ze runtime kernel and have this platform's kernel image owner install and boot it before retrying. A binary self-update does not replace the kernel.", originalAttachmentFix())
}

// The command builds for this running Ze binary's architecture. Installation
// and boot belong to the actual platform image owner, not firmware Apply.
func originalAttachmentFix() string {
	return "./le build host-driver\n./ze-host appliance kernel --target runtime --arch " + runtime.GOARCH
}

func originalAttachmentKernel() (string, error) {
	var name unix.Utsname
	if err := unix.Uname(&name); err != nil {
		return "", err
	}
	return fmt.Sprintf("release=%s version=%s machine=%s", unix.ByteSliceToString(name.Release[:]),
		unix.ByteSliceToString(name.Version[:]), unix.ByteSliceToString(name.Machine[:])), nil
}

// OriginalPPPSupportDoctorCheck constructs the one real query42 check with its
// owner's parsed configuration predicate. It holds no mutable test hook or
// status ledger. Zero requirements means that configuration needs no native PPP.
// The owner MUST register this returned check, and MUST distinguish scope
// failure from an unused dependency. Safe for immutable scope inputs.
func OriginalPPPSupportDoctorCheck(scope func(diagnostic.DoctorCheckContext) (uint32, error)) diagnostic.DoctorCheck {
	return originalAttachmentDoctorCheck(inspectOriginalAttachment, originalAttachmentKernel, scope)
}

// OriginalPPPSupportRequirements returns only the role's required compiled
// bits. Bridge adds bit7; neither role requires unfinished IPv6-control bit9.
func OriginalPPPSupportRequirements(bridge bool) uint32 {
	if bridge {
		return pppSupportSession | pppSupportBridge
	}
	return pppSupportSession
}

func originalAttachmentDoctorCheck(inspect func(uint32) originalAttachmentResult,
	provenance func() (string, error), scope func(diagnostic.DoctorCheckContext) (uint32, error)) diagnostic.DoctorCheck {
	return diagnostic.DoctorCheck{
		Name: originalAttachmentCheckName, Phase: diagnostic.DoctorPhasePostConfig,
		Order: 715, Component: "l2tp", Dependencies: []string{"ppp-original-resource-support"},
		Platforms: []string{diagnostic.DoctorPlatformAny}, Codes: []string{originalAttachmentDoctorCode},
		Check: func(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
			required, scopeErr := scope(ctx)
			if scopeErr == nil {
				if required == 0 {
					return nil
				}
			}
			result := originalAttachmentResult{status: originalAttachmentUnknown, cause: scopeErr}
			if scopeErr == nil {
				result = inspect(required)
			}
			kernel, provenanceErr := provenance()
			if provenanceErr != nil {
				result = originalAttachmentResult{status: originalAttachmentUnknown, cause: errors.Join(result.cause, provenanceErr)}
			}
			var problem, fix, help string
			switch result.status {
			case originalAttachmentSupported:
				return nil
			case originalAttachmentUnavailable:
				problem = "The running kernel lacks required original-resource PPP support"
				fix = originalAttachmentFix()
				help = "Build the matching runtime kernel, then have this platform's kernel image owner install and boot that exact image. These build commands neither install nor boot it; a binary self-update does not replace the kernel."
			case originalAttachmentUnknown:
				problem = "Ze cannot verify the running kernel's original-resource PPP support"
				fix = "ze show doctor"
				help = "Inspect the daemon's parsed configured scope, /dev/ppp access and native capability grant. Retain the cause: Unknown does not establish absence or show that rebuilding repairs permission."
			case originalAttachmentUnspecified:
				panic("BUG: doctor received no original PPP support classification")
			default:
				panic("BUG: doctor received an invalid original PPP support classification")
			}
			message := problem + "; typed PPP admission is refused."
			if kernel != "" {
				message += " Running kernel: " + kernel + "."
			}
			if result.cause != nil {
				message += " Cause: " + result.cause.Error()
			}
			return []diagnostic.Diagnostic{{Code: originalAttachmentDoctorCode, Severity: diagnostic.SeverityError,
				Message: message, Fix: fix, Help: help + " Query42 declares compiled ABI only. Positive original unit51/channel46/transport44 receipts and every actual operation's permission/lifetime checks remain mandatory. uname does not certify profile, series or image bytes."}}
		},
	}
}
