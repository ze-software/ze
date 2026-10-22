// Design: docs/architecture/testing/interop.md -- the FreeRADIUS admin-login suite
//
// Two labs run FreeRADIUS as the peer: the admin-login suite
// (internal/le/interoplab/radius) and the subscriber suite
// (internal/le/interoplab/radiussubscriber). The release they run is one fact,
// declared here once.

package interoplab

// FreeRADIUSImage is the FreeRADIUS release both RADIUS labs pull, pinned to
// an exact tag and to that tag's digest. A moving tag changes what the
// scenarios mean with no ze change, and the failure then looks like a ze
// regression. 3.2.10 is upstream's latest stable 3.2 release (owner decision
// M14, ai/META-DECISIONS.md); the digest was read with `docker inspect` on
// 2026-10-22.
const FreeRADIUSImage = "docker.io/freeradius/freeradius-server:3.2.10@sha256:cc7fd136e7b03e7b332d94297530318e824a4ecfedbce54562cced723e71e812"
