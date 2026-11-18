// Design: docs/architecture/web-workbench-pages.md -- original L2TP detail links.
// Related: handler_l2tp.go -- the one captured original serves snapshot and history.
//go:build ze_l2tp

package web

import (
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// l2tpDetailSelector parses the link's cold filters before the existing original
// capture. Missing optional filters stay absent; invalid or repeated ones refuse.
// This is a wire DTO, never an owner. Its only authority comes from CaptureSession.
func l2tpDetailSelector(request *http.Request, sid uint16) (l2tp.SessionSelector, *opererr.Error) {
	query, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil {
		return l2tp.SessionSelector{}, l2tpDetailSelectorError(err, "The L2TP detail query is malformed")
	}
	selector := l2tp.SessionSelector{SID: sid}
	value, present, refusal := l2tpDetailQueryValue(query, "tunnel-id")
	if refusal != nil {
		return l2tp.SessionSelector{}, refusal
	}
	if present {
		tid, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			return l2tp.SessionSelector{}, l2tpDetailSelectorError(err, "The L2TP tunnel qualifier is invalid")
		}
		if tid == 0 {
			return l2tp.SessionSelector{}, l2tpDetailSelectorError(nil, "The L2TP tunnel qualifier is zero")
		}
		selector.TID = uint16(tid)
	}
	value, present, refusal = l2tpDetailQueryValue(query, "listener")
	if refusal != nil {
		return l2tp.SessionSelector{}, refusal
	}
	if present {
		endpoint, err := netip.ParseAddrPort(value)
		if err != nil {
			return l2tp.SessionSelector{}, l2tpDetailSelectorError(err, "The L2TP listener endpoint is invalid")
		}
		if endpoint.Port() == 0 {
			return l2tp.SessionSelector{}, l2tpDetailSelectorError(nil, "The L2TP listener port is zero")
		}
		selector.Listener = endpoint
	}
	value, present, refusal = l2tpDetailQueryValue(query, "expected-id")
	if refusal != nil {
		return l2tp.SessionSelector{}, refusal
	}
	if present {
		birth, err := subscriberid.ParseL2TPBirth(value)
		if err != nil {
			return l2tp.SessionSelector{}, l2tpDetailSelectorError(err, "The L2TP original-session reference is invalid")
		}
		selector.Birth = birth
	}
	return selector, nil
}

func l2tpDetailQueryValue(query url.Values, key string) (string, bool, *opererr.Error) {
	values, present := query[key]
	if !present {
		return "", false, nil
	}
	if len(values) != 1 {
		return "", false, l2tpDetailSelectorError(nil, fmt.Sprintf("The L2TP detail qualifier %q is repeated", key))
	}
	if values[0] == "" {
		return "", false, l2tpDetailSelectorError(nil, fmt.Sprintf("The L2TP detail qualifier %q has no value", key))
	}
	return values[0], true, nil
}

func l2tpDetailSelectorError(cause error, problem string) *opererr.Error {
	return opererr.Wrap(cause, problem,
		"A detail link must retain the displayed original's tunnel, listener and immutable reference",
		"Open /l2tp")
}

// l2tpSessionDetailURL projects the snapshot's original coordinates into one
// escaped link. The Birth is copied, never reconstructed from numeric labels.
// The fixed three filters bound this projection; no lookup or resource is owned.
func l2tpSessionDetailURL(session l2tp.SessionSnapshot) string {
	query := make(url.Values)
	if session.TunnelLocalTID != 0 {
		query.Set("tunnel-id", strconv.FormatUint(uint64(session.TunnelLocalTID), 10))
	}
	if session.Listener.IsValid() {
		query.Set("listener", session.Listener.String())
	}
	if session.Birth.IsValid() {
		query.Set("expected-id", session.Birth.String())
	}
	link := "/l2tp/" + strconv.FormatUint(uint64(session.LocalSID), 10)
	if len(query) == 0 {
		return link
	}
	return link + "?" + query.Encode()
}

func l2tpSessionListener(session l2tp.SessionSnapshot) string {
	if !session.Listener.IsValid() {
		return ""
	}
	return session.Listener.String()
}
