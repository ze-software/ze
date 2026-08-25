// Design: (none -- tooling imports for go mod vendor)

//go:build tools

// Package main imports tool dependencies so they are vendored.
// Run tools via: go run <import-path> [args...]
// See the `setup` subprogram of ./le (scripts/le/application/setup.py).
package main

import (
	_ "github.com/a-h/templ/cmd/templ"
	_ "github.com/sivchari/gomu/cmd/gomu"
	_ "golang.org/x/tools/cmd/goimports"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)
