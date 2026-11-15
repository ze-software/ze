// Design: docs/guide/bmp.md -- receiver startup and reload bind all or none.
// Related: register_static_rollback.go -- generation-fenced real daemon reload.

package fixture

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/bmp-bind-holder", bmpBindHolder)
	Register("plugin/bmp-bind-free", bmpBindFree)
	Register("plugin/bmp-bind-reload", bmpBindReload, StopsDaemon)
}

// bmpBindHolder owns only its leased socket. The runner's ready fence observes
// the line after Listen succeeds, before it starts the conflicting daemon.
func bmpBindHolder(ctx context.Context, args []string) (result error) {
	if len(args) != 1 {
		return errors.New("BMP bind holder requires one runner-leased port")
	}
	address, err := bmpBindAddress(args[0])
	if err != nil {
		return err
	}
	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("hold BMP conflict endpoint %s: %w", address, err)
	}
	defer func() { result = errors.Join(result, listener.Close()) }()
	if _, err := fmt.Fprintln(os.Stdout, "BMP-BIND-HELD", listener.Addr()); err != nil {
		return err
	}
	<-ctx.Done()
	return nil
}

func bmpBindFree(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("BMP free-port assertion requires one runner-leased port")
	}
	address, err := bmpBindAddress(args[0])
	if err != nil {
		return err
	}
	if err := bmpBindRequireFree(ctx, address); err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout, "BMP bind release verified at", address)
	return err
}

// bmpBindReload observes the running config and real session admission. A
// rejected reload must leave both intact before a valid repair can apply.
func bmpBindReload(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("BMP reload observer requires old, blocked and added runner-leased ports")
	}
	oldAddress, err := bmpBindAddress(args[0])
	if err != nil {
		return err
	}
	blockedAddress, err := bmpBindAddress(args[1])
	if err != nil {
		return err
	}
	addedAddress, err := bmpBindAddress(args[2])
	if err != nil {
		return err
	}
	return Observe(ctx, "bmp-bind-reload", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		return bmpBindReloadScenario(ctx, plugin, oldAddress, blockedAddress, addedAddress)
	})
}

func bmpBindReloadScenario(ctx context.Context, plugin *sdk.Plugin, oldAddress, blockedAddress, addedAddress string) error {
	first, err := bmpBindDial(ctx, oldAddress)
	if err != nil {
		return err
	}
	defer first.Close() //nolint:errcheck // fixture teardown; protocol checks carry the verdict
	if err := bmpBindIdentity(ctx, plugin, first, "before-reload"); err != nil {
		return err
	}
	before, err := done03(ctx, plugin, "show config running")
	if err != nil {
		return err
	}
	// Post-startup plugin callbacks can precede daemon readiness. The runner
	// can publish a PID after its ready wait times out, so require both files.
	for _, path := range []string{fileDaemonReady, fileDaemonPID} {
		if !waitForFile(ctx, path, WaitAttempts(30, 100*time.Millisecond, 100), 100*time.Millisecond) {
			return fmt.Errorf("BMP reload observer did not receive %s before signaling", path)
		}
	}
	pid, err := daemonPID()
	if err != nil {
		return err
	}
	refused, err := os.ReadFile("bmp-bind-refused.conf")
	if err != nil {
		return err
	}
	if err := staticRollbackReload(ctx, plugin, pid, string(refused), "failed"); err != nil {
		return fmt.Errorf("occupied endpoint %s was not refused: %w", blockedAddress, err)
	}
	after, err := done03(ctx, plugin, "show config running")
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(before, after) {
		return fmt.Errorf("refused BMP reload changed the running config: before=%v after=%v", before, after)
	}
	if err := bmpBindIdentity(ctx, plugin, first, "after-refusal"); err != nil {
		return err
	}
	if err := bmpBindRequireFree(ctx, addedAddress); err != nil {
		return err
	}
	if err := bmpBindRequireCapOne(ctx, oldAddress); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(os.Stderr, "BMP refused reload kept running config, live old session, cap 1 and free staged endpoint", addedAddress); err != nil {
		return err
	}
	repair, err := os.ReadFile("bmp-bind-repair.conf")
	if err != nil {
		return err
	}
	if err := staticRollbackReload(ctx, plugin, pid, string(repair), "applied"); err != nil {
		return fmt.Errorf("valid BMP repair did not apply: %w", err)
	}
	if err := bmpBindIdentity(ctx, plugin, first, "after-repair"); err != nil {
		return err
	}
	second, err := bmpBindDial(ctx, oldAddress)
	if err != nil {
		return err
	}
	defer second.Close() //nolint:errcheck // fixture teardown
	if err := bmpBindIdentity(ctx, plugin, second, "cap-two-admitted"); err != nil {
		return err
	}
	added, err := bmpBindDial(ctx, addedAddress)
	if err != nil {
		return err
	}
	defer added.Close() //nolint:errcheck // fixture teardown
	if err := bmpBindIdentity(ctx, plugin, added, "added-listener"); err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stderr, "BMP valid repair preserved old session, admitted session 2 and served added listener", addedAddress)
	return err
}

func bmpBindAddress(port string) (string, error) {
	value, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return "", fmt.Errorf("BMP fixture port %q is not a TCP port: %w", port, err)
	}
	if value == 0 {
		return "", errors.New("BMP fixture requires a runner-leased nonzero port")
	}
	return net.JoinHostPort("127.0.0.1", strconv.FormatUint(value, 10)), nil
}

func bmpBindRequireFree(ctx context.Context, address string) error {
	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("refused BMP install retained staged endpoint %s: %w", address, err)
	}
	return listener.Close()
}

func bmpBindDial(ctx context.Context, address string) (net.Conn, error) {
	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("BMP receiver %s does not accept: %w", address, err)
	}
	return conn, nil
}

// bmpBindRequireCapOne runs only after the first session's message was read.
// An admitted second connection stays open and times out, which fails this
// assertion; a refused connection closes without any BMP response.
func bmpBindRequireCapOne(ctx context.Context, address string) error {
	conn, err := bmpBindDial(ctx, address)
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // fixture teardown
	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return err
	}
	var reply [1]byte
	n, err := conn.Read(reply[:])
	if !errors.Is(err, io.EOF) {
		return fmt.Errorf("BMP old cap 1 did not reject session 2: read %d bytes, error %v", n, err) //nolint:errorlint // diagnostic includes a possibly nil read error
	}
	return nil
}

// bmpBindIdentity waits for this connection's newly parsed sysName. A TCP
// write alone could succeed after closure and would not prove session survival.
func bmpBindIdentity(ctx context.Context, plugin *sdk.Plugin, conn net.Conn, name string) error {
	if len(name) > 65535 {
		return errors.New("BMP fixture identity exceeds its TLV length")
	}
	payload := binary.BigEndian.AppendUint16(nil, 2)
	payload = binary.BigEndian.AppendUint16(payload, uint16(len(name)))
	payload = append(payload, name...)
	description := "BMP bind observer"
	payload = binary.BigEndian.AppendUint16(payload, 1)
	payload = binary.BigEndian.AppendUint16(payload, uint16(len(description)))
	payload = append(payload, description...)
	if err := conn.SetWriteDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return err
	}
	if err := writeBMP04(conn, bmpHeader04(4, payload)); err != nil {
		return err
	}
	remote := conn.LocalAddr().String()
	detail := "no session answer"
	if !Poll(ctx, WaitAttempts(40, 100*time.Millisecond, 80), 100*time.Millisecond, func() bool {
		status, data, err := dispatchMap(ctx, plugin, "show bmp sessions")
		if err != nil {
			detail = err.Error()
			return false
		}
		if status != statusDone {
			detail = "status " + status
			return false
		}
		for _, row := range mapSlice04(data["sessions"]) {
			if row["remote"] == remote {
				if row["sys-name"] == name {
					return true
				}
			}
		}
		detail = fmt.Sprint(data)
		return false
	}) {
		return fmt.Errorf("BMP session %s did not parse identity %q: %s", remote, name, detail)
	}
	return nil
}
