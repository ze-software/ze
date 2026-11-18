// Design: docs/architecture/api/process-protocol.md -- exact accepting connection.
// Related: origin_intake.go -- authenticated local producer capture.
package plugin

import (
	"context"
	"errors"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type originAdmissionContextKey struct{}

// WithOriginAdmissionID is trusted RPC ingress plumbing, not authority.
// The reactor MUST resolve this ID on the captured accepting Process connection.
func WithOriginAdmissionID(ctx context.Context, id uint64) context.Context {
	return context.WithValue(ctx, originAdmissionContextKey{}, id)
}

func OriginAdmissionID(ctx context.Context) uint64 {
	id, _ := ctx.Value(originAdmissionContextKey{}).(uint64)
	return id
}

// WithOriginAdmissionLookup MUST bind before this owner is published. It copies
// the existing immutable reader record; it never looks up a replacement by name.
func (owner SelectedOwner) WithOriginAdmissionLookup(read rpc.OriginAdmissionLookupHandler) (SelectedOwner, error) {
	if err := owner.Current(); err != nil {
		return SelectedOwner{}, err
	}
	if owner.currentSelection == nil {
		return SelectedOwner{}, errors.New("accepted-origin lookup requires the captured Process connection")
	}
	if read == nil {
		return SelectedOwner{}, errors.New("accepted-origin lookup has no handler")
	}
	if owner.currentSelection.admissionLookup != nil {
		return SelectedOwner{}, errors.New("accepted-origin lookup is already bound")
	}
	connection := *owner.currentSelection
	connection.admissionLookup = read
	owner.currentSelection = &connection
	return owner, nil
}

// OriginAdmissionLookup reads only the original accepting connection. It MUST
// honor the supplied work bound; stop and a same-name successor cannot authorize
// a result from this captured original. A reply is data, not peer permission.
func (owner SelectedOwner) OriginAdmissionLookup(ctx context.Context, input rpc.OriginAdmissionLookupInput) (*rpc.OriginAdmissionLookupOutput, error) {
	if err := owner.Current(); err != nil {
		return nil, err
	}
	if owner.currentSelection == nil {
		return nil, errors.New("accepted-origin lookup has no original Process connection")
	}
	if owner.currentSelection.admissionLookup == nil {
		return nil, errors.New("original accepting Process did not declare origin-admission-lookup")
	}
	output, err := owner.currentSelection.admissionLookup(ctx, input)
	if err != nil {
		return nil, err
	}
	if err := owner.Current(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := output.Validate(input); err != nil {
		return nil, err
	}
	return output, nil
}
