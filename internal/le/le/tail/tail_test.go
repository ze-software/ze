package tail

import "testing"

// VALIDATES: Buffer keeps exactly the last limit octets across writes and
// always reports every octet written.
// PREVENTS: a command's last line falling out of the kept tail, or a short
// write stopping the copy of its output.
func TestBufferKeepsTheLastOctets(t *testing.T) {
	kept := New(8)
	for _, chunk := range []string{"0123", "456789", "abcdefghijkl", "XY"} {
		if n, err := kept.Write([]byte(chunk)); n != len(chunk) || err != nil {
			t.Fatalf("Write(%q) = %d, %v", chunk, n, err)
		}
	}
	if got := kept.String(); got != "ghijklXY" {
		t.Errorf("tail = %q, want %q", got, "ghijklXY")
	}
	if n, err := New(0).Write([]byte("abc")); n != 3 || err != nil {
		t.Errorf("a zero-limit Write = %d, %v, want 3, nil", n, err)
	}
}
