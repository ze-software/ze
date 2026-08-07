---
kind: table
level:
stage:
---
| Event type | StructuredEvent fields | RawMessage |
|------------|----------------------|------------|
| UPDATE (received/sent) | PeerAddress, PeerAS, LocalAS, Direction, MessageID, Meta | Set, carries `AttrsWire` (lazy attributes) + `WireUpdate` (zero-copy sections) |
| State (up/down) | PeerAddress, PeerAS, State, Reason | nil |
| OPEN, NOTIFICATION, REFRESH | PeerAddress, PeerAS, Direction, MessageID | Set, carries `RawBytes` for wire decoding |
