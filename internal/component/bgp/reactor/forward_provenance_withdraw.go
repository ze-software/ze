// Design: docs/architecture/bgp/structural-forwarding.md -- final-writer path ownership
// Related: forward_provenance.go -- original section identities and the bounded scratch pool
// Related: session_rfc7607.go -- final admission can replace reach with withdrawal

package reactor

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// projectFwdWithdrawals copies the current body's original path identities
// before its announcements become withdrawals. The body already has the
// recipient's framing; its ADD-PATH bytes MUST NOT become source identities.
// writePath resolves those from the original manifest instead. The caller MUST
// hold writeMu, restore the item's original manifest, and release the copy.
func projectFwdWithdrawals(item *fwdItem, body []byte, ctx *bgpctx.EncodingContext) (*fwdProvenance, error) {
	p, ok := fwdProvenancePool.Get().(*fwdProvenance)
	if !ok {
		return nil, fmt.Errorf("forward ownership: invalid withdrawal provenance pool entry")
	}
	err := fwdBodySections(body, func(data []byte, fam family.Family, withdraw, multiprotocol bool) error {
		var section *fwdProvenanceSection
		for i := range p.count {
			if p.sections[i].family == fam && p.sections[i].multiprotocol == multiprotocol {
				section = &p.sections[i]
				break
			}
		}
		if section == nil {
			if p.count == len(p.sections) {
				return fmt.Errorf("forward ownership: too many withdrawal source sections")
			}
			section = &p.sections[p.count]
			section.family, section.withdraw, section.multiprotocol = fam, true, multiprotocol
			p.count++
		}
		// fwdBodySections and SynthesizeWithdrawFamilies both visit legacy
		// withdrawals before legacy reach, then MP attributes in wire order.
		occurrence := 0
		return fwdProvenancePaths(data, fam, ctx, withdraw, func(raw []byte, _ uint32, _ bool) error {
			path, err := item.writePath(fam, raw, withdraw, multiprotocol, occurrence)
			if err != nil {
				return err
			}
			section.paths = append(section.paths, path)
			occurrence++
			return nil
		})
	})
	if err != nil {
		p.release()
		return nil, err
	}
	return p, nil
}

// bindWithdrawalChunk binds one synthesized, compliant output chunk to the
// copied original identities. Successive split chunks consume successive
// paths, while both ownership passes of one chunk read the same span.
func (p *fwdProvenance) bindWithdrawalChunk(update *message.Update, ctx *bgpctx.EncodingContext) error {
	clear(p.spans)
	return fwdUpdateSections(update, func(data []byte, fam family.Family, withdraw, multiprotocol bool) error {
		if !withdraw {
			return fmt.Errorf("forward ownership: withdrawal replacement contains an announcement")
		}
		var section *fwdProvenanceSection
		for i := range p.count {
			if p.sections[i].family == fam && p.sections[i].multiprotocol == multiprotocol {
				section = &p.sections[i]
				break
			}
		}
		if section == nil {
			return fmt.Errorf("forward ownership: withdrawal replacement has no source section")
		}
		count := 0
		if err := fwdProvenancePaths(data, fam, ctx, true, func(_ []byte, _ uint32, _ bool) error {
			count++
			return nil
		}); err != nil {
			return err
		}
		if count > len(section.paths)-section.consumed {
			return fmt.Errorf("forward ownership: withdrawal replacement exceeds source paths")
		}
		p.spans[fwdOutputSection{family: fam, withdraw: true, multiprotocol: multiprotocol}] =
			fwdProvenanceSpan{section: section, start: section.consumed, count: count}
		section.consumed += count
		return nil
	})
}
