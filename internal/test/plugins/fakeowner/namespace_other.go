//go:build !linux

// Design: docs/architecture/api/process-protocol.md -- owned interface devices
// Related: namespace_linux.go -- the Linux holder

package fakeowner

import (
	"context"
	"errors"
)

// holdNamespaceArg is the CLI word that runs the namespace holder.
const holdNamespaceArg = "hold-namespace"

// namespaceHolder has no form off Linux: network namespaces are a Linux
// primitive. startNamespaceHolder always refuses, so no value is ever made.
type namespaceHolder struct {
	pid   int
	inode uint64
}

func startNamespaceHolder(context.Context) (*namespaceHolder, error) {
	return nil, errors.New("network namespaces need Linux")
}

func (*namespaceHolder) close() {}

func holdNamespace() int { return 1 }
