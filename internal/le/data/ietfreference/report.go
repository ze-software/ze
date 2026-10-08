// Design: reference/README.md -- what the data ietf-reference command answers
// Related: refresh.go -- the refresh that produces it
//
// report.go holds what the one action ANSWERS, apart from what produced it.

package dataietfreference

import (
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Report is what `le data ietf-reference write` answers: the index it wrote,
// how each row compares with the previous index, and every file it fetched.
//
// A row is keyed "<wg>/<stem>". NoLongerListed names files under
// reference/ietf/<wg>/ that no selected row points at: they are kept, never
// deleted, and the owner decides what happens to each.
type Report struct {
	Index          string   `json:"index"`
	Rows           int      `json:"rows"`
	Added          []string `json:"added"`
	Updated        []string `json:"updated"`
	Unchanged      int      `json:"unchanged"`
	NoLongerListed []string `json:"no-longer-listed"`
	Downloaded     []string `json:"downloaded"`
}

// Text renders the counts, then each changed row and each unlisted file by
// name. It ends in a newline.
func (r Report) Text() string {
	var tb textbuf.Buffer
	tb.Str("wrote ").Str(r.Index).Str(": ").Int(int64(r.Rows)).Str(" rows, ").
		Int(int64(len(r.Added))).Str(" added, ").
		Int(int64(len(r.Updated))).Str(" updated, ").
		Int(int64(r.Unchanged)).Str(" unchanged, ").
		Int(int64(len(r.NoLongerListed))).Str(" no longer listed; ").
		Int(int64(len(r.Downloaded))).Str(" texts downloaded\n")
	for _, key := range r.Added {
		tb.Str("added: ").Str(key).Byte('\n')
	}
	for _, key := range r.Updated {
		tb.Str("updated: ").Str(key).Byte('\n')
	}
	for _, rel := range r.NoLongerListed {
		tb.Str("no longer listed, kept: ").Str(rel).Byte('\n')
	}
	return tb.String()
}
