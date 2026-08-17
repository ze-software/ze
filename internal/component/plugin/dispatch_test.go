package plugin

import (
	"context"
	"errors"
	"testing"
)

// sampleData is a typed ResponseData payload used to check that ResponseJSON
// marshals Data exactly as the historical hub adapters did (json.Marshal).
type sampleData struct {
	DataMarker
	PeerCount int    `json:"peer-count"`
	Name      string `json:"name"`
}

// VALIDATES: AC-3 / R-3 -- the single flatten helper produces the exact client
// bytes the two old hub adapters produced for done / error / nil / nil-Data /
// typed-Data cases.
// PREVENTS: byte-drift on the text surfaces after collapsing the two adapters.
func TestResponseJSON(t *testing.T) {
	tests := []struct {
		name    string
		resp    *Response
		err     error
		want    string
		wantErr string
	}{
		{
			name: "typed data",
			resp: NewResponse(StatusDone, &sampleData{PeerCount: 3, Name: "edge"}),
			want: `{"peer-count":3,"name":"edge"}`,
		},
		{
			name: "raw json data passes through",
			resp: NewResponse(StatusDone, RawJSON(`{"a":1}`)),
			want: `{"a":1}`,
		},
		{
			name: "pre-rendered text passes through verbatim (unquoted)",
			resp: NewResponse(StatusDone, Text("line one\nline two")),
			want: "line one\nline two",
		},
		{
			name: "nil response yields empty",
			resp: nil,
			want: "",
		},
		{
			name: "done with nil data yields empty",
			resp: &Response{Status: StatusDone},
			want: "",
		},
		{
			name:    "error message surfaces as go error",
			resp:    newErrorResponse("boom"),
			wantErr: "boom",
		},
		{
			name:    "status error without message is unknown error",
			resp:    &Response{Status: StatusError},
			wantErr: "unknown error",
		},
		{
			name:    "dispatch error propagates",
			resp:    nil,
			err:     errors.New("server not ready"),
			wantErr: "server not ready",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResponseJSON(tc.resp, tc.err)
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("want error %q, got %v", tc.wantErr, err)
				}
				if got != "" {
					t.Fatalf("want empty output on error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("want %q, got %q", tc.want, got)
			}
		})
	}
}

// VALIDATES: CommandDispatcher.JSON dispatches then flattens via ResponseJSON,
// threading the caller and command through to the underlying dispatcher.
// PREVENTS: text surfaces diverging from the shared flatten path.
func TestCommandDispatcherJSON(t *testing.T) {
	var gotCaller CallerIdentity
	var gotCmd string
	completed := false
	d := CommandDispatcher(func(_ context.Context, caller CallerIdentity, cmd string) (*Response, error) {
		gotCaller = caller
		gotCmd = cmd
		resp := NewResponse(StatusDone, RawJSON(`{"ok":true}`))
		resp.OnTransportComplete(func() { completed = true })
		return resp, nil
	})

	out, err := d.JSON(context.Background(), CallerIdentity{Username: "admin", Surface: "web"}, "show status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Output != `{"ok":true}` {
		t.Fatalf("want flattened data, got %q", out.Output)
	}
	if completed {
		t.Fatal("JSON conversion completed the response before its transport wrote it")
	}
	out.TransportComplete()
	if !completed {
		t.Fatal("transport completion did not release the accepted action")
	}
	if gotCmd != "show status" {
		t.Fatalf("command not threaded: %q", gotCmd)
	}
	if gotCaller.Username != "admin" || gotCaller.Surface != "web" {
		t.Fatalf("caller not threaded: %+v", gotCaller)
	}
}

// VALIDATES: JSON surfaces an error Response as a Go error with empty output.
// PREVENTS: error responses being rendered as command output on text surfaces.
func TestCommandDispatcherJSONError(t *testing.T) {
	d := CommandDispatcher(func(_ context.Context, _ CallerIdentity, _ string) (*Response, error) {
		return newErrorResponse("denied"), nil
	})
	out, err := d.JSON(context.Background(), CallerIdentity{}, "request reload")
	if err == nil || err.Error() != "denied" {
		t.Fatalf("want denied error, got %v", err)
	}
	if out.Output != "" {
		t.Fatalf("want empty output, got %q", out.Output)
	}
	out.TransportComplete()
}
