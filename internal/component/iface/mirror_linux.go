// Design: plan/spec-iface-0-umbrella.md — Traffic mirroring via tc mirred
// Overview: iface.go — shared types and topic constants

package iface

import (
	"fmt"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// SetupMirror configures ingress and/or egress traffic mirroring from srcIface
// to dstIface using tc qdiscs and matchall filters with mirred actions.
//
// For ingress mirroring, an ingress qdisc is added on srcIface with a matchall
// filter that mirrors all incoming packets to dstIface.
//
// For egress mirroring, a clsact qdisc is used (preferred over ingress qdisc
// for egress support). Not all kernels support egress mirred; if clsact setup
// fails, egress mirroring returns an error.
//
// When both ingress and egress are requested, clsact is used for both directions
// since it supports filters on both HANDLE_MIN_INGRESS and HANDLE_MIN_EGRESS.
func SetupMirror(srcIface, dstIface string, ingress, egress bool) error {
	if err := validateIfaceName(srcIface); err != nil {
		return fmt.Errorf("iface: mirror: src: %w", err)
	}
	if err := validateIfaceName(dstIface); err != nil {
		return fmt.Errorf("iface: mirror: dst: %w", err)
	}
	if !ingress && !egress {
		return fmt.Errorf("iface: mirror: at least one of ingress or egress must be true")
	}

	src, err := netlink.LinkByName(srcIface)
	if err != nil {
		return fmt.Errorf("iface: mirror: src %q not found: %w", srcIface, err)
	}
	dst, err := netlink.LinkByName(dstIface)
	if err != nil {
		return fmt.Errorf("iface: mirror: dst %q not found: %w", dstIface, err)
	}

	dstIndex := dst.Attrs().Index
	srcIndex := src.Attrs().Index

	// When egress is requested (or both), use clsact qdisc which supports
	// both ingress and egress filter attachment points.
	if egress {
		if err := setupClsactMirror(srcIndex, dstIndex, ingress, egress); err != nil {
			return err
		}
		return nil
	}

	// Ingress only: use the dedicated ingress qdisc.
	return setupIngressMirror(srcIndex, dstIndex)
}

// setupClsactMirror installs a clsact qdisc and attaches mirred filters.
func setupClsactMirror(srcIndex, dstIndex int, ingress, egress bool) error {
	qdisc := &netlink.Clsact{
		QdiscAttrs: netlink.QdiscAttrs{
			LinkIndex: srcIndex,
			Handle:    netlink.MakeHandle(0xffff, 0),
			Parent:    netlink.HANDLE_CLSACT,
		},
	}
	if err := netlink.QdiscAdd(qdisc); err != nil {
		return fmt.Errorf("iface: mirror: add clsact qdisc: %w", err)
	}

	if ingress {
		filter := &netlink.MatchAll{
			FilterAttrs: netlink.FilterAttrs{
				LinkIndex: srcIndex,
				Parent:    netlink.HANDLE_MIN_INGRESS,
				Priority:  1,
				Protocol:  unix.ETH_P_ALL,
			},
			Actions: []netlink.Action{
				&netlink.MirredAction{
					ActionAttrs: netlink.ActionAttrs{
						Action: netlink.TC_ACT_PIPE,
					},
					MirredAction: netlink.TCA_EGRESS_MIRROR,
					Ifindex:      dstIndex,
				},
			},
		}
		if err := netlink.FilterAdd(filter); err != nil {
			_ = netlink.QdiscDel(qdisc) // best-effort cleanup
			return fmt.Errorf("iface: mirror: add ingress filter: %w", err)
		}
	}

	if egress {
		filter := &netlink.MatchAll{
			FilterAttrs: netlink.FilterAttrs{
				LinkIndex: srcIndex,
				Parent:    netlink.HANDLE_MIN_EGRESS,
				Priority:  1,
				Protocol:  unix.ETH_P_ALL,
			},
			Actions: []netlink.Action{
				&netlink.MirredAction{
					ActionAttrs: netlink.ActionAttrs{
						Action: netlink.TC_ACT_PIPE,
					},
					MirredAction: netlink.TCA_EGRESS_MIRROR,
					Ifindex:      dstIndex,
				},
			},
		}
		if err := netlink.FilterAdd(filter); err != nil {
			_ = netlink.QdiscDel(qdisc) // best-effort cleanup
			return fmt.Errorf("iface: mirror: add egress filter: %w", err)
		}
	}

	return nil
}

// setupIngressMirror installs an ingress qdisc and attaches a mirred filter.
func setupIngressMirror(srcIndex, dstIndex int) error {
	qdisc := &netlink.Ingress{
		QdiscAttrs: netlink.QdiscAttrs{
			LinkIndex: srcIndex,
			Handle:    netlink.MakeHandle(0xffff, 0),
			Parent:    netlink.HANDLE_INGRESS,
		},
	}
	if err := netlink.QdiscAdd(qdisc); err != nil {
		return fmt.Errorf("iface: mirror: add ingress qdisc: %w", err)
	}

	filter := &netlink.MatchAll{
		FilterAttrs: netlink.FilterAttrs{
			LinkIndex: srcIndex,
			Parent:    netlink.HANDLE_MIN_INGRESS,
			Priority:  1,
			Protocol:  unix.ETH_P_ALL,
		},
		Actions: []netlink.Action{
			&netlink.MirredAction{
				ActionAttrs: netlink.ActionAttrs{
					Action: netlink.TC_ACT_PIPE,
				},
				MirredAction: netlink.TCA_EGRESS_MIRROR,
				Ifindex:      dstIndex,
			},
		},
	}
	if err := netlink.FilterAdd(filter); err != nil {
		_ = netlink.QdiscDel(qdisc) // best-effort cleanup
		return fmt.Errorf("iface: mirror: add ingress filter: %w", err)
	}

	return nil
}

// RemoveMirror removes all mirroring configuration from srcIface by deleting
// both clsact and ingress qdiscs. It is safe to call even if no mirroring is
// configured; missing qdiscs are silently ignored.
func RemoveMirror(srcIface string) error {
	if err := validateIfaceName(srcIface); err != nil {
		return fmt.Errorf("iface: mirror: %w", err)
	}

	link, err := netlink.LinkByName(srcIface)
	if err != nil {
		return fmt.Errorf("iface: mirror: %q not found: %w", srcIface, err)
	}

	linkIndex := link.Attrs().Index

	// Try removing clsact qdisc (covers both ingress+egress and egress-only).
	clsact := &netlink.Clsact{
		QdiscAttrs: netlink.QdiscAttrs{
			LinkIndex: linkIndex,
			Handle:    netlink.MakeHandle(0xffff, 0),
			Parent:    netlink.HANDLE_CLSACT,
		},
	}
	clsactErr := netlink.QdiscDel(clsact)

	// Try removing ingress qdisc (covers ingress-only setup).
	ingress := &netlink.Ingress{
		QdiscAttrs: netlink.QdiscAttrs{
			LinkIndex: linkIndex,
			Handle:    netlink.MakeHandle(0xffff, 0),
			Parent:    netlink.HANDLE_INGRESS,
		},
	}
	ingressErr := netlink.QdiscDel(ingress)

	// Both clsact and ingress share the same parent handle (HANDLE_CLSACT ==
	// HANDLE_INGRESS), so in practice only one will exist. If both deletions
	// fail, neither qdisc was present, which is fine for idempotent cleanup.
	_ = clsactErr
	_ = ingressErr

	return nil
}
