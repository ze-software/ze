// Design: docs/architecture/mrt.md — ambiguous empty families and exact message boundaries.
package analyze

import (
	"bytes"
	"errors"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/mrt"
	mrtplugin "github.com/ze-software/ze/internal/plugins/mrt"
)

// The recorder legitimately selects AP from the empty IPv6 family. These five
// classic octets also decode as an invented Path ID followed by a default route.
func TestMRTEmptyMPFamilyRequiresContext(t *testing.T) {
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe8, 0x40, 3, 4, 192, 0, 2, 1, 0x80, 15, 3, 0, 2, 1}
	wire := framedUpdate(buildUpdate(nil, attrs, []byte{24, 10, 0, 0, 0}))
	ctx, err := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv6Unicast: true}))
	if err != nil { t.Fatal(err) }
	path := filepath.Join(t.TempDir(), "empty-family.mrt")
	recorder := mrtplugin.New(mrtplugin.Config{AllPath: path}, nil)
	recorder.Start(nil)
	recorder.OnBGPMessage(&plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.1"), LocalAddress: netip.MustParseAddr("192.0.2.2"), PeerAS: 65000, LocalAS: 65001, MessageContextID: ctx}, msgtype.TypeUPDATE, false, wire.Bytes)
	recorder.Stop()
	record, err := os.ReadFile(path)
	if err != nil { t.Fatal(err) }
	if err := mrt.ReadFrom(bytes.NewReader(record), &mrt.Handler{OnMessage: func(h mrt.Header, _ uint32, r *mrt.MessageRecord) error {
		if h.Subtype != mrt.BGP4MPMessageAS4AP || !bytes.Equal(wire.Bytes, r.BGPMessage.Bytes) { t.Fatalf("subtype=%d bytes=%x", h.Subtype, r.BGPMessage.Bytes) }
		_, err := mrt.ParseBGPMessage(r.BGPMessage)
		if !errors.Is(err, mrt.ErrContextUnavailable) { t.Errorf("invented classic interpretation: %v", err) }
		return nil
	}}); err != nil { t.Fatal(err) }
	assertMRTSemanticRefusal(t, record)
	withContext := append(commandOpenPrelude(9, 0, 3), record...)
	if err := mrt.ReadFrom(bytes.NewReader(withContext), &mrt.Handler{OnMessage: func(_ mrt.Header, _ uint32, r *mrt.MessageRecord) error {
		if r.BGPMessage.Bytes[18] != 2 { return nil }
		p, err := mrt.ParseBGPMessage(r.BGPMessage)
		if err != nil { return err }
		want := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24"), netip.MustParsePrefix("0.0.0.0/0")}
		if !slices.Equal(p.Update.AnnouncedPrefixes, want) || len(p.Update.AnnouncedPathIDs) != 0 { t.Fatalf("prefixes=%v IDs=%v", p.Update.AnnouncedPrefixes, p.Update.AnnouncedPathIDs) }
		return nil
	}}); err != nil { t.Fatal(err) }
	code, _, stderr := runSubcommandCapturing(t, withContext, func() int { return runDensity([]string{"-"}) })
	if code != 0 { t.Fatalf("contextual control: %d %s", code, stderr) }
}

func assertMRTSemanticRefusal(t *testing.T, record []byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "filtered.mrt")
	for _, command := range []struct { name string; run func() int }{
		{"show", func() int { return runShow([]string{"-"}) }},
		{"density", func() int { return runDensity([]string{"-"}) }},
		{"filter", func() int { return runFilter([]string{"--as-path", "65000", "-", path}) }},
		{"aspath", func() int { return runASPath([]string{"-"}) }},
		{"attributes", func() int { return runAttributes([]string{"-"}) }},
		{"communities", func() int { return runCommunities([]string{"-"}) }},
	} {
		t.Run(command.name, func(t *testing.T) {
			code, out, stderr := runSubcommandCapturing(t, record, command.run)
			if code == 0 { t.Fatalf("damaged record accepted: output=%q stderr=%q", out, stderr) }
		})
	}
}

func malformedMRTMessages() map[string][]byte {
	fields := []byte{0, 0, 0xfd, 0xe8, 0, 0, 0xfd, 0xe9, 0, 0, 0, 1, 192, 0, 2, 1, 192, 0, 2, 2}
	second := framedUpdate([]byte{0, 0, 0, 0, 0, 0, 0, 1, 24, 10, 0, 0}).Bytes
	trailing := append(framedUpdate([]byte{0, 0, 0, 0}).Bytes, second...)
	cases := map[string][]byte{"trailing-frame": mrtRecord(16, 9, append(bytes.Clone(fields), trailing...))}
	for name, wire := range map[string][]byte{
		"declared-overrun": framedUpdate([]byte{0, 0, 0, 0}).Bytes,
		"declared-short-update": framedUpdate([]byte{0, 0, 0}).Bytes,
	} {
		if name == "declared-overrun" { wire[17]++ }
		cases[name] = mrtRecord(16, 9, append(bytes.Clone(fields), wire...))
	}
	for name, attrs := range map[string][]byte{
		"short-header": {0x80},
		"short-length": {0x80, 15},
		"short-extended-length": {0x90, 15, 0},
		"overlong-mp": {0x80, 15, 13, 0, 2, 1, 0, 0, 0, 1, 32, 0x20, 1, 0x0d, 0xb8},
	} {
		record := mrtRecord(16, 9, append(bytes.Clone(fields), framedUpdate(buildUpdate(nil, attrs, nil)).Bytes...))
		cases[name] = append(commandOpenPrelude(9, 0, 3), record...)
	}
	return cases
}

func TestMRTMalformedMessageSemanticErrors(t *testing.T) {
	for name, record := range malformedMRTMessages() {
		t.Run(name, func(t *testing.T) { assertMRTSemanticRefusal(t, record) })
	}
}

func TestMRTMalformedMessageSocketRefusal(t *testing.T) {
	for name, record := range malformedMRTMessages() {
		t.Run(name+"/serve", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "malformed.mrt")
			if err := os.WriteFile(path, record, 0600); err != nil { t.Fatal(err) }
			server, client := net.Pipe()
			defer server.Close(); defer client.Close()
			_ = client.SetReadDeadline(time.Now().Add(5*time.Second))
			type result struct { n uint64; err error }
			done := make(chan result, 1)
			go func() { n, err := serveFile(server, path, 0, false); server.Close(); done <- result{n, err} }()
			var received bytes.Buffer
			_, _ = received.ReadFrom(client)
			got := <-done
			if got.err == nil || got.n != 0 || received.Len() != 0 { t.Fatalf("result=%+v transmitted=%x", got, received.Bytes()) }
		})
		for _, command := range []struct { name string; run func([]string) int }{{"inject", runInject}, {"replay", runReplay}} {
			t.Run(name+"/"+command.name, func(t *testing.T) {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil { t.Fatal(err) }
				defer listener.Close()
				type observation struct { updates int; err error }
				done := make(chan observation, 1)
				go func() {
					conn, err := listener.Accept()
					if err != nil { done <- observation{err: err}; return }
					defer conn.Close()
					_ = conn.SetDeadline(time.Now().Add(5*time.Second))
					if _, err = serveBGPOpen(conn, 65001, net.ParseIP("192.0.2.2"), 90); err != nil { done <- observation{err: err}; return }
					var seen observation
					for { kind, _, err := bgpReadMsg(conn); if err != nil { done <- seen; return }; if kind == 2 { seen.updates++ } }
				}()
				code, _, stderr := runSubcommandCapturing(t, record, func() int { return command.run([]string{"-", listener.Addr().String()}) })
				seen := <-done
				if seen.err != nil { t.Fatal(seen.err) }
				if code == 0 || seen.updates != 0 { t.Fatalf("code=%d updates=%d stderr=%q", code, seen.updates, stderr) }
			})
		}
	}
}

// An empty MP_UNREACH has no Path Identifier, even under an AP subtype.
func TestMRTStandaloneMPEORRemainsReplayable(t *testing.T) {
	body := []byte{0, 0, 0, 6, 0x80, 15, 3, 0, 2, 1}
	wire := framedUpdate(body)
	wire.AddPath = true
	if err := checkReplayUpdate(wire); err != nil { t.Fatal(err) }
	record := mrtRecord(16, 9, makeBGP4MPRecord(4, 65000, 1, body))
	path := filepath.Join(t.TempDir(), "eor.mrt")
	if err := os.WriteFile(path, record, 0600); err != nil { t.Fatal(err) }
	server, client := net.Pipe()
	defer server.Close(); defer client.Close()
	_ = client.SetReadDeadline(time.Now().Add(5*time.Second))
	type result struct { n uint64; err error }
	done := make(chan result, 1)
	go func() { n, err := serveFile(server, path, 0, false); server.Close(); done <- result{n, err} }()
	var received bytes.Buffer
	_, readErr := received.ReadFrom(client)
	got := <-done
	if got.err != nil || readErr != nil || got.n != 1 || !bytes.Equal(received.Bytes(), wire.Bytes) { t.Fatalf("result=%+v read=%v bytes=%x", got, readErr, received.Bytes()) }
}
