// Design: docs/architecture/config/transaction-protocol.md -- SDK journal for rollback
// Overview: sdk.go -- plugin SDK core

package sdk

// Journal records apply/undo pairs during a config transaction.
// Plugins call Record for each side-effecting change during apply.
// On rollback, the journal replays undos in reverse order.
// On commit (config/committed), the journal is discarded.
//
// Safe for single-goroutine use within a plugin's apply handler.
// Not safe for concurrent use.
type Journal struct {
	entries []journalEntry
}

type journalEntry struct {
	undo func() error
}

// NewJournal creates an empty journal.
func NewJournal() *Journal {
	return &Journal{}
}

// Record calls apply immediately. If apply succeeds, the undo function
// is stored for potential rollback. If apply fails, undo is not stored
// and the error is returned.
func (j *Journal) Record(apply, undo func() error) error {
	if err := apply(); err != nil {
		return err
	}
	j.entries = append(j.entries, journalEntry{undo: undo})
	return nil
}

// Rollback calls all stored undo functions in reverse order.
// Every undo is called even if earlier ones fail. Returns all errors.
func (j *Journal) Rollback() []error {
	var errs []error
	for i := len(j.entries) - 1; i >= 0; i-- {
		if err := j.entries[i].undo(); err != nil {
			errs = append(errs, err)
		}
	}
	j.entries = nil
	return errs
}

// Discard clears the journal without calling any undo functions.
// Called when the transaction commits successfully.
func (j *Journal) Discard() {
	j.entries = nil
}

// Len returns the number of recorded entries.
func (j *Journal) Len() int {
	return len(j.entries)
}
