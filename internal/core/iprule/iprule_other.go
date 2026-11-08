// Design: docs/architecture/policyroute/policy-routing.md -- netlink ip rule and route management
// Related: iprule_linux.go -- the Linux implementation

//go:build !linux

package iprule

import "errors"

// errNoFibRules answers every kernel operation off Linux: fib rules and routing
// tables are Linux kernel state.
var errNoFibRules = errors.New("ip rules and routing tables are Linux kernel state; this operating system has none")

// Add refuses: see errNoFibRules.
func (s Source) Add() error { return errNoFibRules }

// Delete refuses: see errNoFibRules.
func (s Source) Delete() error { return errNoFibRules }

// Replace refuses: see errNoFibRules.
func (r Route) Replace() error { return errNoFibRules }

// Delete refuses: see errNoFibRules.
func (r Route) Delete() error { return errNoFibRules }
