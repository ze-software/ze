// Design: docs/architecture/appliance/remote-operations.md -- config push to device via SSH
// Upstream: golang.org/x/crypto v0.58.0, ssh/channel.go and ssh/client.go;
// https://github.com/golang/crypto/tree/v0.58.0/ssh.

package appliance

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/ze-software/ze/internal/core/configupload"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var errSshAuthSockNotSetStart = errors.New("SSH_AUTH_SOCK not set (start ssh-agent or use eval $(ssh-agent))")

var errNoKnownHostsPath = errors.New("cannot locate known_hosts: $HOME is unset, so the appliance host key cannot be verified")

const (
	sshDialTimeout   = 10 * time.Second
	sshUploadTimeout = 60 * time.Second
	defaultSSHPort   = "22"
	knownHostsDir    = ".ssh"
	knownHostsFile   = "known_hosts"
	keyscanHintHead  = "ssh-keyscan -H "
)

// userKnownHostsPath returns the operator's OpenSSH known_hosts file. Appliance
// host keys are verified against the same file the operator's own ssh(1) uses,
// so there is no second trust store to keep in sync.
func userKnownHostsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, knownHostsDir, knownHostsFile)
}

// applianceHostKeyCallback verifies appliance host keys against the known_hosts
// file at path.
//
// It fails closed: an unreadable known_hosts file, a host that is not listed, a
// revoked key, and a key that does not match the pinned one all refuse the
// connection. An appliance presents a self-signed host key, which is the reason
// it must be pinned rather than trusted: there is no CA to fall back on, so an
// unverified key is an unauthenticated peer, and the config being pushed is
// readable by whoever answers.
func applianceHostKeyCallback(path string) (ssh.HostKeyCallback, error) {
	if path == "" {
		return nil, errNoKnownHostsPath
	}
	verify, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("read known_hosts %s: %w", path, err)
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := verify(hostname, remote, key)

		var keyErr *knownhosts.KeyError
		if !errors.As(err, &keyErr) {
			return err
		}
		// Want non-empty means the host IS pinned but presented a different key.
		// Never suggest re-scanning here: that would talk the operator through
		// overwriting the pin, which is the one thing an attacker needs.
		if len(keyErr.Want) > 0 {
			return fmt.Errorf("host key mismatch for %s: presented %s, which is not the key pinned in %s",
				hostname, ssh.FingerprintSHA256(key), path)
		}
		return fmt.Errorf("host key for %s not in %s: verify %s out of band, then pin it with %s",
			hostname, path, ssh.FingerprintSHA256(key), keyscanHint(hostname, path))
	}, nil
}

// keyscanHint builds the ssh-keyscan command that pins hostname into path.
func keyscanHint(hostname, path string) string {
	var tb textbuf.Buffer
	tb.Str(keyscanHintHead)

	host, port, err := net.SplitHostPort(hostname)
	switch {
	case err != nil:
		tb.Str(hostname)
	case port == defaultSSHPort:
		tb.Str(host)
	default:
		tb.Str("-p ").Str(port).Byte(' ').Str(host)
	}

	return tb.Str(" >> ").Str(path).String()
}

func init() {
	cmdConfigPush = runConfigPush
}

type sshResult struct {
	Output string
	Err    error
}

var sshExecFunc = sshExecReal

func runConfigPush(args []string) int {
	fs := flag.NewFlagSet("appliance config-push", flag.ContinueOnError)
	allFlag := fs.Bool("all", false, "Push config to all appliances with device.address set")
	dryRunFlag := fs.Bool("dry-run", false, "Print merged config without connecting")
	parallelFlag := fs.Int("parallel", 1, "Number of concurrent pushes (with --all)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ze appliance config-push [options] <name>\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  ze appliance config-push lab\n")
		fmt.Fprintf(os.Stderr, "  ze appliance config-push --dry-run lab\n")
		fmt.Fprintf(os.Stderr, "  ze appliance config-push --all\n")
		fmt.Fprintf(os.Stderr, "  ze appliance config-push --all --parallel 4\n")
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	if *allFlag {
		return configPushAll(*dryRunFlag, *parallelFlag)
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: requires <name> or --all\n")
		fs.Usage()
		return exitError
	}

	return configPushOne(fs.Arg(0), *dryRunFlag)
}

func configPushOne(name string, dryRun bool) int {
	dir := getBaseDir()

	cfg, err := LoadConfig(ConfigPath(dir, name))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	merged, err := resolveSeedConfig(dir, name, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	if merged == "" {
		fmt.Fprintf(os.Stderr, "error: no config to push (no base, no overlay)\n")
		return exitError
	}

	if dryRun {
		fmt.Print(merged)
		if merged[len(merged)-1] != '\n' {
			fmt.Println()
		}
		return exitOK
	}

	if cfg.Device.Address == "" {
		fmt.Fprintf(os.Stderr, "error: device %s has no address configured\n", name)
		return exitError
	}

	port := cfg.SSH.Port
	if port == "" {
		port = "22"
	}
	addr := net.JoinHostPort(cfg.Device.Address, port)
	user := cfg.Credentials.Username
	if user == "" {
		user = defaultUsername
	}

	result := sshExecFunc(addr, user, configupload.Subsystem, merged)
	if result.Err != nil {
		var message textbuf.Buffer
		message.Str("error: configuration upload to ").Str(name).Str(" at ").Str(addr).Str(": ").Err(result.Err).Byte('\n').StdErr() //nolint:errcheck // The command is returning failure.
		return exitError
	}

	fmt.Printf("config applied to %s\n", name)
	return exitOK
}

func configPushAll(dryRun bool, parallel int) int {
	names, code := listAddressedAppliances()
	if code != exitOK {
		return code
	}

	return runParallel(names, parallel, func(name string) int {
		fmt.Fprintf(os.Stderr, "pushing config to %s...\n", name)
		return configPushOne(name, dryRun)
	})
}

func sshExecReal(addr, user, command, stdin string) sshResult {
	if command != configupload.Subsystem {
		return sshResult{Err: opererr.New("configuration upload operation is unknown", "The host and device must use the configuration upload subsystem", "./ze-host appliance config-push --help")}
	}
	if len(stdin) == 0 {
		return sshResult{Err: opererr.New("configuration upload is empty", "The device requires a complete replacement configuration", "./ze-host appliance config-push --dry-run <name>")}
	}
	if len(stdin) > configupload.MaxBytes {
		return sshResult{Err: opererr.New("configuration upload exceeds four MiB", "The device refuses an oversized replacement before applying it", "./ze-host appliance config-push --dry-run <name>")}
	}
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return sshResult{Err: errSshAuthSockNotSetStart}
	}

	var d net.Dialer
	ctx, cancel := context.WithTimeout(context.Background(), sshDialTimeout)
	defer cancel()

	agentConn, err := d.DialContext(ctx, "unix", sock)
	if err != nil {
		return sshResult{Err: fmt.Errorf("connect to ssh-agent: %w", err)}
	}
	defer agentConn.Close() //nolint:errcheck // best-effort cleanup
	if err := agentConn.SetDeadline(time.Now().Add(sshDialTimeout)); err != nil {
		return sshResult{Err: fmt.Errorf("bound ssh-agent authentication: %w", err)}
	}

	agentClient := agent.NewClient(agentConn)

	hostKeys, err := applianceHostKeyCallback(userKnownHostsPath())
	if err != nil {
		return sshResult{Err: err}
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(agentClient.Signers),
		},
		HostKeyCallback: hostKeys,
		Timeout:         sshDialTimeout,
	}

	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return sshResult{Err: fmt.Errorf("connect to %s: %w", addr, err)}
	}
	defer conn.Close() //nolint:errcheck // Every failure closes the transport.
	if err := conn.SetDeadline(time.Now().Add(sshUploadTimeout)); err != nil {
		return sshResult{Err: fmt.Errorf("bound SSH configuration upload: %w", err)}
	}
	transport, channels, requests, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		return sshResult{Err: fmt.Errorf("authenticate device %s: %w", addr, err)}
	}
	client := ssh.NewClient(transport, channels, requests)
	defer func() {
		client.Close() //nolint:errcheck // Closing interrupts every pending SSH read/write.
		client.Wait()  //nolint:errcheck // Join the transport even after a protocol refusal.
	}()

	channel, channelRequests, err := client.OpenChannel("session", nil)
	if err != nil {
		return sshResult{Err: fmt.Errorf("create session: %w", err)}
	}
	defer channel.Close() //nolint:errcheck // The receiver may already have closed.
	receipt := sshUploadReceipt{}
	receipt.workers.Add(2)
	go receipt.readRequests(channelRequests)
	go receipt.readDiagnostic(channel.Stderr())
	defer func() {
		conn.Close() //nolint:errcheck // Unblock receipt workers on every error path.
		receipt.workers.Wait()
	}()
	accepted, err := channel.SendRequest("subsystem", true, ssh.Marshal(struct{ Name string }{configupload.Subsystem}))
	if err != nil {
		return sshResult{Err: fmt.Errorf("request device configuration upload: %w", err)}
	}
	if !accepted {
		return sshResult{Err: opererr.New("device refused the configuration upload subsystem", "The connected daemon does not provide this operation", "./ze-host appliance push <name>")}
	}
	if err := configupload.Write(channel, stdin); err != nil {
		return sshResult{Err: uncertainConfigUpload(addr, user, err)}
	}
	if err := channel.CloseWrite(); err != nil {
		return sshResult{Err: uncertainConfigUpload(addr, user, err)}
	}
	output, err := io.ReadAll(io.LimitReader(channel, sshReceiptLimit+1))
	if err != nil {
		return sshResult{Err: uncertainConfigUpload(addr, user, err)}
	}
	if len(output) > sshReceiptLimit {
		return sshResult{Err: uncertainConfigUpload(addr, user, errors.New("device receipt is oversized"))}
	}
	receipt.workers.Wait()
	if receipt.err != nil {
		return sshResult{Err: uncertainConfigUpload(addr, user, receipt.err)}
	}
	if !receipt.hasStatus {
		return sshResult{Err: uncertainConfigUpload(addr, user, errors.New("device closed without an exit status"))}
	}
	if receipt.status != 0 {
		return sshResult{Err: opererr.New("device refused the configuration upload", "No acceptance receipt was received; inspect the device configuration and its diagnostics before retrying", configUploadVerifyCommand(addr, user))}
	}
	if string(output) != configupload.Receipt {
		return sshResult{Err: uncertainConfigUpload(addr, user, errors.New("device did not return the configuration acceptance receipt"))}
	}
	return sshResult{Output: configupload.Receipt}
}

const sshReceiptLimit = 8192

// sshUploadReceipt owns the two independent incoming SSH streams. They are
// always joined after transport closure on failure, and after remote closure on
// success; no Session.Wait assumption is made for a raw subsystem channel.
type sshUploadReceipt struct {
	workers   sync.WaitGroup
	hasStatus bool
	status    uint32
	err       error
}

func (r *sshUploadReceipt) readRequests(requests <-chan *ssh.Request) {
	defer r.workers.Done()
	for request := range requests {
		if request.Type == "exit-status" {
			var payload struct{ Status uint32 }
			if err := ssh.Unmarshal(request.Payload, &payload); err != nil {
				r.err = errors.New("device returned a malformed upload exit status")
				continue
			}
			if r.hasStatus {
				r.err = errors.New("device returned duplicate upload exit statuses")
				continue
			}
			r.hasStatus = true
			r.status = payload.Status
		}
		if request.WantReply {
			request.Reply(false, nil) //nolint:errcheck // The transport deadline bounds this reply.
		}
	}
}

func (r *sshUploadReceipt) readDiagnostic(reader io.Reader) {
	defer r.workers.Done()
	// Diagnostics are deliberately not printed: a foreign or defective receiver
	// may echo the private configuration. The transport deadline bounds excess.
	io.Copy(io.Discard, io.LimitReader(reader, sshReceiptLimit+1)) //nolint:errcheck // Exit status and the exact receipt decide acceptance.
}

func configUploadVerifyCommand(addr, user string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "show config"
	}
	var command textbuf.Buffer
	return command.Str("ssh -p ").Str(port).Byte(' ').Str(user).Byte('@').Str(host).Str(" 'show config'").String()
}

func uncertainConfigUpload(addr, user string, cause error) error {
	return opererr.Wrap(cause, "configuration upload outcome is uncertain", "Delivery started but the device's acceptance was not confirmed; inspect the live configuration before another mutation", configUploadVerifyCommand(addr, user))
}
