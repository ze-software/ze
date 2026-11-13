// Design: docs/architecture/api/process-protocol.md -- interface address replay

package events

import "github.com/ze-software/ze/internal/core/events"

// AddressReplayRequest asks an active interface monitor to publish its current
// usable addresses as addr-added events. A consumer MUST subscribe before it
// emits this signal. The request never starts an inactive backend.
var AddressReplayRequest = events.RegisterSignal(Namespace, EventAddressReplayRequest)
