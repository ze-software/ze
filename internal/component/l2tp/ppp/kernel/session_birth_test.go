// Design: docs/research/l2tpv2-ze-integration.md -- native rate policy birth fence

package kernel

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

func (*releaseTestSeal) NativeBirthTag() string { return "kernel-release-fixture" }

type birthTestSeal struct {
	releaseTestSeal
	tag         string
	getterPanic error
}

func (s *birthTestSeal) NativeBirthTag() string {
	if s.getterPanic != nil {
		panic(s.getterPanic)
	}
	return s.tag
}

func nativeBirthOwner(t *testing.T, rig *nativeReleaseRig) *session {
	t.Helper()
	owner, ok := rig.handle.(*session)
	if !ok {
		t.Fatalf("kernel release owner has type %T, want *session", rig.handle)
	}
	return owner
}

func TestKernelNativeBirthUsesExactPolicyTag(t *testing.T) {
	rig := newNativeReleaseRig(t)
	seal := &birthTestSeal{tag: "full-opaque-policy-birth/no-kernel-prefix"}
	calls := 0
	nativeBirthOwner(t, rig).ops.stampBirth = func(identity ppp.SessionIdentity, tag string) error {
		calls++
		if identity != rig.handle.Identity() || tag != seal.tag {
			t.Fatalf("native stamp changed its admitted identity or tag: %+v %q", identity, tag)
		}
		return nil
	}
	if err := rig.release.BindNativeReleaseSeal(seal); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("native birth stamp calls = %d, want one before binding", calls)
	}
}

func TestKernelNativeBirthRefusesMalformedTagBeforeMutation(t *testing.T) {
	for _, tag := range []string{"", "embedded\x00terminator", strings.Repeat("x", 256)} {
		t.Run(tag, func(t *testing.T) {
			rig := newNativeReleaseRig(t)
			stamps := 0
			nativeBirthOwner(t, rig).ops.stampBirth = func(ppp.SessionIdentity, string) error {
				stamps++
				return nil
			}
			if err := rig.release.BindNativeReleaseSeal(&birthTestSeal{tag: tag}); err == nil {
				t.Fatal("malformed native birth tag was accepted")
			}
			if stamps != 0 || rig.duplicates != 0 {
				t.Fatalf("malformed tag reached native mutation: stamps=%d, duplicates=%d", stamps, rig.duplicates)
			}
		})
	}
}

func TestKernelNativeBirthUnconfirmedStampRefusesBinding(t *testing.T) {
	rig := newNativeReleaseRig(t)
	seal := &birthTestSeal{tag: "opaque-policy-birth"}
	want := errors.New("native alias result is unconfirmed")
	nativeBirthOwner(t, rig).ops.stampBirth = func(ppp.SessionIdentity, string) error { return want }
	if err := rig.release.BindNativeReleaseSeal(seal); !errors.Is(err, want) {
		t.Fatalf("binding lost the unconfirmed stamp: %v", err)
	}
	if rig.duplicates != 0 || rig.release.NativeReleasePending() {
		t.Fatal("an unconfirmed birth stamp published a release binding")
	}
	for _, stream := range rig.streams {
		if stream.closes != 0 {
			t.Fatal("refused stamping closed a caller-owned stream")
		}
	}
	nativeBirthOwner(t, rig).ops.stampBirth = func(ppp.SessionIdentity, string) error { return nil }
	if err := rig.release.BindNativeReleaseSeal(seal); err != nil {
		t.Fatalf("failed stamping consumed the only release binding: %v", err)
	}
}

func TestKernelNativeBirthGetterPanicRefusesBinding(t *testing.T) {
	rig := newNativeReleaseRig(t)
	want := errors.New("birth policy getter panicked")
	seal := &birthTestSeal{getterPanic: want}
	if err := rig.release.BindNativeReleaseSeal(seal); !errors.Is(err, want) {
		t.Fatalf("birth getter panic was not retained: %v", err)
	}
	if rig.duplicates != 0 || rig.release.NativeReleasePending() {
		t.Fatal("failed birth lookup published a native release binding")
	}
}

func TestKernelNativeBirthReceiptSurvivesPolicyAndRelease(t *testing.T) {
	rig := newNativeReleaseRig(t)
	receipt, ok := rig.handle.(interface{ NativeReleaseBirthTag() string })
	if !ok {
		t.Fatal("kernel owner cannot identify its exact native cleanup finding")
	}
	if receipt.NativeReleaseBirthTag() != "" {
		t.Fatal("an unbound owner invented a native birth receipt")
	}
	seal := &birthTestSeal{tag: "first-opaque-native-birth"}
	seal.cause = errors.New("remote cleanup is unconfirmed")
	if err := rig.release.BindNativeReleaseSeal(seal); err != nil {
		t.Fatal(err)
	}
	seal.tag = "changed-provider-value"
	if receipt.NativeReleaseBirthTag() != "first-opaque-native-birth" {
		t.Fatal("native cleanup identity reread a changed policy getter")
	}
	if err := rig.handle.Close(); err == nil {
		t.Fatal("unconfirmed native release reported success")
	}
	if receipt.NativeReleaseBirthTag() != "first-opaque-native-birth" {
		t.Fatal("pending cleanup lost its exact native birth receipt")
	}
	seal.cause = nil
	if err := rig.handle.Close(); err != nil {
		t.Fatal(err)
	}
	if receipt.NativeReleaseBirthTag() != "first-opaque-native-birth" {
		t.Fatal("confirmed cleanup cannot identify only its own warning")
	}
}
