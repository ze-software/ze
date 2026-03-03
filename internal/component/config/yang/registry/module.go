// Design: docs/architecture/config/yang-config-design.md — YANG module registry
//
// Package registry provides init()-based YANG module registration.
// Schema packages register their YANG content here via init().
// The yang Loader reads registered modules via Modules().
//
// This is a leaf package with zero dependencies on yang or schema packages,
// which breaks the import cycle: schema → registry ← yang.
package registry

// Module holds a YANG module registered via init().
type Module struct {
	Name    string
	Content string
}

var modules []Module

// RegisterModule registers a YANG module for loading.
// Called from init() in packages that own YANG files.
// Order of registration does not matter — goyang resolves imports during Resolve().
func RegisterModule(name, content string) {
	modules = append(modules, Module{Name: name, Content: content})
}

// Modules returns all registered YANG modules.
func Modules() []Module {
	return modules
}
