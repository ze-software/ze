// Package wire provides zero-allocation buffer writing for BGP messages.
package wire

import "errors"

// ErrBufferTooSmall indicates the buffer cannot fit the data to be written.
var ErrBufferTooSmall = errors.New("wire: buffer too small")
