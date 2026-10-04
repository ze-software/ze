package runner

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	leroot "github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/test/tmpfs"
)

// ciConfig is the shape of the ze configuration a .ci embeds, with one peer.
func ciConfig(peers ...string) string {
	return "bgp {\n" + strings.Join(peers, "\n") + "\n}\n"
}

// ciPeer writes one BGP peer block. Ze's own AS is 65000 in every case here,
// because what each case varies is the AS ze expects FROM the peer.
func ciPeer(name, dial, source, remoteAS string) string {
	const localAS = "65000"
	return "\tpeer " + name + " {\n" +
		"\t\tconnection {\n" +
		"\t\t\tremote {\n\t\t\t\tip " + dial + "\n\t\t\t}\n" +
		"\t\t\tlocal {\n\t\t\t\tip " + source + "\n\t\t\t}\n" +
		"\t\t}\n" +
		"\t\tsession {\n\t\t\tasn {\n\t\t\t\tlocal " + localAS + "\n\t\t\t\tremote " + remoteAS + "\n\t\t\t}\n\t\t}\n" +
		"\t}"
}

// recordWith builds a Record carrying one daemon configuration and one peer
// script, each named by the command that consumes it.
func recordWith(config, block string) *Record {
	return &Record{
		TmpfsFiles:  map[string]tmpfs.File{"ze-bgp.conf": {Path: "ze-bgp.conf", Content: []byte(config)}},
		StdinBlocks: map[string][]byte{"peer": []byte(block)},
		RunCommands: []RunCommand{
			{Seq: 1, Exec: "le test peer --port 1179", Stdin: "peer"},
			{Seq: 2, Exec: "ze start ze-bgp.conf"},
		},
	}
}

// TestDeclarePeerASFromOneConfiguredPeer is AC-2 for the ordinary shape.
//
// VALIDATES: a peer block that states no AS gets the AS ze is configured to
// expect from its peer, with no key, and the .ci is not edited.
// PREVENTS: an eBGP peer opening with ZE's AS, which ze answers with
// NOTIFICATION 2/2 Bad Peer AS.
func TestDeclarePeerASFromOneConfiguredPeer(t *testing.T) {
	record := recordWith(
		ciConfig(ciPeer("solo", "127.0.0.1", "127.0.0.1", "65001")),
		"expect=bgp:conn=1:seq=1:hex=FFFF001304\n")

	if err := declarePeerAS(record); err != nil {
		t.Fatalf("declarePeerAS: %v", err)
	}
	block := string(record.StdinBlocks["peer"])
	if !strings.HasPrefix(block, "option=asn:value=65001\n") {
		t.Errorf("block = %q, want it to open with the AS ze expects", block)
	}
}

// TestDeclarePeerASLeavesAStatedASAlone pins the override.
//
// VALIDATES: a block that states its own AS keeps it.
// PREVENTS: a .ci that deliberately opens with an AS ze refuses, to test the
// refusal, being corrected into one ze accepts.
func TestDeclarePeerASLeavesAStatedASAlone(t *testing.T) {
	record := recordWith(
		ciConfig(ciPeer("solo", "127.0.0.1", "127.0.0.1", "65001")),
		"option=asn:value=65099\nexpect=bgp:conn=1:seq=1:hex=FFFF001304\n")

	if err := declarePeerAS(record); err != nil {
		t.Fatalf("declarePeerAS: %v", err)
	}
	block := string(record.StdinBlocks["peer"])
	if strings.Contains(block, "65001") {
		t.Errorf("block = %q, want the stated AS untouched", block)
	}
}

// TestDeclarePeerASKeysEachSession is AC-2 for one ze-peer serving several of
// ze's peers.
//
// VALIDATES: several ASNs produce one keyed line per address ze dials.
// PREVENTS: one AS answering every connection of a conn_map batch, which is
// right for the first peer and wrong for the rest.
func TestDeclarePeerASKeysEachSession(t *testing.T) {
	record := recordWith(
		ciConfig(
			ciPeer("source-peer", "127.0.0.1", "127.0.0.1", "65001"),
			ciPeer("receiver-peer", "127.0.0.2", "127.0.0.2", "65002")),
		"option=conn_map:value=remote-ip\nexpect=bgp:conn=1:seq=1:hex=FFFF001304\n")

	if err := declarePeerAS(record); err != nil {
		t.Fatalf("declarePeerAS: %v", err)
	}
	block := string(record.StdinBlocks["peer"])
	for _, want := range []string{"option=asn:peer=127.0.0.1:value=65001", "option=asn:peer=127.0.0.2:value=65002"} {
		if !strings.Contains(block, want) {
			t.Errorf("block = %q, want it to carry %q", block, want)
		}
	}
}

// TestDeclarePeerASPrefersTheDialAddress pins the key precedence.
//
// VALIDATES: two peers sharing one source address still key on the distinct
// address ze dials each of them at.
// PREVENTS: the shared source address being read as an ambiguity and failing a
// file whose sessions are in fact distinguishable.
func TestDeclarePeerASPrefersTheDialAddress(t *testing.T) {
	record := recordWith(
		ciConfig(
			ciPeer("peer1", "127.0.0.1", "127.0.0.1", "65001"),
			ciPeer("peer2", "127.0.0.2", "127.0.0.1", "65002")),
		"expect=bgp:conn=1:seq=1:hex=FFFF001304\n")

	if err := declarePeerAS(record); err != nil {
		t.Fatalf("declarePeerAS: %v", err)
	}
	block := string(record.StdinBlocks["peer"])
	if !strings.Contains(block, "option=asn:peer=127.0.0.1:value=65001") {
		t.Errorf("block = %q, want the contested source address to keep the dial answer", block)
	}
	if !strings.Contains(block, "option=asn:peer=127.0.0.2:value=65002") {
		t.Errorf("block = %q, want the second peer keyed by the address ze dials it at", block)
	}

	// The same precedence where the source address is uncontested and still
	// contradicts a dial address: ze dials peer1 at 127.0.0.1 and speaks to
	// peer2 from it, so the address identifies peer1's session and nothing else.
	record = recordWith(
		ciConfig(
			"\tpeer peer1 {\n\t\tconnection {\n\t\t\tremote {\n\t\t\t\tip 127.0.0.1\n\t\t\t}\n\t\t}\n"+
				"\t\tsession {\n\t\t\tasn {\n\t\t\t\tlocal 65000\n\t\t\t\tremote 65001\n\t\t\t}\n\t\t}\n\t}",
			ciPeer("peer2", "127.0.0.2", "127.0.0.1", "65002")),
		"expect=bgp:conn=1:seq=1:hex=FFFF001304\n")

	if err := declarePeerAS(record); err != nil {
		t.Fatalf("declarePeerAS: %v", err)
	}
	block = string(record.StdinBlocks["peer"])
	if !strings.Contains(block, "option=asn:peer=127.0.0.1:value=65001") {
		t.Errorf("block = %q, want the dial address to keep its own answer", block)
	}
	if strings.Contains(block, "option=asn:peer=127.0.0.1:value=65002") {
		t.Errorf("block = %q, want the source address not to overwrite a dial address", block)
	}
}

// TestDeclarePeerASRefusesOneAddressWithTwoASNs pins the refusal.
//
// VALIDATES: two peers ze dials at ONE address, expecting different ASNs, fail
// the file with an error naming both.
// PREVENTS: the derivation picking one of the two in silence, which would send
// the wrong AS on one of the sessions and read as a harness bug.
func TestDeclarePeerASRefusesOneAddressWithTwoASNs(t *testing.T) {
	record := recordWith(
		ciConfig(
			ciPeer("peer1", "127.0.0.1", "127.0.0.1", "65001"),
			ciPeer("peer2", "127.0.0.1", "127.0.0.1", "65002")),
		"expect=bgp:conn=1:seq=1:hex=FFFF001304\n")

	err := declarePeerAS(record)
	if err == nil {
		t.Fatal("two ASNs at one address must fail the file")
	}
	for _, want := range []string{"65001", "65002", "127.0.0.1", "option=asn"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to name %q", err, want)
		}
	}
}

// TestDeclarePeerASIgnoresANonBGPPeer keeps the derivation to BGP.
//
// VALIDATES: a `peer` block with no AS, which is what an IPsec or WireGuard peer
// is, contributes nothing.
// PREVENTS: an ipsec test's peer names being read as BGP peers and producing a
// declaration from nothing.
func TestDeclarePeerASIgnoresANonBGPPeer(t *testing.T) {
	config := "ipsec {\n\tpeer branch {\n\t\tremote {\n\t\t\tip 10.0.0.1\n\t\t}\n\t}\n}\n"
	record := recordWith(config, "expect=bgp:conn=1:seq=1:hex=FFFF001304\n")

	if err := declarePeerAS(record); err != nil {
		t.Fatalf("declarePeerAS: %v", err)
	}
	if strings.Contains(string(record.StdinBlocks["peer"]), "option=asn") {
		t.Error("a peer with no AS declares none")
	}
}

// TestAsLeafAnswersThreeStates covers the AS reader's three answers.
//
// VALIDATES: a leaf that is not there is ABSENT; a leaf that IS there and does
// not read is an ERROR, never absent; a leaf that reads is a value.
// PREVENTS: a value the reader could not read coming back looking like a value
// nobody wrote, which is what lets a peer leave the population the coverage
// guard walks.
func TestAsLeafAnswersThreeStates(t *testing.T) {
	read := func(t *testing.T, body string) configFile {
		t.Helper()
		config, err := readConfig(body)
		if err != nil {
			t.Fatalf("readConfig(%q): %v", body, err)
		}
		return config.topLevel("session").topLevel("asn")
	}

	as, ok, err := asLeaf(read(t, "session {\n\tasn {\n\t\tlocal 65000\n\t\tremote 65001\n\t}\n}"), "remote")
	if err != nil || !ok || as != 65001 {
		t.Errorf("asn block = %d, %v, %v; want 65001, true, nil", as, ok, err)
	}
	if _, ok, err = asLeaf(read(t, "session {\n\tasn {\n\t\tlocal 65000\n\t}\n}"), "remote"); ok || err != nil {
		t.Errorf("no remote AS = %v, %v; want absent with no error", ok, err)
	}
	// RFC 7607 Section 2 reserves AS 0, and ze refuses a peer that opens with it.
	// Deriving it would turn a configuration defect into a harness one, and
	// reporting it ABSENT would exempt the peer from the coverage guard.
	if _, ok, err = asLeaf(read(t, "session {\n\tasn {\n\t\tremote 0\n\t}\n}"), "remote"); ok || err == nil {
		t.Errorf("AS 0 = %v, %v; want an error, never absent", ok, err)
	}
	if _, ok, err = asLeaf(read(t, "session {\n\tasn {\n\t\tremote sixty-five-thousand\n\t}\n}"), "remote"); ok || err == nil {
		t.Errorf("an unreadable AS = %v, %v; want an error, never absent", ok, err)
	}
	// The daemon's config parser reads the asdot spellings RFC 5396 Section 2
	// defines, so this reader does too. A harness that read fewer spellings
	// would refuse a .ci whose configuration the daemon accepts.
	if as, ok, err = asLeaf(read(t, "session {\n\tasn {\n\t\tremote 1.10\n\t}\n}"), "remote"); err != nil || !ok || as != 65546 {
		t.Errorf("asdot remote AS = %d, %v, %v; want 65546, true, nil", as, ok, err)
	}
}

// TestPeerLocalASReadsTheRouterLevelDeclaration is the classifier's own reader.
//
// VALIDATES: `bgp { session { asn { local N } } }` reaches every peer below it,
// through routerDefaults and the inheritance chain, whichever order the router's
// session and the peers appear in.
// PREVENTS: the coverage guard's classifier being the same blind reader it
// guards. `localKnown` decides whether a peer is eBGP, and a peer whose local AS
// is declared only at the router level came back `localKnown=false`, which
// exempted it from the very refusal it needed. test/plugin/bgp-local-as-options.ci
// writes the AS exactly there and states the rule in its own comment.
func TestPeerLocalASReadsTheRouterLevelDeclaration(t *testing.T) {
	peerBlock := "\tpeer edge {\n\t\tsession {\n\t\t\tasn {\n\t\t\t\tremote 65001\n\t\t\t}\n\t\t}\n\t}\n"
	routerBlock := "\tsession {\n\t\tasn {\n\t\t\tlocal 65000\n\t\t}\n\t}\n"

	for _, order := range []struct {
		name string
		text string
	}{
		{"router session first", "bgp {\n" + routerBlock + peerBlock + "}\n"},
		{"peer first", "bgp {\n" + peerBlock + routerBlock + "}\n"},
	} {
		t.Run(order.name, func(t *testing.T) {
			peers, err := peersFromConfig(order.text)
			if err != nil {
				t.Fatalf("peersFromConfig: %v", err)
			}
			if len(peers) != 1 {
				t.Fatalf("read %d peers, want 1", len(peers))
			}
			if !peers[0].localKnown || peers[0].localAS != 65000 {
				t.Errorf("local AS = %d, known=%v; want 65000 inherited from the router",
					peers[0].localAS, peers[0].localKnown)
			}
			if peers[0].as != 65001 {
				t.Errorf("remote AS = %d, want 65001", peers[0].as)
			}
		})
	}
}

// TestDeclarePeerASReadsAGroupsDeclaration covers inheritance.
//
// VALIDATES: a peer nested in a group or a template, declaring no AS of its own,
// gets the one its container declares.
// PREVENTS: the reader seeing only `peer` blocks. Three .ci files in test/encode
// declare the AS on a group and nothing on the peer; they are iBGP today, so the
// blindness was invisible, and the first eBGP group-scoped file would have opened
// with ze's AS.
func TestDeclarePeerASReadsAGroupsDeclaration(t *testing.T) {
	own := "\t\tsession {\n\t\t\tasn {\n\t\t\t\tlocal 65000\n\t\t\t\tremote 65001\n\t\t\t}\n\t\t}\n"
	nested := "\t\tpeer peer1 {\n\t\t\tconnection {\n\t\t\t\tremote {\n\t\t\t\t\tip 127.0.0.1\n\t\t\t\t}\n\t\t\t}\n\t\t}\n"

	for _, keyword := range []string{"group", "template"} {
		// Both write orders, because the container's own session and the peers
		// inside it sit at the same depth. An unscoped read took whichever came
		// first, so a container whose session was written BELOW a nested peer
		// took that peer's declarations as its default.
		for _, order := range []struct {
			name string
			body string
		}{
			{"own session first", own + nested},
			{"nested peer first", nested + own},
		} {
			t.Run(keyword+", "+order.name, func(t *testing.T) {
				config := "bgp {\n\t" + keyword + " edge {\n" + order.body + "\t}\n}\n"
				record := recordWith(config, "expect=bgp:conn=1:seq=1:hex=FFFF001304\n")

				if err := declarePeerAS(record); err != nil {
					t.Fatalf("declarePeerAS: %v", err)
				}
				if !strings.Contains(string(record.StdinBlocks["peer"]), "option=asn:value=65001") {
					t.Errorf("block = %q, want the AS the %s declares", record.StdinBlocks["peer"], keyword)
				}
			})
		}
	}
}

// TestDeclarePeerASReadsAConfigOnDisk covers option=file:path=.
//
// VALIDATES: the configuration the runner resolved into Record.ConfigFile is read
// like an embedded one.
// PREVENTS: the derivation answering "no peer AS is configured" for a file that
// configures one through a separate .conf, which is a reader failure wearing the
// shape of an answer. No .ci exercises it today: every file with option=file
// either states its own option=asn or has no peer block, so this test is the
// only thing holding the path.
func TestDeclarePeerASReadsAConfigOnDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ze-bgp.conf")
	config := ciConfig(ciPeer("solo", "127.0.0.1", "127.0.0.1", "65001"))
	if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	record := &Record{
		ConfigFile:  path,
		StdinBlocks: map[string][]byte{"peer": []byte("expect=bgp:conn=1:seq=1:hex=FFFF001304\n")},
		RunCommands: []RunCommand{{Seq: 1, Exec: "le test peer --port 1179", Stdin: "peer"}},
	}
	if err := declarePeerAS(record); err != nil {
		t.Fatalf("declarePeerAS: %v", err)
	}
	if !strings.Contains(string(record.StdinBlocks["peer"]), "option=asn:value=65001") {
		t.Errorf("block = %q, want the AS the on-disk config declares", record.StdinBlocks["peer"])
	}
}

// TestDeclarePeerASRefusesAnEBGPPeerItCannotReach is the fail-closed guard.
//
// VALIDATES: an eBGP peer the derivation produced no line for fails the file,
// naming the file and the peer.
// PREVENTS: the guard this whole file exists to be, failing open one layer up.
// A reader that finds nothing and a configuration that declares nothing are the
// same empty result, and the peer then opens with ZE's AS in silence.
func TestDeclarePeerASRefusesAnEBGPPeerItCannotReach(t *testing.T) {
	// Two ASNs, so no single line answers both, and no connection block, so
	// there is no address to key either of them on.
	config := "bgp {\n" +
		"\tpeer peer1 {\n\t\tsession {\n\t\t\tasn {\n\t\t\t\tlocal 65000\n\t\t\t\tremote 65001\n\t\t\t}\n\t\t}\n\t}\n" +
		"\tpeer peer2 {\n\t\tsession {\n\t\t\tasn {\n\t\t\t\tlocal 65000\n\t\t\t\tremote 65002\n\t\t\t}\n\t\t}\n\t}\n}\n"
	record := recordWith(config, "expect=bgp:conn=1:seq=1:hex=FFFF001304\n")
	record.CIFile = "test/plugin/example.ci"

	err := declarePeerAS(record)
	if err == nil {
		t.Fatal("an eBGP peer the derivation cannot reach must fail the file")
	}
	for _, want := range []string{"test/plugin/example.ci", "peer1", "65000", "65001", "Bad Peer AS"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to name %q", err, want)
		}
	}

	// The SAME configuration with the local AS written at the router level, which
	// is where test/plugin/bgp-local-as-options.ci writes it. It has to refuse
	// identically: the guard reads `localKnown` to decide whether a peer is eBGP,
	// so a spelling its reader missed exempted the peer from its own refusal.
	routerLevel := "bgp {\n" +
		"\tsession {\n\t\tasn {\n\t\t\tlocal 65000\n\t\t}\n\t}\n" +
		"\tpeer peer1 {\n\t\tsession {\n\t\t\tasn {\n\t\t\t\tremote 65001\n\t\t\t}\n\t\t}\n\t}\n" +
		"\tpeer peer2 {\n\t\tsession {\n\t\t\tasn {\n\t\t\t\tremote 65002\n\t\t\t}\n\t\t}\n\t}\n}\n"
	record = recordWith(routerLevel, "expect=bgp:conn=1:seq=1:hex=FFFF001304\n")
	record.CIFile = "test/plugin/router-level.ci"

	err = declarePeerAS(record)
	if err == nil {
		t.Fatal("a router-level local AS must classify the peers as eBGP and refuse just the same")
	}
	for _, want := range []string{"test/plugin/router-level.ci", "peer1", "65000", "65001"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to name %q", err, want)
		}
	}
}

// TestDeclarePeerASRefusesAPeerItCannotRead is the other half of failing closed.
//
// VALIDATES: a peer whose declared AS does not read fails the file, naming the
// peer, rather than leaving the population the coverage guard walks.
// PREVENTS: the narrow hole beside the guard. `peersFromConfig` used to drop any
// peer whose remote AS it could not parse, so the guard then walked a shorter
// list and reported every peer on it covered. A reader that drops what it cannot
// read is the defect class this whole file exists to close.
func TestDeclarePeerASRefusesAPeerItCannotRead(t *testing.T) {
	for _, unreadable := range []struct {
		name  string
		leafs string
		names string
	}{
		{"remote is not a number", "\t\t\t\tlocal 65000\n\t\t\t\tremote sixty-five-thousand\n", "sixty-five-thousand"},
		{"remote is AS 0", "\t\t\t\tlocal 65000\n\t\t\t\tremote 0\n", "RFC 7607"},
		{"local is not a number", "\t\t\t\tlocal nought\n\t\t\t\tremote 65001\n", "nought"},
	} {
		t.Run(unreadable.name, func(t *testing.T) {
			config := "bgp {\n\tpeer broken {\n\t\tsession {\n\t\t\tasn {\n" +
				unreadable.leafs + "\t\t\t}\n\t\t}\n\t}\n}\n"
			record := recordWith(config, "expect=bgp:conn=1:seq=1:hex=FFFF001304\n")
			record.CIFile = "test/plugin/unreadable.ci"

			err := declarePeerAS(record)
			if err == nil {
				t.Fatal("a peer the reader cannot read must fail the file, never be dropped")
			}
			for _, want := range []string{"test/plugin/unreadable.ci", "broken", unreadable.names} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error = %q, want it to name %q", err, want)
				}
			}
		})
	}
}

// TestDeclarePeerASLeavesAniBGPPeerUnreached pins the guard's scope.
//
// VALIDATES: an iBGP peer the derivation produced no line for is NOT refused,
// because the mirror answers it with ze's own AS, which is the AS ze expects.
// PREVENTS: the guard failing every file whose peers do not all carry an address,
// which would refuse working tests to close a hole they never had.
func TestDeclarePeerASLeavesAniBGPPeerUnreached(t *testing.T) {
	config := "bgp {\n" +
		"\tpeer inside {\n\t\tsession {\n\t\t\tasn {\n\t\t\t\tlocal 65000\n\t\t\t\tremote 65000\n\t\t\t}\n\t\t}\n\t}\n" +
		"\tpeer outside {\n\t\tconnection {\n\t\t\tremote {\n\t\t\t\tip 127.0.0.2\n\t\t\t}\n\t\t}\n" +
		"\t\tsession {\n\t\t\tasn {\n\t\t\t\tlocal 65000\n\t\t\t\tremote 65001\n\t\t\t}\n\t\t}\n\t}\n}\n"
	record := recordWith(config, "expect=bgp:conn=1:seq=1:hex=FFFF001304\n")

	if err := declarePeerAS(record); err != nil {
		t.Fatalf("declarePeerAS: %v", err)
	}
	block := string(record.StdinBlocks["peer"])
	if !strings.Contains(block, "option=asn:peer=127.0.0.2:value=65001") {
		t.Errorf("block = %q, want the eBGP peer keyed by its address", block)
	}
	if strings.Contains(block, "value=65000") {
		t.Errorf("block = %q, want no line invented for the iBGP peer", block)
	}
}

// TestDeclarePeerASClassifiesConfigConsumers derives the real peer's AS while
// sidecars contain both invalid config syntax and valid, conflicting BGP text.
// PREVENTS: treating a completion marker, script or unused block as configuration.
func TestDeclarePeerASClassifiesConfigConsumers(t *testing.T) {
	record := recordWith(
		ciConfig(ciPeer("real", "127.0.0.1", "127.0.0.1", "65001")),
		"expect=bgp:conn=1:seq=1:hex=FFFF001304\n"+
			"action=rewrite:conn=1:seq=2:source=fence.src:dest=fence.received\n")
	record.TmpfsFiles["fence.src"] = tmpfs.File{
		Path: "fence.src", Content: []byte("peer2 received the rejected route's withdrawal and the permitted fence\n"),
	}
	decoy := ciConfig(ciPeer("decoy", "127.0.0.1", "127.0.0.1", "65099"))
	// Neither suffix nor valid configuration syntax establishes consumption.
	record.TmpfsFiles["unused.conf"] = tmpfs.File{Path: "unused.conf", Content: []byte(decoy)}
	record.StdinBlocks["unused-config"] = []byte(decoy)
	record.StdinBlocks["unused-script"] = []byte(
		"action=rewrite:source=unused.conf:dest=ze-bgp.conf\nunmatched ' quote\n")

	if err := declarePeerAS(record); err != nil {
		t.Fatalf("declarePeerAS with raw sidecars: %v", err)
	}
	if got := declaredASLines(record); !slices.Equal(got, []string{"option=asn:value=65001"}) {
		t.Fatalf("derived declarations = %v, want only the consumed configuration's AS 65001", got)
	}
}

// TestDeclarePeerASConfigurationCarriers proves that selection follows each
// supported daemon input, including extension-independent file and reload paths.
func TestDeclarePeerASConfigurationCarriers(t *testing.T) {
	leroot.RegisterForwarding("test peer-asn-config-probe")
	for _, carrier := range []string{"daemon file", "daemon stdin", "wrapped stdin", "option file tmpfs", "reload source"} {
		t.Run(carrier, func(t *testing.T) {
			config := ciConfig(ciPeer("real", "127.0.0.1", "127.0.0.1", "65001"))
			record := recordWith(config, "expect=bgp:conn=1:seq=1:hex=FFFF001304\n")
			switch carrier {
			case "daemon file":
				record.RunCommands[1].Exec = "ze start ./configuration.src"
				delete(record.TmpfsFiles, "ze-bgp.conf")
				record.TmpfsFiles["configuration.src"] = tmpfs.File{Path: "configuration.src", Content: []byte(config)}
			case "daemon stdin":
				record.RunCommands[1].Exec = "ze -"
				record.RunCommands[1].Stdin = "ze-bgp"
				record.StdinBlocks["ze-bgp"] = []byte(config)
				delete(record.TmpfsFiles, "ze-bgp.conf")
			case "wrapped stdin":
				record.RunCommands[1].Exec = "le test peer-asn-config-probe -- ze -"
				record.RunCommands[1].Stdin = "ze-bgp"
				record.StdinBlocks["ze-bgp"] = []byte(config)
				delete(record.TmpfsFiles, "ze-bgp.conf")
			case "option file tmpfs":
				record.RunCommands = record.RunCommands[:1]
				record.ConfigFile = filepath.Join(t.TempDir(), "configuration.src")
				record.TmpfsFiles["configuration.src"] = tmpfs.File{Path: "configuration.src", Content: []byte(config)}
				delete(record.TmpfsFiles, "ze-bgp.conf")
			case "reload source":
				record.TmpfsFiles["ze-bgp.conf"] = tmpfs.File{Path: "ze-bgp.conf", Content: []byte("bgp {}\n")}
				record.TmpfsFiles["reload.src"] = tmpfs.File{Path: "reload.src", Content: []byte(config)}
				record.StdinBlocks["peer"] = append(record.StdinBlocks["peer"],
					[]byte("action=rewrite:conn=1:seq=2:source=reload.src:dest=ze-bgp.conf\n")...)
			}
			if err := declarePeerAS(record); err != nil {
				t.Fatalf("declarePeerAS: %v", err)
			}
			if got := declaredASLines(record); !slices.Equal(got, []string{"option=asn:value=65001"}) {
				t.Fatalf("derived declarations = %v, want consumed AS 65001", got)
			}
		})
	}
}

// TestDeclarePeerASRefusesMalformedConfigInputs keeps fail-closed parsing on
// real inputs, including the transitive ancestors of a reload destination.
func TestDeclarePeerASRefusesMalformedConfigInputs(t *testing.T) {
	for _, carrier := range []string{"daemon file", "daemon stdin", "option file", "reload source", "transitive reload source"} {
		t.Run(carrier, func(t *testing.T) {
			config := ciConfig(ciPeer("real", "127.0.0.1", "127.0.0.1", "65001"))
			record := recordWith(config, "expect=bgp:conn=1:seq=1:hex=FFFF001304\n")
			const malformed = "bgp { description 'unterminated\n"
			wantPath := "ze-bgp.conf"
			switch carrier {
			case "daemon file":
				record.TmpfsFiles["ze-bgp.conf"] = tmpfs.File{Path: "ze-bgp.conf", Content: []byte(malformed)}
			case "daemon stdin":
				record.RunCommands[1].Exec = "ze -"
				record.RunCommands[1].Stdin = "ze-bgp"
				record.StdinBlocks["ze-bgp"] = []byte(malformed)
				delete(record.TmpfsFiles, "ze-bgp.conf")
			case "option file":
				record.RunCommands = record.RunCommands[:1]
				record.ConfigFile = filepath.Join(t.TempDir(), "ze-bgp.conf")
				delete(record.TmpfsFiles, "ze-bgp.conf")
				if err := os.WriteFile(record.ConfigFile, []byte(malformed), 0o600); err != nil {
					t.Fatal(err)
				}
			case "reload source":
				wantPath = "reload.src"
				record.TmpfsFiles[wantPath] = tmpfs.File{Path: wantPath, Content: []byte(malformed)}
				record.StdinBlocks["peer"] = append(record.StdinBlocks["peer"],
					[]byte("action=rewrite:conn=1:seq=2:source=reload.src:dest=ze-bgp.conf\n")...)
			case "transitive reload source":
				wantPath = "ancestor.src"
				record.TmpfsFiles["reload.src"] = tmpfs.File{Path: "reload.src", Content: []byte(config)}
				record.TmpfsFiles[wantPath] = tmpfs.File{Path: wantPath, Content: []byte(malformed)}
				record.StdinBlocks["peer"] = append(record.StdinBlocks["peer"], []byte(
					"action=rewrite:conn=1:seq=2:source=ancestor.src:dest=reload.src\n"+
						"action=rewrite:conn=1:seq=3:source=reload.src:dest=ze-bgp.conf\n")...)
			}
			err := declarePeerAS(record)
			if !errors.Is(err, errASDerivation) {
				t.Fatalf("error = %v, want an AS derivation refusal", err)
			}
			for _, want := range []string{wantPath, "unterminated"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error = %v, want it to identify %q", err, want)
				}
			}
		})
	}
}

// TestPathASNExportFenceParsesWithDerivedAS exercises the real .ci consumer
// that failed discovery when its raw receiver acknowledgment contained an apostrophe.
func TestPathASNExportFenceParsesWithDerivedAS(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	tests := NewEncodingTests(root)
	record, err := tests.parseAndAdd(filepath.Join(root, "test", "plugin", "path-asn-filter-export-reject.ci"))
	if err != nil {
		t.Fatalf("parse path-asn-filter-export-reject.ci: %v", err)
	}
	for _, block := range []string{"peer", "peer2"} {
		payload, exists := record.StdinBlocks[block]
		if !exists {
			t.Fatalf("missing peer stdin block %q", block)
		}
		for _, want := range []string{
			"option=asn:peer=127.0.0.1:value=65001",
			"option=asn:peer=127.0.0.2:value=65002",
		} {
			if !strings.Contains(string(payload), want) {
				t.Errorf("%s lacks derived declaration %q", block, want)
			}
		}
	}
}

// TestDeclarePeerASTransitiveReloadSources requires AS declarations from the
// entire rewrite ancestry. A cycle must not duplicate or stall that traversal.
func TestDeclarePeerASTransitiveReloadSources(t *testing.T) {
	record := recordWith(
		ciConfig(ciPeer("first", "127.0.0.1", "127.0.0.1", "65001")),
		"expect=bgp:conn=1:seq=1:hex=FFFF001304\n"+
			"action=rewrite:conn=1:seq=2:source=ancestor.src:dest=middle.src\n"+
			"action=rewrite:conn=1:seq=3:source=middle.src:dest=ze-bgp.conf\n"+
			"action=rewrite:conn=1:seq=4:source=ze-bgp.conf:dest=middle.src\n")
	// The first action creates middle.src; no initial file exists for it.
	record.CIFile = filepath.Join(t.TempDir(), "fixture.ci")
	record.TmpfsFiles["ancestor.src"] = tmpfs.File{
		Path:    "ancestor.src",
		Content: []byte(ciConfig(ciPeer("second", "127.0.0.2", "127.0.0.2", "65002"))),
	}
	if err := declarePeerAS(record); err != nil {
		t.Fatalf("declarePeerAS: %v", err)
	}
	want := []string{
		"option=asn:peer=127.0.0.1:value=65001",
		"option=asn:peer=127.0.0.2:value=65002",
	}
	if got := declaredASLines(record); !slices.Equal(got, want) {
		t.Fatalf("derived declarations = %v, want both consumed AS declarations %v", got, want)
	}
}

// TestDeclarePeerASDaemonInputsFollowExecutionOrder checks the same per-daemon
// filename assignment that a rewrite uses, even when commands appear out of order.
func TestDeclarePeerASDaemonInputsFollowExecutionOrder(t *testing.T) {
	config := ciConfig(ciPeer("real", "127.0.0.1", "127.0.0.1", "65001"))
	record := recordWith(config, "expect=bgp:conn=1:seq=1:hex=FFFF001304\n"+
		"action=rewrite:conn=1:seq=2:source=malformed.src:dest=daemon-2/ze-later.conf\n")
	delete(record.TmpfsFiles, "ze-bgp.conf")
	record.StdinBlocks["earlier"] = []byte(config)
	record.StdinBlocks["later"] = []byte(config)
	record.RunCommands[1] = RunCommand{Seq: 3, Exec: "ze -", Stdin: "later"}
	record.RunCommands = append(record.RunCommands, RunCommand{Seq: 2, Exec: "ze -", Stdin: "earlier"})
	record.TmpfsFiles["malformed.src"] = tmpfs.File{
		Path: "malformed.src", Content: []byte("bgp { description 'unterminated\n"),
	}
	err := declarePeerAS(record)
	if !errors.Is(err, errASDerivation) {
		t.Fatalf("error = %v, want refusal of the second daemon's malformed reload", err)
	}
	if !strings.Contains(err.Error(), "malformed.src") {
		t.Fatalf("error = %v, want the selected reload input's name", err)
	}
}

// TestDeclarePeerASExplicitStartFilename exercises the consumer, not just argv
// recognition: bare filenames must contribute AS declarations or fail parsing.
func TestDeclarePeerASExplicitStartFilename(t *testing.T) {
	for _, path := range []string{"routing", "routing.src"} {
		for _, input := range []struct {
			name      string
			malformed bool
		}{
			{name: "valid"},
			{name: "malformed", malformed: true},
		} {
			t.Run(path+"/"+input.name, func(t *testing.T) {
				config := ciConfig(ciPeer("real", "127.0.0.1", "127.0.0.1", "65001"))
				if input.malformed {
					config = "bgp { description 'unterminated\n"
				}
				record := recordWith(config, "expect=bgp:conn=1:seq=1:hex=FFFF001304\n")
				delete(record.TmpfsFiles, "ze-bgp.conf")
				record.RunCommands[1].Exec = "ze start " + path
				record.TmpfsFiles[path] = tmpfs.File{Path: path, Content: []byte(config)}
				err := declarePeerAS(record)
				if input.malformed {
					if !errors.Is(err, errASDerivation) {
						t.Fatalf("error = %v, want refusal of malformed configuration %s", err, path)
					}
					if !strings.Contains(err.Error(), path) {
						t.Fatalf("error = %v, want the selected configuration's name %s", err, path)
					}
					return
				}
				if err != nil {
					t.Fatalf("declarePeerAS: %v", err)
				}
				if got := declaredASLines(record); !slices.Equal(got, []string{"option=asn:value=65001"}) {
					t.Fatalf("derived declarations = %v, want consumed AS 65001", got)
				}
			})
		}
	}
}

// TestDeclarePeerASReadsDaemonFileOnDisk resolves a directly named daemon input
// beside the .ci, rather than assuming every daemon input is embedded.
func TestDeclarePeerASReadsDaemonFileOnDisk(t *testing.T) {
	config := ciConfig(ciPeer("real", "127.0.0.1", "127.0.0.1", "65001"))
	record := recordWith(config, "expect=bgp:conn=1:seq=1:hex=FFFF001304\n")
	record.CIFile = filepath.Join(t.TempDir(), "fixture.ci")
	record.RunCommands[1].Exec = "ze start routing"
	delete(record.TmpfsFiles, "ze-bgp.conf")
	if err := os.WriteFile(filepath.Join(filepath.Dir(record.CIFile), "routing"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := declarePeerAS(record); err != nil {
		t.Fatalf("declarePeerAS: %v", err)
	}
	if got := declaredASLines(record); !slices.Equal(got, []string{"option=asn:value=65001"}) {
		t.Fatalf("derived declarations = %v, want on-disk AS 65001", got)
	}
}

// TestDeclarePeerASStartFlagsSelectOnlyConfig proves that value-less start
// flags neither become filenames nor invent a configuration for stored-mode starts.
func TestDeclarePeerASStartFlagsSelectOnlyConfig(t *testing.T) {
	for _, tt := range []struct {
		name string
		exec string
		want []string
	}{
		{name: "cli", exec: "ze start --cli routing", want: []string{"option=asn:value=65001"}},
		{name: "web-only config", exec: "ze start --web-only --web 3443 routing", want: []string{"option=asn:value=65001"}},
		{name: "web-only without config", exec: "ze start --web-only --web 3443"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			config := ciConfig(ciPeer("real", "127.0.0.1", "127.0.0.1", "65001"))
			record := recordWith(config, "expect=bgp:conn=1:seq=1:hex=FFFF001304\n")
			delete(record.TmpfsFiles, "ze-bgp.conf")
			record.TmpfsFiles["routing"] = tmpfs.File{Path: "routing", Content: []byte(config)}
			record.RunCommands[1].Exec = tt.exec
			if err := declarePeerAS(record); err != nil {
				t.Fatalf("declarePeerAS: %v", err)
			}
			if got := declaredASLines(record); !slices.Equal(got, tt.want) {
				t.Fatalf("derived declarations = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDeclarePeerASRefusesUnavailableRewriteSources distinguishes generated
// intermediates from missing leaves and cycles with no readable initial source.
func TestDeclarePeerASRefusesUnavailableRewriteSources(t *testing.T) {
	for _, tt := range []struct {
		name    string
		actions string
		want    string
	}{
		{
			name: "missing leaf",
			actions: "action=rewrite:conn=1:seq=2:source=missing.src:dest=middle.src\n" +
				"action=rewrite:conn=1:seq=3:source=middle.src:dest=ze-bgp.conf\n",
			want: "missing.src",
		},
		{
			name: "unanchored cycle",
			actions: "action=rewrite:conn=1:seq=2:source=loop.src:dest=middle.src\n" +
				"action=rewrite:conn=1:seq=3:source=middle.src:dest=loop.src\n" +
				"action=rewrite:conn=1:seq=4:source=middle.src:dest=ze-bgp.conf\n",
			want: "no readable rewrite source",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			record := recordWith(
				ciConfig(ciPeer("real", "127.0.0.1", "127.0.0.1", "65001")),
				"expect=bgp:conn=1:seq=1:hex=FFFF001304\n"+tt.actions)
			record.CIFile = filepath.Join(t.TempDir(), "fixture.ci")
			err := declarePeerAS(record)
			if !errors.Is(err, errASDerivation) {
				t.Fatalf("error = %v, want an AS derivation refusal", err)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}
