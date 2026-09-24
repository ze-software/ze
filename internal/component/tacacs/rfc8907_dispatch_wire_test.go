// Design: docs/architecture/aaa-tacacs.md -- trusted dispatch identities and wire display.
package tacacs

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/redact"
)

type dispatchWireRecord struct {
	kind  uint8
	flags uint8
	user  string
	args  []string
}

// dispatchWireServer records decrypted packets, not bridge arguments. Each
// request gets its own TCP connection, so accounting and authorization can run
// concurrently without sharing a synthetic response or bypassing the codecs.
func dispatchWireServer(t *testing.T, authorStatus uint8) (string, <-chan dispatchWireRecord) {
	t.Helper()
	listener := listenTCP(t)
	records := make(chan dispatchWireRecord, 64)
	var connections sync.WaitGroup
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			connections.Add(1)
			go func() {
				defer connections.Done()
				defer closeIgnore(conn)
				if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
					t.Error(err)
					return
				}
				var header [hdrLen]byte
				if _, err := io.ReadFull(conn, header[:]); err != nil {
					t.Error(err)
					return
				}
				hdr, err := UnmarshalPacketHeader(header[:])
				if err != nil {
					t.Error(err)
					return
				}
				body := make([]byte, hdr.Length)
				if _, err := io.ReadFull(conn, body); err != nil {
					t.Error(err)
					return
				}
				Encrypt(body, hdr.SessionID, sessionKey, hdr.Version, hdr.SeqNo)
				record, err := decodeDispatchWireRecord(hdr.Type, body)
				if err != nil {
					t.Error(err)
					return
				}
				records <- record
				reply := []byte{authorStatus, 0, 0, 0, 0, 0}
				if hdr.Type == typeAccounting {
					reply = []byte{0, 0, 0, 0, AcctStatusSuccess}
				}
				if hdr.Type == typeAuthentication {
					reply[0] = AuthenStatusPass
				}
				hdr.SeqNo++
				hdr.Flags = 0
				hdr.Length = uint32(len(reply))
				Encrypt(reply, hdr.SessionID, sessionKey, hdr.Version, hdr.SeqNo)
				if _, err := conn.Write(append(hdr.MarshalBinary(), reply...)); err != nil {
					t.Error(err)
				}
			}()
		}
	}()
	t.Cleanup(func() {
		closeIgnore(listener)
		<-acceptDone
		connections.Wait()
	})
	return listener.Addr().String(), records
}

func decodeDispatchWireRecord(kind uint8, body []byte) (dispatchWireRecord, error) {
	record := dispatchWireRecord{kind: kind}
	fixed, userAt, count := 8, 4, 0
	if kind == typeAccounting {
		fixed, userAt = 9, 5
	}
	if len(body) < fixed {
		return record, fmt.Errorf("short request body")
	}
	if kind == typeAccounting {
		record.flags = body[0]
	}
	if kind != typeAuthentication {
		count = int(body[fixed-1])
	}
	offset := fixed + count
	userLen := int(body[userAt])
	end := offset + userLen + int(body[userAt+1]) + int(body[userAt+2])
	if end > len(body) {
		return record, fmt.Errorf("request fields exceed body")
	}
	record.user = string(body[offset : offset+userLen])
	if kind == typeAuthentication {
		if end+int(body[7]) != len(body) {
			return record, fmt.Errorf("PAP length mismatch")
		}
		return record, nil
	}
	for _, length := range body[fixed : fixed+count] {
		if end+int(length) > len(body) {
			return record, fmt.Errorf("argument exceeds body")
		}
		record.args = append(record.args, string(body[end:end+int(length)]))
		end += int(length)
	}
	if end != len(body) {
		return record, fmt.Errorf("trailing request bytes")
	}
	return record, nil
}

func receiveDispatchWire(t *testing.T, records <-chan dispatchWireRecord, count int) []dispatchWireRecord {
	t.Helper()
	got := make([]dispatchWireRecord, 0, count)
	for range count {
		select {
		case record := <-records:
			got = append(got, record)
		case <-time.After(5 * time.Second):
			t.Fatalf("received %d of %d real TACACS+ requests", len(got), count)
		}
	}
	return got
}

func dispatchWireClient(t *testing.T, address string) *TacacsClient {
	t.Helper()
	client := NewTacacsClient(TacacsClientConfig{Servers: []TacacsServer{{Address: address, Key: sessionKey}}, Timeout: time.Second})
	t.Cleanup(client.Close)
	return client
}

func wireArgument(args []string, name string) string {
	for _, arg := range args {
		if strings.HasPrefix(arg, name+"=") {
			return strings.TrimPrefix(arg, name+"=")
		}
	}
	return ""
}

// RFC requirement: RFC8907-8.3-4 positive -- trusted plugin and shared-token dispatch contexts produce actual START/STOP packets with distinct printable identities; their command is also authorized on the socket.
// RFC requirement: RFC8907-3.7-2 positive -- a Unicode command argument reaches the accounting and authorization sockets as reversible printable ASCII, while the handler receives the original argument.
func TestRFC8907TrustedDispatchIdentitiesReachWire(t *testing.T) {
	address, records := dispatchWireServer(t, AuthorStatusPassAdd)
	client := dispatchWireClient(t, address)
	accountant := NewTacacsAccountant(client, nil)
	accountant.Start()
	t.Cleanup(accountant.Stop)
	d := pluginserver.NewDispatcher()
	d.SetAccountingHook(accountant)
	d.SetAuthorizer(newTacacsAuthorizer(client, authz.StoreAuthorizer{Store: authz.NewStore()}))
	var handled []string
	d.Register("show", func(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
		handled = append(handled, args...)
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	}, "")
	// These are the trusted contexts injected by plugin dispatch and by the
	// validated shared-token API boundary. The local names must stay untouched.
	identities := []string{aaa.ReservedInternalPrefix + "plugin:wire-caller", aaa.ReservedSharedAPIUsername}
	for _, identity := range identities {
		ctx := &pluginserver.CommandContext{Username: identity}
		if _, err := d.Dispatch(ctx, "show café"); err != nil {
			t.Fatal(err)
		}
		if ctx.Username != identity {
			t.Fatal("wire mapping changed the local trusted identity")
		}
		got := receiveDispatchWire(t, records, 3)
		wireUser := "~ze~r:" + base64.RawURLEncoding.EncodeToString([]byte(identity))
		start, stop := "", ""
		for _, record := range got {
			if record.user != wireUser {
				t.Fatalf("wrong wire identity: %q", record.user)
			}
			if _, err := prepareUsername(record.user); err != nil {
				t.Fatal(err)
			}
			if wireArgument(record.args, "cmd-arg") != `caf\u00e9` {
				t.Fatalf("wrong command display: %q", record.args)
			}
			for _, arg := range record.args {
				if err := validateText(arg); err != nil {
					t.Fatal(err)
				}
			}
			if record.flags == AcctFlagStart {
				start = wireArgument(record.args, "task_id")
			}
			if record.flags == AcctFlagStop {
				stop = wireArgument(record.args, "task_id")
			}
		}
		if start == "" || start != stop {
			t.Fatalf("accounting pair diverged: %q/%q", start, stop)
		}
	}
	if !reflect.DeepEqual(handled, []string{"café", "café"}) {
		t.Fatalf("execution changed: %q", handled)
	}
}

func TestTACACSWireIdentityCannotForgeLocalTrust(t *testing.T) {
	address, records := dispatchWireServer(t, AuthorStatusPassAdd)
	accountingClient := dispatchWireClient(t, address)
	accountant := NewTacacsAccountant(accountingClient, nil)
	accountant.Start()
	t.Cleanup(accountant.Stop)
	unreachable := listenTCP(t)
	unreachableAddress := unreachable.Addr().String()
	closeIgnore(unreachable)
	d := pluginserver.NewDispatcher()
	d.SetAccountingHook(accountant)
	d.SetAuthorizer(newTacacsAuthorizer(dispatchWireClient(t, unreachableAddress), authz.StoreAuthorizer{Store: authz.NewStore()}))
	called := 0
	d.Register("show", func(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
		called++
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	}, "")
	for _, raw := range []string{aaa.ReservedInternalPrefix + "plugin:wire-caller", aaa.ReservedSharedAPIUsername} {
		lookalike := "~ze~r:" + base64.RawURLEncoding.EncodeToString([]byte(raw))
		for _, identity := range []string{raw, lookalike} {
			_, err := d.Dispatch(&pluginserver.CommandContext{Username: identity}, "show version")
			if identity == raw && err != nil {
				t.Fatalf("trusted fallback denied: %v", err)
			}
			if identity == lookalike && !errors.Is(err, pluginserver.ErrUnauthorized) {
				t.Fatalf("lookalike acquired local trust: %v", err)
			}
			got := receiveDispatchWire(t, records, 2)
			want := lookalike
			if identity == lookalike {
				want = "~ze~u:" + base64.RawURLEncoding.EncodeToString([]byte(lookalike))
			}
			for _, record := range got {
				if record.user != want {
					t.Fatalf("identity collision: %q, want %q", record.user, want)
				}
			}
		}
	}
	if called != 2 {
		t.Fatalf("executed %d commands, want trusted callers only", called)
	}
	// The printable lookalike gets the same escaped human identity in PAP,
	// never the synthetic identity. A raw reserved name cannot log in at all.
	lookalike := "~ze~r:" + base64.RawURLEncoding.EncodeToString([]byte(aaa.ReservedSharedAPIUsername))
	if _, err := accountingClient.Authenticate(lookalike, "password", "ssh", ""); err != nil {
		t.Fatal(err)
	}
	pap := receiveDispatchWire(t, records, 1)[0]
	if pap.user != "~ze~u:"+base64.RawURLEncoding.EncodeToString([]byte(lookalike)) {
		t.Fatalf("PAP namespace collision: %q", pap.user)
	}
	// Fullwidth tildes trigger the profile's Bidi Rule; a leading neutral
	// character fails it. This lookalike must be refused, not width-folded
	// outside PRECIS to bypass validation and impersonate a synthetic actor.
	fullwidth := strings.ReplaceAll(lookalike, "~", "～")
	if _, err := accountingClient.Authenticate(fullwidth, "password", "ssh", ""); !errors.Is(err, errRequestInvalid) {
		t.Fatalf("PRECIS-invalid fullwidth lookalike reached PAP: %v", err)
	}
	if _, err := accountingClient.Authenticate(aaa.ReservedSharedAPIUsername, "password", "ssh", ""); !errors.Is(err, errRequestInvalid) {
		t.Fatalf("reserved identity reached PAP: %v", err)
	}
}

// RFC requirement: RFC8907-8.3-4 negative -- oversized redacted displays still produce a bounded START/STOP pair rather than disappearing at the uint8 field/count limits.
func TestRFC8907OversizedCommandAccountingStillReachesWire(t *testing.T) {
	address, records := dispatchWireServer(t, AuthorStatusPassAdd)
	client := dispatchWireClient(t, address)
	accountant := NewTacacsAccountant(client, nil)
	accountant.Start()
	t.Cleanup(accountant.Stop)
	d := pluginserver.NewDispatcher()
	d.SetAccountingHook(accountant)
	var handled []string
	d.Register("show", func(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
		handled = append([]string(nil), args...)
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	}, "")
	for _, args := range [][]string{{strings.Repeat("é", 100)}, strings.Fields(strings.Repeat("x ", 300))} {
		if _, err := d.Dispatch(&pluginserver.CommandContext{Username: "alice"}, "show "+strings.Join(args, " ")); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(handled, args) {
			t.Fatal("bounded accounting changed execution")
		}
		got := receiveDispatchWire(t, records, 2)
		var digest, task string
		for _, record := range got {
			if wireArgument(record.args, "ze-command-truncated") != "1" {
				t.Fatalf("missing truncation metadata: %q", record.args)
			}
			if len(record.args) > 255 {
				t.Fatal("too many wire arguments")
			}
			for _, arg := range record.args {
				if len(arg) > 255 || validateText(arg) != nil {
					t.Fatalf("unbounded/non-ASCII argument: %q", arg)
				}
			}
			if wireArgument(record.args, "service") != "shell" || wireArgument(record.args, "cmd") != "show" {
				t.Fatal("truncation lost required command fields")
			}
			if digest == "" {
				digest, task = wireArgument(record.args, "ze-command-sha256"), wireArgument(record.args, "task_id")
			}
			if len(digest) != 64 || wireArgument(record.args, "ze-command-sha256") != digest || wireArgument(record.args, "task_id") != task {
				t.Fatal("START/STOP display identity diverged")
			}
		}
	}
	// Authorization does not use display truncation to approve an oversized
	// command. The ordinary valid control above still executes unmodified.
	d.SetAuthorizer(newTacacsAuthorizer(client, authz.StoreAuthorizer{Store: authz.NewStore()}))
	if _, err := d.Dispatch(&pluginserver.CommandContext{Username: aaa.ReservedSharedAPIUsername}, "show "+strings.Repeat("é", 100)); !errors.Is(err, pluginserver.ErrUnauthorized) {
		t.Fatalf("lossy policy request was authorized: %v", err)
	}
	receiveDispatchWire(t, records, 2)
}

func TestTACACSDisplayDigestOnlyUsesRedactedArguments(t *testing.T) {
	address, records := dispatchWireServer(t, AuthorStatusPassAdd)
	accountant := NewTacacsAccountant(dispatchWireClient(t, address), nil)
	accountant.Start()
	t.Cleanup(accountant.Stop)
	d := pluginserver.NewDispatcher()
	d.SetAccountingHook(accountant)
	var handled string
	d.Register("set", func(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
		handled = args[len(args)-1]
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	}, "")
	longAddress := strings.Repeat("a", 260)
	var digests []string
	for _, secret := range []string{"first private value", "second private value"} {
		input := "set system authentication tacacs server " + longAddress + " key " + strconv.Quote(secret)
		if _, err := d.Dispatch(&pluginserver.CommandContext{Username: "alice"}, input); err != nil {
			t.Fatal(err)
		}
		if handled != secret {
			t.Fatal("redaction changed the execution value")
		}
		got := receiveDispatchWire(t, records, 2)
		for _, record := range got {
			joined := strings.Join(record.args, " ")
			if strings.Contains(joined, "private") || !strings.Contains(joined, redact.Placeholder) {
				t.Fatalf("secret display was not redacted: %q", record.args)
			}
			if wireArgument(record.args, "ze-command-truncated") != "1" {
				t.Fatal("long redacted display was not bounded")
			}
			digests = append(digests, wireArgument(record.args, "ze-command-sha256"))
		}
	}
	for _, digest := range digests {
		if len(digest) != 64 || digest != digests[0] {
			t.Fatal("display digest reveals which secret was entered")
		}
	}
}

func TestTACACSIdentityEncodingLengthLimit(t *testing.T) {
	address, records := dispatchWireServer(t, AuthorStatusPassAdd)
	client := dispatchWireClient(t, address)
	identity := aaa.ReservedInternalPrefix + strings.Repeat("x", 186-len(aaa.ReservedInternalPrefix))
	if _, err := client.SendAuthorization(&AuthorRequest{User: identity, Args: []string{"service=shell", "cmd=show"}}); err != nil {
		t.Fatal(err)
	}
	wire := receiveDispatchWire(t, records, 1)[0].user
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(wire, "~ze~r:"))
	if err != nil || string(decoded) != identity || len(wire) != 254 {
		t.Fatalf("maximum identity did not round-trip: %q, %v", wire, err)
	}
	_, err = client.SendAuthorization(&AuthorRequest{User: identity + "x", Args: []string{"service=shell", "cmd=show"}})
	if !errors.Is(err, errRequestInvalid) {
		t.Fatalf("oversized identity was truncated/hashed: %v", err)
	}
}

func TestTACACSDisplayDigestPreservesArgumentBoundaries(t *testing.T) {
	// Byte framing used by accounting digests does not merge argument boundaries.
	a := accountingArguments("1", "start_time=1", []string{"service=shell", "cmd=show", "cmd-arg=" + strings.Repeat("x", 256), "cmd-arg=a", "cmd-arg=bc"})
	b := accountingArguments("1", "start_time=1", []string{"service=shell", "cmd=show", "cmd-arg=" + strings.Repeat("x", 256), "cmd-arg=ab", "cmd-arg=c"})
	if wireArgument(a, "ze-command-sha256") == wireArgument(b, "ze-command-sha256") {
		t.Fatal("display digest collapsed argument boundaries")
	}
}

// Control characters in command values use reversible ASCII display on the
// authorization socket without changing the command's argument boundaries.
func TestTACACSCommandControlCharacterDisplayReachesWire(t *testing.T) {
	address, records := dispatchWireServer(t, AuthorStatusPassAdd)
	authorizer := newTacacsAuthorizer(dispatchWireClient(t, address), nil)
	if !authorizer.AuthorizeCommandArgs("alice", "", "show", []string{"two\nlines"}, "", true) {
		t.Fatal("a losslessly escaped command argument was refused")
	}
	record := receiveDispatchWire(t, records, 1)[0]
	if got := wireArgument(record.args, "cmd-arg"); got != `two\nlines` {
		t.Fatalf("control character display = %q", got)
	}
}
