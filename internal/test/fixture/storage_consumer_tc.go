// Design: docs/architecture/traffic/tc-original-qdisc-restore.md -- real kernel restart evidence.
// Related: storage_consumer_tc_other.go -- the refusal every other platform answers

//go:build linux

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/pkg/zefs"
)

// qdiscKindHTB is the root qdisc kind the daemon installs, as the kernel
// reports it.
const qdiscKindHTB = "htb"

func storageTCRestart(ctx context.Context, dir string) error {
	name := fmt.Sprintf("zs%x", os.Getpid())
	run := func(program string, args ...string) ([]byte, error) {
		out, err := exec.CommandContext(ctx, program, args...).CombinedOutput() //nolint:gosec // the fixture chooses the program and its arguments
		if err != nil {
			return out, fmt.Errorf("%s %v: %w: %s", program, args, err, out)
		}
		return out, nil
	}
	if _, err := run("ip", "link", "add", name, "type", "dummy"); err != nil {
		return err
	}
	defer func() { _, _ = run("ip", "link", "del", name) }()
	if _, err := run("ip", "link", "set", name, "up"); err != nil {
		return err
	}
	// The qdisc is programmed and read through netlink, as the traffic plugin's
	// own adapter does: Ze's guest ships busybox tc, which cannot replace a
	// root qdisc and has no JSON output.
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("find dummy link %s: %w", name, err)
	}
	custom := netlink.NewFqCodel(netlink.QdiscAttrs{
		LinkIndex: link.Attrs().Index,
		Handle:    netlink.MakeHandle(7, 0),
		Parent:    netlink.HANDLE_ROOT,
	})
	custom.Limit = 321
	custom.Quantum = 777
	if err := netlink.QdiscReplace(custom); err != nil {
		return fmt.Errorf("install fq_codel limit 321 quantum 777 as root 7: on %s: %w", name, err)
	}
	// qdisc is the root qdisc as the kernel reports it. Limit and Quantum are
	// read from an fq_codel only, and stay zero for any other kind.
	type qdisc struct {
		Kind    string
		Handle  uint32
		Limit   uint32
		Quantum uint32
	}
	root := func() (qdisc, error) {
		rows, err := netlink.QdiscList(link)
		if err != nil {
			return qdisc{}, fmt.Errorf("list qdiscs on %s: %w", name, err)
		}
		for _, row := range rows {
			attrs := row.Attrs()
			if attrs.Parent != netlink.HANDLE_ROOT {
				continue
			}
			found := qdisc{Kind: row.Type(), Handle: attrs.Handle}
			if codel, ok := row.(*netlink.FqCodel); ok {
				found.Limit = codel.Limit
				found.Quantum = codel.Quantum
			}
			return found, nil
		}
		return qdisc{}, fmt.Errorf("no root qdisc on %s among %d qdiscs", name, len(rows))
	}
	original, err := root()
	if err != nil {
		return err
	}
	if original.Kind != "fq_codel" || original.Handle != custom.Handle || original.Limit != 321 || original.Quantum != 777 {
		return fmt.Errorf("custom original qdisc absent: %+v", original)
	}
	config := resolveRIRConfig("") + fmt.Sprintf(`traffic { control { backend tc; interface %s { qdisc { type htb; default-class default; class default { rate 10mbit; ceil 10mbit; priority 1; } } } } }`, name)
	first, _, err := storageConsumerDaemon(ctx, dir, config, "first.log")
	if err != nil {
		return fmt.Errorf("first TC daemon startup: %w", err)
	}
	defer first.stop()
	if !Poll(ctx, 80, 100*time.Millisecond, func() bool { q, err := root(); return err == nil && q.Kind == qdiscKindHTB }) {
		return fmt.Errorf("daemon did not install HTB\n%s", first.contents())
	}
	if _, err := storageConsumerKey(dir, zefs.KeyTrafficTCSnapshot.Pattern); err != nil {
		return err
	}
	// Clean shutdown restores the qdisc and removes its snapshot. Crash instead,
	// after observing HTB: its destructive replace follows durable snapshot save.
	if err := first.command.Process.Kill(); err != nil {
		return err
	}
	if err := first.command.Wait(); err == nil {
		return fmt.Errorf("crashed daemon unexpectedly exited successfully")
	}
	_ = first.log.Close()
	first.command = nil
	q, err := root()
	if err != nil {
		return err
	}
	if q.Kind != qdiscKindHTB {
		return fmt.Errorf("crash lost the live HTB witness: %+v", q)
	}
	second, _, err := storageConsumerDaemon(ctx, dir, config, "second.log")
	if err != nil {
		return fmt.Errorf("restarted TC daemon startup: %w", err)
	}
	defer second.stop()
	if !Poll(ctx, 80, 100*time.Millisecond, func() bool { q, err := root(); return err == nil && q.Kind == qdiscKindHTB }) {
		return fmt.Errorf("restarted daemon did not own HTB\n%s", second.contents())
	}
	second.stop() // Backend.Close consumes the original snapshot loaded at boot.
	restored, err := root()
	if err != nil {
		return err
	}
	if restored != original {
		return fmt.Errorf("restart restored %+v, want original %+v", restored, original)
	}
	if _, err := storageConsumerKey(dir, zefs.KeyTrafficTCSnapshot.Pattern); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("restored TC snapshot was not removed: %w", err)
	}
	return nil
}
