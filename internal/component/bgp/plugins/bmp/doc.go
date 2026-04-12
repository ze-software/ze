// Design: docs/architecture/core-design.md -- BGP plugin model
//
// Implements: RFC 7854 -- BGP Monitoring Protocol (BMP) v3.
// Reference: https://www.rfc-editor.org/rfc/rfc7854.html
//
// Package bmp implements a BMP receiver and sender for ze.
//
// Receiver: accepts TCP connections from routers streaming BMP v3,
// parses the wrapped BGP messages via the existing decoder, and
// materializes monitored peer state into a queryable view.
//
// Sender: connects to external BMP collectors and streams ze's own
// peer state changes, route updates, and statistics as BMP messages.
package bmp
