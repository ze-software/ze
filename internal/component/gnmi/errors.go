// Design: docs/architecture/api/architecture.md -- gNMI error definitions
// Related: server.go -- gNMI server core

package gnmi

import "errors"

var (
	errPathTooDeep      = errors.New("gnmi: path exceeds maximum depth")
	errEmptyPathElement = errors.New("gnmi: empty path element name")
	errMultipleKeys     = errors.New("gnmi: multiple list keys not supported")
	errEmptyPath        = errors.New("gnmi: empty path")
)
