// Design: docs/architecture/plugin/plugin-system.md -- concurrent Stage 1 publication.
package process

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestProcessRegistrationPublishesCompleteRecords races a bounded publisher
// against a reader and requires every observed declaration to be a whole record.
// The race detector also proves publication synchronizes the record's contents.
func TestProcessRegistrationPublishesCompleteRecords(t *testing.T) {
	proc := NewProcess(plugin.PluginConfig{})
	start := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for index := range 4096 {
			proc.SetRegistration(&plugin.PluginRegistration{
				Name:   "published",
				Claims: []string{"published"},
			})
			if index == 0 {
				close(start)
			}
		}
	}()
	<-start
	for range 4096 {
		registration := proc.Registration()
		if registration == nil {
			t.Error("publication returned no declaration")
			continue
		}
		if registration.Name != "published" {
			t.Error("publication exposed an incomplete declaration name")
		}
		if len(registration.Claims) != 1 || registration.Claims[0] != "published" {
			t.Error("publication exposed an incomplete declaration")
		}
	}
	<-done
}

// TestProcessRegistrationPreservesAbsence checks the zero value, constructor
// and explicit nil publication without changing the borrowed record's identity.
func TestProcessRegistrationPreservesAbsence(t *testing.T) {
	var zero Process
	if zero.Registration() != nil {
		t.Error("zero Process has a declaration")
	}
	proc := NewProcess(plugin.PluginConfig{})
	if proc.Registration() == nil {
		t.Error("new Process has no initial declaration")
	}
	proc.SetRegistration(nil)
	if proc.Registration() != nil {
		t.Error("nil publication retained a declaration")
	}
	registration := &plugin.PluginRegistration{Name: "published"}
	proc.SetRegistration(registration)
	if proc.Registration() != registration {
		t.Error("publication changed the borrowed declaration")
	}
}
