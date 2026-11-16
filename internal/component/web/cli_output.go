// Design: docs/architecture/web-interface.md -- byte-exact raw-output completion frames.
// Related: cli_stream.go -- shared dispatch, authorization and legacy text output.

package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
)

const (
	cliOutputMediaType     = "application/x-ze-cli-output+ndjson"
	cliOutputChunkBytesMax = 32 * 1024
	cliOutputFrameBytesMax = 64 * 1024
	cliOutputWriteTimeout  = 5 * time.Second
)

// cliOutputFrame is the HTTP wire DTO. Data is base64-encoded by encoding/json,
// preserving producer bytes when a Write splits UTF-8 or carries binary data.
type cliOutputFrame struct {
	Data  []byte `json:"data,omitempty"`
	Done  bool   `json:"done"`
	Error string `json:"error,omitempty"`
}

// cliOutputWriter belongs to one request goroutine. It retains only a bounded
// encoded frame, with no queue, background worker or producer transcript. Its
// zero value is unusable; callers MUST construct it and finish after dispatch.
type cliOutputWriter struct {
	response  http.ResponseWriter
	ctx       context.Context
	cancel    context.CancelFunc
	remaining int
	started   bool
	failure   error
}

func newCLIOutputWriter(w http.ResponseWriter, ctx context.Context, cancel context.CancelFunc) *cliOutputWriter {
	return &cliOutputWriter{response: w, ctx: ctx, cancel: cancel, remaining: cliStreamOutputBytesMax}
}

func acceptsCLIOutput(r *http.Request) bool {
	// A wildcard never opts a plaintext client into framing. The HTTP server
	// bounds this header; malformed or refused media ranges are ignored.
	for _, accepted := range strings.Split(r.Header.Get("Accept"), ",") {
		media, parameters, err := mime.ParseMediaType(strings.TrimSpace(accepted))
		if err != nil {
			continue
		}
		if media != cliOutputMediaType {
			continue
		}
		if quality, present := parameters["q"]; present {
			weight, parseErr := strconv.ParseFloat(quality, 64)
			if parseErr != nil {
				continue
			}
			if weight > 0 {
				if weight <= 1 {
					return true
				}
			}
			continue
		}
		return true
	}
	return false
}

// Write counts producer bytes, not base64 bytes. A producer MUST stop when it
// returns an error. Even one that ignores the error cannot complete successfully.
func (w *cliOutputWriter) Write(p []byte) (int, error) {
	if w.failure != nil {
		return 0, w.failure
	}
	if err := w.ctx.Err(); err != nil {
		return 0, w.fail(err)
	}
	if len(p) > w.remaining {
		return 0, w.fail(errCLIStreamOutputLimit)
	}
	written := 0
	// The checked 4 MiB producer budget bounds this chunk loop.
	for len(p) > 0 {
		if err := w.ctx.Err(); err != nil {
			return written, w.fail(err)
		}
		chunk := p[:min(len(p), cliOutputChunkBytesMax)]
		if err := w.frame(cliOutputFrame{Data: chunk}); err != nil {
			return written, w.fail(err)
		}
		written += len(chunk)
		w.remaining -= len(chunk)
		p = p[len(chunk):]
	}
	return written, nil
}

func (w *cliOutputWriter) fail(err error) error {
	w.failure = err
	w.cancel()
	return err
}

// finish MUST run only after dispatch has returned. A complete result requires
// its successful terminal frame; EOF or cancellation alone never establishes it.
func (w *cliOutputWriter) finish(ctx context.Context, result error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if w.failure != nil {
		// A budget failure leaves the socket usable for one bounded refusal.
		// A failed socket, flush or deadline must never be written again.
		if !errors.Is(w.failure, errCLIStreamOutputLimit) {
			return w.failure
		}
		result = w.failure
	}
	if result != nil {
		refusal := cliStreamRefusal(result)
		if !w.started {
			status := http.StatusBadRequest
			if errors.Is(result, plugin.ErrStreamUnauthorized) {
				status = http.StatusForbidden
			}
			http.Error(w.response, refusal.Error(), status)
			return nil
		}
		return w.frame(cliOutputFrame{Done: true, Error: cliStreamRefusalText(refusal)})
	}
	return w.frame(cliOutputFrame{Done: true})
}

func (w *cliOutputWriter) frame(frame cliOutputFrame) error {
	data, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	if len(data)+1 > cliOutputFrameBytesMax {
		return opererr.New("The live output frame exceeds 64 KiB.",
			"The terminal stopped this response to keep live output bounded.",
			"Open /cli and run the live command again")
	}
	controller := http.NewResponseController(w.response)
	if err := controller.SetWriteDeadline(time.Now().Add(cliOutputWriteTimeout)); err != nil {
		// In-memory handler tests have no socket deadline. Production net/http
		// connections support this operation, as for the interactive transport.
		if !errors.Is(err, http.ErrNotSupported) {
			return err
		}
	}
	if !w.started {
		w.response.Header().Set("Content-Type", cliOutputMediaType)
		w.response.Header().Set("Cache-Control", "no-store")
		w.response.Header().Set("X-Content-Type-Options", "nosniff")
		w.response.Header().Set("X-Accel-Buffering", "no")
		w.response.WriteHeader(http.StatusOK)
		w.started = true
	}
	data = append(data, '\n')
	n, err := w.response.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	return controller.Flush()
}

var _ io.Writer = (*cliOutputWriter)(nil)
