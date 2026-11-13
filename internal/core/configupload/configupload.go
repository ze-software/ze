// Design: docs/architecture/appliance/remote-operations.md -- SSH configuration upload framing.

// Package configupload defines the bounded Ze-to-Ze configuration upload frame.
package configupload

import (
	"encoding/binary"
	"errors"
	"io"
)

const (
	Subsystem = "ze-config-upload"
	MaxBytes  = 4 << 20
	Receipt   = "configuration applied\n"
)

// Read accepts exactly one nonempty, length-prefixed configuration followed by
// EOF. It never returns a partial configuration for validation or publication.
func Read(r io.Reader) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, errors.New("configuration upload header is incomplete")
	}
	size := binary.BigEndian.Uint32(header[:])
	if size == 0 {
		return nil, errors.New("configuration upload is empty")
	}
	if size > MaxBytes {
		return nil, errors.New("configuration upload exceeds four MiB")
	}
	content := make([]byte, int(size))
	if _, err := io.ReadFull(r, content); err != nil {
		return nil, errors.New("configuration upload body is incomplete")
	}
	var extra [1]byte
	_, err := io.ReadFull(r, extra[:])
	if !errors.Is(err, io.EOF) {
		return nil, errors.New("configuration upload has trailing bytes or no complete EOF")
	}
	return content, nil
}

// Write sends the complete declared frame. Its caller sends SSH EOF afterward.
func Write(w io.Writer, content string) error {
	if len(content) == 0 {
		return errors.New("configuration upload is empty")
	}
	if len(content) > MaxBytes {
		return errors.New("configuration upload exceeds four MiB")
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(content)))
	n, err := w.Write(header[:])
	if err != nil {
		return err
	}
	if n != len(header) {
		return io.ErrShortWrite
	}
	n, err = io.WriteString(w, content)
	if err != nil {
		return err
	}
	if n != len(content) {
		return io.ErrShortWrite
	}
	return nil
}
