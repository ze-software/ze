// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- Sampling stub for non-Linux

//go:build !linux

package sampling

import "errors"

var errSamplingNotSupported = errors.New("sampling: not supported on this platform")

func SetupSampling(_ string, _, _, _ uint32) error { return errSamplingNotSupported }
func RemoveSampling(_ string) error                { return errSamplingNotSupported }
