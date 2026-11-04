// Design: docs/architecture/testing/runner-architecture.md -- bounded native observer protocol.
package webtesting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// TestRenderObserverNativeResponses crosses a real local connection and checks
// the native response envelope, framing and bound. It is protocol evidence,
// not a substitute for Chromium installation or rendering evidence.
func TestRenderObserverNativeResponses(t *testing.T) {
	for _, test := range []struct {
		name     string
		response string
		want     string
	}{
		{"complete", "{\"id\":\"ze-render-observer\",\"success\":true,\"data\":{\"identifier\":\"native-script-1\"}}\n", ""},
		{"refused", "{\"id\":\"ze-render-observer\",\"success\":false,\"error\":\"owned registration refused\"}\n", "owned registration refused"},
		{"missing-verdict", "{\"id\":\"ze-render-observer\",\"data\":{}}\n", "no success verdict"},
		{"wrong-identity", "{\"id\":\"foreign-request\",\"success\":true,\"data\":{}}\n", "request identity"},
		{"truncated", "{\"id\":\"ze-render-observer\",\"success\":true}", "complete native observer response"},
		{"malformed", "not-json\n", "decode native observer response"},
		{"oversized", strings.Repeat("x", (1<<20)+1) + "\n", "exceeds 1 MiB"},
	} {
		t.Run(test.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			served := make(chan error, 1)
			go serveRenderObserverResponse(listener, test.response, served)
			defer func() {
				if err := listener.Close(); err != nil {
					t.Error(err)
				}
				if err := <-served; err != nil {
					if test.name == "oversized" {
						t.Logf("bounded reader closed the oversized peer response: %v", err)
					} else {
						t.Error(err)
					}
				}
			}()
			observer := &renderObserver{network: "tcp", address: listener.Addr().String()}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			data, err := observer.command(ctx, "addinitscript", "owned-observer-source")
			if test.want != "" {
				if err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("native protocol admitted %s: data=%s error=%v", test.name, data, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != `{"identifier":"native-script-1"}` {
				t.Fatalf("native response data changed: %s", data)
			}
		})
	}
}

func serveRenderObserverResponse(listener net.Listener, response string, served chan<- error) {
	served <- func() (err error) {
		connection, err := listener.Accept()
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, connection.Close()) }()
		if err := connection.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			return err
		}
		var request struct {
			ID     string `json:"id"`
			Action string `json:"action"`
			Script string `json:"script"`
		}
		if err := json.NewDecoder(io.LimitReader(connection, 4096)).Decode(&request); err != nil {
			return err
		}
		if request.ID != "ze-render-observer" || request.Action != "addinitscript" || request.Script != "owned-observer-source" {
			return fmt.Errorf("native observer request does not match its wire contract: %+v", request)
		}
		_, err = io.WriteString(connection, response)
		return err
	}()
}

// TestRenderObserverCancellationInterruptsRead cancels only after an actual
// connection has delivered its request. Return must preserve cancellation,
// finish the callback, and close the connection while the peer withholds a reply.
func TestRenderObserverCancellationInterruptsRead(t *testing.T) {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := listener.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	observer := &renderObserver{network: "tcp", address: listener.Addr().String()}
	result := make(chan error, 1)
	go func() {
		_, err := observer.command(ctx, "addinitscript", "held-observer-source")
		result <- err
	}()
	joined := false
	defer func() {
		if !joined {
			cancel()
			<-result
		}
	}()
	connection, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := connection.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := connection.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var request map[string]json.RawMessage
	if err := json.NewDecoder(io.LimitReader(connection, 4096)).Decode(&request); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-result:
		joined = true
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("native read lost cancellation: %v", err)
		}
	case <-time.After(time.Second):
		t.Error("native read ignored cancellation while peer withheld response")
		<-result
		joined = true
	}
	var next [1]byte
	if _, err := connection.Read(next[:]); !errors.Is(err, io.EOF) {
		t.Fatalf("native command returned without closing its connection: %v", err)
	}
}
