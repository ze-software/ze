// Design: plan/deployment-readiness-deep-review.md -- tc original-qdisc restore
// Related: ops_linux.go -- tc operation seam used by snapshot checks

//go:build linux

package trafficnetlink

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vishvananda/netlink"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
)

const tcSnapshotVersion = 1

var _ = env.MustRegister(env.EnvEntry{Key: "ze.config.dir", Type: "string", Description: "Override default config directory"})

type tcSnapshotStore struct {
	Version    int                            `json:"version"`
	Interfaces map[string]tcInterfaceSnapshot `json:"interfaces"`
}

type tcInterfaceSnapshot struct {
	Interface    string          `json:"interface"`
	IfIndex      int             `json:"if-index"`
	HardwareAddr string          `json:"hardware-address"`
	BootID       string          `json:"boot-id"`
	Qdisc        tcQdiscSnapshot `json:"qdisc"`
}

type tcQdiscSnapshot struct {
	Type  string          `json:"type"`
	Attrs tcQdiscAttrs    `json:"attrs"`
	Data  json.RawMessage `json:"data"`
}

type tcQdiscAttrs struct {
	Handle       uint32  `json:"handle"`
	Parent       uint32  `json:"parent"`
	IngressBlock *uint32 `json:"ingress-block,omitempty"`
}

func defaultSnapshotPath() (string, error) {
	dir := env.Get("ze.config.dir")
	if dir == "" {
		dir = paths.DefaultConfigDir()
	}
	if dir == "" {
		return "", fmt.Errorf("cannot resolve config directory")
	}
	return filepath.Join(dir, "state", "traffic-tc-snapshots.json"), nil
}

func currentBootID() (string, error) {
	b, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", fmt.Errorf("read linux boot id: %w", err)
	}
	id := strings.TrimSpace(string(b))
	if id == "" {
		return "", fmt.Errorf("linux boot id is empty")
	}
	return id, nil
}

func loadTCSnapshots(path string) (map[string]tcInterfaceSnapshot, error) {
	if path == "" {
		return map[string]tcInterfaceSnapshot{}, nil
	}
	b, err := os.ReadFile(path) //nolint:gosec // path is the Ze state-dir snapshot store, not external input
	if errors.Is(err, os.ErrNotExist) {
		return map[string]tcInterfaceSnapshot{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read tc snapshot store %q: %w", path, err)
	}
	var store tcSnapshotStore
	if err := json.Unmarshal(b, &store); err != nil {
		return nil, fmt.Errorf("parse tc snapshot store %q: %w", path, err)
	}
	if store.Version != tcSnapshotVersion {
		return nil, fmt.Errorf("tc snapshot store %q: unsupported version %d", path, store.Version)
	}
	if store.Interfaces == nil {
		store.Interfaces = map[string]tcInterfaceSnapshot{}
	}
	return store.Interfaces, nil
}

func saveTCSnapshots(path string, snapshots map[string]tcInterfaceSnapshot) error {
	if path == "" {
		return fmt.Errorf("tc snapshot store path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create tc snapshot store directory: %w", err)
	}
	if len(snapshots) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove tc snapshot store %q: %w", path, err)
		}
		return nil
	}
	store := tcSnapshotStore{Version: tcSnapshotVersion, Interfaces: snapshots}
	b, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal tc snapshot store: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("write tc snapshot store %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace tc snapshot store %q: %w", path, err)
	}
	return nil
}

func newInterfaceSnapshot(link netlink.Link, bootID string, qdisc netlink.Qdisc) (tcInterfaceSnapshot, error) {
	qs, err := newQdiscSnapshot(qdisc)
	if err != nil {
		return tcInterfaceSnapshot{}, err
	}
	attrs := link.Attrs()
	return tcInterfaceSnapshot{
		Interface:    attrs.Name,
		IfIndex:      attrs.Index,
		HardwareAddr: attrs.HardwareAddr.String(),
		BootID:       bootID,
		Qdisc:        qs,
	}, nil
}

func (s tcInterfaceSnapshot) validateLink(link netlink.Link, bootID string) error {
	attrs := link.Attrs()
	if s.BootID != bootID {
		return fmt.Errorf("snapshot boot id %q does not match current boot id %q", s.BootID, bootID)
	}
	if s.Interface != attrs.Name {
		return fmt.Errorf("snapshot interface %q does not match current interface %q", s.Interface, attrs.Name)
	}
	if s.IfIndex != attrs.Index {
		return fmt.Errorf("snapshot ifindex %d does not match current ifindex %d", s.IfIndex, attrs.Index)
	}
	if s.HardwareAddr != attrs.HardwareAddr.String() {
		return fmt.Errorf("snapshot hardware address %q does not match current hardware address %q", s.HardwareAddr, attrs.HardwareAddr.String())
	}
	return nil
}

func newQdiscSnapshot(qdisc netlink.Qdisc) (tcQdiscSnapshot, error) {
	if _, ok := qdisc.(*netlink.GenericQdisc); ok {
		return tcQdiscSnapshot{}, fmt.Errorf("qdisc %q cannot be snapshotted exactly by backend tc", qdisc.Type())
	}
	switch qdisc.(type) {
	case *netlink.PfifoFast, *netlink.Prio, *netlink.Htb, *netlink.Hfsc,
		*netlink.Tbf, *netlink.Netem, *netlink.Fq, *netlink.FqCodel, *netlink.Sfq:
		b, err := json.Marshal(qdisc)
		if err != nil {
			return tcQdiscSnapshot{}, fmt.Errorf("snapshot qdisc %q: %w", qdisc.Type(), err)
		}
		return tcQdiscSnapshot{Type: qdisc.Type(), Attrs: snapshotAttrs(qdisc.Attrs()), Data: b}, nil
	default:
		return tcQdiscSnapshot{}, fmt.Errorf("qdisc %q cannot be restored exactly by backend tc", qdisc.Type())
	}
}

func snapshotAttrs(attrs *netlink.QdiscAttrs) tcQdiscAttrs {
	var ingressBlock *uint32
	if attrs.IngressBlock != nil {
		v := *attrs.IngressBlock
		ingressBlock = &v
	}
	return tcQdiscAttrs{Handle: attrs.Handle, Parent: attrs.Parent, IngressBlock: ingressBlock}
}

func (a tcQdiscAttrs) toNetlink(linkIndex int) netlink.QdiscAttrs {
	return netlink.QdiscAttrs{LinkIndex: linkIndex, Handle: a.Handle, Parent: a.Parent, IngressBlock: a.IngressBlock}
}

func (s tcQdiscSnapshot) toNetlink(linkIndex int) (netlink.Qdisc, error) {
	switch s.Type {
	case "pfifo_fast":
		var q netlink.PfifoFast
		return finishSnapshotQdisc(&q, s, linkIndex)
	case "prio":
		var q netlink.Prio
		return finishSnapshotQdisc(&q, s, linkIndex)
	case "htb":
		var q netlink.Htb
		return finishSnapshotQdisc(&q, s, linkIndex)
	case "hfsc":
		var q netlink.Hfsc
		return finishSnapshotQdisc(&q, s, linkIndex)
	case "tbf":
		var q netlink.Tbf
		return finishSnapshotQdisc(&q, s, linkIndex)
	case "netem":
		var q netlink.Netem
		return finishSnapshotQdisc(&q, s, linkIndex)
	case "fq":
		var q netlink.Fq
		return finishSnapshotQdisc(&q, s, linkIndex)
	case "fq_codel":
		var q netlink.FqCodel
		return finishSnapshotQdisc(&q, s, linkIndex)
	case "sfq":
		var q netlink.Sfq
		return finishSnapshotQdisc(&q, s, linkIndex)
	default:
		return nil, fmt.Errorf("qdisc %q cannot be restored exactly by backend tc", s.Type)
	}
}

func finishSnapshotQdisc[T netlink.Qdisc](q T, s tcQdiscSnapshot, linkIndex int) (netlink.Qdisc, error) {
	if err := json.Unmarshal(s.Data, q); err != nil {
		return nil, fmt.Errorf("restore qdisc %q snapshot: %w", s.Type, err)
	}
	*q.Attrs() = s.Attrs.toNetlink(linkIndex)
	return q, nil
}
