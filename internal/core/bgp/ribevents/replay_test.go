package ribevents

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// replayScript answers RelayReplay's reads from a fixed list of
// (status, page) pairs, recording each call's arguments.
type replayScript struct {
	answers []struct {
		status string
		page   string
	}
	calls [][]string
}

func (s *replayScript) dispatch(_ context.Context, command string, args ...string) (string, json.RawMessage, error) {
	if command != ReplayRoutesCommand {
		return "", nil, errors.New("unexpected command " + command)
	}
	s.calls = append(s.calls, slices.Clone(args))
	if len(s.calls) > len(s.answers) {
		return "", nil, errors.New("read past the scripted pages")
	}
	answer := s.answers[len(s.calls)-1]
	return answer.status, json.RawMessage(answer.page), nil
}

func script(pairs ...string) *replayScript {
	s := &replayScript{}
	for i := 0; i+1 < len(pairs); i += 2 {
		s.answers = append(s.answers, struct {
			status string
			page   string
		}{pairs[i], pairs[i+1]})
	}
	return s
}

const oneRoute = `{"source-peer":"10.0.0.9","family":"ipv4/unicast","nlri-hex":"180a0000"}`

// TestRelayReplayRelaysEveryPageThroughTheCaller covers the owner's loop.
//
// Method: script a two-page walk with a cut, and a relay that records pages.
//
// VALIDATES: each page is relayed through the caller's relay before the next is
// read, the cursor resumes from the page's last-index, the cut rides on every
// read, and the progress sums the pages.
// PREVENTS: a page left unrelayed, or a resume that re-reads the same page.
func TestRelayReplayRelaysEveryPageThroughTheCaller(t *testing.T) {
	s := script(
		"done", `{"routes":[`+oneRoute+`,`+oneRoute+`],"last-index":4,"more":true}`,
		"done", `{"routes":[`+oneRoute+`],"last-index":9,"more":false,"ingested-msg-id":12}`,
	)
	var pages []int
	relay := func(_ context.Context, destination string, routes []rpc.StoredRoute) error {
		if destination != "192.0.2.2" {
			t.Errorf("relayed to %q, want 192.0.2.2", destination)
		}
		pages = append(pages, len(routes))
		return nil
	}
	progress, err := RelayReplay(t.Context(), s.dispatch, relay, "192.0.2.2", 0, "7")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(pages, []int{2, 1}) {
		t.Fatalf("relayed pages %v, want [2 1]", pages)
	}
	want := [][]string{{"192.0.2.2", "0", "7"}, {"192.0.2.2", "4", "7"}}
	if !slices.EqualFunc(s.calls, want, slices.Equal[[]string]) {
		t.Fatalf("reads %v, want %v", s.calls, want)
	}
	if progress.Replayed != 3 || progress.LastIndex != 9 {
		t.Fatalf("progress %+v, want 3 routes ending at 9", progress)
	}
	if progress.IngestedMsgID == nil || *progress.IngestedMsgID != 12 {
		t.Fatalf("ingested %v, want 12 from the last page", progress.IngestedMsgID)
	}
}

// TestRelayReplayStopsOnFailure covers every way the loop must end early.
//
// VALIDATES: a relay refusal, a non-done status, an undecodable page and a page
// that claims more without advancing each end the walk with an error, and no
// page after the failure is read.
// PREVENTS: a refused relay being read as a delivered replay, and an endless
// loop on a store that does not advance.
func TestRelayReplayStopsOnFailure(t *testing.T) {
	refused := errors.New("send refused")
	for _, tt := range []struct {
		name     string
		s        *replayScript
		relayErr error
		reads    int
		want     error
	}{
		{name: "relay refused", s: script("done", `{"routes":[`+oneRoute+`],"last-index":3,"more":true}`,
			"done", `{"routes":[],"more":false}`), relayErr: refused, reads: 1, want: refused},
		{name: "status error", s: script("error", `{}`), reads: 1},
		{name: "undecodable", s: script("done", `not json`), reads: 1},
		{name: "stalled cursor", s: script("done", `{"routes":[`+oneRoute+`],"last-index":5,"more":true}`,
			"done", `{"routes":[`+oneRoute+`],"last-index":5,"more":true}`), reads: 2, want: errReplayPageStalled},
	} {
		t.Run(tt.name, func(t *testing.T) {
			relay := func(context.Context, string, []rpc.StoredRoute) error { return tt.relayErr }
			_, err := RelayReplay(t.Context(), tt.s.dispatch, relay, "192.0.2.2", 0, "")
			if err == nil {
				t.Fatal("the walk must fail")
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("error %v, want %v", err, tt.want)
			}
			if len(tt.s.calls) != tt.reads {
				t.Fatalf("read %d pages, want %d", len(tt.s.calls), tt.reads)
			}
		})
	}
}

// TestRelayReplayPathRelaysTheAnsweredPath pins the revalidation half of the
// owner relay: the store answers one path's state and the CALLER relays it.
//
// VALIDATES: an answered route is relayed to the destination through the
// caller's relay, live (not marked initial); an empty answer (destination is
// the source) relays nothing; a refused or undecodable answer is an error and
// relays nothing.
// PREVENTS: a revalidated path that reaches the destination only when the
// store, which holds no send grant, relays it.
func TestRelayReplayPathRelaysTheAnsweredPath(t *testing.T) {
	const route = `{"source-peer":"192.0.2.1","family":"ipv4/unicast","nlri-hex":"18cb0071","withdraw":true}`
	cases := []struct {
		name, status, page string
		want               int
		wantErr            bool
	}{
		{name: "one path", status: "done", page: `{"routes":[` + route + `],"last-index":0,"more":false}`, want: 1},
		{name: "destination is the source", status: "done", page: `{"routes":[],"last-index":0,"more":false}`},
		{name: "refused", status: "error", page: `{}`, wantErr: true},
		{name: "undecodable", status: "done", page: `[`, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotArgs []string
			dispatch := func(_ context.Context, command string, args ...string) (string, json.RawMessage, error) {
				if command != ReplayPathCommand {
					return "", nil, errors.New("unexpected command " + command)
				}
				gotArgs = slices.Clone(args)
				return tc.status, json.RawMessage(tc.page), nil
			}
			var relayed []rpc.StoredRoute
			relay := func(_ context.Context, destination string, routes []rpc.StoredRoute) error {
				if destination != "192.0.2.2" {
					t.Errorf("relayed to %s, want 192.0.2.2", destination)
				}
				relayed = append(relayed, routes...)
				return nil
			}
			n, err := RelayReplayPath(t.Context(), dispatch, relay, "192.0.2.2", "192.0.2.1", "ipv4/unicast", "203.0.113.0/24", "7")
			if !slices.Equal(gotArgs, []string{"192.0.2.2", "192.0.2.1", "ipv4/unicast", "203.0.113.0/24", "7"}) {
				t.Errorf("read the store with %v", gotArgs)
			}
			if tc.wantErr != (err != nil) {
				t.Fatalf("error = %v, want error %v", err, tc.wantErr)
			}
			if n != tc.want || len(relayed) != tc.want {
				t.Fatalf("relayed %d (reported %d), want %d", len(relayed), n, tc.want)
			}
			if tc.want == 1 && (!relayed[0].Withdraw || relayed[0].InitialUpdate) {
				t.Errorf("relayed %+v, want the answered live withdrawal", relayed[0])
			}
		})
	}
}
