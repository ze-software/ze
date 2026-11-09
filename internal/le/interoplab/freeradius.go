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
// regression. 3.2.10 is upstream's latest stable 3.2 release (owner decision,
// ai/META-DECISIONS.md).
//
// The digest MUST be the tag's manifest list (the registry's
// docker-content-digest for the tag, media type manifest.list.v2), never one
// platform's manifest: a platform digest makes every daemon pull that one
// architecture, and the arm64 docker-lab guest then fails the container start
// with "exec format error" before FreeRADIUS runs. The earlier pin
// (sha256:cc7fd136...) was the amd64 manifest. This list, read from
// registry-1.docker.io on 2026-11-09, names linux/amd64, linux/arm64 and
// linux/arm/v7.
const FreeRADIUSImage = "docker.io/freeradius/freeradius-server:3.2.10@sha256:337856ea35b77472f15dad159064ce46219a54b499bef5f6a7cba38aefd39fe9"
