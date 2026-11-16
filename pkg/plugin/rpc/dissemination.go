// Design: docs/architecture/api/process-protocol.md -- declared dissemination roles.
// Related: types.go -- the Stage 1 declaration carrying these pure values.
package rpc

// DisseminationDecl names an opaque peer-output role and the configured grants
// one process needs to perform it. It grants nothing. SourceReceive must be
// complete on at least one peer of the same configured process instance;
// DestinationReceive and DestinationSend must be complete on the recipient.
// Receive tokens use the attach-process direction vocabulary. A process that
// also Claims Role is an explicit owner; an unclaimed role is an ordinary owner.
// These raw declaration values are validated before either registration stores them.
type DisseminationDecl struct {
	Namespace          string   `json:"namespace"`
	Role               string   `json:"role"`
	SourceReceive      []string `json:"source-receive"`
	DestinationReceive []string `json:"destination-receive"`
	DestinationSend    []string `json:"destination-send"`
}
