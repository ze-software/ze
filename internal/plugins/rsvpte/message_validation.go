// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RSVP message grammar.
// RFC: rfc/short/rfc2205.md, rfc/short/rfc3209.md
package rsvpte

import "fmt"

// flowDescriptor is one FF reservation or one shared SE reservation. Raw objects
// borrow the packet storage; state holders MUST copy them before retaining them.
// RFC 3209 Section 3.2 wire order (each item starts with a four-byte header):
// FF: FLOWSPEC FILTER_SPEC LABEL [RRO] [FLOWSPEC] FILTER_SPEC LABEL [RRO] ...
// SE: FLOWSPEC FILTER_SPEC LABEL [RRO] FILTER_SPEC LABEL [RRO] ...
type flowDescriptor struct {
	FlowSpec    FlowSpec
	FlowSpecRaw []byte
	Filters     []reservationFilter
}

type reservationFilter struct {
	Filter   senderTemplateIPv4
	Label    labelObject
	HasLabel bool
	RRO      []rroEntry
	HasRRO   bool
}

func reservationMessage(kind uint8) bool {
	switch kind {
	case MsgTypeResv, MsgTypeResvErr, MsgTypeResvTear, MsgTypeResvConf:
		return true
	}
	return false
}

func objectAllowed(kind, class uint8) bool {
	switch class {
	case ClassNull, ClassIntegrity, ClassSession:
		return true
	case ClassRSVPHop:
		return kind == MsgTypePath || kind == MsgTypePathTear || kind == MsgTypeResv || kind == MsgTypeResvTear || kind == MsgTypeResvErr
	case ClassTimeValues:
		return kind == MsgTypePath || kind == MsgTypeResv
	case ClassSenderTemplate, ClassSenderTSpec, ClassAdspec:
		return kind == MsgTypePath || kind == MsgTypePathTear || kind == MsgTypePathErr
	case ClassLabelRequest, ClassExplicitRoute, ClassFastReroute, ClassSessionAttr:
		return kind == MsgTypePath
	case ClassErrorSpec:
		return kind == MsgTypePathErr || kind == MsgTypeResvErr || kind == MsgTypeResvConf
	case ClassPolicyData:
		return kind == MsgTypePath || kind == MsgTypeResv || kind == MsgTypePathErr || kind == MsgTypeResvErr
	case ClassResvConfirm:
		return kind == MsgTypeResv || kind == MsgTypeResvConf
	case ClassScope:
		return kind == MsgTypeResv || kind == MsgTypeResvErr || kind == MsgTypeResvTear
	case ClassStyle, ClassFilterSpec, ClassFlowSpec:
		return reservationMessage(kind)
	case ClassLabel:
		return kind == MsgTypeResv
	case ClassRecordRoute:
		return kind == MsgTypePath || kind == MsgTypeResv
	}
	return true // Unknown classes follow Section 3.10, not the known grammar.
}

func knownCType(h objectHeader) bool {
	switch h.ClassNum {
	case ClassSession, ClassSenderTemplate, ClassFilterSpec:
		return h.CType == CTypeLSPTunnelIPv4
	case ClassRSVPHop, ClassErrorSpec, ClassResvConfirm, ClassScope:
		return h.CType == CTypeIPv4
	case ClassTimeValues, ClassStyle, ClassLabel, ClassLabelRequest, ClassExplicitRoute, ClassRecordRoute, ClassFastReroute:
		return h.CType == 1
	case ClassSenderTSpec, ClassFlowSpec, ClassAdspec:
		return h.CType == 2
	case ClassSessionAttr:
		return h.CType == CTypeSessionAttr || h.CType == CTypeSessionAttrRA
	}
	return true
}

// checkObjectPlacement checks message-level order and singleton multiplicity.
// NULL is permitted at every object boundary, even within a descriptor.
func checkObjectPlacement(msg *ParsedMessage, h objectHeader, off int, seen *[256]bool) error {
	if !objectAllowed(msg.Header.MsgType, h.ClassNum) {
		return fmt.Errorf("rsvp: class %d is illegal in message %d", h.ClassNum, msg.Header.MsgType)
	}
	// RFC 2205 Section 3.1.3: "If the INTEGRITY object is present, it must
	// immediately follow the common header."
	if h.ClassNum == ClassIntegrity && off != rsvpHdrLen {
		return fmt.Errorf("rsvp: INTEGRITY does not follow the common header")
	}
	if h.ClassNum == ClassNull {
		return nil
	}
	if reservationMessage(msg.Header.MsgType) {
		flow := h.ClassNum == ClassFlowSpec || h.ClassNum == ClassFilterSpec || h.ClassNum == ClassLabel || h.ClassNum == ClassRecordRoute
		// RFC 2205 Section 3.1.4: "The STYLE object followed by the flow
		// descriptor list must occur at the end of the message".
		if msg.HasStyle && !flow {
			return fmt.Errorf("rsvp: non-descriptor object after STYLE")
		}
		if flow && !msg.HasStyle {
			return fmt.Errorf("rsvp: flow descriptor before STYLE")
		}
		if flow {
			return nil
		}
	}
	switch h.ClassNum {
	case ClassPolicyData, ClassSessionAttr:
		return nil
	}
	if h.ClassNum&0xc0 != 0 {
		return nil
	}
	if seen[h.ClassNum] {
		return fmt.Errorf("rsvp: duplicate object class %d", h.ClassNum)
	}
	seen[h.ClassNum] = true
	return nil
}

func (msg *ParsedMessage) appendFlowSpec(fs FlowSpec, raw []byte) error {
	if msg.Style == StyleSharedExplicit && len(msg.FlowDescriptors) != 0 {
		return fmt.Errorf("rsvp: multiple SE FLOWSPEC objects")
	}
	if n := len(msg.FlowDescriptors); n > 0 && len(msg.FlowDescriptors[n-1].Filters) == 0 {
		return fmt.Errorf("rsvp: FLOWSPEC without FILTER_SPEC")
	}
	msg.FlowDescriptors = append(msg.FlowDescriptors, flowDescriptor{FlowSpec: fs, FlowSpecRaw: raw})
	return nil
}

func (msg *ParsedMessage) appendFilter(filter senderTemplateIPv4) error {
	n := len(msg.FlowDescriptors)
	if n == 0 {
		// RFC 2205 Section 3.1.6: "FLOWSPEC objects in the flow descriptor
		// list of a ResvTear message will be ignored and may be omitted."
		if msg.Header.MsgType != MsgTypeResvTear {
			return fmt.Errorf("rsvp: first filter lacks FLOWSPEC")
		}
		msg.FlowDescriptors = append(msg.FlowDescriptors, flowDescriptor{})
		n = 1
	}
	last := &msg.FlowDescriptors[n-1]
	if msg.Header.MsgType == MsgTypeResv && len(last.Filters) > 0 && !last.Filters[len(last.Filters)-1].HasLabel {
		return fmt.Errorf("rsvp: filter lacks LABEL")
	}
	for _, descriptor := range msg.FlowDescriptors {
		for _, existing := range descriptor.Filters {
			if existing.Filter == filter {
				return fmt.Errorf("rsvp: duplicate FILTER_SPEC")
			}
		}
	}
	if msg.Style == StyleFixedFilter && len(last.Filters) > 0 {
		// RFC 2205 Section 3.1.4: "A FLOWSPEC object can be omitted if it is
		// identical to the most recent such object that appeared in the list".
		msg.FlowDescriptors = append(msg.FlowDescriptors, flowDescriptor{FlowSpec: last.FlowSpec, FlowSpecRaw: last.FlowSpecRaw})
		last = &msg.FlowDescriptors[len(msg.FlowDescriptors)-1]
	}
	last.Filters = append(last.Filters, reservationFilter{Filter: filter})
	return nil
}

func (msg *ParsedMessage) lastFilter() *reservationFilter {
	if len(msg.FlowDescriptors) == 0 {
		return nil
	}
	descriptor := &msg.FlowDescriptors[len(msg.FlowDescriptors)-1]
	if len(descriptor.Filters) == 0 {
		return nil
	}
	return &descriptor.Filters[len(descriptor.Filters)-1]
}

func checkFlowDescriptors(msg *ParsedMessage) error {
	if !reservationMessage(msg.Header.MsgType) {
		return nil
	}
	if msg.Style != StyleFixedFilter && msg.Style != StyleSharedExplicit {
		return nil // The engine returns Unknown Reservation Style.
	}
	if len(msg.FlowDescriptors) == 0 {
		if msg.Header.MsgType == MsgTypeResvErr {
			return nil
		}
		return fmt.Errorf("%w: flow descriptor", errObjectAbsent)
	}
	if msg.Header.MsgType == MsgTypeResvErr && len(msg.FlowDescriptors) != 1 {
		return fmt.Errorf("rsvp: ResvErr has multiple flow descriptors")
	}
	for _, descriptor := range msg.FlowDescriptors {
		if len(descriptor.Filters) == 0 {
			return fmt.Errorf("%w: FILTER_SPEC", errObjectAbsent)
		}
		for _, filter := range descriptor.Filters {
			if msg.Header.MsgType == MsgTypeResv && !filter.HasLabel {
				return fmt.Errorf("%w: LABEL", errObjectAbsent)
			}
		}
	}
	return nil
}
