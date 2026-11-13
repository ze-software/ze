// Design: docs/architecture/config/syntax.md -- quoted path words stay on one displayed line.

package configtext

import "testing"

// TestQuoteTokenEscapesCarriageReturn keeps a permitted key's CR byte out of
// its printed spelling without changing the value that spelling represents.
func TestQuoteTokenEscapesCarriageReturn(t *testing.T) {
	if got, want := QuoteToken("return\rkey"), `"return\rkey"`; got != want {
		t.Errorf("quoted token = %q, want %q", got, want)
	}
}
