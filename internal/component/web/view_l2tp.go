// Design: docs/architecture/web-workbench-pages.md -- L2TP web management UI
// Related: handler_l2tp.go -- the handlers that fill these, behind ze_l2tp
// Related: l2tp_list.templ, l2tp_detail.templ -- the components that read them

package web

import "time"

// The L2TP pages render through templ, and the generated Go carries no build
// tag. Their view models therefore name no type from internal/component/l2tp.
// A field of that package's type would drag the import into every build,
// including one without ze_l2tp. handler_l2tp.go converts a snapshot into the
// structs below. It is the only file that knows both shapes.
//
// THE JSON BODY AND THE VIEW COME FROM ONE VALUE. The list handler builds its
// rows once and answers a JSON request with them. The detail handler builds
// l2tpDetailBody (handler_l2tp.go) once, then projects the view out of it. Two
// constructions of one session are what let the body and the page differ.
//
// Field ORDER is part of a JSON body. A Go map serializes in sorted key order,
// and a struct serializes in declaration order. A struct that replaces a map
// declares its fields alphabetically to keep the response bytes.

// l2tpSessionRow is one row of the L2TP session list. It is also the element
// type of the "Sessions" value in the list handler's JSON body. Its field names
// and their order are part of that response.
type l2tpSessionRow struct {
	LocalSID     uint16
	TunnelTID    uint16
	Username     string
	AssignedAddr string
	PeerAddr     string
	State        string
	Interface    string
	CreatedAt    time.Time
}

// l2tpListView is what the session list page renders.
type l2tpListView struct {
	TunnelCount  int
	SessionCount int
	Sessions     []l2tpSessionRow
}

// l2tpSessionView is the session the detail page describes.
type l2tpSessionView struct {
	LocalSID       uint16
	Username       string
	State          string
	AssignedAddr   string
	PppInterface   string
	TunnelLocalTID uint16
	Family         string
}

// l2tpEventRow is one line of the detail page's event timeline. Type and RTT
// are text because the page shows what String() produced, and an empty RTT is
// the zero duration the markup skips.
type l2tpEventRow struct {
	Timestamp time.Time
	Type      string
	Actor     string
	Reason    string
	RTT       string
}

// l2tpDetailView is what the session detail page renders.
type l2tpDetailView struct {
	Session l2tpSessionView
	Login   string
	Events  []l2tpEventRow
}
