package bgp

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// TestPathsLimitReceivePredicates drives both polarities of every reading
// checkPathsLimitReceive makes of Ze, with no container: the received rows of
// BIRD's session and the refusal counter. A removed ceiling (four held, none
// counted), a ceiling that dropped without counting, a miscount, and a lost
// second prefix must each be refused, and a wait must not end before every
// offered path reached a verdict.
func TestPathsLimitReceivePredicates(t *testing.T) {
	const bird = "172.30.0.4"
	rows := func(held, other int, extra ...map[string]string) string {
		var routes []map[string]string
		// The row shape bgp-rib answers for an ADD-PATH session, read off the
		// first live run: the path identifier follows the prefix.
		for index := range held {
			routes = append(routes, map[string]string{"peer": bird, "direction": "received", "prefix": pathsLimitReceivePrefix + " [pathID=" + strconv.Itoa(index+1) + "]", "family": zeIPv4Unicast})
		}
		for index := range other {
			routes = append(routes, map[string]string{"peer": bird, "direction": "received", "prefix": pathsLimitReceiveOtherPrefix + " [pathID=" + strconv.Itoa(index+1) + "]", "family": zeIPv4Unicast})
		}
		routes = append(routes, extra...)
		if routes == nil {
			routes = []map[string]string{}
		}
		data, err := json.Marshal(routes)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	metrics := func(series ...string) string {
		text := "# TYPE ze_bgp_paths_limit_refused_total counter\n" + strings.Join(series, "")
		data, err := json.Marshal(map[string]string{"metrics": text})
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	refused := func(peer, count string) string {
		return `ze_bgp_paths_limit_refused_total{family="ipv4/unicast",peer="` + peer + `"} ` + count + "\n"
	}
	observe := func(t *testing.T, routes, values string) pathsLimitReceiveObservation {
		t.Helper()
		observation, err := decodePathsLimitReceive(routes, values, bird)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		return observation
	}

	t.Run("ceiling held and counted", func(t *testing.T) {
		observation := observe(t, rows(2, 1), metrics(refused(bird, "2")))
		if !observation.accounts {
			t.Fatalf("every path is accounted for, but the wait would go on: %+v", observation)
		}
		if err := requirePathsLimitReceive(observation); err != nil {
			t.Fatalf("the correct outcome was refused: %v", err)
		}
	})
	t.Run("ceiling removed", func(t *testing.T) {
		observation := observe(t, rows(4, 1), metrics())
		if !observation.accounts {
			t.Fatalf("four held paths account for the offer, but the wait would go on: %+v", observation)
		}
		if requirePathsLimitReceive(observation) == nil {
			t.Fatal("four held paths passed")
		}
	})
	t.Run("dropped without counting", func(t *testing.T) {
		observation := observe(t, rows(2, 1), metrics(refused(bird, "1")))
		if observation.accounts {
			t.Fatalf("one path has no verdict yet, but the wait would end: %+v", observation)
		}
		if requirePathsLimitReceive(observation) == nil {
			t.Fatal("a refusal the counter missed passed")
		}
	})
	t.Run("counted twice", func(t *testing.T) {
		if requirePathsLimitReceive(observe(t, rows(2, 1), metrics(refused(bird, "3")))) == nil {
			t.Fatal("three counted refusals of two passed")
		}
	})
	t.Run("other prefix lost", func(t *testing.T) {
		observation := observe(t, rows(2, 0), metrics(refused(bird, "2")))
		if observation.accounts {
			t.Fatalf("the other prefix is absent, but the wait would end: %+v", observation)
		}
		if requirePathsLimitReceive(observation) == nil {
			t.Fatal("a lost second prefix passed")
		}
	})
	t.Run("other sessions and families are not counted", func(t *testing.T) {
		foreign := map[string]string{"peer": "172.30.0.3", "direction": "received", "prefix": pathsLimitReceivePrefix, "family": zeIPv4Unicast}
		sent := map[string]string{"peer": bird, "direction": "sent", "prefix": pathsLimitReceivePrefix, "family": zeIPv4Unicast}
		observation := observe(t, rows(2, 1, foreign, sent), metrics(refused("172.30.0.3", "5"), refused(bird, "2")))
		if err := requirePathsLimitReceive(observation); err != nil {
			t.Fatalf("another session's rows or counter leaked in: %v (%+v)", err, observation)
		}
	})
	t.Run("a timed-out wait shows what Ze held", func(t *testing.T) {
		answer := rows(1, 0)
		shown := observe(t, answer, metrics(refused(bird, "2"))).String()
		if !strings.Contains(shown, answer) {
			t.Fatalf("the last output omits Ze's received rows: %s", shown)
		}
		if !strings.Contains(shown, "refused 2") {
			t.Fatalf("the last output omits the refusal count: %s", shown)
		}
	})
	t.Run("absent series is zero refusals", func(t *testing.T) {
		if got := observe(t, rows(2, 1), metrics()).refused; got != 0 {
			t.Fatalf("refused = %d with no series", got)
		}
	})
	t.Run("unreadable answers are refused", func(t *testing.T) {
		if _, err := decodePathsLimitReceive(rows(2, 1), `{"metrics":""}`, bird); err == nil {
			t.Fatal("an empty metrics reply was read as zero refusals")
		}
		if _, err := decodePathsLimitReceive("null", metrics(), bird); err == nil {
			t.Fatal("a reply with no routes array was read as no routes")
		}
		if _, err := decodePathsLimitReceive(rows(2, 1), metrics(refused(bird, "1"), refused(bird, "1")), bird); err == nil {
			t.Fatal("two series for one session were read as one")
		}
	})
}
