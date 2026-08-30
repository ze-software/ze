// Package main starts freeRtr and its raw Ethernet bridge inside the interop image.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

const (
	javaPath       = "java"
	routerJAR      = "/opt/freertr/rtr.jar"
	rawIntPath     = "/opt/freertr/rawInt.bin"
	hardwareConfig = "/etc/freertr/freertr-hw.txt"
	softwareConfig = "/etc/freertr/freertr-sw.txt"
	consoleAddress = "127.0.0.1:2323"
	workingDir     = "/tmp"
)

type child struct {
	cmd  *exec.Cmd
	done <-chan error
}

func main() {
	code, err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "freertr-entrypoint: %v\n", err)
	}
	if code != 0 {
		os.Exit(code)
	}
}

func run() (int, error) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)

	java, err := startChild(javaPath,
		"-Xmx2g", "-XX:+UseZGC", "-jar", routerJAR, "routercs", hardwareConfig, softwareConfig)
	if err != nil {
		return 1, fmt.Errorf("start java: %w", err)
	}

	if code, err := waitForTCP(java, consoleAddress, 30*time.Second, signals); err != nil {
		return 1, err
	} else if code != 0 {
		return code, nil
	}
	if err := configureEthernet(); err != nil {
		stopAndWait(java, syscall.SIGTERM)
		return 1, err
	}

	rawInt, err := startChild(rawIntPath,
		"eth0", "20002", "127.0.0.1", "20001", "127.0.0.1")
	if err != nil {
		stopAndWait(java, syscall.SIGTERM)
		return 1, fmt.Errorf("start rawInt: %w", err)
	}

	select {
	case err := <-java.done:
		stopAndWait(rawInt, syscall.SIGTERM)
		return exitCode(err), processResult("java", err)
	case err := <-rawInt.done:
		stopAndWait(java, syscall.SIGTERM)
		if err == nil {
			return 1, errors.New("rawInt exited while freeRtr was still running")
		}
		return exitCode(err), fmt.Errorf("rawInt exited while freeRtr was still running: %w", err)
	case received := <-signals:
		forward(java, received)
		forward(rawInt, received)
		waitOrKill(java)
		waitOrKill(rawInt)
		return signalExitCode(received), nil
	}
}

func startChild(program string, args ...string) (child, error) {
	cmd := exec.CommandContext(context.Background(), program, args...) //nolint:gosec // program is one of this file's own path constants
	cmd.Dir = workingDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return child{}, err
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	return child{cmd: cmd, done: done}, nil
}

// waitForTCP blocks until the freeRtr console accepts a TCP connection on
// address. It returns 0 when the console answered, and the process exit code
// when a signal stopped the wait first. signalExitCode never returns 0.
func waitForTCP(java child, address string, timeout time.Duration, signals <-chan os.Signal) (int, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	retry := time.NewTicker(250 * time.Millisecond)
	defer retry.Stop()
	dialer := net.Dialer{Timeout: 200 * time.Millisecond}

	for {
		conn, err := dialer.DialContext(context.Background(), "tcp", address)
		if err == nil {
			_ = conn.Close()
			return 0, nil
		}
		select {
		case processErr := <-java.done:
			if processErr == nil {
				return 0, errors.New("java exited before the freeRtr console listened on " + address)
			}
			return 0, fmt.Errorf("java exited before the freeRtr console listened on %s: %w", address, processErr)
		case <-deadline.C:
			stopAndWait(java, syscall.SIGTERM)
			return 0, fmt.Errorf("freeRtr console did not listen on %s within %s", address, timeout)
		case received := <-signals:
			stopAndWait(java, received)
			return signalExitCode(received), nil
		case <-retry.C:
		}
	}
}

func configureEthernet() error {
	for _, command := range [][]string{
		{"ip", "addr", "flush", "dev", "eth0"},
		{"ip", "link", "set", "eth0", "up", "promisc", "on"},
	} {
		cmd := exec.CommandContext(context.Background(), command[0], command[1:]...) //nolint:gosec // commands come from the fixed table above
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("run %q: %w", command, err)
		}
	}

	// Offload controls differ between container hosts. The old launcher treated
	// an unsupported control as harmless after the required link setup succeeded.
	cmd := exec.CommandContext(context.Background(), "ethtool", "-K", "eth0", "rx", "off", "tx", "off", "sg", "off", "tso", "off", "gso", "off", "gro", "off")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	_ = cmd.Run()
	return nil
}

func forward(process child, sig os.Signal) {
	if process.cmd != nil && process.cmd.Process != nil {
		_ = process.cmd.Process.Signal(sig)
	}
}

func stopAndWait(process child, sig os.Signal) {
	forward(process, sig)
	waitOrKill(process)
}

func waitOrKill(process child) {
	if process.cmd == nil || process.cmd.Process == nil {
		return
	}
	select {
	case <-process.done:
		return
	case <-time.After(5 * time.Second):
		_ = process.cmd.Process.Kill()
		<-process.done
	}
}

func processResult(name string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s exited: %w", name, err)
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			if status.Signaled() {
				return 128 + int(status.Signal())
			}
			return status.ExitStatus()
		}
	}
	return 1
}

func signalExitCode(sig os.Signal) int {
	if value, ok := sig.(syscall.Signal); ok {
		return 128 + int(value)
	}
	return 1
}
