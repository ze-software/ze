// Package radius implements a RADIUS client for RFC 2865 (authentication)
// and RFC 2866 (accounting). Wire encoding follows ze's buffer-first
// discipline; the client provides UDP transport with retransmit,
// exponential backoff, and server failover.
package radius
