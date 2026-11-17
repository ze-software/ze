// Design: docs/architecture/command-ownership.md -- unsupported browser results end the joined request.
// Related: view_browser_test.go -- registered view success and busy command-worker cancellation.

package cli

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"testing/synctest"

	tea "charm.land/bubbletea/v2"
)

// browserWrongModelView owns one request-bound producer. The runner starts it
// with the view, and release MUST join it after request cancellation.
type browserWrongModelView struct {
	canceled        chan struct{}
	producerRelease chan struct{}
	producerDone    chan struct{}
	released        chan struct{}
}

func (*browserWrongModelView) update(*Model, tea.Msg) (tea.Model, tea.Cmd) {
	return &browserViewModel{}, nil
}

func (*browserWrongModelView) render(*Model) string    { return "synthetic live view" }
func (*browserWrongModelView) key(*Model, string) bool { return false }
func (*browserWrongModelView) problem(*Model) string   { return "" }

func (v *browserWrongModelView) release() {
	<-v.producerDone
	close(v.released)
}

// TestBrowserViewUnsupportedModelCancelsAndJoins drives a registered view's
// wrong concrete model through the real runner. Cancellation must reach its
// producer, return must wait for release, and the model's actual Close failure
// must survive alongside the unsupported-model diagnosis. Synctest observes
// blocked ownership rather than allowing a scheduling delay to prove the join.
func TestBrowserViewUnsupportedModelCancelsAndJoins(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		view := &browserWrongModelView{
			canceled:        make(chan struct{}),
			producerRelease: make(chan struct{}),
			producerDone:    make(chan struct{}),
			released:        make(chan struct{}),
		}
		var producerRelease sync.Once
		release := func() { producerRelease.Do(func() { close(view.producerRelease) }) }
		defer release()
		defer cancel()
		const key = "test-browser-unsupported-model"
		const command = "monitor synthetic-unsupported-model"
		var bound context.Context
		RegisterView(viewSpec{
			key: key, prefix: command,
			browser: func(*Model) []browserViewControl {
				return []browserViewControl{browserViewStopControl()}
			},
			start: func(m *Model, _ string) tea.Cmd {
				m.activeView = view
				go func() {
					<-bound.Done()
					close(view.canceled)
					<-view.producerRelease
					close(view.producerDone)
				}()
				return func() tea.Msg { return dashboardTickMsg{} }
			},
		})
		defer unregisterView(key)
		closeCause := errors.New("history publication failed during browser cleanup")
		done := make(chan error, 1)
		go func() {
			done <- RunBrowserView(ctx, command, func(request context.Context, m *Model) error {
				bound = request
				m.SetHistory(NewHistory(&failingHistoryPublication{failure: closeCause}, key))
				m.recordHistory("show bgp")
				return nil
			}, make(chan BrowserViewAction, 1), make(chan BrowserViewFrame, 1))
		}()
		synctest.Wait()
		select {
		case <-view.canceled:
		default:
			t.Fatal("unsupported model did not cancel the request-bound producer")
		}
		select {
		case err := <-done:
			t.Fatalf("runner returned before its producer joined: %v", err)
		default:
		}
		release()
		synctest.Wait()
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("unsupported model was accepted")
			}
			if !strings.Contains(err.Error(), "live view returned an unsupported model") {
				t.Fatalf("unsupported-model cause lost: %v", err)
			}
			if !errors.Is(err, closeCause) {
				t.Fatalf("model Close cause lost: %v", err)
			}
		default:
			t.Fatal("runner did not return after its producer exited")
		}
		select {
		case <-view.released:
		default:
			t.Fatal("runner returned without releasing the active view")
		}
		if ctx.Err() != nil {
			t.Fatal("test caller cancellation caused the failure")
		}
	})
}
