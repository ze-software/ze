package main

import (
	"context"
	"net"
	"strconv"
	"testing"
)

// listenLoopback opens a listener on a free loopback port and closes it when
// the test ends. It answers the listener and its port.
func listenLoopback(t *testing.T) (net.Listener, string) {
	t.Helper()
	var config net.ListenConfig
	listener, err := config.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Logf("close: %v", err)
		}
	})
	address, isTCP := listener.Addr().(*net.TCPAddr)
	if !isTCP {
		t.Fatalf("the listener's address %v is not TCP", listener.Addr())
	}
	return listener, strconv.Itoa(address.Port)
}

// TestPublishedReadsTheServedLine: the host side's published check accepts a
// connection that serve answers, so the line it requires is the line the
// serving entrypoint writes.
func TestPublishedReadsTheServedLine(t *testing.T) {
	listener, port := listenLoopback(t)
	go serve(listener)
	if !published("127.0.0.1", port) {
		t.Fatal("published refused the line serve wrote")
	}
}

// TestPublishedFailsOnAnotherAnswer: a port that answers another line is a
// FAIL at once, so a service of the router that holds the port cannot pass
// for the container.
func TestPublishedFailsOnAnotherAnswer(t *testing.T) {
	listener, port := listenLoopback(t)
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close() //nolint:errcheck // the test judges the client's verdict
		if _, err := connection.Write([]byte("SSH-2.0-other\n")); err != nil {
			t.Errorf("write: %v", err)
		}
	}()
	if published("127.0.0.1", port) {
		t.Fatal("published accepted a line the container did not write")
	}
}

// TestRefusedJudgesTheAnswer: refused accepts a closed port, where the stack
// answers with a reset, and fails an open one.
func TestRefusedJudgesTheAnswer(t *testing.T) {
	open, openPort := listenLoopback(t)
	go serve(open)
	if refused("127.0.0.1", openPort) {
		t.Fatal("refused accepted a port that opened")
	}
	closed, closedPort := listenLoopback(t)
	if err := closed.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if !refused("127.0.0.1", closedPort) {
		t.Fatal("refused failed a closed port")
	}
}
