// Design: docs/features/interfaces.md -- Traffic mirroring via tc mirred
// Overview: ifacenetlink.go -- package hub

//go:build linux

package ifacenetlink

import (
	"errors"
	"fmt"
	"strings"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

func isNotFound(err error) bool {
	return err != nil && (errors.Is(err, unix.ENOENT) || errors.Is(err, unix.EINVAL) || strings.Contains(err.Error(), "no such"))
}

func (b *netlinkBackend) SetupMirror(srcIface, dstIface string, ingress, egress bool) error {
	if err := iface.ValidateIfaceName(srcIface); err != nil {
		return fmt.Errorf("iface: mirror: src: %w", err)
	}
	if err := iface.ValidateIfaceName(dstIface); err != nil {
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

	if egress {
		return setupClsactMirror(srcIndex, dstIndex, ingress, egress)
	}
	return setupIngressMirror(srcIndex, dstIndex)
}

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
					ActionAttrs:  netlink.ActionAttrs{Action: netlink.TC_ACT_PIPE},
					MirredAction: netlink.TCA_EGRESS_MIRROR,
					Ifindex:      dstIndex,
				},
			},
		}
		if err := netlink.FilterAdd(filter); err != nil {
			_ = netlink.QdiscDel(qdisc)
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
					ActionAttrs:  netlink.ActionAttrs{Action: netlink.TC_ACT_PIPE},
					MirredAction: netlink.TCA_EGRESS_MIRROR,
					Ifindex:      dstIndex,
				},
			},
		}
		if err := netlink.FilterAdd(filter); err != nil {
			_ = netlink.QdiscDel(qdisc)
			return fmt.Errorf("iface: mirror: add egress filter: %w", err)
		}
	}

	return nil
}

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
				ActionAttrs:  netlink.ActionAttrs{Action: netlink.TC_ACT_PIPE},
				MirredAction: netlink.TCA_EGRESS_MIRROR,
				Ifindex:      dstIndex,
			},
		},
	}
	if err := netlink.FilterAdd(filter); err != nil {
		_ = netlink.QdiscDel(qdisc)
		return fmt.Errorf("iface: mirror: add ingress filter: %w", err)
	}

	return nil
}

func (b *netlinkBackend) RemoveMirror(srcIface string) error {
	if err := iface.ValidateIfaceName(srcIface); err != nil {
		return fmt.Errorf("iface: mirror: %w", err)
	}

	link, err := netlink.LinkByName(srcIface)
	if err != nil {
		return fmt.Errorf("iface: mirror: %q not found: %w", srcIface, err)
	}

	linkIndex := link.Attrs().Index

	clsact := &netlink.Clsact{
		QdiscAttrs: netlink.QdiscAttrs{
			LinkIndex: linkIndex,
			Handle:    netlink.MakeHandle(0xffff, 0),
			Parent:    netlink.HANDLE_CLSACT,
		},
	}
	clsactErr := netlink.QdiscDel(clsact)

	ingress := &netlink.Ingress{
		QdiscAttrs: netlink.QdiscAttrs{
			LinkIndex: linkIndex,
			Handle:    netlink.MakeHandle(0xffff, 0),
			Parent:    netlink.HANDLE_INGRESS,
		},
	}
	ingressErr := netlink.QdiscDel(ingress)

	if clsactErr != nil && !isNotFound(clsactErr) {
		return fmt.Errorf("iface: mirror: remove clsact qdisc: %w", clsactErr)
	}
	if ingressErr != nil && !isNotFound(ingressErr) {
		return fmt.Errorf("iface: mirror: remove ingress qdisc: %w", ingressErr)
	}

	return nil
}
