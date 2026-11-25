// Design: docs/architecture/chaos-web-dashboard.md — BGP peer simulation
// Overview: simulator.go — main simulation loop and types

package peer

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// sendInitialIPRoutes preserves individual UPDATEs for small workloads and
// route sets. A larger seeded workload shares path attributes across a packet.
func sendInitialIPRoutes(ctx context.Context, conn net.Conn, sender *Sender, fam family.Family, prefixes []netip.Prefix, emit func(Event)) (int, error) {
	if len(prefixes) == 0 {
		return 0, nil
	}
	if sender.routeSetMode {
		return sendIndividualIPRoutes(ctx, conn, sender, fam, prefixes, emit)
	}
	update, err := sender.seededUpdate(fam, prefixes[0])
	if err != nil {
		return 0, err
	}
	limit := update.nlriLimit()
	count, size := initialIPBatchSize(prefixes, limit)
	if count == len(prefixes) {
		return sendIndividualIPRoutes(ctx, conn, sender, fam, prefixes, emit)
	}

	name := fam.String()
	var nlriScratch, attrScratch [message.MaxMsgLen]byte
	sent := 0
	for count > 0 {
		if err := ctx.Err(); err != nil {
			return sent, err
		}
		batch := prefixes[:count]
		data := update.pack(batch, nlriScratch[:size], attrScratch[:])
		at := time.Now()
		n, writeErr := conn.Write(data)
		if writeErr != nil {
			return sent, writeErr
		}
		if n != len(data) {
			return sent, io.ErrShortWrite
		}
		for i, prefix := range batch {
			event := Event{Type: EventRouteSent, Time: at, Prefix: prefix, Family: name}
			// MRT and throughput observers account for one complete packet.
			if i == 0 {
				event.BGPMessage = data
				event.BytesSent = int64(n)
			}
			emit(event)
		}
		sent += count
		prefixes = prefixes[count:]
		count, size = initialIPBatchSize(prefixes, limit)
	}
	return sent, nil
}

func sendIndividualIPRoutes(ctx context.Context, conn net.Conn, sender *Sender, fam family.Family, prefixes []netip.Prefix, emit func(Event)) (int, error) {
	name := fam.String()
	for i, prefix := range prefixes {
		if err := ctx.Err(); err != nil {
			return i, err
		}
		var data []byte
		if fam.SAFI == family.SAFIMulticast {
			data = sender.buildMulticastRoute(prefix)
		} else {
			data = sender.BuildRoute(prefix)
		}
		if err := sendRouteUpdate(conn, prefix, name, data, emit); err != nil {
			return i, err
		}
	}
	return len(prefixes), nil
}

// seededUpdate keeps the ordinary sender's attributes borrowed from its
// builder. Its caller packs only that family before using the builder again.
type seededUpdate struct {
	family      family.Family
	beforeReach []byte
	afterReach  []byte
	reach       *attribute.MPReachNLRI
}

func (s *Sender) seededUpdate(fam family.Family, first netip.Prefix) (seededUpdate, error) {
	sample, err := s.builder.BuildUnicast(&message.UnicastParams{
		Prefix: first, NextHop: s.nextHop, Origin: attribute.OriginIGP, SAFI: attribute.SAFI(fam.SAFI),
	})
	if err != nil {
		return seededUpdate{}, err
	}
	update := seededUpdate{family: fam, beforeReach: sample.PathAttributes}
	if fam == family.IPv4Unicast {
		return update, nil
	}

	index, err := attribute.BuildSpanIndex(sample.PathAttributes)
	if err != nil {
		return seededUpdate{}, err
	}
	span, ok := index.Find(attribute.AttrMPReachNLRI)
	if !ok {
		return seededUpdate{}, fmt.Errorf("seeded %s UPDATE has no MP_REACH_NLRI", fam)
	}
	start := int(span.Offset)
	end := start + int(span.Length)
	reach, err := attribute.ParseMPReachNLRI(sample.PathAttributes[start:end])
	if err != nil {
		return seededUpdate{}, err
	}
	reach.NLRI = nil
	update.beforeReach = sample.PathAttributes[:start-int(span.HdrLen)]
	update.afterReach = sample.PathAttributes[end:]
	update.reach = reach
	return update, nil
}

func (u *seededUpdate) nlriLimit() int {
	space := message.MaxMsgLen - message.MinUpdateLen - len(u.beforeReach) - len(u.afterReach)
	if u.reach != nil {
		// Reserve the extra header byte when the packed value exceeds 255 bytes.
		space -= attribute.AttrWireLen(u.reach) + 1
	}
	return space
}

func initialIPBatchSize(prefixes []netip.Prefix, limit int) (count, size int) {
	for i, prefix := range prefixes {
		n := 1 + nlri.PrefixBytes(prefix.Bits())
		if size+n > limit {
			return i, size
		}
		size += n
	}
	return len(prefixes), size
}

func (u *seededUpdate) pack(prefixes []netip.Prefix, nlriBytes, attrs []byte) []byte {
	off := 0
	for _, prefix := range prefixes {
		off += nlri.NewINET(u.family, prefix, 0, false).WriteTo(nlriBytes, off)
	}
	if u.reach == nil {
		return serializeMessage(&message.Update{PathAttributes: u.beforeReach, NLRI: nlriBytes})
	}

	reach := *u.reach
	reach.NLRI = nlriBytes
	off = copy(attrs, u.beforeReach)
	off += attribute.WriteAttrTo(&reach, attrs, off)
	off += copy(attrs[off:], u.afterReach)
	return serializeMessage(&message.Update{PathAttributes: attrs[:off]})
}
