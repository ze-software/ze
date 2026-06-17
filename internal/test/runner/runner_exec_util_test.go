package runner

import "testing"

func TestSyncWriterCapsOutput(t *testing.T) {
	sw := &syncWriter{pattern: "needle"}

	// Fill to exactly the cap with a first write.
	half := make([]byte, maxOutputBytes/2)
	for i := range half {
		half[i] = 'a'
	}
	n1, err := sw.Write(half)
	if err != nil {
		t.Fatalf("first Write error: %v", err)
	}
	if n1 != len(half) {
		t.Fatalf("first Write returned %d, want %d", n1, len(half))
	}

	// Second write overflows the cap; buffer should stop at maxOutputBytes.
	overflow := make([]byte, maxOutputBytes)
	for i := range overflow {
		overflow[i] = 'b'
	}
	n2, err := sw.Write(overflow)
	if err != nil {
		t.Fatalf("second Write error: %v", err)
	}
	// Write returns len(p) after capping, so n2 == remaining capacity.
	wantN2 := maxOutputBytes - maxOutputBytes/2
	if n2 != wantN2 {
		t.Fatalf("second Write returned %d, want %d (remaining cap)", n2, wantN2)
	}

	if len(sw.String()) != maxOutputBytes {
		t.Fatalf("buffered output should be capped at %d, got %d", maxOutputBytes, len(sw.String()))
	}
}
