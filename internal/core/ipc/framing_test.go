package ipc

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rpc "codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// TestNULFramingRead verifies reading NUL-terminated messages.
//
// VALIDATES: Reader correctly splits on NUL byte boundaries.
// PREVENTS: Messages merged or truncated at NUL delimiters.
func TestNULFramingRead(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantMsgs []string
		wantErr  bool
	}{
		{
			name:     "single_message",
			input:    `{"method":"ze-bgp:peer-list"}` + "\x00",
			wantMsgs: []string{`{"method":"ze-bgp:peer-list"}`},
		},
		{
			name:     "trailing_empty_segment",
			input:    `{"method":"ping"}` + "\x00" + "\x00",
			wantMsgs: []string{`{"method":"ping"}`, ""},
		},
		{
			name:  "two_messages",
			input: `{"method":"a"}` + "\x00" + `{"method":"b"}` + "\x00",
			wantMsgs: []string{
				`{"method":"a"}`,
				`{"method":"b"}`,
			},
		},
		{
			name:  "three_messages",
			input: `{"id":1}` + "\x00" + `{"id":2}` + "\x00" + `{"id":3}` + "\x00",
			wantMsgs: []string{
				`{"id":1}`,
				`{"id":2}`,
				`{"id":3}`,
			},
		},
		{
			name:     "empty_input",
			input:    "",
			wantMsgs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := rpc.NewFrameReader(strings.NewReader(tt.input))
			var got []string
			for {
				msg, err := reader.Read()
				if errors.Is(err, io.EOF) {
					break
				}
				if tt.wantErr {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)
				got = append(got, string(msg))
			}
			assert.Equal(t, tt.wantMsgs, got)
		})
	}
}

// TestNULFramingWrite verifies writing NUL-terminated messages.
//
// VALIDATES: Writer appends NUL byte after each message.
// PREVENTS: Missing NUL terminator causing message merging.
func TestNULFramingWrite(t *testing.T) {
	tests := []struct {
		name string
		msgs [][]byte
		want string
	}{
		{
			name: "single_message",
			msgs: [][]byte{[]byte(`{"method":"ping"}`)},
			want: `{"method":"ping"}` + "\x00",
		},
		{
			name: "two_messages",
			msgs: [][]byte{
				[]byte(`{"method":"a"}`),
				[]byte(`{"method":"b"}`),
			},
			want: `{"method":"a"}` + "\x00" + `{"method":"b"}` + "\x00",
		},
		{
			name: "empty_message",
			msgs: [][]byte{[]byte("")},
			want: "\x00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := rpc.NewFrameWriter(&buf)
			for _, msg := range tt.msgs {
				err := writer.Write(msg)
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, buf.String())
		})
	}
}

// TestNULFramingMultiple verifies multiple messages buffered together.
//
// VALIDATES: Scanner handles multiple NUL-delimited messages in a single buffer.
// PREVENTS: Only first message being read from a batch.
func TestNULFramingMultiple(t *testing.T) {
	// Simulate a batch of messages arriving together
	batch := `{"id":1}` + "\x00" + `{"id":2}` + "\x00" + `{"id":3}` + "\x00"
	reader := rpc.NewFrameReader(strings.NewReader(batch))

	msg1, err := reader.Read()
	require.NoError(t, err)
	assert.Equal(t, `{"id":1}`, string(msg1))

	msg2, err := reader.Read()
	require.NoError(t, err)
	assert.Equal(t, `{"id":2}`, string(msg2))

	msg3, err := reader.Read()
	require.NoError(t, err)
	assert.Equal(t, `{"id":3}`, string(msg3))

	_, err = reader.Read()
	assert.ErrorIs(t, err, io.EOF)
}

// TestNULFramingPartial verifies partial message buffering.
//
// VALIDATES: Reader handles messages split across multiple reads.
// PREVENTS: Partial messages returned before NUL terminator arrives.
func TestNULFramingPartial(t *testing.T) {
	// Use a pipe to simulate slow arrival
	pr, pw := io.Pipe()

	reader := rpc.NewFrameReader(pr)
	done := make(chan []byte, 1)

	go func() {
		msg, err := reader.Read()
		if err != nil {
			done <- nil
			return
		}
		done <- msg
	}()

	// Send message in two parts
	_, err := pw.Write([]byte(`{"meth`))
	require.NoError(t, err)
	_, err = pw.Write([]byte(`od":"ping"}` + "\x00"))
	require.NoError(t, err)

	msg := <-done
	assert.Equal(t, `{"method":"ping"}`, string(msg))

	require.NoError(t, pw.Close())
}

// TestNULFramingRoundTrip verifies write then read produces same messages.
//
// VALIDATES: FrameWriter output is correctly parsed by FrameReader.
// PREVENTS: Encoding/decoding mismatch in framing layer.
func TestNULFramingRoundTrip(t *testing.T) {
	messages := []string{
		`{"method":"ze-bgp:peer-list","id":1}`,
		`{"result":{"peers":[]},"id":1}`,
		`{"method":"ze-bgp:subscribe","more":true,"id":2}`,
	}

	var buf bytes.Buffer
	writer := rpc.NewFrameWriter(&buf)
	for _, msg := range messages {
		err := writer.Write([]byte(msg))
		require.NoError(t, err)
	}

	reader := rpc.NewFrameReader(&buf)
	for _, want := range messages {
		got, err := reader.Read()
		require.NoError(t, err)
		assert.Equal(t, want, string(got))
	}

	_, err := reader.Read()
	assert.ErrorIs(t, err, io.EOF)
}

// TestNULFramingMaxSize verifies message size limit enforcement.
//
// VALIDATES: Messages exceeding MaxMessageSize are rejected.
// PREVENTS: Memory exhaustion from oversized messages.
// BOUNDARY: 16 MB (16777216) is last valid, 16777217 is first invalid.
func TestNULFramingMaxSize(t *testing.T) {
	// Message at exactly MaxMessageSize should succeed
	t.Run("at_limit", func(t *testing.T) {
		msg := bytes.Repeat([]byte("x"), rpc.MaxMessageSize)
		var buf bytes.Buffer
		buf.Write(msg)
		buf.WriteByte(0)

		reader := rpc.NewFrameReader(&buf)
		got, err := reader.Read()
		require.NoError(t, err)
		assert.Len(t, got, rpc.MaxMessageSize)
	})

	// Message exceeding MaxMessageSize should fail
	t.Run("over_limit", func(t *testing.T) {
		msg := bytes.Repeat([]byte("x"), rpc.MaxMessageSize+1)
		var buf bytes.Buffer
		buf.Write(msg)
		buf.WriteByte(0)

		reader := rpc.NewFrameReader(&buf)
		_, err := reader.Read()
		require.Error(t, err, "should reject oversized message")
		assert.Contains(t, err.Error(), "exceeds maximum", "error should mention size limit")
	})
}
