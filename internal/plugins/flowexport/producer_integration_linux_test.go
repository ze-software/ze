//go:build integration && linux

// Design: docs/architecture/flowexport/flow-export-1-counter-export.md -- Linux raw counter producer.
package flowexport_test

import (
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

var exportDaemonPath = flag.String("flowexport-ze-path", "", "Ze daemon executable with ze_core and ze_flowexport for isolated Linux counter production")

// TestFlowExportDaemonCounterGeneration observes real kernel traffic through
// iface's rate tracker, the internal plugin subscription, and both UDP encoders.
// Stopping the daemon across index reuse prevents an intervening empty snapshot
// from concealing a missing generation. The replacement has larger counters.
func TestFlowExportDaemonCounterGeneration(t *testing.T) {
	if *exportDaemonPath == "" {
		t.Skip("requires explicit -flowexport-ze-path and CAP_SYS_ADMIN/CAP_NET_ADMIN/CAP_NET_RAW")
	}
	binaryPath, err := filepath.Abs(*exportDaemonPath)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(binaryPath)
	if err != nil {
		t.Fatalf("daemon executable %q: %v", binaryPath, err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("daemon path %q is not an executable regular file", binaryPath)
	}
	exportProducerNamespace(t)
	if err := os.WriteFile("/proc/sys/net/ipv6/conf/default/disable_ipv6", []byte("1"), 0o644); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkSetUp(lo); err != nil {
		t.Fatal(err)
	}
	link := &netlink.Dummy{Name: "counter0", Index: 77}
	addLink := func() {
		if err := netlink.LinkAdd(link); err != nil {
			t.Fatal(err)
		}
		if err := netlink.LinkSetUp(link); err != nil {
			t.Fatal(err)
		}
	}
	addLink()
	exportProducerFrames(t, 77, 1)
	sflow := exportProducerListener(t)
	ipfix := exportProducerListener(t)
	work := t.TempDir()
	configPath := filepath.Join(work, "ze.conf")
	sflowAddr, ok := sflow.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("sFlow collector address is %T", sflow.LocalAddr())
	}
	ipfixAddr, ok := ipfix.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("IPFIX collector address is %T", ipfix.LocalAddr())
	}
	config := fmt.Sprintf(`flow-export {
    collector sflow {
        address 127.0.0.1
        port %d
        protocol sflow
        agent-address 127.0.0.1
        polling-interval 1
    }
    collector ipfix {
        address 127.0.0.1
        port %d
        protocol ipfix
        observation-domain 41
        polling-interval 1
    }
}
`, sflowAddr.Port, ipfixAddr.Port)
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	cmd := exec.CommandContext(ctx, binaryPath, "start", configPath)
	cmd.Dir = work
	// Do not inherit operator Ze settings (privilege dropping, collectors,
	// config directories, kernel-probe escapes, etc.) into this private daemon.
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		normalized := strings.ToLower(strings.ReplaceAll(key, "_", "."))
		if !strings.HasPrefix(normalized, "ze.") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "ZE_CONFIG_DIR="+work)
	daemon := startExportProcess(t, cmd, cancel)
	// A failed assertion while paused must not strand the owned child.
	t.Cleanup(func() { _ = daemon.cmd.Process.Signal(syscall.SIGCONT) })
	first := exportProducerCounter(t, daemon, sflow, false, 1, 2)
	exportProducerCounter(t, daemon, ipfix, true, 1, 0)

	exportPauseDaemon(t, daemon)
	if err := netlink.LinkSetDown(link); err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	exportProducerFrames(t, 77, 1)
	if err := daemon.cmd.Process.Signal(syscall.SIGCONT); err != nil {
		t.Fatal(err)
	}
	stable := exportProducerCounter(t, daemon, sflow, false, 2, 0)
	if stable.sample <= first.sample || stable.datagram <= first.datagram {
		t.Fatalf("down/up reset sequence: before=%+v after=%+v", first, stable)
	}
	exportProducerCounter(t, daemon, ipfix, true, 2, 0)

	exportPauseDaemon(t, daemon)
	if err := netlink.LinkDel(link); err != nil {
		t.Fatal(err)
	}
	addLink()
	exportProducerFrames(t, 77, 3)
	if err := daemon.cmd.Process.Signal(syscall.SIGCONT); err != nil {
		t.Fatal(err)
	}
	replaced := exportProducerCounter(t, daemon, sflow, false, 3, 0)
	if replaced.sample != 1 || replaced.datagram <= stable.datagram {
		t.Fatalf("same-index replacement must restart only the source sequence: before=%+v after=%+v", stable, replaced)
	}
	exportProducerCounter(t, daemon, ipfix, true, 3, 0)
	if err := daemon.stop(); err != nil {
		t.Fatal(err)
	}
	t.Log("daemon exported raw kernel totals and unavailable sentinels; down/up preserved the sample sequence, same-index replacement with larger counters restarted it")
}

func exportProducerNamespace(t *testing.T) {
	t.Helper()
	runtime.LockOSThread()
	original, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatal(err)
	}
	private, err := netns.New()
	if err != nil {
		_ = original.Close()
		runtime.UnlockOSThread()
		t.Fatalf("create private network namespace: %v", err)
	}
	t.Cleanup(func() {
		if err := netns.Set(original); err != nil {
			// Keep this thread locked: Go destroys it instead of reusing a
			// thread left in the wrong namespace when this goroutine exits.
			t.Fatal(err)
		}
		if err := private.Close(); err != nil {
			t.Error(err)
		}
		if err := original.Close(); err != nil {
			t.Error(err)
		}
		runtime.UnlockOSThread()
	})
}

func exportProducerFrames(t *testing.T, index, count int) {
	t.Helper()
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := unix.Close(fd); err != nil {
			t.Error(err)
		}
	}()
	frame := [60]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 2, 0, 0, 0, 0, 1, 0x88, 0xb5}
	for range count {
		if err := unix.Sendto(fd, frame[:], 0, &unix.SockaddrLinklayer{Ifindex: index}); err != nil {
			t.Fatal(err)
		}
	}
}

func exportProducerListener(t *testing.T) net.PacketConn {
	t.Helper()
	var lc net.ListenConfig
	pc, err := lc.ListenPacket(t.Context(), "udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pc.Close() })
	return pc
}

func exportPauseDaemon(t *testing.T, daemon *exportCollector) {
	t.Helper()
	if err := daemon.cmd.Process.Signal(syscall.SIGSTOP); err != nil {
		t.Fatal(err)
	}
	daemon.wait(t, "stopped daemon", 5*time.Second, func(context.Context) (bool, error) {
		root := "/proc/" + strconv.Itoa(daemon.cmd.Process.Pid) + "/task"
		threads, err := os.ReadDir(root)
		if err != nil {
			return false, err
		}
		for _, thread := range threads {
			data, err := os.ReadFile(filepath.Join(root, thread.Name(), "status"))
			if errors.Is(err, os.ErrNotExist) {
				return false, nil // A thread exited while the group was stopping.
			}
			if err != nil {
				return false, err
			}
			stopped := false
			for line := range strings.SplitSeq(string(data), "\n") {
				if strings.HasPrefix(line, "State:") {
					fields := strings.Fields(line)
					stopped = len(fields) > 1 && fields[1] == "T"
					break
				}
			}
			if !stopped {
				return false, nil
			}
		}
		return len(threads) > 0, nil
	})
}

type exportProducerReading struct {
	sample, datagram uint32
	packets, bytes   uint64
}

func exportProducerCounter(t *testing.T, daemon *exportCollector, pc net.PacketConn, ipfix bool, packets uint64, minSample uint32) exportProducerReading {
	t.Helper()
	var result exportProducerReading
	description := fmt.Sprintf("counter0 raw totals: packets=%d bytes=%d ipfix=%t", packets, packets*60, ipfix)
	daemon.wait(t, description, 15*time.Second, func(context.Context) (bool, error) {
		if err := pc.SetReadDeadline(time.Now().Add(20 * time.Millisecond)); err != nil {
			return false, err
		}
		var buf [2048]byte
		n, _, err := pc.ReadFrom(buf[:])
		if err != nil {
			var timeout net.Error
			if errors.As(err, &timeout) && timeout.Timeout() {
				return false, nil
			}
			return false, err
		}
		if n > 464 {
			return false, fmt.Errorf("daemon exceeded default datagram bound: %d", n)
		}
		var found bool
		if ipfix {
			result, found = exportProducerIPFIX(t, buf[:n])
		} else {
			result, found = exportProducerSFlow(t, buf[:n])
		}
		if !found || result.packets < packets || result.sample < minSample {
			return false, nil
		}
		if result.packets != packets || result.bytes != packets*60 {
			return false, fmt.Errorf("got %d packets / %d bytes, want %d / %d", result.packets, result.bytes, packets, packets*60)
		}
		return true, nil
	})
	return result
}

func exportProducerSFlow(t *testing.T, data []byte) (exportProducerReading, bool) {
	t.Helper()
	be := binary.BigEndian
	if len(data) < 28 || be.Uint32(data) != 5 || be.Uint32(data[4:]) != 1 {
		t.Fatalf("invalid IPv4 sFlow header: %x", data)
	}
	for off, count := 28, be.Uint32(data[24:]); count > 0; count-- {
		if len(data)-off < 8 {
			t.Fatal("truncated sFlow sample header")
		}
		size := int(be.Uint32(data[off+4:])) + 8
		if size != 120 || size > len(data)-off || be.Uint32(data[off:]) != 4 {
			t.Fatalf("invalid expanded counter sample: %x", data[off:])
		}
		sample := data[off : off+size]
		off += size
		if be.Uint32(sample[16:]) != 77 {
			continue
		}
		if be.Uint32(sample[12:]) != 0 || be.Uint32(sample[20:]) != 1 || be.Uint32(sample[24:]) != 1 || be.Uint32(sample[28:]) != 88 {
			t.Fatalf("invalid interface counter envelope: %x", sample)
		}
		record := sample[32:]
		if be.Uint32(record) != 77 || be.Uint64(record[24:]) != 0 || be.Uint32(record[32:]) != 0 {
			t.Fatalf("unexpected counter0 ingress fields: %x", record)
		}
		for _, offset := range []int{40, 52, 68, 72} {
			if be.Uint32(record[offset:]) != ^uint32(0) {
				t.Fatalf("unavailable counter at offset %d is not sentinel: %x", offset, record)
			}
		}
		return exportProducerReading{sample: be.Uint32(sample[8:]), datagram: be.Uint32(data[16:]), packets: uint64(be.Uint32(record[64:])), bytes: be.Uint64(record[56:])}, true
	}
	return exportProducerReading{}, false
}

func exportProducerIPFIX(t *testing.T, data []byte) (exportProducerReading, bool) {
	t.Helper()
	be := binary.BigEndian
	if len(data) < 16 || be.Uint16(data) != 10 || int(be.Uint16(data[2:])) != len(data) || be.Uint32(data[12:]) != 41 {
		t.Fatalf("invalid IPFIX header: %x", data)
	}
	for off := 16; off < len(data); {
		if len(data)-off < 4 {
			t.Fatal("truncated IPFIX set header")
		}
		size := int(be.Uint16(data[off+2:]))
		if size < 4 || size > len(data)-off {
			t.Fatalf("invalid IPFIX set length: %x", data[off:])
		}
		id := be.Uint16(data[off:])
		set := data[off+4 : off+size]
		off += size
		if id == 2 {
			// This carrier consumes the documented interface template: ingress,
			// total octets, total packets, egress, start seconds, end seconds.
			want := []uint16{256, 6, 10, 4, 85, 8, 86, 8, 14, 4, 150, 4, 151, 4}
			if len(set) != len(want)*2 {
				t.Fatalf("unexpected counter template: %x", set)
			}
			for i, field := range want {
				if be.Uint16(set[i*2:]) != field {
					t.Fatalf("unexpected counter template: %x", set)
				}
			}
			continue
		}
		if id != 256 || len(set)%32 != 0 {
			t.Fatalf("unexpected counter data set: id=%d data=%x", id, set)
		}
		for len(set) >= 32 {
			if be.Uint32(set) == 77 {
				if be.Uint32(set[20:]) != 77 {
					t.Fatalf("incorrect counter egress index: %x", set[:32])
				}
				return exportProducerReading{packets: be.Uint64(set[12:]), bytes: be.Uint64(set[4:])}, true
			}
			set = set[32:]
		}
	}
	return exportProducerReading{}, false
}
