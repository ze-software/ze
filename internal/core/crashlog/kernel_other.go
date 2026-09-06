// Design: docs/architecture/diagnostics/crash-capture.md -- kernel crash capture on non-Linux
// Overview: kernel.go -- the harvest and the readiness answer this file serves
//
// pstore is a Linux facility. On every other platform there is no kernel crash
// store to read, and each probe here says so rather than answering with a zero
// that a caller cannot tell from "no kernel crash happened".

//go:build !linux

package crashlog

import (
	"errors"
	"runtime"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// errKernelSourceUnsupported names the platform rather than returning an empty
// record set, which a caller cannot tell from a machine that never faulted.
var errKernelSourceUnsupported = errors.New("kernel crash records are a Linux pstore facility")

// unsupportedKernelSource answers every call with the platform limit.
type unsupportedKernelSource struct{}

func openPlatformKernelSource() (KernelSource, error) {
	return unsupportedKernelSource{}, nil
}

func (unsupportedKernelSource) Available() (bool, string) {
	return false, errKernelSourceUnsupported.Error()
}

func (unsupportedKernelSource) Pending() ([]KernelRecord, error) {
	return nil, errKernelSourceUnsupported
}

func (unsupportedKernelSource) Clear(string) error { return errKernelSourceUnsupported }

func platformPstoreAvailable() (bool, string) {
	return false, errKernelSourceUnsupported.Error()
}

func platformKernelCmdline() (string, error) {
	return "", errKernelSourceUnsupported
}

func platformTotalMemoryBytes() int64 { return 0 }

func platformFreeBytes(string) (int64, error) { return 0, errKernelSourceUnsupported }

// memoryImageArchSupport reports the platform limit. A capture kernel is staged
// by kexec, which is a Linux facility, so no non-Linux build can offer one.
func memoryImageArchSupport() (bool, string) {
	var tb textbuf.Buffer
	return false, tb.Str("a memory image needs a kexec-staged capture kernel on linux/amd64; this build is ").
		Str(runtime.GOOS).Byte('/').Str(runtime.GOARCH).String()
}
