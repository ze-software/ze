// Design: docs/architecture/config/transaction-protocol.md -- which participants a rollback reaches
// Overview: sdk.go -- plugin SDK core
// Related: sdk_callbacks.go -- OnConfigApply and OnConfigRollback, which consult the gate
// Related: journal.go -- the undo set a rollback handler usually replays

package sdk

import "sync"

// configTxGate decides whether a section `config-rollback` reaches the
// plugin's rollback handler.
//
// The engine broadcasts `config-rollback` to every participant of a failed
// transaction, including a participant whose `config-apply` never ran because
// another plugin failed first. A plugin's rollback handler replays whatever
// undo set it kept from its last apply, and that set outlives the transaction
// that produced it: nothing in the plugin learns that the transaction
// committed. Without this gate a rollback of T2 that reached a plugin which
// last applied in the committed T1 replayed T1's undo, restoring the state from
// two commits back.
//
// The gate holds one fact: an apply this plugin accepted is still open, being
// neither committed nor rolled back. `config-apply` success opens it,
// `config-committed` closes it, and `config-rollback` runs the handler only
// when it is open, closing it. A plugin whose own apply failed leaves it closed:
// a failing apply undoes its own partial work before it returns the error,
// which is what every in-tree apply handler does with its journal.
//
// Safe for concurrent use.
type configTxGate struct {
	mu   sync.Mutex
	open bool
}

// applied records that the plugin accepted a `config-apply`.
func (g *configTxGate) applied() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.open = true
}

// committed records that the open apply, if any, is permanent: no rollback
// can undo it any more.
func (g *configTxGate) committed() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.open = false
}

// rollbackOwed reports whether a `config-rollback` has an apply to undo, and
// closes the gate. A false answer means this plugin accepted no apply since
// its last commit or rollback, so the rollback is a no-op for it.
func (g *configTxGate) rollbackOwed() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	owed := g.open
	g.open = false
	return owed
}
