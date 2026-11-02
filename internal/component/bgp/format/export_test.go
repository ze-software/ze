// Design: docs/architecture/api/json-format.md -- format test fixtures shared with format_test

package format

// EncodingContextForTest and PeerForTest hand the package's own fixtures to the
// external test package format_test, which exists for tests that link a plugin
// importing format through policyview (an import cycle from inside package format).
var (
	EncodingContextForTest = testEncodingContext
	PeerForTest            = testPeer
)
