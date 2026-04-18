// Design: docs/architecture/core-design.md -- tc traffic control backend plugin

// Package trafficnetlink implements the traffic control backend using
// vishvananda/netlink. It translates ze's InterfaceQoS types to tc
// qdiscs, classes, and filters on Linux network interfaces.
package trafficnetlink
