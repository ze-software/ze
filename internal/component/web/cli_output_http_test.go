// Design: docs/architecture/web-interface.md -- bounded raw-output completion frames.
// Related: cli_stream_test.go -- unchanged legacy text transport controls.

package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

const cliOutputTestMediaType = "application/x-ze-cli-output+ndjson"

type cliOutputTestFrame struct {
	Data  []byte `json:"data"`
	Done  bool   `json:"done"`
	Error string `json:"error"`
}

// TestWebCLIOutputFramesPreserveBytes splits UTF-8 inside a rune, includes invalid
// UTF-8 and error-looking producer text, and observes exact bytes plus success.
func TestWebCLIOutputFramesPreserveBytes(t *testing.T) {
	want := []byte("\xff\x00\xe2\x82\xac\nerror: legitimate producer output\n")
	w := cliOutputTestRequest(t, func(_ context.Context, _ plugin.CallerIdentity, _ string, output io.Writer) (bool, error) {
		for _, chunk := range [][]byte{want[:3], want[3:4], want[4:]} {
			n, err := output.Write(chunk)
			require.NoError(t, err)
			require.Len(t, chunk, n)
		}
		return true, nil
	}, nil)
	frames, answer := cliOutputTestFrames(t, w)
	assert.Equal(t, want, answer)
	require.NotEmpty(t, frames)
	assert.True(t, frames[len(frames)-1].Done)
	assert.Empty(t, frames[len(frames)-1].Error)
}

// TestWebCLIOutputFramesBoundLargeWrites observes independently decodable frames
// and complete bytes when one producer Write is larger than a transport frame.
func TestWebCLIOutputFramesBoundLargeWrites(t *testing.T) {
	want := bytes.Repeat([]byte("\xf0\x9f\x8c\x8d"), 40*1024)
	w := cliOutputTestRequest(t, func(_ context.Context, _ plugin.CallerIdentity, _ string, output io.Writer) (bool, error) {
		n, err := output.Write(want)
		require.NoError(t, err)
		require.Equal(t, len(want), n)
		return true, nil
	}, nil)
	frames, answer := cliOutputTestFrames(t, w)
	assert.Equal(t, want, answer)
	assert.Greater(t, len(frames), 2)
	assert.True(t, frames[len(frames)-1].Done)
	assert.Empty(t, frames[len(frames)-1].Error)
}

// TestWebCLIOutputFramesPartialFailure keeps prior producer bytes and carries a
// bounded authoritative failure separately from those bytes.
func TestWebCLIOutputFramesPartialFailure(t *testing.T) {
	w := cliOutputTestRequest(t, func(_ context.Context, _ plugin.CallerIdentity, _ string, output io.Writer) (bool, error) {
		_, err := io.WriteString(output, "partial answer\n")
		require.NoError(t, err)
		return true, errors.New(strings.Repeat("producer failed ", 10000))
	}, nil)
	frames, answer := cliOutputTestFrames(t, w)
	assert.Equal(t, []byte("partial answer\n"), answer)
	last := frames[len(frames)-1]
	assert.True(t, last.Done)
	assert.Contains(t, last.Error, "The live command could not continue.")
	assert.Contains(t, last.Error, "fix: Open /cli and run the live command again")
	assert.LessOrEqual(t, len(last.Error), maxCommandLength)
}

// TestWebCLIOutputFramesOutputLimit proves the complete budget is useful, then
// an ignored writer error still prevents a successful dispatch return winning.
func TestWebCLIOutputFramesOutputLimit(t *testing.T) {
	want := bytes.Repeat([]byte("x"), 4*1024*1024)
	w := cliOutputTestRequest(t, func(ctx context.Context, _ plugin.CallerIdentity, _ string, output io.Writer) (bool, error) {
		n, err := output.Write(want)
		require.NoError(t, err)
		require.Equal(t, len(want), n)
		n, err = io.WriteString(output, "beyond budget")
		assert.Zero(t, n)
		assert.Error(t, err)
		assert.ErrorIs(t, ctx.Err(), context.Canceled)
		return true, nil
	}, nil)
	frames, answer := cliOutputTestFrames(t, w)
	assert.Equal(t, want, answer)
	last := frames[len(frames)-1]
	assert.True(t, last.Done)
	assert.Contains(t, last.Error, "4 MiB limit")
}

// TestWebCLIOutputFramesDeniedBeforeWrite retains the real HTTP authorization
// status and never introduces output frames before admission.
func TestWebCLIOutputFramesDeniedBeforeWrite(t *testing.T) {
	w := cliOutputTestRequest(t, func(context.Context, plugin.CallerIdentity, string, io.Writer) (bool, error) {
		return true, plugin.ErrStreamUnauthorized
	}, nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), plugin.UnauthorizedMessage)
	assert.NotEqual(t, cliOutputTestMediaType, w.Header().Get("Content-Type"))
}

// TestWebCLIOutputFramesFiniteFallthrough proves opting in writes nothing when
// the streaming dispatcher declines and leaves the finite JSON result intact.
func TestWebCLIOutputFramesFiniteFallthrough(t *testing.T) {
	w := cliOutputTestRequest(t, func(context.Context, plugin.CallerIdentity, string, io.Writer) (bool, error) {
		return false, nil
	}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := io.WriteString(w, `{"output":"finite answer"}`)
		require.NoError(t, err)
	}))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"output":"finite answer"}`, w.Body.String())
}

// TestWebCLIOutputFramesCanceledHasNoCompletion proves request cancellation
// invalidates partial output even if a producer incorrectly returns success.
func TestWebCLIOutputFramesCanceledHasNoCompletion(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	handler := HandleCLITerminalStream(nil, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("a handled stream reached the finite handler")
	}), func(_ context.Context, _ plugin.CallerIdentity, _ string, output io.Writer) (bool, error) {
		_, err := io.WriteString(output, "before cancellation")
		require.NoError(t, err)
		cancel()
		return true, nil
	})
	w := httptest.NewRecorder()
	handler(w, cliOutputTestHTTP(t).WithContext(context.WithValue(ctx, ctxKeyUsername, "alice")))
	frames, answer := cliOutputTestFrames(t, w)
	assert.Equal(t, []byte("before cancellation"), answer)
	for _, frame := range frames {
		assert.False(t, frame.Done)
	}
}

// TestWebCLIOutputFramesWriteFailureCancels proves socket failure cancels the
// producer and cannot be turned into success by a later nil dispatch error.
func TestWebCLIOutputFramesWriteFailureCancels(t *testing.T) {
	w := &cliOutputFailResponse{ResponseRecorder: httptest.NewRecorder()}
	handler := HandleCLITerminalStream(nil, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("a handled stream reached the finite handler")
	}), func(ctx context.Context, _ plugin.CallerIdentity, _ string, output io.Writer) (bool, error) {
		n, err := io.WriteString(output, "cannot arrive")
		assert.Zero(t, n)
		assert.Error(t, err)
		assert.ErrorIs(t, ctx.Err(), context.Canceled)
		return true, nil
	})
	handler(w, cliOutputTestHTTP(t))
	assert.Empty(t, w.Body.String())
	assert.Equal(t, 1, w.writes)
	assert.NotEmpty(t, w.deadlines)
	for _, deadline := range w.deadlines {
		assert.WithinDuration(t, time.Now().Add(5*time.Second), deadline, time.Second)
	}
}

type cliOutputFailResponse struct {
	*httptest.ResponseRecorder
	writes    int
	deadlines []time.Time
}

func (w *cliOutputFailResponse) Write([]byte) (int, error) {
	w.writes++
	return 0, io.ErrClosedPipe
}

func (w *cliOutputFailResponse) SetWriteDeadline(deadline time.Time) error {
	w.deadlines = append(w.deadlines, deadline)
	return nil
}

func cliOutputTestRequest(t *testing.T, dispatch plugin.CommandStreamDispatcher, finite http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	if finite == nil {
		finite = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Error("a handled stream reached the finite handler")
		})
	}
	w := httptest.NewRecorder()
	HandleCLITerminalStream(nil, finite, dispatch)(w, cliOutputTestHTTP(t))
	return w
}

func cliOutputTestHTTP(t *testing.T) *http.Request {
	t.Helper()
	r := authedRequest(t, http.MethodPost, "/cli/stream", url.Values{
		"command": {"monitor event"}, "mode": {terminalModeOperational},
	})
	r.Header.Set("Accept", cliOutputTestMediaType)
	return r
}

func cliOutputTestFrames(t *testing.T, w *httptest.ResponseRecorder) ([]cliOutputTestFrame, []byte) {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, cliOutputTestMediaType, w.Header().Get("Content-Type"))
	var frames []cliOutputTestFrame
	var answer []byte
	lines := bytes.Split(bytes.TrimSuffix(w.Body.Bytes(), []byte("\n")), []byte("\n"))
	for _, line := range lines {
		require.LessOrEqual(t, len(line)+1, 64*1024)
		var frame cliOutputTestFrame
		require.NoError(t, json.Unmarshal(line, &frame))
		if frame.Done {
			require.Empty(t, frame.Data)
		} else {
			require.Empty(t, frame.Error)
		}
		frames = append(frames, frame)
		answer = append(answer, frame.Data...)
	}
	return frames, answer
}
