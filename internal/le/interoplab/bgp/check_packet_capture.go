// Design: docs/architecture/testing/interop.md -- bounded native packet observation.
package bgp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// packetCapture names files inside one fresh, lab-owned Ze container. GNU
// timeout owns and reaps tcpdump; callers MUST await joined completion before
// making any absence claim. Prestart users MUST activate it before exec Ze.
type packetCapture struct {
	base    string
	filter  string
	seconds int
	packets int
}

var nssaPacketCapture = packetCapture{base: "/tmp/ospf-nssa", filter: "ip proto 89", seconds: 120, packets: 4096}

func (capture packetCapture) file() string     { return capture.base + ".pcap" }
func (capture packetCapture) log() string      { return capture.base + "-capture.log" }
func (capture packetCapture) exit() string     { return capture.base + "-capture.exit" }
func (capture packetCapture) started() string  { return capture.base + "-capture.started" }
func (capture packetCapture) finished() string { return capture.base + "-capture.finished" }
func (capture packetCapture) version() string  { return capture.base + "-capture.version" }

// command uses the explicitly installed GNU coreutils executable, never the
// BusyBox watchdog. GNU 9.11 timeout.c blocks cleanup around waitpid, preventing
// a late signal to a reused PID, and returns 124 only after its child is reaped.
// A forced kill returns 137 and cannot support an absence assertion.
func (capture packetCapture) command() (string, error) {
	if capture.seconds <= 0 {
		return "", errors.New("native capture needs an explicit positive time bound")
	}
	if capture.packets <= 0 {
		return "", errors.New("native capture needs an explicit positive packet bound")
	}
	var command textbuf.Buffer
	command.Str("LC_ALL=C; export LC_ALL; /usr/bin/timeout --version >").Str(capture.version()).Str(" || exit 1; ")
	command.Str("/bin/date -u +%Y-%m-%dT%H:%M:%S.%NZ >").Str(capture.started()).Str("; ")
	command.Str("if /usr/bin/timeout --signal=INT --kill-after=5 ").Int(int64(capture.seconds))
	command.Str(" tcpdump -n -U -s 65535 -B 4096 -c ").Int(int64(capture.packets)).Str(" -i eth0 -w ").Str(capture.file())
	command.Str(" '").Str(capture.filter).Str("' 2>").Str(capture.log()).Str("; then status=0; else status=$?; fi; ")
	command.Str("/bin/date -u +%Y-%m-%dT%H:%M:%S.%NZ >").Str(capture.finished()).Str("; printf '%s\\n' \"$status\" >").Str(capture.exit())
	return command.String(), nil
}

// prestart leaves capture running while the same container execs Ze. The
// listening marker is emitted after packet-socket activation, not process spawn.
func (capture packetCapture) prestart() (string, error) {
	command, err := capture.command()
	if err != nil {
		return "", err
	}
	// sleep(readiness): polls tcpdump's activated packet socket, at most 100 times.
	return "(" + command + ") &\n" +
		"i=0; until grep -q '^tcpdump: listening on eth0,' " + capture.log() + " 2>/dev/null; do " +
		"i=$((i+1)); test \"$i\" -lt 100 || exit 1; sleep 0.1; done\n", nil
}

func (capture packetCapture) waitActivated(ctx context.Context, lab interoplab.CheckerLab) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 10 * time.Second, Interval: 100 * time.Millisecond, Description: "native capture socket activated on eth0"}, func(probe context.Context) (string, error) {
		return lab.Query(probe, "ze", []string{cmdCat, capture.log()}, nil)
	}, packetCaptureActivated)
	return err
}

func packetCaptureActivated(log string) bool {
	for line := range strings.SplitSeq(log, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 4 && fields[0] == "tcpdump:" && fields[1] == "listening" && fields[2] == "on" && fields[3] == "eth0," {
			return true
		}
	}
	return false
}

func (capture packetCapture) waitCompleted(ctx context.Context, lab interoplab.CheckerLab, peer, expected string, timeout time.Duration) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: timeout, Interval: 100 * time.Millisecond, Description: "joined native capture completion"}, func(probe context.Context) (string, error) {
		code, err := lab.Query(probe, peer, []string{cmdCat, capture.exit()}, nil)
		if err != nil {
			return "", err
		}
		code = strings.TrimSpace(code)
		if code != expected {
			return "", fmt.Errorf("native capture completed with %q, expected %s", code, expected)
		}
		return code, nil
	}, func(code string) bool { return code == expected })
	return err
}

func packetCaptureComplete(log string, limit int) (int, error) {
	captured, dropped := -1, -1
	for line := range strings.SplitSeq(log, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		value, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		if strings.Join(fields[1:], " ") == "packets captured" {
			captured = value
		}
		if strings.Join(fields[1:], " ") == "packets dropped by kernel" {
			dropped = value
		}
	}
	if captured <= 0 || captured >= limit || dropped != 0 {
		return captured, fmt.Errorf("incomplete DelayOpen capture: captured=%d dropped=%d; capture log: %s", captured, dropped, log)
	}
	return captured, nil
}
