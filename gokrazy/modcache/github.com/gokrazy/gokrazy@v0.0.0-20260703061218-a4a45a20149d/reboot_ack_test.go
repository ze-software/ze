package gokrazy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestSynchronousPowerAcknowledgesBeforeShutdown crosses a real HTTP exchange.
// The shutdown callback stands in for service termination and interface removal.
// Acceptance must reach the client before it blocks, and power must wait for it.
func TestSynchronousPowerAcknowledgesBeforeShutdown(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	returned, powered := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	stopShutdown := func() { releaseOnce.Do(func() { close(release) }) }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		powerHandlerWithShutdown(w, r, "reboot", "Rebooting", func() error {
			close(powered)
			return nil
		}, func(time.Duration) {
			close(entered)
			<-release
			close(returned)
		})
	}))
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	type answer struct {
		response *http.Response
		err      error
	}
	result := make(chan answer, 1)
	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		response, err := server.Client().Do(request)
		result <- answer{response: response, err: err}
	}()
	t.Cleanup(func() {
		cancel()
		stopShutdown()
		server.Close()
		<-clientDone
		select {
		case <-entered:
			<-powered
		default:
		}
	})
	select {
	case <-entered:
	case <-ctx.Done():
		stopShutdown()
		t.Fatal("synchronous shutdown callback did not start")
	}
	select {
	case result := <-result:
		if result.err != nil {
			stopShutdown()
			t.Fatalf("no reboot acceptance before shutdown: %v", result.err)
		}
		if result.response.StatusCode != http.StatusOK {
			stopShutdown()
			t.Fatalf("reboot acceptance status = %d", result.response.StatusCode)
		}
		if err := result.response.Body.Close(); err != nil {
			stopShutdown()
			t.Fatal(err)
		}
	case <-ctx.Done():
		stopShutdown()
		t.Fatal("reboot acceptance waited for service shutdown")
	}
	select {
	case <-powered:
		stopShutdown()
		t.Fatal("power operation ran before service shutdown returned")
	default:
	}
	stopShutdown()
	select {
	case <-returned:
	case <-ctx.Done():
		t.Fatal("controlled shutdown did not return")
	}
	select {
	case <-powered:
	case <-ctx.Done():
		t.Fatal("accepted reboot never reached its power operation")
	}
}
