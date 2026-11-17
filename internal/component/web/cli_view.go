// Design: docs/architecture/web-interface.md -- request-owned interactive monitors.
// Related: cli_stream.go -- shared terminal grammar and finite/text fallthrough.

package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
)

const (
	// CLIViewMediaType explicitly opts a consumer into framed controls.
	CLIViewMediaType      = "application/x-ze-cli-view+ndjson"
	cliViewHandlesMax     = 64
	cliViewActionBytesMax = 1024
	cliViewWriteTimeout   = 5 * time.Second
)

// CLIViewBinding installs registered factories using the admitted request and
// caller. It MUST bind all polls and detail RPCs to that context and identity.
type CLIViewBinding func(context.Context, plugin.CallerIdentity, *cli.Model) error

// CLIViewTransport has no background worker or durable session. Its bounded
// handles live only while their original HTTP stream runs. Each stream owns one
// RunBrowserView worker, cancels it on every exit and joins it before removal.
// Methods are safe for concurrent HTTP requests; callers MUST authenticate and
// apply RequireSameOrigin to both stream and action routes.
type CLIViewTransport struct {
	admit  plugin.CommandStreamAdmission
	bind   CLIViewBinding
	mu     sync.Mutex
	active map[string]*cliViewRequest
}

type cliViewRequest struct {
	username      string
	ctx           context.Context
	cancel        context.CancelFunc
	actions       chan cli.BrowserViewAction
	controls      []cli.BrowserViewControl // guarded by transport.mu
	sequence      uint64                   // owned by the stream writer
	stopRequested atomic.Bool
}

type cliViewFrame struct {
	Handle string `json:"handle"`
	cli.BrowserViewFrame
	Error string `json:"error,omitempty"`
}

// NewCLIViewTransport creates an empty, bounded active-request table. A nil
// admission or binding leaves raw/finite stream handling available.
func NewCLIViewTransport(admit plugin.CommandStreamAdmission, bind CLIViewBinding) *CLIViewTransport {
	return &CLIViewTransport{admit: admit, bind: bind, active: make(map[string]*cliViewRequest)}
}

// Handler retains the terminal's grammar/history and raw/finite fallthrough.
// Authentication and RequireSameOrigin MUST wrap this handler and Action.
func (v *CLIViewTransport) Handler(mgr *EditorManager, terminal http.Handler, dispatch plugin.CommandStreamDispatcher, registered ...func(string) bool) http.HandlerFunc {
	return handleCLITerminalStream(mgr, terminal, dispatch, v, registered...)
}

func (v *CLIViewTransport) accepts(r *http.Request, input string) bool {
	if v == nil {
		return false
	}
	if v.admit == nil {
		return false
	}
	if v.bind == nil {
		return false
	}
	if !cli.BrowserViewAvailable(input) {
		return false
	}
	// A request header is bounded by net/http's server limit. No wildcard opts
	// a legacy text consumer into this wire format.
	for accepted := range strings.SplitSeq(r.Header.Get("Accept"), ",") {
		media, parameters, err := mime.ParseMediaType(strings.TrimSpace(accepted))
		if err != nil {
			continue
		}
		if media != CLIViewMediaType {
			continue
		}
		if quality, present := parameters["q"]; present {
			weight, parseErr := strconv.ParseFloat(quality, 64)
			if parseErr != nil {
				continue
			}
			if weight <= 0 {
				continue
			}
			if weight > 1 {
				continue
			}
		}
		return true
	}
	return false
}

func (v *CLIViewTransport) stream(w http.ResponseWriter, r *http.Request, input string) bool {
	if !v.accepts(r, input) {
		return false
	}
	started := false
	var active *cliViewRequest
	var handle string
	handled, err := v.admit(r.Context(), plugin.CallerIdentity{
		Username: GetUsernameFromRequest(r), RemoteAddr: r.RemoteAddr,
		Authorizer: plugin.CallerAuthorizer(r.Context()),
	}, input, func(ctx context.Context, caller plugin.CallerIdentity) error {
		var addErr error
		handle, active, addErr = v.add(ctx, caller)
		if addErr != nil {
			return addErr
		}
		defer func() {
			active.cancel()
			v.mu.Lock()
			delete(v.active, handle)
			v.mu.Unlock()
		}()
		return v.run(w, handle, active, caller, input, &started)
	})
	if !handled {
		return false
	}
	if err != nil {
		if r.Context().Err() != nil {
			return true
		}
		if !started {
			status := http.StatusBadRequest
			if errors.Is(err, plugin.ErrStreamUnauthorized) {
				status = http.StatusForbidden
			}
			http.Error(w, cliStreamRefusal(err).Error(), status)
			return true
		}
		// The admission layer can report daemon shutdown only after its
		// continuation ends. The joined request retains no usable controls.
		if active != nil {
			if writeErr := v.write(w, active, cliViewFrame{Handle: handle,
				Done:  true,
				Error: cliStreamRefusalText(cliStreamRefusal(err))}, &started); writeErr != nil {
				return true
			}
		}
	}
	return true
}

func (v *CLIViewTransport) add(ctx context.Context, caller plugin.CallerIdentity) (string, *cliViewRequest, error) {
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", nil, cliViewFailure("A monitor control handle could not be created.")
	}
	handle := hex.EncodeToString(token[:])
	lifetime, cancel := context.WithCancel(ctx)
	request := &cliViewRequest{username: caller.Username, ctx: lifetime, cancel: cancel,
		actions: make(chan cli.BrowserViewAction, cli.BrowserViewActionQueueMax)}
	v.mu.Lock()
	defer v.mu.Unlock()
	if len(v.active) >= cliViewHandlesMax {
		cancel()
		return "", nil, cliViewFailure("Too many live monitors are open.")
	}
	if _, duplicate := v.active[handle]; duplicate {
		cancel()
		return "", nil, cliViewFailure("A monitor control handle could not be created.")
	}
	v.active[handle] = request
	return handle, request, nil
}

func (v *CLIViewTransport) run(w http.ResponseWriter, handle string, request *cliViewRequest, caller plugin.CallerIdentity, input string, started *bool) error {
	frames := make(chan cli.BrowserViewFrame, 1)
	done := make(chan error, 1)
	// One worker owns this request's registered view lifecycle, not an event.
	go func() {
		done <- cli.RunBrowserView(request.ctx, input, func(viewCtx context.Context, model *cli.Model) error {
			return v.bind(viewCtx, caller, model)
		}, request.actions, frames)
	}()
	joined := false
	defer func() {
		request.cancel()
		if !joined {
			<-done
		}
	}()
	var last cli.BrowserViewFrame
	// This pump lasts exactly as long as the HTTP request. Only the latest
	// frame is retained; the runner owns and never closes that frame slot.
	for {
		select {
		case frame := <-frames:
			if err := v.write(w, request, cliViewFrame{Handle: handle, BrowserViewFrame: frame}, started); err != nil {
				return err
			}
			last = frame
		case runErr := <-done:
			joined = true
			if request.stopRequested.Load() {
				last.Sequence = 0
				last.Done = true
				last.Controls = nil
				return v.write(w, request, cliViewFrame{Handle: handle, BrowserViewFrame: last}, started)
			}
			// Completion can race the final slot. Observe it after the worker
			// joins, otherwise Stop and its final controls can be lost.
			select {
			case frame := <-frames:
				if err := v.write(w, request, cliViewFrame{Handle: handle, BrowserViewFrame: frame}, started); err != nil {
					return err
				}
			default:
			}
			return runErr
		case <-request.ctx.Done():
			<-done
			joined = true
			if request.stopRequested.Load() {
				last.Sequence = 0
				last.Done = true
				last.Controls = nil
				return v.write(w, request, cliViewFrame{Handle: handle, BrowserViewFrame: last}, started)
			}
			return request.ctx.Err()
		}
	}
}

func (v *CLIViewTransport) write(w http.ResponseWriter, request *cliViewRequest, frame cliViewFrame, started *bool) error {
	if frame.Sequence == 0 {
		frame.Sequence = request.sequence + 1
	}
	data, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	if len(data)+1 > cli.BrowserViewFrameBytesMax {
		return cliViewFailure("The live monitor frame exceeds 64 KiB.")
	}
	request.sequence = frame.Sequence
	v.mu.Lock()
	request.controls = append(request.controls[:0], frame.Controls...)
	v.mu.Unlock()
	controller := http.NewResponseController(w)
	if err := controller.SetWriteDeadline(time.Now().Add(cliViewWriteTimeout)); err != nil {
		// In-memory handler tests have no socket deadline. Every production
		// net/http connection supports this controller operation.
		if !errors.Is(err, http.ErrNotSupported) {
			return err
		}
	}
	if !*started {
		w.Header().Set("Content-Type", CLIViewMediaType)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		*started = true
	}
	data = append(data, '\n')
	if _, err := w.Write(data); err != nil {
		return err
	}
	return controller.Flush()
}

// Action admits only a control in the owning request's latest projection. No
// command, caller identity, config path or policy is accepted from the form.
func (v *CLIViewTransport) Action(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, cliViewFailure("Monitor controls require a POST request.").Error(), http.StatusMethodNotAllowed)
		return
	}
	username := GetUsernameFromRequest(r)
	if username == "" {
		http.Error(w, cliViewFailure("Your browser session is not signed in.").Error(), http.StatusUnauthorized)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, cliViewActionBytesMax)
	if err := r.ParseForm(); err != nil {
		http.Error(w, cliViewFailure("The monitor control could not be read.").Error(), http.StatusBadRequest)
		return
	}
	if len(r.PostForm) != 2 {
		http.Error(w, cliViewFailure("The monitor control contains unexpected fields.").Error(), http.StatusBadRequest)
		return
	}
	if len(r.PostForm["handle"]) != 1 {
		http.Error(w, cliViewFailure("The monitor handle is malformed.").Error(), http.StatusBadRequest)
		return
	}
	if len(r.PostForm["action"]) != 1 {
		http.Error(w, cliViewFailure("The monitor action is malformed.").Error(), http.StatusBadRequest)
		return
	}
	handle, action := r.PostForm.Get("handle"), r.PostForm.Get("action")
	if len(handle) > 64 {
		http.Error(w, cliViewFailure("The monitor handle is too long.").Error(), http.StatusBadRequest)
		return
	}
	if action == "" {
		http.Error(w, cliViewFailure("Choose a monitor control.").Error(), http.StatusBadRequest)
		return
	}
	if len(action) > 64 {
		http.Error(w, cliViewFailure("The monitor action is too long.").Error(), http.StatusBadRequest)
		return
	}
	var status int
	var refusal *opererr.Error
	status, refusal = v.action(username, handle, action)
	if refusal != nil {
		http.Error(w, refusal.Error(), status)
		return
	}
	w.WriteHeader(status)
}

func (v *CLIViewTransport) action(username, handle, action string) (int, *opererr.Error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	request, found := v.active[handle]
	if !found {
		return http.StatusGone, cliViewFailure("This live monitor has ended.")
	}
	if request.username != username {
		return http.StatusForbidden, cliViewFailure("This monitor belongs to another browser session.")
	}
	if request.ctx.Err() != nil {
		return http.StatusGone, cliViewFailure("This live monitor has ended.")
	}
	declared := false
	for _, control := range request.controls {
		if control.ID == action {
			declared = true
			break
		}
	}
	if !declared {
		return http.StatusBadRequest, cliViewFailure("This control is not available in the current monitor view.")
	}
	// Stop cancels at this HTTP boundary, so a blocked detail RPC or a full
	// action queue cannot prevent the owning stream from joining its runner.
	if action == "stop" {
		request.stopRequested.Store(true)
		request.cancel()
		return http.StatusNoContent, nil
	}
	select {
	case request.actions <- cli.BrowserViewAction{ID: action}:
		return http.StatusNoContent, nil
	default:
		return http.StatusTooManyRequests, cliViewFailure("The monitor is still processing earlier controls.")
	}
}

func cliViewFailure(problem string) *opererr.Error {
	return opererr.New(problem, "No additional monitor action was run.", "Open /cli and run the live monitor again")
}
