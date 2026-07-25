// VALIDATES: mrt.Writer with pattern "-" writes records to stdout, unrotated,
// and Close does not close os.Stdout (AC-8 for analyze filter/convert/record).
// PREVENTS: a "-" output path being treated as a strftime file pattern.
package mrt

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/cliio"
)

func TestWriterStdout(t *testing.T) {
	var out bytes.Buffer
	restore := cliio.SwapStreams(strings.NewReader(""), &out)
	defer restore()

	rec := mrtRecord()
	w := NewWriter("-")
	if err := w.Write(rec); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Write(rec); err != nil {
		t.Fatalf("Write 2: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	want := append(append([]byte{}, rec...), rec...)
	if !bytes.Equal(out.Bytes(), want) {
		t.Fatalf("stdout = %d bytes, want %d (two records concatenated)", out.Len(), len(want))
	}
}
