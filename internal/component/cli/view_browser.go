// Design: docs/architecture/command-ownership.md -- registered headless live views.
// Related: view_registry.go -- owner registrations and lifecycle.

package cli

import (
	"context"
	"errors"
	"io"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ze-software/ze/internal/core/opererr"
)

const (
	// BrowserViewActionQueueMax bounds the caller's pending control actions.
	BrowserViewActionQueueMax = 8
	// BrowserViewFrameBytesMax bounds one detached plain-text frame.
	BrowserViewFrameBytesMax   = 64 * 1024
	browserViewControlsMax     = 16
	browserViewCommandQueueMax = 8
	browserViewInputBytesMax   = 512
)

// BrowserViewControl is owner-declared presentation metadata. ID is the only
// action token accepted by the runner; terminal keys never cross this boundary.
type BrowserViewControl struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type browserViewControl struct {
	BrowserViewControl
	key  string
	stop bool
}

// BrowserViewAction addresses one of the controls in the current frame.
type BrowserViewAction struct{ ID string }

// BrowserViewFrame is detached from the mutable Model and safe to retain.
// Sequence increases within one request. Text contains no terminal controls.
type BrowserViewFrame struct {
	Sequence uint64               `json:"sequence"`
	Text     string               `json:"text"`
	Controls []BrowserViewControl `json:"controls"`
	Done     bool                 `json:"done"`
}

// BrowserViewBinding installs the caller's existing factories and executor.
// It MUST bind every producer to ctx and preserve the original caller identity.
// The caller MUST authorize/account the monitor before calling RunBrowserView.
type BrowserViewBinding func(ctx context.Context, model *Model) error

// RunBrowserView drives the registered Model for exactly one request lifetime.
// It is safe to call concurrently with distinct channels and bindings. The
// caller MUST supply an action queue of capacity 1..BrowserViewActionQueueMax
// and a frame channel of capacity one, and MUST NOT close either while running.
// The caller MUST only receive from frames. The runner replaces an unread
// frame with the latest frame, never queues history. The caller MUST cancel ctx
// and wait for return on disconnect.
//
// The program never reads stdin, renders, opens a TTY or installs signal
// handlers. A single command worker executes the view's poll/timer chain; the
// Model alone serializes actions and state. All owned workers are joined before
// return. Factories MUST return promptly when their bound context is canceled.
func RunBrowserView(ctx context.Context, input string, bind BrowserViewBinding, actions <-chan BrowserViewAction, frames chan BrowserViewFrame) (runErr error) {
	if ctx == nil {
		return browserViewFailure("browser view requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(input) > browserViewInputBytesMax {
		return browserViewFailure("browser view command exceeds 512 bytes")
	}
	if bind == nil {
		return browserViewFailure("browser view requires caller factories")
	}
	if cap(actions) < 1 {
		return browserViewFailure("browser view requires a bounded action queue")
	}
	if cap(actions) > BrowserViewActionQueueMax {
		return browserViewFailure("browser view action queue exceeds limit")
	}
	if cap(frames) != 1 {
		return browserViewFailure("browser view requires one latest-frame slot")
	}
	spec, found := resolveView(input)
	if !found {
		return browserViewFailure("command has no registered live view")
	}
	if spec.browser == nil {
		return browserViewFailure("registered live view has no browser controls")
	}
	lifetime, cancel := context.WithCancel(ctx)
	m := NewCommandModel(FilesystemAuthorityUnknown)
	// Replace the constructor's local lifetime before any factory or timer can
	// observe it: request values, deadline and cancellation remain authoritative.
	m.dispatch.end()
	m.dispatch.ctx, m.dispatch.cancel = context.WithCancel(lifetime)
	m.width, m.height = 120, 40
	b := &browserViewModel{model: m, spec: spec, cancel: cancel, frames: frames,
		commands: make(chan tea.Cmd, browserViewCommandQueueMax), generation: &browserViewGeneration{}}
	workerDone := make(chan struct{})
	workerStarted := false
	defer func() {
		cancel()
		b.model.dispatch.end()
		if b.model.activeView != nil {
			b.model.activeView.release()
		}
		if workerStarted {
			<-workerDone
		}
		runErr = errors.Join(runErr, b.model.Close())
	}()
	if err := bind(lifetime, &b.model); err != nil {
		return err
	}
	if err := lifetime.Err(); err != nil {
		return err
	}
	initial := spec.start(&b.model, input)
	if b.model.activeView == nil {
		if b.model.err != nil {
			return b.model.err
		}
		return opererr.New("live view did not start", b.model.statusMessage, "system command list")
	}
	b.submit(initial)
	b.publish(false)
	if b.err != nil {
		return b.err
	}
	program := newBrowserViewProgram(lifetime, b)
	workerStarted = true
	go browserViewCommands(lifetime, program, b.commands, b.generation, workerDone)
	programDone := make(chan error, 1)
	go func() { _, err := program.Run(); programDone <- err }()
	requestedStop := false
	// This is the request's action pump. Its lifetime, not an action count,
	// bounds the loop; the caller's declared queue bounds pending actions.
	for {
		select {
		case err := <-programDone:
			if b.err != nil {
				return b.err
			}
			if requestedStop {
				b.publish(true)
				return b.err
			}
			if b.stopped {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		case action, open := <-actions:
			if !open {
				actions = nil
				continue
			}
			// Stop is the compulsory control for every browser view. Cancel at
			// admission so it also interrupts a context-bound detail RPC that
			// currently owns the Model loop. Publish only after that loop joins.
			if action.ID == cmdStop {
				requestedStop = true
				cancel()
				actions = nil
				continue
			}
			program.Send(action)
		case <-lifetime.Done():
			// Program.Send also observes lifetime through Bubble Tea's context.
			// Always join the program before reading its final model or fault.
			err := <-programDone
			if b.err != nil {
				return b.err
			}
			if requestedStop {
				b.publish(true)
				return b.err
			}
			if b.stopped {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
	}
}

// newBrowserViewProgram makes the library's terminal/resource options explicit.
// Its actual Program configuration is checked without changing process stdin,
// output or signal handlers in tests that share the test process.
func newBrowserViewProgram(ctx context.Context, model tea.Model) *tea.Program {
	return tea.NewProgram(model, tea.WithContext(ctx), tea.WithInput(nil),
		tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithoutSignalHandler(),
		tea.WithWindowSize(120, 40), tea.WithEnvironment([]string{"TERM=dumb", "NO_COLOR=1"}))
}

// A nonzero-sized owner token keeps pointer identities distinct across runners.
type browserViewGeneration struct{ _ byte }
type browserViewResult struct {
	generation *browserViewGeneration
	message    tea.Msg
}

type browserViewModel struct {
	model      Model
	spec       viewSpec
	cancel     context.CancelFunc
	commands   chan tea.Cmd
	frames     chan BrowserViewFrame
	generation *browserViewGeneration
	sequence   uint64
	err        error
	stopped    bool
}

func (b *browserViewModel) Init() tea.Cmd  { return nil }
func (b *browserViewModel) View() tea.View { return tea.NewView("") }

func (b *browserViewModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	if b.stopped {
		return b, nil
	}
	switch msg := message.(type) {
	case BrowserViewAction:
		b.action(msg)
	case browserViewResult:
		if msg.generation != b.generation {
			return b, nil
		}
		b.update(msg.message)
	}
	return b, nil // commands run only on our bounded, joined worker
}

func (b *browserViewModel) update(message tea.Msg) {
	// Current registered views produce only these messages. Batch/Sequence or
	// a new command protocol must be supported deliberately, never discarded.
	switch message.(type) {
	case nil:
		return
	case viewMsg, TimerMsg, modelTickMsg:
		updated, cmd := b.model.Update(message)
		model, ok := updated.(Model)
		if !ok {
			b.fail(browserViewFailure("live view returned an unsupported model"))
			return
		}
		b.model = model
		b.submit(cmd)
		b.publish(b.model.activeView == nil)
	default:
		b.fail(browserViewFailure("live view returned an unsupported update"))
	}
}

func (b *browserViewModel) action(action BrowserViewAction) {
	if len(action.ID) > 64 {
		b.fail(browserViewFailure("browser view action exceeds limit"))
		return
	}
	controls := b.spec.browser(&b.model)
	if len(controls) > browserViewControlsMax {
		b.fail(browserViewFailure("live view controls exceed limit"))
		return
	}
	for _, control := range controls {
		if control.ID != action.ID {
			continue
		}
		if control.stop {
			b.publish(true)
			b.stopped = true
			b.cancel()
			return
		}
		if b.model.activeView == nil {
			b.fail(browserViewFailure("live view has ended"))
			return
		}
		b.model.activeView.key(&b.model, control.key)
		b.publish(b.model.activeView == nil)
		return
	}
	b.fail(browserViewFailure("browser view action is not declared in the current frame"))
}

func (b *browserViewModel) submit(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	select {
	case b.commands <- cmd:
	default:
		b.fail(browserViewFailure("live view command queue is full"))
	}
}

func (b *browserViewModel) publish(done bool) {
	text := browserViewPlain(b.model.View().Content)
	if len(text) > BrowserViewFrameBytesMax {
		b.fail(browserViewFailure("live view frame exceeds 64 KiB"))
		return
	}
	controls := b.spec.browser(&b.model)
	if len(controls) > browserViewControlsMax {
		b.fail(browserViewFailure("live view controls exceed limit"))
		return
	}
	if !done {
		stops := 0
		for _, control := range controls {
			if control.stop {
				if control.ID == cmdStop {
					stops++
				}
			}
		}
		if stops != 1 {
			b.fail(browserViewFailure("live view must declare exactly one Stop control"))
			return
		}
	}
	b.sequence++
	frame := BrowserViewFrame{Sequence: b.sequence, Text: text, Done: done,
		Controls: make([]BrowserViewControl, 0, len(controls))}
	if !done {
		for _, control := range controls {
			if len(control.ID) > 64 {
				b.fail(browserViewFailure("live view control ID exceeds limit"))
				return
			}
			if len(control.Label) > 80 {
				b.fail(browserViewFailure("live view control label exceeds limit"))
				return
			}
			frame.Controls = append(frame.Controls, control.BrowserViewControl)
		}
	}
	select {
	case <-b.frames:
	default:
	} // supersede only this request's unread frame
	b.frames <- frame // capacity one, with exactly one writer
	if done {
		b.stopped = true
		b.cancel()
	}
}

func (b *browserViewModel) fail(err error) { b.err = err; b.cancel() }

func browserViewFailure(problem string) error {
	return opererr.New(problem,
		"The live monitor cannot continue with this request. Choose a supported monitor and retry.",
		"system command list")
}

func browserViewPlain(text string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' {
			return r
		}
		if r == '\t' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, ansi.Strip(text))
}

// browserViewCommands owns one bounded command chain for the request. Unlike
// Bubble Tea's detached per-Cmd goroutines, this worker is canceled and joined.
func browserViewCommands(ctx context.Context, program *tea.Program, commands <-chan tea.Cmd, generation *browserViewGeneration, done chan<- struct{}) {
	defer close(done)
	// A view's request lifetime bounds this worker; no retries or event history.
	for {
		select {
		case <-ctx.Done():
			return
		case cmd := <-commands:
			if ctx.Err() != nil {
				return
			}
			message := cmd()
			if ctx.Err() != nil {
				return
			}
			program.Send(browserViewResult{generation: generation, message: message})
		}
	}
}

func browserViewStopControl() browserViewControl {
	return browserViewControl{ID: cmdStop, Label: "Stop", stop: true}
}
