// Design: docs/architecture/traffic/tc-original-qdisc-restore.md -- real kernel restart evidence.
package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/ze-software/ze/pkg/zefs"
)

func storageTCRestart(ctx context.Context, dir string) error {
	name := fmt.Sprintf("zs%x", os.Getpid())
	run := func(program string, args ...string) ([]byte, error) {
		out, err := exec.CommandContext(ctx, program, args...).CombinedOutput()
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
	if _, err := run("tc", "qdisc", "replace", "dev", name, "root", "handle", "7:", "fq_codel", "limit", "321", "quantum", "777"); err != nil {
		return err
	}
	type qdisc struct {
		Kind    string `json:"kind"`
		Handle  string `json:"handle"`
		Root    bool   `json:"root"`
		Options struct {
			Limit   int `json:"limit"`
			Quantum int `json:"quantum"`
		} `json:"options"`
	}
	root := func() (qdisc, error) {
		out, err := run("tc", "-j", "qdisc", "show", "dev", name)
		if err != nil {
			return qdisc{}, err
		}
		var rows []qdisc
		if err := json.Unmarshal(out, &rows); err != nil {
			return qdisc{}, err
		}
		for _, row := range rows {
			if row.Root {
				return row, nil
			}
		}
		return qdisc{}, fmt.Errorf("no root qdisc: %s", out)
	}
	original, err := root()
	if err != nil {
		return err
	}
	if original.Kind != "fq_codel" || original.Options.Limit != 321 || original.Options.Quantum != 777 {
		return fmt.Errorf("custom original qdisc absent: %+v", original)
	}
	config := resolveRIRConfig("") + fmt.Sprintf(`traffic { control { backend tc; interface %s { qdisc { type htb; default-class default; class default { rate 10mbit; ceil 10mbit; priority 1; } } } } }`, name)
	first, _, err := storageConsumerDaemon(ctx, dir, config, "first.log")
	if err != nil {
		return fmt.Errorf("first TC daemon startup: %w", err)
	}
	defer first.stop()
	if !Poll(ctx, 80, 100*time.Millisecond, func() bool { q, err := root(); return err == nil && q.Kind == "htb" }) {
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
	if q.Kind != "htb" {
		return fmt.Errorf("crash lost the live HTB witness: %+v", q)
	}
	second, _, err := storageConsumerDaemon(ctx, dir, config, "second.log")
	if err != nil {
		return fmt.Errorf("restarted TC daemon startup: %w", err)
	}
	defer second.stop()
	if !Poll(ctx, 80, 100*time.Millisecond, func() bool { q, err := root(); return err == nil && q.Kind == "htb" }) {
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
		return fmt.Errorf("restored TC snapshot was not removed: %v", err)
	}
	return nil
}
