// Design: (none -- tooling imports for go mod vendor)

//go:build tools

// Package main imports tool dependencies so they are vendored.
// Run tools via: go run <import-path> [args...]
// See Makefile ze-setup target.
package main

import (
	_ "golang.org/x/tools/cmd/goimports"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)
