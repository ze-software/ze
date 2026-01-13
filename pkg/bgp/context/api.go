package context

// APIContextID identifies API-originated wire data.
// Registered at init with ASN4=true for modern encoding.
//
// Init safety: Registry is package-level var (registry.go), initialized before
// init() runs. Go guarantees package-level vars init before init() functions.
var APIContextID ContextID

func init() {
	APIContextID = Registry.Register(EncodingContextForASN4(true))
}
