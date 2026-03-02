// Design: docs/architecture/api/process-protocol.md — text-mode plugin SDK startup
// Overview: sdk.go — JSON-mode Plugin struct and Run()

package sdk

import (
	"context"
	"fmt"
	"net"
	"strings"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// NewTextFromEnv creates a text-mode plugin by reading ZE_ENGINE_FD and ZE_CALLBACK_FD
// environment variables. This is the primary constructor for external text plugins
// launched as subprocesses by the ze engine.
func NewTextFromEnv(name string) (*Plugin, error) {
	engineFD, err := envFD("ZE_ENGINE_FD")
	if err != nil {
		return nil, err
	}
	callbackFD, err := envFD("ZE_CALLBACK_FD")
	if err != nil {
		return nil, err
	}
	engineConn, err := connFromFD(engineFD, "ze-engine")
	if err != nil {
		return nil, fmt.Errorf("engine fd %d: %w", engineFD, err)
	}
	callbackConn, err := connFromFD(callbackFD, "ze-callback")
	if err != nil {
		engineConn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return nil, fmt.Errorf("callback fd %d: %w", callbackFD, err)
	}
	return NewTextPlugin(name, engineConn, callbackConn), nil
}

// NewTextPlugin creates a plugin that speaks text protocol.
// The text protocol uses line-based framing for the 5-stage startup
// and TextMuxConn for post-startup concurrent RPCs on Socket A.
func NewTextPlugin(name string, engineConn, callbackConn net.Conn) *Plugin {
	return &Plugin{
		name:         name,
		textMode:     true,
		rawEngineA:   engineConn,
		rawCallbackB: callbackConn,
	}
}

// runText executes the 5-stage text startup protocol and enters the text event loop.
// This is the text-mode equivalent of the JSON path in Run().
func (p *Plugin) runText(ctx context.Context, reg Registration) error {
	// Create TextConns from the raw connections.
	p.textConnA = rpc.NewTextConn(p.rawEngineA, p.rawEngineA)
	p.textConnB = rpc.NewTextConn(p.rawCallbackB, p.rawCallbackB)

	if err := p.textStartup(ctx, reg); err != nil {
		return err
	}

	// Startup complete: create TextMuxConn for concurrent engine calls.
	// TextMuxConn takes ownership of Socket A's reader.
	p.textMux = rpc.NewTextMuxConn(p.textConnA)

	// Post-startup callback.
	p.mu.Lock()
	startedFn := p.onStarted
	p.mu.Unlock()

	if startedFn != nil {
		if err := startedFn(ctx); err != nil {
			return fmt.Errorf("post-startup: %w", err)
		}
	}

	// Enter text event loop on Socket B.
	return p.textEventLoop(ctx)
}

// textStartup runs the 5-stage text handshake on the plugin side.
// Socket A (tcA): plugin sends stages 1, 3, 5 and reads "ok" responses.
// Socket B (tcB): plugin reads stages 2, 4 and sends "ok" responses.
func (p *Plugin) textStartup(ctx context.Context, reg Registration) error {
	tcA := p.textConnA
	tcB := p.textConnB

	// Stage 1: send registration on Socket A.
	regText, err := rpc.FormatRegistrationText(reg)
	if err != nil {
		return fmt.Errorf("stage 1 format: %w", err)
	}
	if err := tcA.WriteMessage(ctx, regText); err != nil {
		return fmt.Errorf("stage 1 write: %w", err)
	}
	resp, err := tcA.ReadLine(ctx)
	if err != nil {
		return fmt.Errorf("stage 1 read response: %w", err)
	}
	if resp != "ok" {
		return fmt.Errorf("stage 1: engine responded %q", resp)
	}

	// Stage 2: read configure from Socket B.
	configText, err := tcB.ReadMessage(ctx)
	if err != nil {
		return fmt.Errorf("stage 2 read: %w", err)
	}
	config, err := rpc.ParseConfigureText(configText)
	if err != nil {
		if writeErr := tcB.WriteLine(ctx, "error "+err.Error()); writeErr != nil {
			return fmt.Errorf("stage 2 error response: %w", writeErr)
		}
		return fmt.Errorf("stage 2 parse: %w", err)
	}
	p.mu.Lock()
	configureFn := p.onConfigure
	p.mu.Unlock()
	if configureFn != nil {
		if err := configureFn(config.Sections); err != nil {
			return fmt.Errorf("stage 2 handler: %w", err)
		}
	}
	if err := tcB.WriteLine(ctx, "ok"); err != nil {
		return fmt.Errorf("stage 2 respond: %w", err)
	}

	// Stage 3: send capabilities on Socket A.
	p.mu.Lock()
	capsInput := rpc.DeclareCapabilitiesInput{
		Capabilities: append([]rpc.CapabilityDecl(nil), p.capabilities...),
	}
	p.mu.Unlock()
	capsText, err := rpc.FormatCapabilitiesText(capsInput)
	if err != nil {
		return fmt.Errorf("stage 3 format: %w", err)
	}
	if err := tcA.WriteMessage(ctx, capsText); err != nil {
		return fmt.Errorf("stage 3 write: %w", err)
	}
	resp, err = tcA.ReadLine(ctx)
	if err != nil {
		return fmt.Errorf("stage 3 read response: %w", err)
	}
	if resp != "ok" {
		return fmt.Errorf("stage 3: engine responded %q", resp)
	}

	// Stage 4: read registry from Socket B.
	registryText, err := tcB.ReadMessage(ctx)
	if err != nil {
		return fmt.Errorf("stage 4 read: %w", err)
	}
	registry, err := rpc.ParseRegistryText(registryText)
	if err != nil {
		if writeErr := tcB.WriteLine(ctx, "error "+err.Error()); writeErr != nil {
			return fmt.Errorf("stage 4 error response: %w", writeErr)
		}
		return fmt.Errorf("stage 4 parse: %w", err)
	}
	p.mu.Lock()
	registryFn := p.onShareRegistry
	p.mu.Unlock()
	if registryFn != nil {
		registryFn(registry.Commands)
	}
	if err := tcB.WriteLine(ctx, "ok"); err != nil {
		return fmt.Errorf("stage 4 respond: %w", err)
	}

	// Stage 5: send ready on Socket A.
	p.mu.Lock()
	readyInput := rpc.ReadyInput{}
	if p.startupSubscription != nil {
		readyInput.Subscribe = p.startupSubscription
	}
	p.mu.Unlock()
	readyText, err := rpc.FormatReadyText(readyInput)
	if err != nil {
		return fmt.Errorf("stage 5 format: %w", err)
	}
	if err := tcA.WriteMessage(ctx, readyText); err != nil {
		return fmt.Errorf("stage 5 write: %w", err)
	}
	resp, err = tcA.ReadLine(ctx)
	if err != nil {
		return fmt.Errorf("stage 5 read response: %w", err)
	}
	if resp != "ok" {
		return fmt.Errorf("stage 5: engine responded %q", resp)
	}

	return nil
}

// textEventLoop reads text lines from Socket B for runtime callbacks.
// Events are delivered as plain text lines. "bye" signals shutdown.
func (p *Plugin) textEventLoop(ctx context.Context) error {
	tcB := p.textConnB

	for {
		line, err := tcB.ReadLine(ctx)
		if err != nil {
			// Context canceled or connection closed = clean shutdown.
			if ctx.Err() != nil || isConnectionClosed(err) {
				return nil //nolint:nilerr // EOF/context-cancel during shutdown is not an error
			}
			return fmt.Errorf("text event loop read: %w", err)
		}

		// "bye" or "bye <reason>" signals clean shutdown.
		if line == "bye" || strings.HasPrefix(line, "bye ") {
			reason := strings.TrimPrefix(line, "bye ")
			if line == "bye" {
				reason = ""
			}
			p.mu.Lock()
			fn := p.onBye
			p.mu.Unlock()
			if fn != nil {
				fn(reason)
			}
			return nil
		}

		// All other lines are events.
		p.mu.Lock()
		fn := p.onEvent
		p.mu.Unlock()
		if fn != nil {
			if err := fn(line); err != nil {
				return fmt.Errorf("text event handler: %w", err)
			}
		}
	}
}

// closeText closes text-mode connections in the correct order.
// TextMuxConn first (stops its background reader), then TextConns.
func (p *Plugin) closeText() error {
	if p.textMux != nil {
		// textMux.Close() closes textConnA's underlying reader.
		if err := p.textMux.Close(); err != nil {
			return err
		}
	} else if p.textConnA != nil {
		// No textMux (startup failed before stage 5) — close textConnA directly.
		if err := p.textConnA.Close(); err != nil {
			return err
		}
	}
	if p.textConnB != nil {
		if err := p.textConnB.Close(); err != nil {
			return err
		}
	}
	return nil
}
