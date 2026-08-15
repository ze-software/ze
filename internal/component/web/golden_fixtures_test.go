package web

import (
	"time"
)

// Fixture data for the templ golden capture in golden_test.go.
//
// Each builder returns one view shape with fixed values. The capture renders
// the shape and compares the bytes against testdata/golden.

const webNotificationErrorText = "commit rejected: hold-time out of range"

func webFreeformData() *ConfigViewData {
	return &ConfigViewData{
		CurrentPath: "system/banner",
		Entries:     []string{"first line", "second line"},
	}
}

func webFlexChildren() []ChildEntry {
	return []ChildEntry{
		{Name: "timers", Kind: "container", URL: "/show/bgp/timers/", HxPath: "bgp/timers"},
	}
}

func webCommandResult(failed bool) CommandResultData {
	if failed {
		return CommandResultData{
			CommandName: "peer 192.0.2.9 teardown", Output: "no such peer", Error: true,
		}
	}

	return CommandResultData{
		CommandName: "peer 192.0.2.1 teardown", Output: "session reset",
	}
}

func webBannerData() notificationBannerData {
	return notificationBannerData{
		Reason: "Config changed by admin: peer added", RefreshURL: "/show/",
	}
}

func webErrorData() ErrorData {
	return ErrorData{
		ID:      "42",
		Path:    "bgp/peer/london/hold-time",
		Message: "value 1 below minimum 3",
	}
}

func webToolOutput() toolPageData {
	return toolPageData{Output: "5 packets transmitted, 5 received"}
}

func webToolError() toolPageData {
	return toolPageData{Error: "destination unreachable"}
}

func webBreadcrumbOne() *FragmentData {
	return &FragmentData{
		Breadcrumbs: webBreadcrumbs()[:1], Username: "admin", ActiveUI: "finder",
	}
}

func webBreadcrumbNone() *FragmentData {
	return &FragmentData{ActiveUI: "finder"}
}

func webSidebarRoot() *FragmentData {
	return &FragmentData{Sidebar: webSidebarSections()}
}

func webFragmentDataMonitor() *FragmentData {
	data := webFragmentDataFields()
	data.Monitor = true

	return data
}

func webCommitData(shape string) configCommitData {
	switch shape {
	case "diff":
		return configCommitData{
			Diff: "+ bgp peer 192.0.2.1",
			DiffLines: []configDiffLine{
				{Text: "+ peer 192.0.2.1", IsAdd: true},
				{Text: "- peer 192.0.2.2", IsDel: true},
				{Text: "~ hold-time 90", IsChange: true},
				{Text: "  bgp"},
			},
		}
	case "conflict":
		return configCommitData{
			Error:         "config changed under you",
			ConflictPaths: []string{"bgp/peer/192.0.2.1", "bgp/local-as"},
		}
	}

	return configCommitData{}
}

func webAddFormView(keyless bool) addFormData {
	data := addFormData{
		AddURL:     "/config/add/bgp/peer/",
		ListName:   "Peer",
		Heading:    "New Peer",
		KeyName:    "name",
		KeyLabel:   "name",
		KeyInputID: "field-name",
		Keyless:    keyless,
		Workbench:  keyless,
		Fields: []addFormField{
			{Path: "remote/as", Placeholder: "peer AS", Category: "required", Inherited: "64500"},
			{Path: "remote/ip", Placeholder: "peer address", Category: "suggest"},
			{Path: "description", Placeholder: "free text", Category: "unique"},
		},
	}

	if keyless {
		data.KeyName = ""
		data.DisplayKey = "address"
	}

	return data
}

func webL2TPListView(populated bool) l2tpListView {
	if !populated {
		return l2tpListView{}
	}

	return l2tpListView{
		TunnelCount:  1,
		SessionCount: 2,
		Sessions: []l2tpSessionRow{
			{
				LocalSID: 1, TunnelTID: 7, Username: "alice", AssignedAddr: "192.0.2.10",
				PeerAddr: "198.51.100.1", State: "established", Interface: "ppp0",
			},
			{
				LocalSID: 2, TunnelTID: 7, Username: "bob", AssignedAddr: "192.0.2.11",
				PeerAddr: "198.51.100.1", State: "establishing", Interface: "ppp1",
			},
		},
	}
}

func webL2TPDetailView(events bool) l2tpDetailView {
	view := l2tpDetailView{
		Session: l2tpSessionView{
			LocalSID: 1, Username: "alice", State: "established",
			AssignedAddr: "192.0.2.10", PppInterface: "ppp0",
			TunnelLocalTID: 7, Family: "ipv4",
		},
		Login: "alice",
	}

	if !events {
		view.Login = ""

		return view
	}

	// A fixed timestamp keeps the comparison stable: the page formats it, so
	// time.Now would move the bytes on every run.
	stamp := time.Date(2026, time.January, 2, 15, 4, 5, 0, time.UTC)
	view.Events = []l2tpEventRow{
		{Timestamp: stamp, Type: "connect", Actor: "lac"},
		{Timestamp: stamp.Add(time.Minute), Type: "echo", RTT: "12ms"},
		{Timestamp: stamp.Add(2 * time.Minute), Type: "disconnect", Reason: "admin request"},
	}

	return view
}
