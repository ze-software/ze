package main

import "testing"

// redisListed is the reference the short name redis completes to.
const redisListed = "docker.io/library/redis:latest"

// TestListingRowsJudgeTheFetch: the fetch mode reads the listing's rows of
// its reference only, with the JSON the listing writes (a bare list, with
// spaces after the colons as `| json` prints them), and judges
// them: present wins over failed, failed over running, and no row is absent.
func TestListingRowsJudgeTheFetch(t *testing.T) {
	cases := []struct {
		name   string
		answer string
		want   fetchVerdict
	}{
		{"no row", `[]`, fetchAbsent},
		{"another image only", `[{"reference": "docker.io/library/probe:latest", "state": "present"}]`, fetchAbsent},
		{"fetching", `[{"reference": "` + redisListed + `", "state": "fetching", "received": 1024}]`, fetchRunning},
		{"failed", `[{"reference": "` + redisListed + `", "state": "failed", "error": "e", "code": "c"}]`, fetchFailed},
		{"present beside failed", `[{"reference": "` + redisListed + `", "state": "failed"}, {"reference": "` +
			redisListed + `", "state": "present", "size": 1}]`, fetchPresent},
	}
	for _, c := range cases {
		rows, err := listingRows([]byte(c.answer), redisListed)
		if err != nil {
			t.Errorf("%s: listingRows: %v", c.name, err)
			continue
		}
		got, _ := judgeRows(rows)
		if got != c.want {
			t.Errorf("%s: verdict %d, want %d", c.name, got, c.want)
		}
	}
}

// TestListingRowsRefuseAnotherShape: an answer without the rows' key, or
// not JSON, is an error the fetch mode quotes, never a listing with no row.
func TestListingRowsRefuseAnotherShape(t *testing.T) {
	for _, answer := range []string{`{"rows": []`, `show container image list`, ``} {
		if _, err := listingRows([]byte(answer), redisListed); err == nil {
			t.Errorf("listingRows(%q) answered no error", answer)
		}
	}
}

// TestJudgeRowsQuotesTheFailure: a failed row's error and code reach the
// verdict's row, so the FAIL line can quote them.
func TestJudgeRowsQuotesTheFailure(t *testing.T) {
	rows, err := listingRows([]byte(`[{"reference": "`+redisListed+`", "state": "failed", "error": "the registry refused", "code": "doctor-x"}]`), redisListed)
	if err != nil {
		t.Fatalf("listingRows: %v", err)
	}
	verdict, row := judgeRows(rows)
	if verdict != fetchFailed {
		t.Fatalf("verdict %d, want failed", verdict)
	}
	if row.Error != "the registry refused" {
		t.Errorf("error %q", row.Error)
	}
	if row.Code != "doctor-x" {
		t.Errorf("code %q", row.Code)
	}
}
