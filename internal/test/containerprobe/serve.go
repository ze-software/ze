// Design: docs/architecture/container/runtime.md -- the test store and its fixture image
// Related: main.go -- the run-serving entrypoint and the published and refused modes
// Related: network.go -- the network facts the serving entrypoint also records

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// servePort is the port the serving entrypoint listens on inside the
// container. A fixed number is safe: it is bound in the container's own
// network namespace, which nothing else shares, and binding a port above
// 1023 needs no capability, so the default confinement allows it.
const servePort = 7070

// servedLine is the line the serving entrypoint answers every connection
// with, so a client that reads it was answered from inside the container.
const servedLine = "ze-containerprobe answers from inside the container"

// The bounds of the host side. A published port's table is installed before
// the container starts, so the wait covers a loaded guest, not a race.
const (
	dialTimeout   = time.Second
	dialStep      = 200 * time.Millisecond
	dialSteps     = 40 // with dialTimeout, at most 48 s when every attempt times out
	answerTimeout = 5 * time.Second
)

// listenInside opens the serving listener on every address of the container.
func listenInside(ctx context.Context) (net.Listener, error) {
	var config net.ListenConfig
	return config.Listen(ctx, "tcp", ":"+strconv.Itoa(servePort))
}

// serve answers servedLine to each connection, one at a time, until the
// listener fails. It is the serving entrypoint's one worker: it lives as long
// as the process, whose end is its stop path, and it answers each connection
// inline rather than in a goroutine of its own.
func serve(listener net.Listener) {
	for {
		connection, err := listener.Accept()
		if err != nil {
			fmt.Fprintln(os.Stderr, "probe serve: "+err.Error()) //nolint:errcheck // stderr is the only place left to say it
			return
		}
		answer(connection)
	}
}

// answer writes servedLine to one connection and closes it.
func answer(connection net.Conn) {
	defer connection.Close() //nolint:errcheck // the client judges the line it read, not the close
	if err := connection.SetWriteDeadline(time.Now().Add(answerTimeout)); err != nil {
		fmt.Fprintln(os.Stderr, "probe serve: "+err.Error()) //nolint:errcheck // stderr is the only place left to say it
		return
	}
	if _, err := connection.Write([]byte(servedLine + "\n")); err != nil {
		fmt.Fprintln(os.Stderr, "probe serve: "+err.Error()) //nolint:errcheck // stderr is the only place left to say it
	}
}

// published runs on the client's side: it dials ADDRESS:PORT until a
// connection opens, within a bounded wait, then requires the one line it
// reads to be servedLine. A connection that opens and answers anything else
// is a FAIL at once: something other than the container holds the port.
func published(address, port string) bool {
	target := net.JoinHostPort(address, port)
	var failure error
	for range dialSteps {
		connection, err := dialOnce(target)
		if err != nil {
			failure = err
			time.Sleep(dialStep)
			continue
		}
		fmt.Println("probe: connected to " + target)
		line, err := readAnswer(connection)
		if closeErr := connection.Close(); closeErr != nil {
			fmt.Println("probe: the connection did not close cleanly: " + closeErr.Error())
		}
		if err != nil {
			fmt.Println("probe: FAIL the connection carried the container's line: " + err.Error())
			return false
		}
		if line != servedLine {
			fmt.Println("probe: FAIL the connection carried the container's line: saw " + strconv.Quote(line))
			return false
		}
		fmt.Println("probe: the container answered " + strconv.Quote(line))
		return true
	}
	fmt.Println("probe: FAIL connected to " + target + ": " + failure.Error())
	return false
}

// refused runs on the client's side: one connection attempt to ADDRESS:PORT
// must be refused, which needs a packet there and a reset back. A timeout,
// an unreachable host or an open connection is a FAIL.
func refused(address, port string) bool {
	target := net.JoinHostPort(address, port)
	connection, err := dialOnce(target)
	if err == nil {
		if closeErr := connection.Close(); closeErr != nil {
			fmt.Println("probe: the connection did not close cleanly: " + closeErr.Error())
		}
		fmt.Println("probe: FAIL the connection to " + target + " was refused: it opened")
		return false
	}
	if !errors.Is(err, syscall.ECONNREFUSED) {
		fmt.Println("probe: FAIL the connection to " + target + " was refused: " + err.Error())
		return false
	}
	fmt.Println("probe: the connection to " + target + " was refused")
	return true
}

// dialOnce makes one TCP connection attempt bounded by dialTimeout.
func dialOnce(target string) (net.Conn, error) {
	return dialWithin(target, dialTimeout)
}

// dialWithin makes one TCP connection attempt that ends after wait.
func dialWithin(target string, wait time.Duration) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	var dialer net.Dialer
	return dialer.DialContext(ctx, "tcp", target)
}

// readAnswer reads one line, bounded in time.
func readAnswer(connection net.Conn) (string, error) {
	if err := connection.SetReadDeadline(time.Now().Add(answerTimeout)); err != nil {
		return "", err
	}
	line, err := bufio.NewReader(connection).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(line, "\n"), nil
}
