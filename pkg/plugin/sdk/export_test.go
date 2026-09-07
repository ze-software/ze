package sdk

// DialAndAuth gives the external test package this package's own hub dial.
//
// The chain tests in dial_chain_test.go import internal/component/pki, and that
// package's closure reaches pkg/plugin/sdk again through
// internal/component/pki/show.go, internal/component/plugin/server and
// internal/component/iface. An in-package test file may not close a cycle, so
// those tests live in package sdk_test, which may import a package that imports
// the package under test. This name is how they reach the unexported dial.
var DialAndAuth = dialAndAuth
