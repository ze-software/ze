// Design: docs/architecture/api/commands.md -- bounded caller-supplied bytes.

package command

import (
	"encoding/base64"
	"errors"
)

// ContentEncodedMax and ContentOctetsMax are the common command argument
// budget: one SSH exec packet must also hold its command and other arguments.
const (
	ContentEncodedMax = 261120
	ContentOctetsMax  = 195840
)

// DecodeContent decodes the padded base64 content argument every command
// surface carries. Its errors never repeat the supplied bytes. Empty content
// stays empty so each owner can explain whether its operation permits it.
func DecodeContent(encoded string) ([]byte, error) {
	if len(encoded) > ContentEncodedMax {
		return nil, errors.New("content exceeds 261,120 base64 characters (195,840 bytes); use a smaller input")
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("content is not padded base64; encode the input bytes as base64")
	}
	// Padded base64 carries at most three bytes per four characters, so
	// the encoded bound also proves the decoded bound before allocation.
	return data, nil
}
