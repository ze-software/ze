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

// TestLineFramingRead verifies reading newline-delimited messages.
//
// VALIDATES: Reader correctly splits on newline boundaries.
// PREVENTS: Messages merged or truncated at newline delimiters.
func TestLineFramingRead(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantMsgs []string
	}{
		{
			name:     "single_message",
			input:    "#1 ze-bgp:peer-list\n",
			wantMsgs: []string{"#1 ze-bgp:peer-list"},
		},
		{
			name:  "two_messages",
			input: "#1 ok\n#2 ok {\"peers\":[]}\n",
			wantMsgs: []string{
				"#1 ok",
				`#2 ok {"peers":[]}`,
			},
		},
		{
			name:  "three_messages",
			input: "#1 ok\n#2 ok\n#3 ok\n",
			wantMsgs: []string{
				"#1 ok",
				"#2 ok",
				"#3 ok",
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
				require.NoError(t, err)
				got = append(got, string(msg))
			}
			assert.Equal(t, tt.wantMsgs, got)
		})
	}
}

// TestLineFramingWrite verifies writing newline-terminated messages.
//
// VALIDATES: Writer appends newline after each message.
// PREVENTS: Missing newline causing message merging.
func TestLineFramingWrite(t *testing.T) {
	tests := []struct {
		name string
		msgs [][]byte
		want string
	}{
		{
			name: "single_message",
			msgs: [][]byte{[]byte("#1 ok")},
			want: "#1 ok\n",
		},
		{
			name: "two_messages",
			msgs: [][]byte{
				[]byte("#1 ok"),
				[]byte(`#2 ok {"result":true}`),
			},
			want: "#1 ok\n#2 ok {\"result\":true}\n",
		},
		{
			name: "empty_message",
			msgs: [][]byte{[]byte("")},
			want: "\n",
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

// TestLineFramingMultiple verifies multiple messages buffered together.
//
// VALIDATES: Scanner handles multiple newline-delimited messages in a single buffer.
// PREVENTS: Only first message being read from a batch.
func TestLineFramingMultiple(t *testing.T) {
	batch := "#1 ok\n#2 ok\n#3 ok\n"
	reader := rpc.NewFrameReader(strings.NewReader(batch))

	msg1, err := reader.Read()
	require.NoError(t, err)
	assert.Equal(t, "#1 ok", string(msg1))

	msg2, err := reader.Read()
	require.NoError(t, err)
	assert.Equal(t, "#2 ok", string(msg2))

	msg3, err := reader.Read()
	require.NoError(t, err)
	assert.Equal(t, "#3 ok", string(msg3))

	_, err = reader.Read()
	assert.ErrorIs(t, err, io.EOF)
}

// TestLineFramingPartial verifies partial message buffering.
//
// VALIDATES: Reader handles messages split across multiple reads.
// PREVENTS: Partial messages returned before newline arrives.
func TestLineFramingPartial(t *testing.T) {
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

	// Send message in two parts.
	_, err := pw.Write([]byte("#1 ze-bgp"))
	require.NoError(t, err)
	_, err = pw.Write([]byte(":peer-list\n"))
	require.NoError(t, err)

	msg := <-done
	assert.Equal(t, "#1 ze-bgp:peer-list", string(msg))

	require.NoError(t, pw.Close())
}

// TestLineFramingRoundTrip verifies write then read produces same messages.
//
// VALIDATES: FrameWriter output is correctly parsed by FrameReader.
// PREVENTS: Encoding/decoding mismatch in framing layer.
func TestLineFramingRoundTrip(t *testing.T) {
	messages := []string{
		`#1 ze-bgp:peer-list {"selector":"*"}`,
		`#1 ok {"peers":[]}`,
		`#2 ze-bgp:subscribe {"events":["update"]}`,
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

// TestLineFramingMaxSize verifies message size limit enforcement.
//
// VALIDATES: Messages exceeding MaxMessageSize are rejected.
// PREVENTS: Memory exhaustion from oversized messages.
// BOUNDARY: 16 MB (16777216) is last valid, 16777217 is first invalid.
func TestLineFramingMaxSize(t *testing.T) {
	t.Run("at_limit", func(t *testing.T) {
		msg := bytes.Repeat([]byte("x"), rpc.MaxMessageSize)
		var buf bytes.Buffer
		buf.Write(msg)
		buf.WriteByte('\n')

		reader := rpc.NewFrameReader(&buf)
		got, err := reader.Read()
		require.NoError(t, err)
		assert.Len(t, got, rpc.MaxMessageSize)
	})

	t.Run("over_limit", func(t *testing.T) {
		msg := bytes.Repeat([]byte("x"), rpc.MaxMessageSize+1)
		var buf bytes.Buffer
		buf.Write(msg)
		buf.WriteByte('\n')

		reader := rpc.NewFrameReader(&buf)
		_, err := reader.Read()
		require.Error(t, err, "should reject oversized message")
		assert.Contains(t, err.Error(), "exceeds maximum", "error should mention size limit")
	})
}
