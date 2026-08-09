// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- psample stub for non-Linux

//go:build !linux

package sampling

import "errors"

var errPsampleNotSupported = errors.New("psample: not supported on this platform")

// PsampleReader is a stub for non-Linux platforms.
type PsampleReader struct{}

func NewPsampleReader() (*PsampleReader, error)       { return nil, errPsampleNotSupported }
func (r *PsampleReader) Read() (SampledPacket, error) { return SampledPacket{}, errPsampleNotSupported }
func (r *PsampleReader) Close() error                 { return nil }
