// Design: docs/architecture/testing/interop.md -- FRR original-session EOR evidence.
package bgp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// noFamilyEORIdentity identifies the original configured session and the process
// writing its startup-fenced log. PID alone cannot reject process-ID reuse.
// The zero value is invalid; readNoFamilyEORIdentity admits each native field.
type noFamilyEORIdentity struct {
	pid, startTicks uint64
	logDevice, logInode uint64
	session aigpRecipientFence
}

func readNoFamilyEORIdentity(ctx context.Context, lab interoplab.CheckerLab, address string) (noFamilyEORIdentity, error) {
	var identity noFamilyEORIdentity
	pidText, err := lab.Query(ctx, peerFRR, []string{cmdCat, frrBGPDPIDFile}, nil)
	if err != nil { return identity, err }
	identity.pid, err = strconv.ParseUint(strings.TrimSpace(pidText), 10, 64)
	if err != nil || identity.pid == 0 { return identity, fmt.Errorf("invalid FRR bgpd PID %q", pidText) }
	statText, err := lab.Query(ctx, peerFRR, []string{cmdCat, "/proc/" + strconv.FormatUint(identity.pid, 10) + "/stat"}, nil)
	if err != nil { return identity, err }
	pid, tail, ok := strings.Cut(strings.TrimSpace(statText), " (bgpd) ")
	fields := strings.Fields(tail)
	if !ok || pid != strconv.FormatUint(identity.pid, 10) || len(fields) < 20 {
		return identity, errors.New("FRR bgpd process identity is incomplete or belongs to another process")
	}
	// proc_pid_stat(5), field 22; fields begins with field 3 (state).
	identity.startTicks, err = strconv.ParseUint(fields[19], 10, 64)
	if err != nil || identity.startTicks == 0 { return identity, errors.New("FRR bgpd start time is absent or invalid") }
	logStat, err := lab.Query(ctx, peerFRR, []string{"stat", "-Lc", "%d:%i", frrLogPath}, nil)
	if err != nil { return identity, err }
	device, inode, ok := strings.Cut(strings.TrimSpace(logStat), ":")
	if !ok { return identity, errors.New("FRR log identity is incomplete") }
	identity.logDevice, err = strconv.ParseUint(device, 10, 64)
	if err != nil { return identity, fmt.Errorf("FRR log device: %w", err) }
	identity.logInode, err = strconv.ParseUint(inode, 10, 64)
	if err != nil || identity.logInode == 0 { return identity, errors.New("FRR log inode is absent or invalid") }
	neighbor, err := lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show bgp neighbor " + address + " json"}, nil)
	if err != nil { return identity, err }
	identity.session, err = readFRRAIGPRecipientFence(neighbor, address)
	if err != nil { return identity, err }
	if identity.session.established != 1 || identity.session.dropped != 0 {
		return identity, errors.New("FRR did not retain its original no-family session")
	}
	return identity, nil
}

func requireNoFamilyEORIdentity(ctx context.Context, lab interoplab.CheckerLab, address string, want noFamilyEORIdentity) error {
	got, err := readNoFamilyEORIdentity(ctx, lab, address)
	if err != nil { return err }
	if got != want { return fmt.Errorf("FRR no-family daemon, log or session changed: before=%+v after=%+v", want, got) }
	return nil
}

// FRR 10.7.1's file target prefixes a native message with local date/time,
// protocol and xref UID. Parsing the envelope prevents arbitrary text from
// embedding an apparently matching receive event.
func noFamilyFRRLogMessage(line string) (string, bool) {
	timestamp, message, ok := strings.Cut(line, " BGP: ")
	if !ok { return "", false }
	if _, err := time.Parse("2006/01/02 15:04:05", timestamp); err != nil { return "", false }
	if !strings.HasPrefix(message, "[") { return "", false }
	uid, message, ok := strings.Cut(message[1:], "] ")
	if !ok || strings.ContainsAny(uid, " \t[]") || uid == "" { return "", false }
	return message, true
}

