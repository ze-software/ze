// Design: docs/architecture/iface/logical-name-resolution.md -- held native access and backend generations
package iface

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// A missing birth MUST stay unknown at both native and JSON boundaries.
// Exercise the packed zero directly; no native creation can manufacture proof.
func TestNativeVLANBirthUnknownCannotEncode(t *testing.T) {
	var unknown NativeVLANBirth
	if _, err := unknown.Alias(); err == nil {
		t.Fatal("unknown birth invented an alias")
	}
	if _, err := json.Marshal(unknown); err == nil {
		t.Fatal("unknown birth encoded as a valid value")
	}
}

// A known birth MUST survive the value protocol without selector reconstruction.
// Marshal and parse the typed value, then compare its exact native alias.
func TestNativeVLANBirthWireRoundTrip(t *testing.T) {
	const alias = NativeVLANAliasPrefix + "00112233445566778899aabbccddeeff"
	birth, err := ParseNativeVLANBirth(alias)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(birth)
	if err != nil {
		t.Fatal(err)
	}
	var decoded NativeVLANBirth
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded != birth {
		t.Fatal("wire changed the packed birth")
	}
	result, err := decoded.Alias()
	if err != nil {
		t.Fatal(err)
	}
	if result != alias {
		t.Fatal("wire changed the original native alias")
	}
}

// Invalid peer input MUST NOT damage an already proven identity.
// Apply malformed aliases to one known receiver and check atomic parse failure.
func TestNativeVLANBirthParseFailurePreservesPriorValue(t *testing.T) {
	birth, err := ParseNativeVLANBirth(NativeVLANAliasPrefix + "00112233445566778899aabbccddeeff")
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"", NativeVLANAliasPrefix + "00000000000000000000000000000000", NativeVLANAliasPrefix + "00112233445566778899AABBCCDDEEFF", NativeVLANAliasPrefix + "0", NativeVLANRetiredAliasPrefix + "00112233445566778899aabbccddeeff"} {
		previous := birth
		if err := birth.UnmarshalText([]byte(invalid)); err == nil {
			t.Fatalf("accepted malformed birth %q", invalid)
		}
		if birth != previous {
			t.Fatal("failed parser damaged a known birth")
		}
	}
}

// Every admitted birth MUST have one canonical native representation.
// Fuzz the raw alias boundary and require an exact successful round trip.
func FuzzParseNativeVLANBirth(f *testing.F) {
	for _, seed := range []string{"", NativeVLANAliasPrefix + "00112233445566778899aabbccddeeff", NativeVLANAliasPrefix + "00000000000000000000000000000000", "other:00112233445566778899aabbccddeeff"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, alias string) {
		birth, err := ParseNativeVLANBirth(alias)
		if err != nil {
			return
		}
		encoded, err := birth.Alias()
		if err != nil {
			t.Fatal(err)
		}
		if encoded != alias {
			t.Fatal("accepted noncanonical native birth")
		}
	})
}

type nativeResolveLease struct {
	identity        NativeVLANIdentity
	closed          bool
	closes, borrows int
}

func (l *nativeResolveLease) Identity() (NativeVLANIdentity, error) { return l.identity, nil }
func (l *nativeResolveLease) Borrow(context.Context) (NativeVLANLease, error) {
	if l.closed {
		return nil, errors.New("original lease closed")
	}
	l.borrows++
	return &nativeResolveLease{identity: l.identity}, nil
}
func (*nativeResolveLease) Snapshot(context.Context) (NativeVLANMaps, error) {
	return NativeVLANMaps{}, errors.New("resolver fixture has no native maps")
}
func (*nativeResolveLease) Replace(context.Context, NativeVLANMaps) error {
	return errors.New("resolver fixture has no native writer")
}
func (l *nativeResolveLease) Close() error { l.closed = true; l.closes++; return nil }

type nativeResolveBackend struct {
	*exactResolveBackend
	capture    func(context.Context, Binding) (NativeVLANPreparation, error)
	closeCause error
	closes     int
}

func (b *nativeResolveBackend) CaptureNativeVLAN(ctx context.Context, binding Binding) (NativeVLANPreparation, error) {
	return b.capture(ctx, binding)
}
func (b *nativeResolveBackend) Close() error { b.closes++; return b.closeCause }

// Unowned discovery MUST remain useful without becoming native write authority.
// Resolve a read-only backend and require its explicit unsupported capability.
func TestPreparedAccessKeepsExplicitUnsupportedDiscovery(t *testing.T) {
	const name = "test-native-access-unowned"
	owner := &exactResolveBackend{link: func(string) (*InterfaceInfo, error) { return exactResolveInfo(), nil }}
	withExactResolveBackends(t, map[string]func() (Backend, error){name: func() (Backend, error) { return owner, nil }})
	if err := LoadBackend(name); err != nil {
		t.Fatal(err)
	}
	prepared, err := ResolvePreparedAccess(t.Context(), name, "actual0")
	if err != nil {
		t.Fatal(err)
	}
	binding, err := prepared.Binding()
	if err != nil {
		t.Fatal(err)
	}
	if binding.Ifindex != 7 {
		t.Fatal("ordinary discovery was lost")
	}
	if _, err := prepared.BorrowNativeVLAN(t.Context()); !errors.Is(err, ErrNativeAccessUnsupported) {
		t.Fatalf("unsupported access became authority: %v", err)
	}
	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}
}

// Borrowing MUST use the preparation's original owner and preserve its identity.
// Count the captured owner's borrows and release its preparation independently.
func TestPreparedAccessBorrowsOriginalNativeOwner(t *testing.T) {
	const name = "test-native-access-owner"
	birth, err := NewNativeVLANBirth()
	if err != nil {
		t.Fatal(err)
	}
	alias, err := birth.Alias()
	if err != nil {
		t.Fatal(err)
	}
	original := &nativeResolveLease{identity: NativeVLANIdentity{Device: "actual0", Index: 7, Birth: birth}}
	owner := &nativeResolveBackend{exactResolveBackend: &exactResolveBackend{link: func(string) (*InterfaceInfo, error) { info := exactResolveInfo(); info.Alias = alias; return info, nil }}}
	owner.capture = func(_ context.Context, binding Binding) (NativeVLANPreparation, error) {
		if binding.Alias != alias {
			t.Error("resolver dropped native birth")
		}
		return original, nil
	}
	withExactResolveBackends(t, map[string]func() (Backend, error){name: func() (Backend, error) { return owner, nil }})
	if err := LoadBackend(name); err != nil {
		t.Fatal(err)
	}
	prepared, err := ResolvePreparedAccess(t.Context(), name, "actual0")
	if err != nil {
		t.Fatal(err)
	}
	borrowed, err := prepared.BorrowNativeVLAN(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := borrowed.Close(); err != nil {
			t.Error(err)
		}
	})
	identity, err := borrowed.Identity()
	if err != nil {
		t.Fatal(err)
	}
	if identity != original.identity {
		t.Fatal("borrow changed original native identity")
	}
	if original.borrows != 1 {
		t.Fatal("prepared access resolved another owner")
	}
	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}
	if original.closes != 1 {
		t.Fatal("prepared access lost its creator lease")
	}
}

// An invalid replacement MUST NOT dispose of the selected native owner.
// Return a typed-nil factory result after selecting a concrete first backend.
func TestPreparedAccessRejectsTypedNilBackendReplacement(t *testing.T) {
	const name = "test-native-access-null-backend"
	original := &nativeResolveBackend{}
	calls := 0
	withExactResolveBackends(t, map[string]func() (Backend, error){name: func() (Backend, error) {
		calls++
		if calls == 1 {
			return original, nil
		}
		return (*nativeResolveBackend)(nil), nil
	}})
	if err := LoadBackend(name); err != nil {
		t.Fatal(err)
	}
	if err := LoadBackend(name); err == nil {
		t.Fatal("typed-nil replacement backend was accepted")
	}
	if GetBackend() != original {
		t.Fatal("typed-nil replacement displaced the original backend")
	}
	if original.closes != 0 {
		t.Fatal("invalid replacement closed the original backend")
	}
}

// An independent factory MUST also refuse a typed-nil native backend owner.
// Open that factory without selecting it and require an absent result and error.
func TestPreparedAccessRejectsTypedNilIndependentBackend(t *testing.T) {
	const name = "test-native-access-null-independent"
	withExactResolveBackends(t, map[string]func() (Backend, error){name: func() (Backend, error) { return (*nativeResolveBackend)(nil), nil }})
	owner, err := OpenBackend(name)
	if err == nil {
		t.Fatal("typed-nil independent backend was accepted")
	}
	if owner != nil {
		t.Fatal("invalid independent backend was published")
	}
}

// A typed-nil provider result MUST NOT publish a fictitious native capability.
// Resolve a concrete binding whose optional provider returns that invalid value.
func TestPreparedAccessRejectsTypedNilNativeLease(t *testing.T) {
	const name = "test-native-access-nil"
	owner := &nativeResolveBackend{exactResolveBackend: &exactResolveBackend{link: func(string) (*InterfaceInfo, error) { return exactResolveInfo(), nil }}}
	owner.capture = func(context.Context, Binding) (NativeVLANPreparation, error) { return (*nativeResolveLease)(nil), nil }
	withExactResolveBackends(t, map[string]func() (Backend, error){name: func() (Backend, error) { return owner, nil }})
	if err := LoadBackend(name); err != nil {
		t.Fatal(err)
	}
	prepared, err := ResolvePreparedAccess(t.Context(), name, "actual0")
	if err == nil {
		t.Fatal("typed-nil native owner was accepted")
	}
	if prepared != nil {
		t.Fatal("typed-nil native owner was published")
	}
}

// A failed native close MUST preserve the original selection and generation.
// Refuse replacement and shutdown, checking the unpublished replacement closes.
// MUTATION: discarding a backend after failed close loses its live native owner.
func TestPreparedAccessBackendCloseFailureKeepsOriginalSelection(t *testing.T) {
	const name = "test-native-access-close"
	cause := errors.New("original native resource remains in use")
	old := &nativeResolveBackend{closeCause: cause}
	next := &nativeResolveBackend{}
	calls := 0
	withExactResolveBackends(t, map[string]func() (Backend, error){name: func() (Backend, error) {
		calls++
		if calls == 1 {
			return old, nil
		}
		return next, nil
	}})
	if err := LoadBackend(name); err != nil {
		t.Fatal(err)
	}
	before := activeBackendGeneration
	if err := LoadBackend(name); !errors.Is(err, cause) {
		t.Fatalf("replacement close cause: %v", err)
	}
	if GetBackend() != old {
		t.Fatal("replacement displaced an unconfirmed native owner")
	}
	if activeBackendGeneration != before {
		t.Fatal("replacement advanced an unconfirmed native generation")
	}
	if next.closes != 1 {
		t.Fatal("failed replacement leaked its unpublished backend")
	}
	if err := CloseBackend(); !errors.Is(err, cause) {
		t.Fatalf("shutdown close cause: %v", err)
	}
	if GetBackend() != old {
		t.Fatal("shutdown forgot an unconfirmed native owner")
	}
	if activeBackendGeneration != before {
		t.Fatal("shutdown advanced an unconfirmed native generation")
	}
	old.closeCause = nil
}
