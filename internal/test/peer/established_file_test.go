// Design: docs/architecture/testing/ci-format.md -- option=established-file
// Related: peer.go -- markEstablished, the code under test

package peer

import (
	"bytes"
	"encoding/hex"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestEstablishedFileWrittenOnFirstUpdate proves the marker's contract: the file
// is absent while the daemon has only answered with a KEEPALIVE, and present once
// its first UPDATE arrived.
//
// The method is a socket pair on which the far side plays the daemon. After each
// message it sends it reads the peer's KEEPALIVE reply, which the peer writes
// only after it handled that message, so each file check sees the peer's state
// for the message just sent.
//
// VALIDATES: option=established-file marks the daemon's Established state, which
//
//	a reload fixture waits on instead of a fixed delay.
//
// PREVENTS: a marker written on the KEEPALIVE, which the daemon sends while it
//
//	can still be in OpenConfirm.
func TestEstablishedFileWrittenOnFirstUpdate(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "established")
	peer, err := New(&Config{
		Mode:            ModeCheck,
		Expect:          []string{"expect=bgp:conn=1:seq=1:hex=" + unmetEOR},
		EstablishedFile: marker,
		Output:          new(bytes.Buffer),
	})
	if err != nil {
		t.Fatalf("new peer: %v", err)
	}

	local, remote := net.Pipe()
	defer local.Close()  //nolint:errcheck // test socket
	defer remote.Close() //nolint:errcheck // test socket

	checked := make(chan error, 1)
	go func() { checked <- playDaemon(remote, marker) }()

	result := peer.handleConnection(t.Context(), local)
	if err := <-checked; err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("peer failed: %v", result.Error)
	}
}

// playDaemon sends OPEN, KEEPALIVE and End-of-RIB, checking the marker after the
// KEEPALIVE (absent) and after the UPDATE (present). It closes the connection on
// every return, so a failed check ends the peer rather than leaving it reading.
func playDaemon(conn net.Conn, marker string) error {
	defer conn.Close() //nolint:errcheck // test socket, the peer reads the close
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	if _, err := conn.Write(minimalOpenMsg(65001)); err != nil {
		return err
	}
	reader := newMessageReader(conn)
	for range 2 { // the peer's OPEN and KEEPALIVE
		if _, _, err := reader.readMessage(); err != nil {
			return err
		}
	}
	if err := sendAndReadReply(conn, KeepaliveMsg()); err != nil {
		return err
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		return errors.New("the marker exists after a KEEPALIVE alone")
	}
	eor, err := hex.DecodeString(unmetEOR)
	if err != nil {
		return err
	}
	if err := sendAndReadReply(conn, eor); err != nil {
		return err
	}
	if _, err := os.Stat(marker); err != nil {
		return errors.New("the marker is absent after the first UPDATE: " + err.Error())
	}
	return nil
}

func sendAndReadReply(conn net.Conn, frame []byte) error {
	if _, err := conn.Write(frame); err != nil {
		return err
	}
	header, _, err := newMessageReader(conn).readMessage()
	if err != nil {
		return err
	}
	if header[18] != MsgKEEPALIVE {
		return errors.New("the peer answered with something other than a KEEPALIVE")
	}
	return nil
}

// TestEstablishedFileOptionNeedsPath proves the option parser refuses a
// declaration with no file name, which would otherwise write nothing and leave
// the waiting fixture to time out with no cause.
func TestEstablishedFileOptionNeedsPath(t *testing.T) {
	config := &Config{}
	claimed, err := parseOptionConfig(config, "established-file", map[string]string{"path": "established"})
	if err != nil || !claimed || config.EstablishedFile != "established" {
		t.Fatalf("path=established: claimed=%v err=%v file=%q", claimed, err, config.EstablishedFile)
	}
	if _, err := parseOptionConfig(&Config{}, "established-file", map[string]string{}); err == nil {
		t.Fatal("option=established-file with no path was accepted")
	}
}

// TestEstablishedFileWrittenPerSession proves each session writes its own
// marker on its first UPDATE: the first session <name>, the second <name>-2,
// and a second UPDATE on one session writes no other.
//
// VALIDATES: a reload fixture can wait for the session a reload restarted
//
//	(await-file established-2) as it waits for the first.
//
// PREVENTS: a second session that is never observable, which left a reload
//
//	test unable to wait for the restarted session before it shut ze down.
func TestEstablishedFileWrittenPerSession(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "established")
	peer, err := New(&Config{
		Mode: ModeCheck,
		Expect: []string{
			"expect=bgp:conn=1:seq=1:hex=" + unmetEOR,
			"expect=bgp:conn=1:seq=2:hex=" + unmetEOR,
			"expect=bgp:conn=2:seq=1:hex=" + unmetEOR,
		},
		EstablishedFile: marker,
		Output:          new(bytes.Buffer),
	})
	if err != nil {
		t.Fatalf("new peer: %v", err)
	}

	runSession := func(play func(net.Conn) error) {
		t.Helper()
		local, remote := net.Pipe()
		defer local.Close() //nolint:errcheck // test socket

		checked := make(chan error, 1)
		go func() { checked <- play(remote) }()
		peer.handleConnection(t.Context(), local)
		if err := <-checked; err != nil {
			t.Fatal(err)
		}
	}

	runSession(func(conn net.Conn) error {
		defer conn.Close() //nolint:errcheck // test socket, the peer reads the close
		if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
			return err
		}
		if _, err := conn.Write(minimalOpenMsg(65001)); err != nil {
			return err
		}
		reader := newMessageReader(conn)
		for range 2 { // the peer's OPEN and KEEPALIVE
			if _, _, err := reader.readMessage(); err != nil {
				return err
			}
		}
		eor, err := hex.DecodeString(unmetEOR)
		if err != nil {
			return err
		}
		for range 2 { // two UPDATEs on one session
			if err := sendAndReadReply(conn, eor); err != nil {
				return err
			}
		}
		if _, err := os.Stat(marker); err != nil {
			return errors.New("the first session wrote no marker: " + err.Error())
		}
		return nil
	})
	if _, err := os.Stat(marker + "-2"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the first session wrote the second session's marker")
	}
	runSession(func(conn net.Conn) error { return playDaemon(conn, marker+"-2") })
	if _, err := os.Stat(marker + "-3"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("two sessions wrote a third marker")
	}
}
