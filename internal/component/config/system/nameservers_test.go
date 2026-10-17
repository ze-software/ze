// Goal: prove the published name servers answer "nothing published" apart
// from "the config names none", that a later publication replaces an earlier
// one, and that neither the publisher nor the reader can change the stored
// list through its slice. Method: publish, mutate the slices on both sides,
// and read back.

package system

import (
	"slices"
	"testing"
)

func TestNameServersPublication(t *testing.T) {
	previous := publishedNameServers.Load()
	t.Cleanup(func() { publishedNameServers.Store(previous) })

	publishedNameServers.Store(nil)
	if servers, ok := NameServers(); ok {
		t.Fatalf("nothing published answered %v, true; want false", servers)
	}

	input := []string{"192.0.2.53", "192.0.2.54"}
	PublishNameServers(input)
	input[0] = "198.51.100.1"
	servers, ok := NameServers()
	if !ok {
		t.Fatal("a publication answered false")
	}
	if !slices.Equal(servers, []string{"192.0.2.53", "192.0.2.54"}) {
		t.Errorf("published %v; want the list as it was when published", servers)
	}
	servers[0] = "198.51.100.2"
	again, _ := NameServers()
	if again[0] != "192.0.2.53" {
		t.Errorf("a reader changed the published list: %v", again)
	}

	// A reload that removes the leaf-list publishes the empty list.
	PublishNameServers(nil)
	servers, ok = NameServers()
	if !ok {
		t.Fatal("an empty publication answered false; want true with no servers")
	}
	if len(servers) != 0 {
		t.Errorf("an empty publication answered %v", servers)
	}
}
