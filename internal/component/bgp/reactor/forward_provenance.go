// Design: docs/architecture/bgp/structural-forwarding.md -- forwarding path ownership.
// Related: forward_pool.go -- the item owns and releases transformed provenance.
package reactor

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// fwdOutputSection identifies a section before ownership filtering. The writer
// MUST resolve these positions before compacting sections; the producer MUST
// set writeBody before each writer call. Split bodies retain separate ordinals.
type fwdOutputSection struct {
	body          int
	family        family.Family
	withdraw      bool
	multiprotocol bool
}

type fwdProvenanceSpan struct {
	section *fwdProvenanceSection
	start   int
	count   int
}

type fwdProvenanceSection struct {
	family        family.Family
	withdraw      bool
	multiprotocol bool
	paths         []adjOutPath
	consumed      int
}

// fwdProvenance is scratch for transformations that destroy ingress identity.
// Each map and slice is bounded by the NLRIs in one extended UPDATE. The item
// MUST release it exactly once; neither cache reuse nor overflow borrows it.
type fwdProvenance struct {
	spans                 map[fwdOutputSection]fwdProvenanceSpan
	withdrawn             map[fwdPathKey]struct{}
	sections              [4]fwdProvenanceSection
	count                 int
	synthesizedWithdrawal bool
}

var fwdProvenancePool = sync.Pool{New: func() any {
	return &fwdProvenance{spans: make(map[fwdOutputSection]fwdProvenanceSpan), withdrawn: make(map[fwdPathKey]struct{})}
}}

func (p *fwdProvenance) release() {
	clear(p.spans)
	clear(p.withdrawn)
	for i := range p.sections {
		clear(p.sections[i].paths)
		p.sections[i].paths = p.sections[i].paths[:0]
		p.sections[i].consumed = 0
	}
	p.count = 0
	p.synthesizedWithdrawal = false
	fwdProvenancePool.Put(p)
}

// writePath returns the source path before ADD-PATH identifiers were stripped
// or regenerated. RFC 7911 Section 5: "If a BGP speaker receives a message
// to withdraw a prefix with a Path Identifier not seen before, it SHOULD
// silently ignore it."
func (item *fwdItem) writePath(fam family.Family, _ []byte, withdraw, multiprotocol bool, occurrence int) (adjOutPath, error) {
	if item.authority == adjOutExpected {
		return adjOutPath{revision: item.expectedMessage}, nil
	}
	if item.authority == adjOutInitial && item.provenance == nil {
		return adjOutPath{source: item.receivedPeer, received: item.initialSourcePath,
			addPath: item.initialSourceAddPath}, nil
	}
	if item.provenance == nil {
		return adjOutPath{source: item.receivedPeer}, nil
	}
	span, found := item.provenance.spans[fwdOutputSection{body: item.writeBody, family: fam, withdraw: withdraw, multiprotocol: multiprotocol}]
	if !found {
		return adjOutPath{}, fmt.Errorf("forward ownership: missing section provenance for body %d, family %s", item.writeBody, fam)
	}
	if occurrence < 0 {
		return adjOutPath{}, fmt.Errorf("forward ownership: negative path occurrence")
	}
	if occurrence >= span.count {
		return adjOutPath{}, fmt.Errorf("forward ownership: path occurrence exceeds source section")
	}
	return span.section.paths[span.start+occurrence], nil
}

// prepareFwdProvenance snapshots only identity, not attributes or payloads.
// RFC 7911 Section 2: "A BGP speaker that re-advertises a route MUST generate
// its own Path Identifier to be associated with the re-advertised route."
// The manifest therefore precedes the destination writer's identity lookup.
// Callers MUST release it through releaseItem, including pre-dispatch errors.
func prepareFwdProvenance(item *fwdItem, original, transformed *wireu.WireUpdate, synthesized bool) error {
	ctx := bgpctx.Registry.Get(transformed.SourceCtxID())
	if !synthesized {
		if ctx == nil {
			return nil
		}
		if !ctx.AnyAddPath() {
			return nil
		}
		hasFramedPath := false
		// RFC 7911 Section 4 negotiates ADD-PATH independently per family.
		if err := fwdBodySections(transformed.Payload(), func(_ []byte, fam family.Family, _, _ bool) error {
			hasFramedPath = hasFramedPath || ctx.AddPath(fam)
			return nil
		}); err != nil {
			return err
		}
		if !hasFramedPath {
			return nil
		}
	}
	p, ok := fwdProvenancePool.Get().(*fwdProvenance)
	if !ok {
		return fmt.Errorf("forward ownership: invalid provenance pool entry")
	}
	item.provenance = p
	originalCtx := bgpctx.Registry.Get(original.SourceCtxID())
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	if synthesized {
		err := fwdBodySections(original.Payload(), func(data []byte, fam family.Family, withdraw, _ bool) error {
			if !withdraw {
				return nil
			}
			return fwdProvenancePaths(data, fam, originalCtx, true, func(raw []byte, received uint32, _ bool) error {
				var key fwdPathKey
				if err := fwdPathKeyFor(&key, fam, received, raw, true, scratch[:]); err != nil {
					return err
				}
				p.withdrawn[key] = struct{}{}
				return nil
			})
		})
		if err != nil {
			return err
		}
	}
	err := fwdBodySections(transformed.Payload(), func(data []byte, fam family.Family, withdraw, multiprotocol bool) error {
		if p.count == len(p.sections) {
			return fmt.Errorf("forward ownership: too many NLRI sections")
		}
		section := &p.sections[p.count]
		section.family, section.withdraw, section.multiprotocol = fam, withdraw, multiprotocol
		p.count++
		return fwdProvenancePaths(data, fam, ctx, withdraw, func(raw []byte, received uint32, addPath bool) error {
			path := adjOutPath{source: item.receivedPeer, received: received, addPath: addPath}
			if synthesized && withdraw {
				var key fwdPathKey
				if err := fwdPathKeyFor(&key, fam, received, raw, true, scratch[:]); err != nil {
					return err
				}
				_, receivedWithdrawal := p.withdrawn[key]
				path.synthesized = !receivedWithdrawal
				p.synthesizedWithdrawal = p.synthesizedWithdrawal || path.synthesized
			}
			section.paths = append(section.paths, path)
			return nil
		})
	})
	if err != nil {
		return err
	}
	destCtx := bgpctx.Registry.Get(item.peer.sendContextID())
	bodyIndex := 0
	bind := func(data []byte, fam family.Family, withdraw, multiprotocol bool) error {
		var section *fwdProvenanceSection
		for i := range p.count {
			candidate := &p.sections[i]
			if candidate.family == fam && candidate.withdraw == withdraw && candidate.multiprotocol == multiprotocol {
				section = candidate
				break
			}
		}
		if section == nil {
			return fmt.Errorf("forward ownership: output section has no source")
		}
		split := nlrisplit.Get(fam)
		if withdraw {
			split = nlrisplit.GetWithdraw(fam)
		}
		if split == nil {
			return nlrisplit.ErrUnsupported
		}
		count, err := split(data, destCtx != nil && destCtx.AddPath(fam), nil)
		if err != nil {
			return err
		}
		if count > len(section.paths)-section.consumed {
			return fmt.Errorf("forward ownership: output exceeds source paths")
		}
		p.spans[fwdOutputSection{body: bodyIndex, family: fam, withdraw: withdraw, multiprotocol: multiprotocol}] =
			fwdProvenanceSpan{section: section, start: section.consumed, count: count}
		section.consumed += count
		return nil
	}
	for _, body := range item.rawBodies {
		if err := fwdBodySections(body, bind); err != nil {
			return err
		}
		bodyIndex++
	}
	for _, update := range item.updates {
		if err := fwdUpdateSections(update, bind); err != nil {
			return err
		}
		bodyIndex++
	}
	return nil
}

// fwdProvenancePaths uses registered native framing, including withdrawal-only
// label Compatibility encodings. RFC 8277 Section 2.4: "Upon transmission,
// the Compatibility field SHOULD be set to 0x800000." "Upon reception, the
// value of the Compatibility field MUST be ignored."
func fwdProvenancePaths(data []byte, fam family.Family, ctx *bgpctx.EncodingContext, withdraw bool, visit func([]byte, uint32, bool) error) error {
	split := nlrisplit.Get(fam)
	if withdraw {
		split = nlrisplit.GetWithdraw(fam)
	}
	if split == nil {
		return nlrisplit.ErrUnsupported
	}
	addPath := ctx != nil && ctx.AddPath(fam)
	var visitErr error
	_, err := split(data, addPath, func(raw []byte) {
		if visitErr != nil {
			return
		}
		var received uint32
		if addPath {
			if len(raw) < 4 {
				visitErr = fmt.Errorf("forward ownership: truncated path identifier")
				return
			}
			received, raw = binary.BigEndian.Uint32(raw), raw[4:]
		}
		visitErr = visit(raw, received, addPath)
	})
	if err != nil {
		return err
	}
	return visitErr
}

// fwdBodySections borrows RFC 4271 Section 4.3 fields without allocating a
// parsed UPDATE. "The UPDATE message always includes the fixed-size BGP
// header, and also includes the other fields, as shown below (note, some of
// the shown fields may not be present in every UPDATE message):"
func fwdBodySections(body []byte, visit func([]byte, family.Family, bool, bool) error) error {
	if len(body) < 4 {
		return fmt.Errorf("forward ownership: truncated UPDATE")
	}
	withdrawnEnd := 2 + int(binary.BigEndian.Uint16(body))
	if withdrawnEnd+2 > len(body) {
		return fmt.Errorf("forward ownership: truncated withdrawn section")
	}
	attrsEnd := withdrawnEnd + 2 + int(binary.BigEndian.Uint16(body[withdrawnEnd:]))
	if attrsEnd > len(body) {
		return fmt.Errorf("forward ownership: truncated attribute section")
	}
	update := message.Update{WithdrawnRoutes: body[2:withdrawnEnd], PathAttributes: body[withdrawnEnd+2 : attrsEnd], NLRI: body[attrsEnd:]}
	return fwdUpdateSections(&update, visit)
}

func fwdUpdateSections(update *message.Update, visit func([]byte, family.Family, bool, bool) error) error {
	if len(update.WithdrawnRoutes) != 0 {
		if err := visit(update.WithdrawnRoutes, family.IPv4Unicast, true, false); err != nil {
			return err
		}
	}
	if len(update.NLRI) != 0 {
		if err := visit(update.NLRI, family.IPv4Unicast, false, false); err != nil {
			return err
		}
	}
	attrs := update.PathAttributes
	for len(attrs) != 0 {
		_, code, length, header, err := attribute.ParseHeader(attrs)
		if err != nil {
			return err
		}
		end := header + int(length)
		if end > len(attrs) {
			return fmt.Errorf("forward ownership: truncated multiprotocol attribute")
		}
		if code == attribute.AttrMPReachNLRI || code == attribute.AttrMPUnreachNLRI {
			value := attrs[header:end]
			fam, offset, err := pathsLimitMP(value, code == attribute.AttrMPReachNLRI)
			if err != nil {
				return err
			}
			if len(value) > offset {
				if err := visit(value[offset:], fam, code == attribute.AttrMPUnreachNLRI, true); err != nil {
					return err
				}
			}
		}
		attrs = attrs[end:]
	}
	return nil
}
