// Design: docs/architecture/diagnostics/production-diagnostics.md -- VPP trace stubs for non-Linux

//go:build !linux

package vpp

import "errors"

func TraceStart(string, int) (string, error) { return "", errTraceNotAvailable }
func TraceShow() (string, error)             { return "", errTraceNotAvailable }
func TraceClear() (string, error)            { return "", errTraceNotAvailable }
func ShowRuntime() (string, error)           { return "", errTraceNotAvailable }

var errTraceNotAvailable = errors.New("vpp trace: not available on this platform")
