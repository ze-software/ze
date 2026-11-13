// Design: docs/architecture/testing/qemu-integration.md -- pinned native configuration observations.
// Related: install_config_push.go -- the three-boot proof sequence.
package testqemu

import (
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/configupload"
)

// configPushCapture is an initially empty bounded response. Its zero value is
// ready to accept at most one configuration-sized response from the guest.
type configPushCapture struct{ data []byte }

func (capture *configPushCapture) Write(data []byte) (int, error) {
	if len(data) > configupload.MaxBytes-len(capture.data) {
		return 0, errors.New("native SSH response exceeds four MiB")
	}
	capture.data = append(capture.data, data...)
	return len(data), nil
}

func (guest *otaGuest) configPushAddress() string {
	return net.JoinHostPort(forwardHost, strconv.Itoa(guest.sshPort))
}

// configPushClient owns the transport, cancellation callback and client.
// It MUST close the transport and join a running callback before returning.
func (guest *otaGuest) configPushClient(ctx context.Context, user string, auth ssh.AuthMethod, verify ssh.HostKeyCallback, operation func(*ssh.Client) error) (resultErr error) {
	deadline, cancel := context.WithTimeout(ctx, otaUploadTimeout)
	defer cancel()
	address := guest.configPushAddress()
	connection, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(deadline, "tcp", address)
	if err != nil {
		return errors.New("native pinned SSH connection could not be opened")
	}
	callbackDone := make(chan struct{})
	stopCallback := context.AfterFunc(deadline, func() {
		defer close(callbackDone)
		connection.Close() //nolint:errcheck // Cancellation owns transport shutdown.
	})
	defer func() {
		if !stopCallback() {
			<-callbackDone
		}
		connection.Close() //nolint:errcheck // The operation result owns transport errors.
		if deadline.Err() != nil {
			resultErr = errors.Join(resultErr, deadline.Err())
		}
	}()
	if end, ok := deadline.Deadline(); ok {
		if err := connection.SetDeadline(end); err != nil {
			return err
		}
	}
	transport, channels, requests, err := ssh.NewClientConn(connection, address, &ssh.ClientConfig{User: user, Auth: []ssh.AuthMethod{auth}, HostKeyCallback: verify, Timeout: 5 * time.Second})
	if err != nil {
		return errors.New("native SSH identity or host pin was refused")
	}
	client := ssh.NewClient(transport, channels, requests)
	defer client.Close() //nolint:errcheck // The operation result owns transport errors.
	return operation(client)
}

func (guest *otaGuest) configPushSSH(ctx context.Context, user string, auth ssh.AuthMethod, verify ssh.HostKeyCallback, command string) (string, error) {
	var output configPushCapture
	err := guest.configPushClient(ctx, user, auth, verify, func(client *ssh.Client) error {
		session, err := client.NewSession()
		if err != nil {
			return errors.New("native SSH command session could not be opened")
		}
		defer session.Close() //nolint:errcheck // The command result owns session errors.
		session.Stdin = strings.NewReader("")
		session.Stdout, session.Stderr = &output, io.Discard
		if err := session.Run(command); err != nil {
			return errors.New("native SSH observation command failed")
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return string(output.data), nil
}

// configPushHostname proves one fresh pinned login: the user authenticates, is
// allowed to run the hostname query, and the guest answers the expected name.
func (guest *otaGuest) configPushHostname(ctx context.Context, user string, auth ssh.AuthMethod, verify ssh.HostKeyCallback, hostname string) error {
	output, err := guest.configPushSSH(ctx, user, auth, verify, HostnameQuery)
	if err != nil {
		return err
	}
	actual, err := hostnameOf(output)
	if err != nil {
		return errors.New("native SSH hostname response is invalid")
	}
	if actual != hostname {
		return errors.New("native SSH reports an unexpected live hostname")
	}
	return nil
}

func (guest *otaGuest) configPushLive(ctx context.Context, signer ssh.Signer, verify ssh.HostKeyCallback, hostname string) (string, error) {
	for _, auth := range []ssh.AuthMethod{ssh.Password(guest.installer.Options.SSHPassword), ssh.PublicKeys(signer)} {
		if err := guest.configPushHostname(ctx, guest.installer.Options.SSHUser, auth, verify, hostname); err != nil {
			return "", err
		}
	}
	content, err := guest.configPushSSH(ctx, guest.installer.Options.SSHUser, ssh.PublicKeys(signer), verify, "show config")
	if err != nil {
		return "", err
	}
	if err := configPushHost(content, hostname); err != nil {
		return "", err
	}
	return content, nil
}

func configPushUnchanged(before, after string) error {
	previous, err := config.ParseTreeForValidation(before)
	if err != nil {
		return errors.New("native proof baseline is not a complete configuration")
	}
	current, err := config.ParseTreeForValidation(after)
	if err != nil {
		return errors.New("native proof observation is not a complete configuration")
	}
	left, err := configPushSerialize(previous)
	if err != nil {
		return err
	}
	right, err := configPushSerialize(current)
	if err != nil {
		return err
	}
	if left != right {
		return errors.New("native proof configuration changed outside the accepted publication")
	}
	return nil
}

func (guest *otaGuest) configPushUnchanged(ctx context.Context, signer ssh.Signer, verify ssh.HostKeyCallback, baseline string) error {
	observed, err := guest.configPushLive(ctx, signer, verify, configPushAcceptedHost)
	if err != nil {
		return err
	}
	return configPushUnchanged(baseline, observed)
}

func (guest *otaGuest) configPushMalformed(ctx context.Context, signer ssh.Signer, verify ssh.HostKeyCallback) error {
	const private = "native-proof-private-malformed-payload"
	return guest.configPushClient(ctx, guest.installer.Options.SSHUser, ssh.PublicKeys(signer), verify, func(client *ssh.Client) error {
		upload, err := configPushExchange(client, "set "+private+" true\n")
		if err != nil {
			return errors.Join(errors.New("malformed control did not complete the actual configuration upload exchange"), err)
		}
		if upload.status == 0 {
			return errors.New("malformed control returned a zero exit status")
		}
		for _, data := range [][]byte{upload.output, upload.diagnostic} {
			if strings.Contains(string(data), configupload.Receipt) {
				return errors.New("malformed control returned a success receipt")
			}
			if strings.Contains(string(data), private) {
				return errors.New("malformed control returned its private payload")
			}
		}
		return nil
	})
}

// configPushUpload is the device's complete answer to one upload exchange:
// its exit status and everything it wrote on both streams.
type configPushUpload struct {
	output, diagnostic []byte
	status             uint32
}

// configPushExchange sends one framed configuration over a raw session channel
// of an authenticated client and returns the device's own answer. It follows
// the product client (internal/appliance, sshExecReal): x/crypto's
// Session.RequestSubsystem never starts the session's workers, so Session.Wait
// answers a local "session not started" error that says nothing about the
// device. An error means the exchange has no device exit status; a refusal by
// the device is a nil error with a nonzero status.
//
// It joins both stream workers before returning. On failure it MUST close the
// client to unblock them, so the caller's client is unusable after an error.
func configPushExchange(client *ssh.Client, content string) (upload configPushUpload, resultErr error) {
	channel, requests, err := client.OpenChannel("session", nil)
	if err != nil {
		return upload, errors.New("configuration upload session channel could not be opened")
	}
	defer channel.Close() //nolint:errcheck // The device may already have closed.
	answer := configPushAnswer{requests: requests, stderr: channel.Stderr()}
	answer.workers.Go(answer.readRequests)
	answer.workers.Go(answer.readDiagnostic)
	defer func() {
		if resultErr != nil {
			client.Close() //nolint:errcheck // Closing the transport unblocks both workers.
		}
		answer.workers.Wait()
	}()
	accepted, err := channel.SendRequest("subsystem", true, ssh.Marshal(struct{ Name string }{configupload.Subsystem}))
	if err != nil {
		return upload, errors.New("configuration upload subsystem request was not answered")
	}
	if !accepted {
		return upload, errors.New("device does not provide the configuration upload subsystem")
	}
	if err := configupload.Write(channel, content); err != nil {
		return upload, errors.New("configuration upload frame was not delivered completely")
	}
	if err := channel.CloseWrite(); err != nil {
		return upload, errors.New("configuration upload input could not be completed")
	}
	var output configPushCapture
	if _, err := io.Copy(&output, channel); err != nil {
		return upload, errors.New("configuration upload response could not be read completely")
	}
	answer.workers.Wait()
	if answer.err != nil {
		return upload, answer.err
	}
	if answer.diagnosticErr != nil {
		return upload, answer.diagnosticErr
	}
	if !answer.hasStatus {
		return upload, errors.New("device closed the configuration upload without an exit status")
	}
	return configPushUpload{output: output.data, diagnostic: answer.diagnostic.data, status: answer.status}, nil
}

// configPushAnswer owns the two incoming streams of one upload channel. Each
// worker writes only its own fields; read them only after workers.Wait.
type configPushAnswer struct {
	requests      <-chan *ssh.Request
	stderr        io.Reader
	workers       sync.WaitGroup
	diagnostic    configPushCapture
	diagnosticErr error
	hasStatus     bool
	status        uint32
	err           error
}

// readRequests runs until the device or the transport closes the channel.
func (answer *configPushAnswer) readRequests() {
	for request := range answer.requests {
		if request.Type == "exit-status" {
			answer.recordStatus(request.Payload)
		}
		if request.WantReply {
			request.Reply(false, nil) //nolint:errcheck // The transport deadline bounds this reply.
		}
	}
}

func (answer *configPushAnswer) recordStatus(payload []byte) {
	var decoded struct{ Status uint32 }
	if err := ssh.Unmarshal(payload, &decoded); err != nil {
		answer.err = errors.New("device returned a malformed upload exit status")
		return
	}
	if answer.hasStatus {
		answer.err = errors.New("device returned duplicate upload exit statuses")
		return
	}
	answer.hasStatus, answer.status = true, decoded.Status
}

// readDiagnostic runs until the device's stderr ends or exceeds the capture.
func (answer *configPushAnswer) readDiagnostic() {
	if _, err := io.Copy(&answer.diagnostic, answer.stderr); err != nil {
		answer.diagnosticErr = errors.New("configuration upload diagnostic could not be read completely")
	}
}
