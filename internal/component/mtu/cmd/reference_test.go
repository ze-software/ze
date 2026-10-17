// Design: docs/architecture/diagnostics/path-mtu.md -- the reference address
//
// Goal: prove which address a default `show mtu` run measures as its
// reference. The reference-address leaf wins; with no leaf the first system
// name server of the running config is measured; with neither, the run is
// refused before a single probe is sent, and the refusal names both fixes. A
// host run needs no reference and is never refused for want of one. Method:
// each case drives the registered handler over the fake world, with the
// leaf set or cleared through the env layer and the name-server seam set on
// the fake, and reads which targets were probed.

package cmd

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/env"
)

// clearReferenceLeaf removes any reference-address the process environment
// carries, so the case sees only what it sets itself.
func clearReferenceLeaf(t *testing.T) {
	t.Helper()
	t.Setenv(envKeyReferenceAddress, "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

// setReferenceLeaf stands for `environment mtu reference-address <value>`,
// which the config package plumbs into the same key.
func setReferenceLeaf(t *testing.T, value string) {
	t.Helper()
	t.Setenv(envKeyReferenceAddress, value)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

// referenceHost answers the host of the payload's reference row.
func referenceHost(t *testing.T, doc map[string]any) string {
	t.Helper()
	row, ok := doc[fieldReference].(map[string]any)
	if !ok {
		t.Fatalf("the payload carries no reference row: %v", doc)
	}
	host, ok := row[fieldHost].(string)
	if !ok {
		t.Fatalf("the reference row carries no host: %v", row)
	}
	return host
}

// refusedShowMTU drives a default run and answers its error text, failing
// the test when the run was not refused or a probe was sent.
func refusedShowMTU(t *testing.T, f *fakeDeps) string {
	t.Helper()
	resp, err := registeredShowMTU(t)(nil, commandtest.Args())
	if err != nil {
		t.Fatalf("show mtu: %v", err)
	}
	if resp.Status != plugin.StatusError {
		t.Fatalf("show mtu ran: status %q; want a refusal", resp.Status)
	}
	if len(f.probers) != 0 {
		t.Errorf("a refused run opened %d probers; want none", len(f.probers))
	}
	return resp.Error
}

// TestReferenceAddressLeafWinsOverNameServer: the configured leaf is the
// reference even when a name server is configured too.
func TestReferenceAddressLeafWinsOverNameServer(t *testing.T) {
	leaf := netip.MustParseAddr("192.0.2.53")
	setReferenceLeaf(t, leaf.String())
	f := newFakeDeps()
	f.paths[leaf] = clampedAt(1500)
	f.paths[refV4] = clampedAt(1500)
	f.install(t)

	doc := showMTU(t)
	if got := referenceHost(t, doc); got != leaf.String() {
		t.Errorf("the reference is %s; want the leaf %s", got, leaf)
	}
}

// TestReferenceAddressDefaultsToFirstNameServer: with no leaf, the first
// system name-server is the reference, never a hard-coded public address.
func TestReferenceAddressDefaultsToFirstNameServer(t *testing.T) {
	clearReferenceLeaf(t)
	first := netip.MustParseAddr("203.0.113.53")
	second := netip.MustParseAddr("203.0.113.54")
	f := newFakeDeps()
	f.nameServers = []string{first.String(), second.String()}
	f.paths[first] = clampedAt(1500)
	f.paths[second] = clampedAt(1500)
	f.install(t)

	doc := showMTU(t)
	if got := referenceHost(t, doc); got != first.String() {
		t.Errorf("the reference is %s; want the first name server %s", got, first)
	}
}

// TestReferenceAddressRefusedWithNeither: no leaf and no name server refuses
// the default run before a probe, naming what is missing and both fixes.
func TestReferenceAddressRefusedWithNeither(t *testing.T) {
	clearReferenceLeaf(t)
	f := newFakeDeps()
	f.nameServers = nil
	f.paths[refV4] = clampedAt(1500)
	f.install(t)

	text := refusedShowMTU(t, f)
	for _, want := range []string{
		"no reference address",
		"set environment mtu reference-address <ip>",
		"set system name-server <ip>",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the refusal %q does not carry %q", text, want)
		}
	}
}

// TestReferenceAddressRefusedWhenNameServersUnpublished: a daemon that has
// not published its name servers is not read as one that configured none.
func TestReferenceAddressRefusedWhenNameServersUnpublished(t *testing.T) {
	clearReferenceLeaf(t)
	f := newFakeDeps()
	f.nameServersUnpublished = true
	f.paths[refV4] = clampedAt(1500)
	f.install(t)

	text := refusedShowMTU(t, f)
	if !strings.Contains(text, "were never published in this process") {
		t.Errorf("the refusal %q does not say the name servers are unknown", text)
	}
}

// TestReferenceAddressRefusesAMalformedLeaf: a leaf value that is not an
// address is refused by name rather than read as no reference, and the name
// server does not stand in for it.
func TestReferenceAddressRefusesAMalformedLeaf(t *testing.T) {
	setReferenceLeaf(t, "resolver.example")
	f := newFakeDeps()
	f.paths[refV4] = clampedAt(1500)
	f.install(t)

	text := refusedShowMTU(t, f)
	if !strings.Contains(text, `"resolver.example"`) {
		t.Errorf("the refusal %q does not name the malformed value", text)
	}
}

// TestHostRunNeedsNoReference: `show mtu host <address>` measures one host
// and is not refused when neither the leaf nor a name server is configured.
func TestHostRunNeedsNoReference(t *testing.T) {
	clearReferenceLeaf(t)
	f := newFakeDeps()
	f.nameServers = nil
	f.paths[peerA] = clampedAt(1400)
	f.install(t)

	doc := showMTU(t, argHost, peerA.String())
	if doc[fieldStatus] != runStatusOK.String() {
		t.Errorf("status %v, want ok", doc[fieldStatus])
	}
}
