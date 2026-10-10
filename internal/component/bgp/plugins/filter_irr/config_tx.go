// Design: docs/architecture/bgp/filter-irr.md -- IRR filter config at boot and on reload
// Related: filter_irr.go -- handleConfigure, the one place a config becomes the filter's state

package filter_irr

import sdk "github.com/ze-software/ze/pkg/plugin/sdk"

// irrConfigTx is the filter's side of the config transaction: the bgp subtree
// the filter runs on, the verified candidate, and what the last apply
// replaced, so a rollback can put it back.
//
// Not safe for concurrent use. The SDK serializes the configure, verify, apply
// and rollback callbacks that call it.
type irrConfigTx struct {
	plug *irrPlugin

	current  map[string]any
	pending  map[string]any
	previous map[string]any
	// verified is set by a verify that accepted the candidate and cleared by
	// the apply or rollback that consumes it. pending alone cannot say it: a
	// verify that carried no bgp section leaves pending nil and is still a
	// verified transaction that changes nothing here.
	verified bool
}

// configure is the boot delivery (Stage 2): the subtree becomes the filter's
// config at once.
func (tx *irrConfigTx) configure(sections []sdk.ConfigSection) error {
	bgpCfg, err := bgpSubtreeFromSections(sections)
	if err != nil {
		return err
	}
	if bgpCfg == nil {
		return nil
	}
	tx.current = bgpCfg
	tx.plug.handleConfigure(bgpCfg)
	return nil
}

// verify checks a reload candidate and holds it for apply. A runtime verify
// carries the changed roots only, so no bgp section means the transaction
// leaves this filter's config as it is.
func (tx *irrConfigTx) verify(sections []sdk.ConfigSection) error {
	tx.pending, tx.previous, tx.verified = nil, nil, false
	bgpCfg, err := bgpSubtreeFromSections(sections)
	if err != nil {
		return err
	}
	tx.pending, tx.verified = bgpCfg, true
	return nil
}

// apply makes the verified candidate the filter's config. An apply with no
// verified candidate is refused: answering OK would report a commit the filter
// never took, the fail-open this transaction exists to remove.
func (tx *irrConfigTx) apply() error {
	if !tx.verified {
		return errFilterIrrApplyUnverified
	}
	next := tx.pending
	tx.pending, tx.verified = nil, false
	if next == nil {
		return nil
	}
	tx.plug.handleConfigure(next)
	tx.previous, tx.current = tx.current, next
	return nil
}

// rollback puts back the config the last apply replaced, when this
// transaction applied one.
func (tx *irrConfigTx) rollback() error {
	tx.pending, tx.verified = nil, false
	if tx.previous == nil {
		return nil
	}
	tx.plug.handleConfigure(tx.previous)
	tx.current, tx.previous = tx.previous, nil
	return nil
}
