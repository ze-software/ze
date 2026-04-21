package server

import (
	"reflect"
	"sync/atomic"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
)

func markProcessRunning(t *testing.T, proc *process.Process) {
	t.Helper()

	field := reflect.ValueOf(proc).Elem().FieldByName("running")
	require.True(t, field.IsValid(), "process.running field must exist")
	require.True(t, field.CanAddr(), "process.running field must be addressable")

	running := (*atomic.Bool)(unsafe.Pointer(field.UnsafeAddr()))
	running.Store(true)
}
