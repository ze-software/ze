//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- exact support declarations.
// Related: original_attachment_linux.go -- compiled support is not original authority.

package kernel

import (
	"errors"
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

// Literal native declarations distinguish known optional capabilities, reserved
// bits and each missing role requirement through the actual production probe.
func TestOriginalSupportDeclaredCapabilities(t *testing.T) {
	if got := OriginalPPPSupportRequirements(false); got != 320 {
		t.Fatalf("session requirements %#x, want capture8 plus connect6", got)
	}
	if got := OriginalPPPSupportRequirements(true); got != 448 {
		t.Fatalf("bridge requirements %#x, want capture8, bridge7 and connect6", got)
	}
	for _, test := range []struct {
		name     string
		features uint32
		required uint32
		want     originalAttachmentStatus
	}{
		{"full-1535", 1535, 320, originalAttachmentSupported},
		{"earlier-511", 511, 320, originalAttachmentSupported},
		{"minimal-320", 320, 320, originalAttachmentSupported},
		{"optional-exact-pair", 1344, 320, originalAttachmentSupported},
		{"bridge-minimal-448", 448, 448, originalAttachmentSupported},
		{"full-missing-connect", 1471, 320, originalAttachmentUnavailable},
		{"unit-mask-lacks-capture", 1279, 320, originalAttachmentUnavailable},
		{"full-missing-bridge", 1407, 448, originalAttachmentUnavailable},
		{"exact-pair-required", 1535, 1024, originalAttachmentSupported},
		{"exact-pair-missing", 511, 1024, originalAttachmentUnavailable},
		{"reserved-9", 2047, 320, originalAttachmentUnknown},
		{"undeclared-11", 3583, 320, originalAttachmentUnknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := os.CreateTemp(t.TempDir(), "support-capabilities-")
			if err != nil {
				t.Fatal(err)
			}
			var calls, closes int
			result := inspectOriginalAttachmentWith(test.required, originalAttachmentProbeOps{
				open: func() (*os.File, error) { return file, nil },
				inspect: func(actual *os.File, reply *pppSupportReply) error {
					calls++
					if actual != file {
						t.Fatal("queried a different original file")
					}
					if *reply != (pppSupportReply{abi: 1}) {
						t.Fatalf("query payload %#v", *reply)
					}
					reply.features = test.features
					return nil
				},
				close: func(actual *os.File) error {
					closes++
					if actual != file {
						t.Fatal("closed a different original file")
					}
					return actual.Close()
				},
			})
			if result.status != test.want {
				t.Fatalf("features=%#x required=%#x: status=%v cause=%v", test.features, test.required, result.status, result.cause)
			}
			if calls != 1 {
				t.Fatalf("queries %d", calls)
			}
			if closes != 1 {
				t.Fatalf("closes %d", closes)
			}
			if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("probe file remains open: %v", err)
			}
			if test.want == originalAttachmentSupported {
				if result.cause != nil {
					t.Fatalf("supported declaration retained failure: %v", result.cause)
				}
			} else {
				if result.cause == nil {
					t.Fatal("refused declaration lost its cause")
				}
			}
		})
	}
}

// Preserve the old EBADF plus EIO input: failed cleanup must remain Unknown
// and retain both causes even when the inspected entry was not a declaration.
func TestOriginalSupportProbeDispatchCleanupUnknown(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "support-dispatch-close-")
	if err != nil {
		t.Fatal(err)
	}
	var calls, closes int
	result := inspectOriginalAttachmentWith(pppSupportSession, originalAttachmentProbeOps{
		open: func() (*os.File, error) { return file, nil },
		inspect: func(actual *os.File, reply *pppSupportReply) error {
			calls++
			if actual != file {
				t.Fatal("inspected another original file")
			}
			if *reply != (pppSupportReply{abi: 1}) {
				t.Fatalf("query payload %#v", *reply)
			}
			return unix.EBADF
		},
		close: func(actual *os.File) error {
			closes++
			if actual != file {
				t.Fatal("closed another original file")
			}
			return errors.Join(actual.Close(), unix.EIO)
		},
	})
	if result.status != originalAttachmentUnknown {
		t.Fatal("failed cleanup granted support or absence")
	}
	for _, cause := range []error{unix.EBADF, unix.EIO} {
		if !errors.Is(result.cause, cause) {
			t.Fatalf("lost cause %v: %v", cause, result.cause)
		}
	}
	if calls != 1 {
		t.Fatalf("queries %d", calls)
	}
	if closes != 1 {
		t.Fatalf("closes %d", closes)
	}
	if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("original probe file leaked: %v", err)
	}
}
