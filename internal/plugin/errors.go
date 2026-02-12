package plugin

import "errors"

// UpdateText parsing errors.
// Note: ErrInvalidPrefix and ErrInvalidFamily are defined in internal/plugins/bgp/route/.
var (
	ErrInvalidAttrMode    = errors.New("invalid attr mode (expected set, add, or del)")
	ErrMissingAttrMode    = errors.New("missing attr mode")
	ErrUnknownAttribute   = errors.New("unknown attribute")
	ErrAddOnScalar        = errors.New("'add' not valid for scalar attribute (use 'set')")
	ErrDelOnScalar        = errors.New("'del' not valid for scalar attribute (use 'set')")
	ErrASPathNotAddable   = errors.New("as-path does not support add/del (use 'set')")
	ErrMissingAddDel      = errors.New("expected 'add' or 'del' before prefix")
	ErrEmptyNLRISection   = errors.New("nlri section has no prefixes")
	ErrFamilyMismatch     = errors.New("NLRI does not match declared family")
	ErrFamilyNotSupported = errors.New("family not supported in text mode")
)

// Reactor errors.
var (
	ErrNoPeersMatch          = errors.New("no peers match selector")
	ErrNoPeersAcceptedFamily = errors.New("no peers have family negotiated")
)
