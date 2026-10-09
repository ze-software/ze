//go:build linux

// Design: docs/functional-tests.md -- complete captured TCP/BGP completion fence.
package fixture

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net/netip"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/pcap"
)

// TestJointSubnetFRRCaptureFence exercises packet and TCP boundaries rather
// than matching a marker substring in arbitrary captured bytes.
func TestJointSubnetFRRCaptureFence(t *testing.T) {
	const port = 31791
	flow := pcap.Flow{SourceAddr: netip.MustParseAddr("2001:db8:a::1"), TargetAddr: netip.MustParseAddr("2001:db8:a::2"), SourcePort: 41000, TargetPort: port}
	open := make([]byte, 29)
	for i := range 16 {
		open[i] = 0xff
	}
	binary.BigEndian.PutUint16(open[16:18], 29)
	open[18], open[19] = 1, 4
	binary.BigEndian.PutUint16(open[20:22], 65001)
	copy(open[24:28], []byte{192, 0, 2, 1})
	badOpen := bytes.Clone(open)
	badOpen[28] = 1
	subject := jointSubnetFRRUpdate("2001:db8:a::9", true)
	control := jointSubnetFRRUpdate("2001:db8:a::9", false)
	withdrawal := append(bytes.Repeat([]byte{0xff}, 16), 0, 36, 2, 0, 0, 0, 13, 0x80, 15, 10, 0, 2, 1, 48, 0x20, 1, 0x0d, 0xb8, 0x57, 1)
	for _, tc := range []struct {
		name                         string
		parts                        [][]byte
		reverse, gap, truncateRecord bool
		ready, wantErr               bool
	}{
		{"open-only", [][]byte{open}, false, false, false, false, false},
		{"subject-only", [][]byte{open, subject}, false, false, false, false, false},
		{"complete", [][]byte{open, subject, control}, false, false, false, true, false},
		{"split-tcp-segments", [][]byte{open, subject[:17], subject[17:], control[:25], control[25:]}, false, false, false, true, false},
		{"coalesced-tcp-segment", [][]byte{bytes.Join([][]byte{open, subject, control}, nil)}, false, false, false, true, false},
		{"incomplete-control", [][]byte{open, subject, control[:len(control)-1]}, false, false, false, false, false},
		{"incomplete-record", [][]byte{open, subject, control}, false, false, true, false, false},
		{"reverse-direction", [][]byte{open, subject, control}, true, false, false, false, false},
		{"tcp-hole", [][]byte{open, subject, control}, false, true, false, false, false},
		{"missing-open", [][]byte{subject, control}, false, false, false, false, true},
		{"control-before-subject", [][]byte{open, control, subject}, false, false, false, false, true},
		{"unframed-marker-bait", [][]byte{open, append([]byte{0}, subject...), control}, false, false, false, false, true},
		{"malformed-open", [][]byte{badOpen, subject, control}, false, false, false, false, true},
		{"second-open", [][]byte{open, subject, control, open}, false, false, false, false, true},
		{"withdrawal-after-completion", [][]byte{open, subject, control, withdrawal}, false, false, false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var capture bytes.Buffer
			if err := pcap.WriteFileHeader(&capture, 65535, pcap.LinkTypeRaw); err != nil {
				t.Fatal(err)
			}
			framer := pcap.NewFramer()
			direction := flow
			if tc.reverse {
				direction = direction.Reverse()
			}
			for i, part := range tc.parts {
				original := len(part)
				if tc.gap && i == 0 {
					original++
				}
				if err := framer.WriteMessage(&capture, time.Unix(int64(i), 0), direction, part, original); err != nil {
					t.Fatal(err)
				}
			}
			data := capture.Bytes()
			if tc.truncateRecord {
				data = data[:len(data)-1]
			}
			ready, err := jointSubnetFRRCaptured(bytes.NewReader(data), port, jointSubnetFRRSameLink)
			if ready != tc.ready {
				t.Fatalf("ready=%v, want %v; error=%v", ready, tc.ready, err)
			}
			if (err != nil) != tc.wantErr {
				t.Fatalf("error=%v, want error=%v", err, tc.wantErr)
			}
		})
	}
}

// TestJointSubnetFRRCapturedFields checks complete UPDATE fields, independently
// of PCAP framing, including failure when the requested marker or pair is wrong.
func TestJointSubnetFRRCapturedFields(t *testing.T) {
	pair := jointSubnetFRRUpdate("2001:db8:a::9", true)
	control := jointSubnetFRRUpdate("2001:db8:a::9", false)
	badMarker := bytes.Clone(control)
	marker := bytes.Index(badMarker, []byte{0xc0, 8, 4, 0xfd, 0xe9, 0, 7})
	if marker < 0 {
		t.Fatal("control input has no community")
	}
	badMarker[marker+6] = 0
	splitSubject := jointSubnetFRRUpdate("2001:db8:b::9", false)
	splitSubject[len(splitSubject)-1] = 1
	badFlags := bytes.Clone(control)
	badFlags[marker] = 0x80
	badReach := bytes.Clone(pair)
	reach := bytes.Index(badReach, []byte{0x80, 14, 44, 0, 2, 1, 32})
	if reach < 0 {
		t.Fatal("subject input has no MP_REACH")
	}
	badReach[reach+3] = 1
	badLocal := bytes.Clone(pair)
	badLocal[reach+3+35] = 8
	withdrawal := bytes.Repeat([]byte{0xff}, 19)
	withdrawal = append(withdrawal, 0, 0, 0, 13, 0x80, 15, 10, 0, 2, 1, 48, 0x20, 1, 0x0d, 0xb8, 0x57, 1)
	for _, tc := range []struct {
		name     string
		scenario jointSubnetFRRScenario
		message  []byte
		which    int
		wantErr  bool
	}{
		{"common-subject", jointSubnetFRRSameLink, pair, 1, false},
		{"common-control", jointSubnetFRRSameLink, control, 2, false},
		{"wrong-marker", jointSubnetFRRSameLink, badMarker, 0, true},
		{"pair-on-split", jointSubnetFRRSplitLink, jointSubnetFRRUpdate("2001:db8:b::9", true), 0, true},
		{"wrong-global", jointSubnetFRRSameLink, jointSubnetFRRUpdate("2001:db8:b::9", false), 0, true},
		{"split-subject", jointSubnetFRRSplitLink, splitSubject, 1, false},
		{"split-control", jointSubnetFRRSplitLink, jointSubnetFRRUpdate("2001:db8:b::9", false), 2, false},
		{"nontransitive-marker", jointSubnetFRRSameLink, badFlags, 0, true},
		{"wrong-family", jointSubnetFRRSameLink, badReach, 0, true},
		{"wrong-link-local", jointSubnetFRRSameLink, badLocal, 0, true},
		{"scoped-withdrawal", jointSubnetFRRSameLink, withdrawal, 0, true},
		{"short-update", jointSubnetFRRSameLink, pair[:21], 0, true},
		{"unspecified-is-not-split-link", jointSubnetFRRUnspecified, splitSubject, 0, true},
		{"unknown-is-not-split-link", jointSubnetFRRScenario(255), splitSubject, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			which, err := jointSubnetFRRCapturedUpdate(tc.message[19:], tc.scenario)
			if which != tc.which {
				t.Fatalf("which=%d, want %d", which, tc.which)
			}
			if (err != nil) != tc.wantErr {
				t.Fatalf("error=%v, want error=%v", err, tc.wantErr)
			}
		})
	}
}

// TestJointSubnetFRRCaptureReassembly requires reordered segments and identical
// retransmissions to reconstruct exactly once, without closing a sequence hole.
func TestJointSubnetFRRCaptureReassembly(t *testing.T) {
	const port = 31791
	flow := pcap.Flow{SourceAddr: netip.MustParseAddr("2001:db8:a::1"), TargetAddr: netip.MustParseAddr("2001:db8:a::2"), SourcePort: 41000, TargetPort: port}
	open := append(bytes.Repeat([]byte{0xff}, 16), 0, 29, 1, 4, 0xfd, 0xe9, 0, 0, 192, 0, 2, 1, 0)
	subject := jointSubnetFRRUpdate("2001:db8:a::9", true)
	control := jointSubnetFRRUpdate("2001:db8:a::9", false)
	parts := [][]byte{open, subject[:20], subject[20:], control}
	records := make([][]byte, len(parts))
	framer := pcap.NewFramer()
	for i, part := range parts {
		var record bytes.Buffer
		if err := framer.WriteMessage(&record, time.Unix(int64(i), 0), flow, part, len(part)); err != nil {
			t.Fatal(err)
		}
		records[i] = record.Bytes()
	}
	for _, tc := range []struct {
		name  string
		order []int
		ready bool
	}{
		{"reordered-with-retransmission", []int{2, 0, 1, 1, 3}, true},
		{"hole-with-retransmission", []int{0, 2, 2, 3}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var capture bytes.Buffer
			if err := pcap.WriteFileHeader(&capture, 65535, pcap.LinkTypeRaw); err != nil {
				t.Fatal(err)
			}
			for _, index := range tc.order {
				capture.Write(records[index])
			}
			ready, err := jointSubnetFRRCaptured(bytes.NewReader(capture.Bytes()), port, jointSubnetFRRSameLink)
			if err != nil {
				t.Fatal(err)
			}
			if ready != tc.ready {
				t.Fatalf("ready=%v, want %v", ready, tc.ready)
			}
		})
	}
}

// TestJointSubnetFRRPlanPort checks the one CLI admission boundary, including
// noncanonical decimal input that must become the same typed port.
func TestJointSubnetFRRPlanPort(t *testing.T) {
	for _, tc := range []struct {
		input string
		port  uint16
	}{
		{"1", 1}, {"031791", 31791}, {"65535", 65535},
		{"0", 0}, {"65536", 0}, {"-1", 0}, {"bgp", 0},
	} {
		t.Run(tc.input, func(t *testing.T) {
			args := []string{"case", "same-link", "netns", "frr", "port", tc.input, "runtime", "/tmp/frr", "output", "/tmp/frr-output", "ze", "/bin/ze", "le", "/bin/le", "zebra", "/bin/zebra", "bgpd", "/bin/bgpd", "vtysh", "/bin/vtysh", "tcpdump", "/bin/tcpdump"}
			plan, err := parseJointSubnetFRRPlan(args)
			if tc.port == 0 {
				if err == nil {
					t.Fatal("invalid port admitted")
				}
				if plan != nil {
					t.Fatal("failed parse published a plan")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if plan.port != tc.port {
				t.Fatalf("port=%d, want %d", plan.port, tc.port)
			}
		})
	}
	for _, plan := range []*jointSubnetFRRPlan{nil, {}} {
		if err := runJointSubnetFRR(context.Background(), plan, nil, 0); err == nil {
			t.Fatal("uninitialized plan admitted for execution")
		}
	}
}

// TestJointSubnetFRRPollCompletion cannot accept readiness after the deadline
// or after a child exit already queued by its sole waiter.
func TestJointSubnetFRRPollCompletion(t *testing.T) {
	for _, tc := range []string{"deadline", "daemon-exit", "capture-exit"} {
		t.Run(tc, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			watched := &watchedProcesses{exits: make(chan processExit, 1)}
			capture := &watchedProcesses{exits: make(chan processExit, 1)}
			switch tc {
			case "deadline":
				cancel()
			case "daemon-exit":
				watched.pending = 1
				watched.exits <- processExit{label: "ze"}
			case "capture-exit":
				capture.pending = 1
				capture.exits <- processExit{label: "capture"}
			}
			if err := pollJointSubnetFRR(ctx, watched, capture, func() (bool, error) { return true, nil }); err == nil {
				t.Fatal("ready predicate hid deadline or premature exit")
			}
			if watched.pending+capture.pending != 0 {
				t.Fatal("queued exit was not consumed")
			}
		})
	}
}

// TestJointSubnetFRRProcessAdmission checks invalid handles and consumed joins
// without starting a subprocess or granting an unstarted exec.Cmd permission.
func TestJointSubnetFRRProcessAdmission(t *testing.T) {
	var absent *jointSubnetFRRProcess
	if err := absent.flush(); err == nil {
		t.Fatal("nil process accepted")
	}
	var zero jointSubnetFRRProcess
	if err := zero.flush(); err == nil {
		t.Fatal("zero process accepted")
	}
	joined := jointSubnetFRRProcess{command: &exec.Cmd{}, watched: &watchedProcesses{}}
	if err := joined.flush(); err != nil {
		t.Fatalf("consumed join waited again: %v", err)
	}
}

// TestJointSubnetFRRCaptureSnapshotBound rejects an oversized savefile before
// reassembly, so a growing external file cannot expand the fixture's read budget.
func TestJointSubnetFRRCaptureSnapshotBound(t *testing.T) {
	directory := t.TempDir()
	file, err := os.Create(filepath.Join(directory, "recipient.pcap"))
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(1<<20 + 1); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	plan, err := parseJointSubnetFRRPlan([]string{"case", "same-link", "netns", "frr", "port", "31791", "runtime", "/tmp/frr", "output", directory, "ze", "/bin/ze", "le", "/bin/le", "zebra", "/bin/zebra", "bgpd", "/bin/bgpd", "vtysh", "/bin/vtysh", "tcpdump", "/bin/tcpdump"})
	if err != nil {
		t.Fatal(err)
	}
	var size int64
	if ready, err := jointSubnetFRRCaptureReady(plan, &size); err == nil || ready {
		t.Fatalf("oversized capture: ready=%v error=%v", ready, err)
	}
}

// TestJointSubnetFRRJoinBound distinguishes a consumed join from an outstanding
// child at the deadline, without creating or abandoning a waiter goroutine.
func TestJointSubnetFRRJoinBound(t *testing.T) {
	watched := &watchedProcesses{exits: make(chan processExit, 1), pending: 1}
	if err := joinJointSubnetFRRProcesses(watched, 0); err == nil {
		t.Fatal("outstanding child accepted at deadline")
	}
	if watched.pending != 1 {
		t.Fatal("timeout claimed an unconsumed child")
	}
	watched.exits <- processExit{label: "ze"}
	if err := joinJointSubnetFRRProcesses(watched, time.Second); err != nil {
		t.Fatal(err)
	}
	if watched.pending != 0 {
		t.Fatal("completed join remains pending")
	}
}

// TestJointSubnetFRRChildCancellation runs this test binary as a real child.
// Cancellation between construction and start MUST NOT hand lifecycle ownership
// to exec's automatic SIGKILL path; the fixture alone signals and joins children.
func TestJointSubnetFRRChildCancellation(t *testing.T) {
	const childArgument = "joint-subnet-frr-cancellation-child"
	if os.Args[len(os.Args)-1] == childArgument {
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	child := newJointSubnetFRRChild(ctx, []string{executable, "-test.run=^TestJointSubnetFRRChildCancellation$", "--", childArgument})
	cancel()
	var output bytes.Buffer
	child.Stdout, child.Stderr = &output, &output
	if err := child.Run(); err != nil {
		t.Fatalf("execution cancellation took child lifecycle ownership: %v; output=%s", err, output.Bytes())
	}
	if !bytes.Contains(output.Bytes(), []byte("PASS")) {
		t.Fatalf("child did not complete its real test entry point: %s", output.Bytes())
	}
}

// TestJointSubnetFRRFinalDeadline requires the final verdict to preserve both
// the oracle failure and cancellation that occurred after the polling fence.
func TestJointSubnetFRRFinalDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	oracle := errors.New("original oracle failure")
	for _, original := range []error{nil, oracle} {
		watched := &watchedProcesses{exits: make(chan processExit, 1)}
		result := finishJointSubnetFRR(ctx, watched, original)
		if !errors.Is(result, context.Canceled) {
			t.Fatalf("post-fence cancellation lost: %v", result)
		}
		if original != nil {
			if !errors.Is(result, oracle) {
				t.Fatalf("original oracle failure lost: %v", result)
			}
		}
	}
}

// TestJointSubnetFRRCaptureTrailingRecord exercises the consumer after both
// completion routes are present. A next record whose header is complete but
// whose payload has not arrived MUST remain pending, not masquerade as EOF.
func TestJointSubnetFRRCaptureTrailingRecord(t *testing.T) {
	const port = 31791
	flow := pcap.Flow{SourceAddr: netip.MustParseAddr("2001:db8:a::1"), TargetAddr: netip.MustParseAddr("2001:db8:a::2"), SourcePort: 41000, TargetPort: port}
	open := append(bytes.Repeat([]byte{0xff}, 16), 0, 29, 1, 4, 0xfd, 0xe9, 0, 0, 192, 0, 2, 1, 0)
	var capture bytes.Buffer
	if err := pcap.WriteFileHeader(&capture, 65535, pcap.LinkTypeRaw); err != nil {
		t.Fatal(err)
	}
	framer := pcap.NewFramer()
	for i, message := range [][]byte{open, jointSubnetFRRUpdate("2001:db8:a::9", true), jointSubnetFRRUpdate("2001:db8:a::9", false)} {
		if err := framer.WriteMessage(&capture, time.Unix(int64(i), 0), flow, message, len(message)); err != nil {
			t.Fatal(err)
		}
	}
	complete := capture.Len()
	ready, err := jointSubnetFRRCaptured(bytes.NewReader(capture.Bytes()), port, jointSubnetFRRSameLink)
	if err != nil {
		t.Fatal(err)
	}
	if !ready {
		t.Fatal("positive control never reached capture completion")
	}
	withdrawal := append(bytes.Repeat([]byte{0xff}, 16), 0, 36, 2, 0, 0, 0, 13, 0x80, 15, 10, 0, 2, 1, 48, 0x20, 1, 0x0d, 0xb8, 0x57, 1)
	if err := framer.WriteMessage(&capture, time.Unix(3, 0), flow, withdrawal, len(withdrawal)); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		end  int
	}{
		{"partial-header", complete + pcap.RecordHeaderLen - 1},
		{"header-only", complete + pcap.RecordHeaderLen},
		{"one-payload-byte", complete + pcap.RecordHeaderLen + 1},
		{"partial-payload", capture.Len() - 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ready, err := jointSubnetFRRCaptured(bytes.NewReader(capture.Bytes()[:tc.end]), port, jointSubnetFRRSameLink)
			if err != nil {
				t.Fatalf("growing record is not pending: %v", err)
			}
			if ready {
				t.Fatal("incomplete trailing record released the capture fence")
			}
		})
	}
	ready, err = jointSubnetFRRCaptured(bytes.NewReader(capture.Bytes()), port, jointSubnetFRRSameLink)
	if err == nil {
		t.Fatal("completed trailing withdrawal accepted")
	}
	if ready {
		t.Fatal("completed trailing withdrawal released the capture fence")
	}
}

// TestJointSubnetFRRCancellationDuringFlush gates a real child after SIGINT,
// cancels execution while flush is waiting, then permits a clean child exit.
// A successful flush MUST NOT erase that late deadline from the final verdict.
func TestJointSubnetFRRCancellationDuringFlush(t *testing.T) {
	const childArgument = "joint-subnet-frr-flush-child"
	if os.Args[len(os.Args)-1] == childArgument {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGINT)
		defer signal.Stop(signals)
		if _, err := os.Stdout.Write([]byte{'R'}); err != nil {
			t.Fatal(err)
		}
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()
		select {
		case <-signals:
		case <-timer.C:
			t.Fatal("fixture never requested graceful capture flush")
		}
		if _, err := os.Stdout.Write([]byte{'I'}); err != nil {
			t.Fatal(err)
		}
		var gate [1]byte
		if _, err := io.ReadFull(os.Stdin, gate[:]); err != nil {
			t.Fatal(err)
		}
		if gate[0] != 'X' {
			t.Fatal("unexpected flush-release gate")
		}
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	input, release, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()   //nolint:errcheck // Read-only child-side descriptor.
	defer release.Close() //nolint:errcheck // A test failure must unblock the child's read.
	observe, output, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer observe.Close() //nolint:errcheck // Read-only observation descriptor.
	defer output.Close()  //nolint:errcheck // Child owns its inherited descriptor after Start.
	if err := observe.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	child := newJointSubnetFRRChild(ctx, []string{executable, "-test.run=^TestJointSubnetFRRCancellationDuringFlush$", "--", childArgument})
	child.Stdin, child.Stdout, child.Stderr = input, output, output
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	watched := &watchedProcesses{exits: make(chan processExit, 1)}
	watched.watch(child, "capture", false)
	t.Cleanup(func() {
		if err := stopJointSubnetFRRProcesses(watched); err != nil {
			t.Errorf("child cleanup: %v", err)
		}
	})
	var acknowledgment [1]byte
	if _, err := io.ReadFull(observe, acknowledgment[:]); err != nil {
		t.Fatal(err)
	}
	if acknowledgment[0] != 'R' {
		t.Fatal("child did not install its signal handler")
	}
	capture := jointSubnetFRRProcess{command: child, watched: watched}
	flushed := make(chan error, 1)
	go func() { flushed <- capture.flush() }()
	joined := false
	defer func() {
		if !joined {
			release.Close() //nolint:errcheck // Unblock the child before joining a failed test's flush.
			<-flushed
		}
	}()
	if _, err := io.ReadFull(observe, acknowledgment[:]); err != nil {
		t.Fatal(err)
	}
	if acknowledgment[0] != 'I' {
		t.Fatal("capture did not receive SIGINT before teardown")
	}
	cancel()
	if _, err := release.Write([]byte{'X'}); err != nil {
		t.Fatal(err)
	}
	err = <-flushed
	joined = true
	if err != nil {
		t.Fatalf("graceful flush lost ownership to execution cancellation: %v", err)
	}
	if !child.ProcessState.Success() {
		t.Fatal("child did not exit cleanly after the flush gate")
	}
	result := finishJointSubnetFRR(ctx, watched, nil)
	if !errors.Is(result, context.Canceled) {
		t.Fatalf("cancellation during a successful capture flush became success: %v", result)
	}
}

// TestJointSubnetFRRScenarioAdmission follows CLI cases through their typed
// next hop and emitted config, and refuses missing/unknown cases before output.
func TestJointSubnetFRRScenarioAdmission(t *testing.T) {
	for _, tc := range []struct {
		input    string
		scenario jointSubnetFRRScenario
		global   string
		source   string
	}{
		{"same-link", jointSubnetFRRSameLink, "2001:db8:a::9", "2001:db8:a::1"},
		{"split-link", jointSubnetFRRSplitLink, "2001:db8:b::9", "2001:db8:b::1"},
		{"", jointSubnetFRRUnspecified, "", ""},
		{"unknown", jointSubnetFRRUnspecified, "", ""},
		{"SAME-LINK", jointSubnetFRRUnspecified, "", ""},
	} {
		t.Run(tc.input, func(t *testing.T) {
			output := t.TempDir()
			plan, err := parseJointSubnetFRRPlan([]string{"case", tc.input, "netns", "frr", "port", "31791", "runtime", "/tmp/frr", "output", output, "ze", "/bin/ze", "le", "/bin/le", "zebra", "/bin/zebra", "bgpd", "/bin/bgpd", "vtysh", "/bin/vtysh", "tcpdump", "/bin/tcpdump"})
			if tc.scenario == jointSubnetFRRUnspecified {
				if err == nil {
					t.Fatal("invalid CLI case admitted")
				}
				if plan != nil {
					t.Fatal("invalid CLI case published a plan")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if plan.scenario != tc.scenario {
				t.Fatalf("scenario=%d, want %d", plan.scenario, tc.scenario)
			}
			global, err := plan.scenario.nextHop()
			if err != nil {
				t.Fatal(err)
			}
			if global != netip.MustParseAddr(tc.global) {
				t.Fatalf("global=%s, want %s", global, tc.global)
			}
			if err := writeJointSubnetFRRFiles(plan); err != nil {
				t.Fatal(err)
			}
			config, err := os.ReadFile(filepath.Join(output, "ze.conf"))
			if err != nil {
				t.Fatal(err)
			}
			want := "remote { ip " + tc.global + "; port 31791; } local { ip " + tc.source + "; accept false; }"
			if !bytes.Contains(config, []byte(want)) {
				t.Fatalf("case config does not contain %q: %s", want, config)
			}
		})
	}
	for _, scenario := range []jointSubnetFRRScenario{jointSubnetFRRUnspecified, 255} {
		global, err := scenario.nextHop()
		if err == nil {
			t.Fatalf("invalid scenario %d produced %s", scenario, global)
		}
		if global.IsValid() {
			t.Fatal("invalid scenario published a valid next hop")
		}
		output := t.TempDir()
		plan := jointSubnetFRRPlan{scenario: scenario, output: output, port: 31791}
		if err := runJointSubnetFRR(context.Background(), &plan, nil, 0); err == nil {
			t.Fatalf("invalid scenario %d admitted for execution", scenario)
		}
		if err := writeJointSubnetFRRFiles(&plan); err == nil {
			t.Fatalf("invalid scenario %d emitted files", scenario)
		}
		if _, err := os.Stat(filepath.Join(output, "frr.conf")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("invalid scenario created a configuration: %v", err)
		}
	}
}
