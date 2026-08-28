package fixture

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
)

func init() {
	Register("plugin/exec-answer-unconditional", fixture06ExecAnswerDriver)
}

func fixture06ExecAnswerDriver(ctx context.Context, _ []string) error {
	if err := os.WriteFile("daemon.ready", nil, 0o600); err != nil {
		return fmt.Errorf("signal fixture readiness: %w", err)
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate SSH key: %w", err)
	}
	privateBlock, err := ssh.MarshalPrivateKey(privateKey, "ze functional fixture")
	if err != nil {
		return fmt.Errorf("marshal SSH private key: %w", err)
	}
	if err := os.WriteFile("testkey", pem.EncodeToMemory(privateBlock), 0o600); err != nil {
		return fmt.Errorf("write SSH private key: %w", err)
	}
	sshPublicKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("marshal SSH public key: %w", err)
	}
	publicFields := strings.Fields(string(ssh.MarshalAuthorizedKey(sshPublicKey)))
	if len(publicFields) != 2 {
		return fmt.Errorf("authorized key has %d fields, want 2", len(publicFields))
	}
	if err := os.WriteFile("testkey.pub", ssh.MarshalAuthorizedKey(sshPublicKey), 0o644); err != nil {
		return fmt.Errorf("write SSH public key: %w", err)
	}

	config := fmt.Sprintf(`bgp {
	peer peer1 {
		connection {
			remote {
				ip 127.0.0.2
			}
			local {
				ip 127.0.0.1
				accept false
			}
		}
		session {
			asn {
				local 65533
				remote 65533
			}
		}
	}
}

system {
	authentication {
		user keyuser {
			password "$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO"
			profile [ admin ]
			public-keys testkey {
				type ssh-ed25519
				key %s
			}
		}
	}
	authorization {
		profile admin {
			run {
				default-action allow
			}
			edit {
				default-action allow
			}
		}
	}
}

environment {
	ssh {
		enabled true
		server main {
			ip 127.0.0.1;
			port 0;
		}
	}
}
`, publicFields[1])
	if err := os.WriteFile("exec-answer-unconditional.conf", []byte(config), 0o600); err != nil {
		return fmt.Errorf("write daemon config: %w", err)
	}

	logFile, err := os.OpenFile("daemon.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create daemon log: %w", err)
	}
	defer logFile.Close()

	daemon := exec.Command("ze", "start", "exec-answer-unconditional.conf")
	daemon.Stdout = os.Stdout
	daemon.Stderr = logFile
	daemon.Env = append(os.Environ(), "ze_test_bgp_port="+strconv.Itoa(10000+os.Getpid()%50000))
	if err := daemon.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	var daemonErr error
	daemonDone := make(chan struct{})
	go func() {
		daemonErr = daemon.Wait()
		close(daemonDone)
	}()
	defer fixture06StopDaemon(daemon, daemonDone)

	addressPattern := regexp.MustCompile(`127\.0\.0\.1:([0-9]+)`)
	var sshPort string
	var daemonExited bool
	if !Poll(ctx, 50, 200*time.Millisecond, func() bool {
		select {
		case <-daemonDone:
			daemonExited = true
			return true
		default:
		}
		logData, readErr := os.ReadFile("daemon.log")
		if readErr != nil {
			return false
		}
		match := addressPattern.FindSubmatch(logData)
		if len(match) == 2 {
			sshPort = string(match[1])
			return true
		}
		return false
	}) || sshPort == "" {
		logData, _ := os.ReadFile("daemon.log")
		if daemonExited {
			if daemonErr != nil {
				return fmt.Errorf("daemon exited before SSH server started: %w\n%s", daemonErr, logData)
			}
			return fmt.Errorf("daemon exited before SSH server started: <nil>\n%s", logData)
		}
		return fmt.Errorf("SSH server did not start (no address in daemon.log)\n%s", logData)
	}
	fmt.Fprintf(os.Stderr, "SSH port: %s\n", sshPort)

	if err := fixture06Wait(ctx, 500*time.Millisecond); err != nil {
		return err
	}

	ask := func(name, command string, wantedExit int) error {
		bodyPath := name + ".body"
		framePath := name + ".frame"
		bodyFile, err := os.OpenFile(bodyPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		frameFile, err := os.OpenFile(framePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			bodyFile.Close()
			return err
		}
		client := exec.CommandContext(ctx, "ssh",
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			"-o", "LogLevel=ERROR",
			"-o", "PasswordAuthentication=no",
			"-o", "BatchMode=yes",
			"-o", "IdentitiesOnly=yes",
			"-i", "./testkey",
			"-p", sshPort,
			"keyuser@127.0.0.1", command,
		)
		client.Env = fixture06WithoutEnv(os.Environ(), "SSH_AUTH_SOCK")
		client.Stdout = bodyFile
		client.Stderr = frameFile
		runErr := client.Run()
		bodyCloseErr := bodyFile.Close()
		frameCloseErr := frameFile.Close()
		if bodyCloseErr != nil || frameCloseErr != nil {
			return errors.Join(bodyCloseErr, frameCloseErr)
		}
		exitCode := 0
		if runErr != nil {
			var exitErr *exec.ExitError
			if !errors.As(runErr, &exitErr) {
				return fmt.Errorf("run SSH command %q: %w", command, runErr)
			}
			exitCode = exitErr.ExitCode()
		}
		if exitCode != wantedExit {
			frame, _ := os.ReadFile(framePath)
			return fmt.Errorf("%s exited %d, want %d\n%s", command, exitCode, wantedExit, frame)
		}
		frameInfo, err := os.Stat(framePath)
		if err != nil {
			return err
		}
		if frameInfo.Size() == 0 {
			return fmt.Errorf("%s received an empty frame; every peer must be framed without negotiation", command)
		}
		return fixture06FrameCheck(ctx, []string{name, framePath, bodyPath})
	}

	for _, test := range []struct {
		name       string
		command    string
		wantedExit int
	}{
		{name: "document", command: "show version", wantedExit: 0},
		{name: "streamed", command: "system command list | ndjson", wantedExit: 0},
		{name: "unknown", command: "shwo bgp peers", wantedExit: 1},
		{name: "failed", command: "show pki certificate name Жé", wantedExit: 1},
	} {
		if err := ask(test.name, test.command, test.wantedExit); err != nil {
			return err
		}
	}

	fmt.Fprintln(os.Stderr, "OK: a client that declares nothing read every frame by arithmetic")
	return nil
}

func fixture06WithoutEnv(environment []string, key string) []string {
	prefix := key + "="
	filtered := make([]string, 0, len(environment))
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func fixture06StopDaemon(command *exec.Cmd, done <-chan struct{}) {
	if command.Process == nil {
		return
	}
	_ = command.Process.Signal(syscall.SIGTERM)
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		_ = command.Process.Kill()
		<-done
	}
}
