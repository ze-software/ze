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

// configPushSession owns the transport, cancellation callback and session.
// It MUST close the transport and join a running callback before returning.
func (guest *otaGuest) configPushSession(ctx context.Context, user string, auth ssh.AuthMethod, verify ssh.HostKeyCallback, operation func(*ssh.Session) error) (resultErr error) {
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
		connection.Close() //nolint:errcheck // The session result owns transport errors.
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
	defer client.Close() //nolint:errcheck // The session result owns transport errors.
	session, err := client.NewSession()
	if err != nil {
		return errors.New("native SSH command session could not be opened")
	}
	defer session.Close() //nolint:errcheck // The operation result owns session errors.
	return operation(session)
}

func (guest *otaGuest) configPushSSH(ctx context.Context, user string, auth ssh.AuthMethod, verify ssh.HostKeyCallback, command string) (string, error) {
	var output configPushCapture
	err := guest.configPushSession(ctx, user, auth, verify, func(session *ssh.Session) error {
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

func (guest *otaGuest) configPushLive(ctx context.Context, signer ssh.Signer, verify ssh.HostKeyCallback, hostname string) (string, error) {
	for _, auth := range []ssh.AuthMethod{ssh.Password(guest.installer.Options.SSHPassword), ssh.PublicKeys(signer)} {
		output, err := guest.configPushSSH(ctx, guest.installer.Options.SSHUser, auth, verify, HostnameQuery)
		if err != nil {
			return "", err
		}
		actual, err := hostnameOf(output)
		if err != nil {
			return "", errors.New("native SSH hostname response is invalid")
		}
		if actual != hostname {
			return "", errors.New("native SSH reports an unexpected live hostname")
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
	return guest.configPushSession(ctx, guest.installer.Options.SSHUser, ssh.PublicKeys(signer), verify, func(session *ssh.Session) error {
		input, err := session.StdinPipe()
		if err != nil {
			return err
		}
		var output, diagnostic configPushCapture
		session.Stdout, session.Stderr = &output, &diagnostic
		if err := session.RequestSubsystem(configupload.Subsystem); err != nil {
			return errors.New("malformed control could not open the actual configuration upload adapter")
		}
		if err := configupload.Write(input, "set "+private+" true\n"); err != nil {
			return errors.New("malformed control could not deliver its complete frame")
		}
		if err := input.Close(); err != nil {
			return errors.New("malformed control could not complete its SSH input")
		}
		err = session.Wait()
		exit, ok := errors.AsType[*ssh.ExitError](err)
		if !ok {
			return errors.New("malformed control has no remote nonzero exit status")
		}
		if exit.ExitStatus() == 0 {
			return errors.New("malformed control returned a zero exit status")
		}
		for _, data := range [][]byte{output.data, diagnostic.data} {
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
