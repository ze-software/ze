// Design: docs/architecture/wire/ospf.md -- OSPF Segment Routing registration seam.
// registerSRConsumer wires the SR TLV builders into the process-global RFC 7770 RI
// and RFC 7684 Extended Prefix/Link registries. It is idempotent-tolerant: the
// v4 engine factory (wireV4Engine) runs per RFC 6549 instance, so a duplicate
// registration is expected and ignored -- the builders are process-global and the
// same set serves every instance and the OSPFv3 RI LSA.
// RFC: rfc/short/rfc8665.md (§3 RI capability TLVs; §5/§6 Prefix-SID/Adj-SID sub-TLVs)

package ospf

import (
	"errors"

	"github.com/ze-software/ze/internal/plugins/ospf/sr"
)

// registerSRConsumer registers the SR TLV builders. Wired from wireV4Engine.
func registerSRConsumer(_ *engine) error { return srRegisterWire() }

// srRegisterWire registers the RI capability TLV builders and the Extended
// Prefix/Link SR sub-TLV codecs, tolerating an already-registered error so the
// per-instance engine factory can call it repeatedly.
func srRegisterWire() error {
	if err := srTolerateDup(registerRITLV(sr.V4TypeSRAlgorithm, OpaqueScopeArea, srBuildAlgorithm), ErrRITLVRegistered); err != nil {
		return err
	}
	if err := srTolerateDup(registerRITLV(sr.V4TypeSRGB, OpaqueScopeArea, srBuildSRGB), ErrRITLVRegistered); err != nil {
		return err
	}
	if err := srTolerateDup(registerRITLV(sr.V4TypeSRLB, OpaqueScopeArea, srBuildSRLB), ErrRITLVRegistered); err != nil {
		return err
	}
	if err := srTolerateDup(registerRITLV(sr.V4TypeSRMS, OpaqueScopeArea, srBuildSRMS), ErrRITLVRegistered); err != nil {
		return err
	}
	if err := srTolerateDup(registerPrefixSubTLV(sr.V4TypePrefixSID, extSubTLVCodec{
		Build:   srBuildPrefixSID,
		Receive: srReceivePrefixSID,
		Render:  srRenderPrefixSID,
	}), ErrExtSubTLVRegistered); err != nil {
		return err
	}
	if err := srTolerateDup(registerLinkSubTLV(sr.V4TypeAdjSID, extSubTLVCodec{
		Build:   srBuildAdjSID,
		Receive: srReceiveAdjSID,
		Render:  srRenderAdjSID,
	}), ErrExtSubTLVRegistered); err != nil {
		return err
	}
	if err := srTolerateDup(registerLinkSubTLV(sr.V4TypeLANAdjSID, extSubTLVCodec{
		Build:   srBuildLANAdjSID,
		Receive: srReceiveLANAdjSID,
		Render:  srRenderLANAdjSID,
	}), ErrExtSubTLVRegistered); err != nil {
		return err
	}
	return nil
}

// srTolerateDup drops an "already registered" error (the expected outcome of a
// repeated registration by the per-instance engine factory).
func srTolerateDup(err, dup error) error {
	if err != nil && !errors.Is(err, dup) {
		return err
	}
	return nil
}
