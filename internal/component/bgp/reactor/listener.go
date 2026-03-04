// Design: docs/architecture/core-design.md — BGP reactor event loop

package reactor

import (
	"context"
	"errors"
	"net"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/core/network"
	"codeberg.org/thomas-mangin/ze/internal/core/syncutil"
)

// Listener errors.
var (
	ErrAlreadyListening = errors.New("already listening")
	ErrNotListening     = errors.New("not listening")
)

// ConnectionHandler is called for each accepted connection.
type ConnectionHandler func(conn net.Conn)

// Listener accepts incoming BGP connections.
//
// It listens on a TCP address and calls the handler for each
// accepted connection. The handler is responsible for determining
// if the connection is from a configured peer.
type Listener struct {
	addr            string
	handler         ConnectionHandler
	clock           clock.Clock
	listenerFactory network.ListenerFactory

	listener net.Listener
	running  bool

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu sync.RWMutex
}

// NewListener creates a new listener for the given address.
// Address format: "host:port" (e.g., "0.0.0.0:179", "127.0.0.1:1179").
func NewListener(addr string) *Listener {
	return &Listener{
		addr:            addr,
		clock:           clock.RealClock{},
		listenerFactory: network.RealListenerFactory{},
	}
}

// SetClock sets the clock used for deadline calculations.
// Must be called before Start.
func (l *Listener) SetClock(c clock.Clock) {
	l.clock = c
}

// SetListenerFactory sets the factory used to create listeners.
// Must be called before Start.
func (l *Listener) SetListenerFactory(f network.ListenerFactory) {
	l.listenerFactory = f
}

// SetHandler sets the connection handler.
func (l *Listener) SetHandler(h ConnectionHandler) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.handler = h
}

// Running returns true if the listener is accepting connections.
func (l *Listener) Running() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.running
}

// Addr returns the listener's address, or nil if not listening.
func (l *Listener) Addr() net.Addr {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.listener != nil {
		return l.listener.Addr()
	}
	return nil
}

// Start begins listening with a background context.
func (l *Listener) Start() error {
	return l.StartWithContext(context.Background())
}

// StartWithContext begins listening with the given context.
func (l *Listener) StartWithContext(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.running {
		return ErrAlreadyListening
	}

	ln, err := l.listenerFactory.Listen(ctx, "tcp", l.addr)
	if err != nil {
		return err
	}

	l.listener = ln
	l.running = true
	l.ctx, l.cancel = context.WithCancel(ctx)

	l.wg.Add(1)
	go l.acceptLoop()

	return nil
}

// Stop signals the listener to stop accepting connections.
func (l *Listener) Stop() {
	l.mu.Lock()
	cancel := l.cancel
	ln := l.listener
	l.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if ln != nil {
		_ = ln.Close()
	}
}

// Wait waits for the listener to stop.
func (l *Listener) Wait(ctx context.Context) error {
	return syncutil.WaitGroupWait(ctx, &l.wg)
}

// acceptLoop accepts connections until stopped.
//
// Uses close-on-cancel pattern: a goroutine watches ctx.Done() and closes
// the net.Listener to unblock Accept(). This replaces the previous 100ms
// SetDeadline polling approach, providing instant cancellation response
// on all listener types (TCP, Unix, mock listeners for simulation).
func (l *Listener) acceptLoop() {
	defer l.wg.Done()
	defer l.cleanup()

	// Close listener on context cancellation to unblock Accept().
	go func() {
		<-l.ctx.Done()
		l.mu.RLock()
		ln := l.listener
		l.mu.RUnlock()
		if ln != nil {
			if err := ln.Close(); err != nil {
				// Expected: listener may already be closed by Stop().
				sessionLogger().Debug("listener close on cancel", "err", err)
			}
		}
	}()

	for {
		conn, err := l.listener.Accept()
		if err != nil {
			// Distinguish shutdown from transient accept errors.
			if l.ctx.Err() != nil {
				return // Shutdown requested — exit cleanly.
			}
			sessionLogger().Warn("accept error", "err", err, "addr", l.addr)
			continue
		}

		// Get handler
		l.mu.RLock()
		handler := l.handler
		l.mu.RUnlock()

		if handler != nil {
			go handler(conn)
		} else {
			if err := conn.Close(); err != nil {
				sessionLogger().Debug("close unhandled conn", "err", err)
			}
		}
	}
}

// cleanup runs when listener stops.
func (l *Listener) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.listener != nil {
		_ = l.listener.Close()
		l.listener = nil
	}
	l.running = false
	l.cancel = nil
}
