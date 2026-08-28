package toolstub

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func appendCall(path string, args []string) error {
	if path == "" {
		return fmt.Errorf("recording path is empty")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
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

// Run implements the external commands used by the le evidence fixture.
func Run(name string, args []string) (int, bool) {
	switch name {
	case "docker":
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
		for index, arg := range args {
			if index > 0 && args[index-1] == "-o" {
				if err := os.MkdirAll(filepath.Dir(arg), 0o755); err != nil {
					fmt.Fprintln(os.Stderr, err)
					return 1, true
				}
				if err := os.WriteFile(arg, nil, 0o755); err != nil {
					fmt.Fprintln(os.Stderr, err)
					return 1, true
				}
			}
		}
		return 0, true
	}
	return 0, false
}
