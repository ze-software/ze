package toolstub

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func appendCall(path string, args []string) error {
	if path == "" {
		return fmt.Errorf("recording path is empty")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600) //nolint:gosec // path is the recording file the fixture named in ZE_RECORD_*
	if err != nil {
		return err
	}
	_, writeErr := file.WriteString(strings.Join(args, "\x1f") + "\x1e")
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

// kernelProbe is the command the Docker kernel check (internal/le/interoplab,
// checkDockerKernel) runs in its probe container, and kernelProbeAnswer is a
// kernel that has every feature Ze enrolls, in the shape that command prints.
const (
	kernelProbe       = "doctor --json kernel-capabilities"
	kernelProbeAnswer = `{"ready":true,"capabilities":[{"subsystem":"netlink","kernel":"CONFIG_NET","state":"present"}]}`
)

// dockerKernelCheck answers the questions the Docker kernel check asks of the
// daemon, and false for every other call. The daemon runs on the machine the
// stub was built for, applies seccomp and no AppArmor as a Docker Desktop or
// colima daemon does, and its kernel has every feature Ze enrolls. An info
// format it does not know is answered empty, which the asker refuses, so a new
// question shows up as a red rather than as a pass.
func dockerKernelCheck(args []string) (string, bool) {
	if len(args) == 0 {
		return "", false
	}
	joined := strings.Join(args, " ")
	switch args[0] {
	case "version":
		return runtime.GOARCH, true
	case "info":
		if strings.Contains(joined, ".SecurityOptions") {
			return `["name=seccomp,profile=builtin"]`, true
		}
		if strings.Contains(joined, ".KernelVersion") {
			return "6.10.14-linuxkit", true
		}
		return "", true
	case "run":
		if strings.Contains(joined, kernelProbe) {
			return kernelProbeAnswer, true
		}
	}
	return "", false
}

// Run implements the external commands used by the le verify evidence fixture.
func Run(name string, args []string) (int, bool) {
	switch name {
	case "docker":
		// The kernel check's questions read the daemon and change nothing, so
		// they are answered before the recording, which holds only the calls
		// that pull, start, run in, or remove a container the proof asked for.
		if answer, asked := dockerKernelCheck(args); asked {
			fmt.Println(answer)
			return 0, true
		}
		if err := appendCall(os.Getenv("ZE_RECORD_DOCKER"), args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1, true
		}
		if len(args) == 0 {
			return 0, true
		}
		switch args[0] {
		case "image", "pull", "rm":
			return 0, true
		case "run":
			fmt.Println("deadbeef")
			code, err := strconv.Atoi(os.Getenv("ZE_DOCKER_EXIT"))
			if err != nil {
				code = 0
			}
			return code, true
		case "exec":
			joined := strings.Join(args, " ")
			switch {
			case strings.Contains(joined, "apk add"):
				return 0, true
			case strings.Contains(joined, "xl2tpd"):
				time.Sleep(3 * time.Second)
				return 0, true
			default:
				_, _ = io.Copy(io.Discard, os.Stdin)
				fmt.Fprintln(os.Stderr, "l2tp: L2TP listener bound on 127.0.0.1:1701")
				time.Sleep(time.Second)
				fmt.Fprintln(os.Stderr, "l2tp: tunnel 1 session established")
				time.Sleep(3 * time.Second)
				return 0, true
			}
		default:
			return 0, true
		}
	case "git":
		if strings.Contains(strings.Join(args, " "), "status") {
			fmt.Print(os.Getenv("ZE_GIT_STATUS"))
		}
		return 0, true
	case "ip":
		if err := appendCall(os.Getenv("ZE_RECORD_IP"), args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1, true
		}
		return 0, true
	case "ping", "xl2tpd", "pppd":
		return 0, true
	case "go":
		// Toolchain derivation (gen.Resolve) asks the go on PATH for its
		// GOROOT and GOVERSION. The fixture names both, so the patched tree
		// le activates is built over a stand-in GOROOT whose bin/go is this
		// stub, and the stub stays first on PATH after activation.
		if len(args) > 0 && args[0] == "env" {
			if len(args) == 1 {
				fmt.Fprintln(os.Stderr, "toolstub: go env with no key has no answer: the stub answers only named keys")
				return 1, true
			}
			for _, key := range args[1:] {
				value := os.Getenv("ZE_STUB_GO_" + key)
				if value == "" {
					fmt.Fprintln(os.Stderr, "toolstub: go env "+key+" has no answer: the fixture sets none in ZE_STUB_GO_"+key)
					return 1, true
				}
				fmt.Println(value)
			}
			return 0, true
		}
		for index, arg := range args {
			if index > 0 && args[index-1] == "-o" {
				if err := os.MkdirAll(filepath.Dir(arg), 0o750); err != nil {
					fmt.Fprintln(os.Stderr, err)
					return 1, true
				}
				if err := os.WriteFile(arg, nil, 0o755); err != nil { //nolint:gosec // stands in for a compiled binary, so it keeps its executable bits
					fmt.Fprintln(os.Stderr, err)
					return 1, true
				}
			}
		}
		return 0, true
	}
	return 0, false
}
