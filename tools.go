// Design: (none -- tooling imports for go mod vendor)

//go:build tools

// Package main imports executable tool dependencies so `go mod vendor` retains
// their source. Every tool this repository runs is built from `vendor/`, so a
// build needs no network. Build and install them through `./le setup`.
package main

import (
	_ "github.com/a-h/templ/cmd/templ"
	_ "github.com/golangci/golangci-lint/v2/cmd/golangci-lint"
	_ "github.com/sivchari/gomu/cmd/gomu"
	_ "golang.org/x/tools/cmd/goimports"
	_ "golang.org/x/tools/gopls"
	_ "golang.org/x/vuln/cmd/govulncheck"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
	_ "honnef.co/go/tools/cmd/staticcheck"
)
