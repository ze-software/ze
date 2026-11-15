// Design: docs/architecture/irr-owner.md -- show irr and the doctor before the worker's first read
package irr

import (
	"context"
	"testing"
	"time"
)

// VALIDATES: show irr and the irr-datasets doctor check refuse until the
// worker has read the store and published its first view, then answer the
// installed config.
// PREVENTS: show irr answering {"server":"","refresh-interval":0,"datasets":null}
// from the zero view while the worker still reads the store, which a restart
// observer read as "nothing stored" (test/plugin/irr-cold-start-stores.ci), and
// the doctor reporting no finding it never looked for.
//
// Method: drive the worker's steps by hand over a fake store holding one
// interest, and ask both readers before the load, after the load but before
// the publish, and after the publish.
func TestShowRefusesBeforeFirstView(t *testing.T) {
	cfg := defaultOwnerConfig()
	cfg.server = "whois.example.net"
	cfg.refreshInterval = 900 * time.Second
	h := newHarness(t, newFakeState(), cfg)

	assertRefused := func(when string) {
		t.Helper()
		if show, err := h.owner.showIRR(); err == nil {
			t.Fatalf("show irr %s answered %+v, want a refusal", when, show)
		}
		status, data, err := h.owner.handleCommand(cmdShowIRR, nil)
		if status != statusError || err == nil || data != nil {
			t.Fatalf("the show irr command %s answered status %q data %+v error %v, want an error and no data", when, status, data, err)
		}
		if findings, err := h.owner.answerDoctorCheck(datasetCheckName); err == nil {
			t.Fatalf("the doctor check %s answered %+v, want a refusal", when, findings)
		}
	}

	assertRefused("before the worker ran")
	ctx := context.Background()
	h.owner.absorb(ctx)
	h.owner.load(ctx)
	assertRefused("after the load and before the first view")

	h.owner.publishView()
	show, err := h.owner.showIRR()
	if err != nil {
		t.Fatalf("show irr after the first view: %v", err)
	}
	if show.Server != cfg.server || show.RefreshInterval != 900 {
		t.Fatalf("show irr %+v, want server %s and refresh-interval 900", show, cfg.server)
	}
	status, data, err := h.owner.handleCommand(cmdShowIRR, nil)
	if status != statusDone || err != nil || data == nil {
		t.Fatalf("the show irr command answered status %q data %+v error %v, want done", status, data, err)
	}
	if _, err := h.owner.answerDoctorCheck(datasetCheckName); err != nil {
		t.Fatalf("the doctor check after the first view: %v", err)
	}
}
