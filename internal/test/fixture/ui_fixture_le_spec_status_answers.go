package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

func init() {
	Register("ui/le-spec-status-answers", uiDriver(leSpecStatusAnswers))
}

type uiLeSpecStatusAnswersCommandAnswer struct {
	stdout []byte
	stderr []byte
	code   int
}

func leSpecStatusAnswers(ctx context.Context) error {
	root := os.Getenv("ZE_REPO_ROOT")
	if root == "" {
		return uiLeSpecStatusAnswersFailf("ZE_REPO_ROOT is not set")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return uiLeSpecStatusAnswersFailf("make ZE_REPO_ROOT absolute: %v", err)
	}
	here, _, err := temporaryLEFixtureWorkspace("le-spec-status-answers-")
	if err != nil {
		return uiLeSpecStatusAnswersFailf("create fixture working directory: %v", err)
	}
	defer os.RemoveAll(here) //nolint:errcheck // fixture cleanup

	binary, err := nativeLEBinary()
	if err != nil {
		return uiLeSpecStatusAnswersFailf("%v", err)
	}

	// Read each rendering twice. Besides requiring deterministic bytes, this
	// distinguishes an unstable checkout from a disagreement between answer
	// renderings without consulting any retired implementation.
	//
	// It now ACTS on that distinction, which it did not until 2026-09-21. This
	// checkout is shared and several sessions edit it at once, so a spec file
	// written between the two readings moves a count: measured that day, the
	// first read 340 specs and the second 341 because another session created
	// one in between, and the case reported that as two renderings disagreeing.
	// A tree that will not hold still makes the experiment invalid, which is not
	// the same as the property being false, so the readings are retaken and the
	// difference is named for what it is.
	page1, json1, json2, err := specStatusOverAStillTree(ctx, root, here, binary)
	if err != nil {
		return err
	}

	if page1.code != 0 {
		return uiLeSpecStatusAnswersFailf("le spec status exited %d; stderr: %q", page1.code, page1.stderr)
	}
	if len(page1.stderr) != 0 {
		return uiLeSpecStatusAnswersFailf("le spec status wrote warnings: %q", page1.stderr)
	}
	if bytes.Count(page1.stdout, []byte{'\n'}) <= 100 {
		return uiLeSpecStatusAnswersFailf("the comparison ran over %d lines, which is too few to mean anything", bytes.Count(page1.stdout, []byte{'\n'}))
	}
	trimmedPage := bytes.TrimSpace(page1.stdout)
	if len(trimmedPage) == 0 {
		return uiLeSpecStatusAnswersFailf("le spec status returned an empty page")
	}
	if trimmedPage[0] == '[' || trimmedPage[0] == '{' {
		return uiLeSpecStatusAnswersFailf("le spec status returned structured records instead of the default page")
	}

	if json1.code != 0 {
		return uiLeSpecStatusAnswersFailf("le spec status | json exited %d; stderr: %q", json1.code, json1.stderr)
	}
	if len(json1.stderr) != 0 {
		return uiLeSpecStatusAnswersFailf("le spec status | json wrote warnings: %q", json1.stderr)
	}
	if json2.code != json1.code || !bytes.Equal(json2.stderr, json1.stderr) || !bytes.Equal(json2.stdout, json1.stdout) {
		return uiLeSpecStatusAnswersFailf("two structured inventory answers disagree")
	}

	records1, err := decodeRecords(json1.stdout)
	if err != nil {
		return uiLeSpecStatusAnswersFailf("the structured answer did not decode: %v", err)
	}
	records2, err := decodeRecords(json2.stdout)
	if err != nil {
		return uiLeSpecStatusAnswersFailf("the second structured answer did not decode: %v", err)
	}
	if !reflect.DeepEqual(records1, records2) {
		return uiLeSpecStatusAnswersFailf("two decoded inventory answers disagree")
	}
	if len(records1) <= 50 {
		return uiLeSpecStatusAnswersFailf("%d records is too few for this checkout to mean anything", len(records1))
	}
	if err := checkRecordContract(records1, page1.stdout); err != nil {
		return err
	}

	counted, err := uiLeSpecStatusAnswersRunCommand(ctx, here, os.Environ(), binary, "spec status", "|", "count")
	if err != nil {
		return err
	}
	if counted.code != 0 {
		return uiLeSpecStatusAnswersFailf("le spec status | count exited %d; stderr: %q", counted.code, counted.stderr)
	}
	if len(counted.stderr) != 0 {
		return uiLeSpecStatusAnswersFailf("le spec status | count wrote warnings: %q", counted.stderr)
	}
	wantCount := fmt.Sprintf("%d", len(records1))
	if strings.TrimSpace(string(counted.stdout)) != wantCount {
		return uiLeSpecStatusAnswersFailf("le spec status | count answered %q, want %s", counted.stdout, wantCount)
	}

	refused, err := uiLeSpecStatusAnswersRunCommand(ctx, here, os.Environ(), binary, "spec status", "--json")
	if err != nil {
		return err
	}
	if refused.code != 2 {
		return uiLeSpecStatusAnswersFailf("le spec status --json exited %d, want 2", refused.code)
	}
	if len(refused.stdout) != 0 {
		return uiLeSpecStatusAnswersFailf("le spec status --json wrote unexpected stdout: %q", refused.stdout)
	}
	if !bytes.Contains(refused.stderr, []byte("usage: le spec status")) ||
		!bytes.Contains(refused.stderr, []byte(`got "--json"`)) {
		return uiLeSpecStatusAnswersFailf("the refusal does not identify the invalid argument: %q", refused.stderr)
	}

	fmt.Println("OK")
	return nil
}

func checkRecordContract(records []map[string]json.RawMessage, page []byte) error {
	required := []string{fieldName, fieldStatus, "bucket", "category", fieldUpdated, "git-modified", statusStale}
	lines := strings.Split(string(page), "\n")
	lineAt := 0
	section := ""

	for i, record := range records {
		for _, key := range required {
			if _, ok := record[key]; !ok {
				return uiLeSpecStatusAnswersFailf("record %d carries no %q field", i, key)
			}
		}

		name, err := stringField(record, "name")
		if err != nil || name == "" {
			return uiLeSpecStatusAnswersFailf("record %d has an invalid name: %v", i, err)
		}

		status, err := stringField(record, "status")
		if err != nil {
			return uiLeSpecStatusAnswersFailf("record %q has an invalid status: %v", name, err)
		}
		bucket, err := stringField(record, "bucket")
		if err != nil {
			return uiLeSpecStatusAnswersFailf("record %q has an invalid bucket: %v", name, err)
		}
		category, err := stringField(record, "category")
		if err != nil {
			return uiLeSpecStatusAnswersFailf("record %q has an invalid category: %v", name, err)
		}
		updated, err := stringField(record, "updated")
		if err != nil {
			return uiLeSpecStatusAnswersFailf("record %q has an invalid updated value: %v", name, err)
		}
		modified, err := stringField(record, "git-modified")
		if err != nil {
			return uiLeSpecStatusAnswersFailf("record %q has an invalid git-modified value: %v", name, err)
		}
		if _, err := time.Parse("2006-01-02", modified); err != nil {
			return uiLeSpecStatusAnswersFailf("record %q has git-modified %q, want an ISO git date", name, modified)
		}
		var stale bool
		if err := json.Unmarshal(record["stale"], &stale); err != nil {
			return uiLeSpecStatusAnswersFailf("record %q has a non-boolean stale value: %v", name, err)
		}

		row := -1
		for j := lineAt; j < len(lines); j++ {
			if strings.HasPrefix(lines[j], "── ") {
				section = lines[j]
			}
			if strings.Contains(lines[j], name) {
				row = j
				break
			}
		}
		if row < 0 {
			return uiLeSpecStatusAnswersFailf("the page has no row for record %q in record order", name)
		}
		lineAt = row + 1
		// The page's SECTIONS are the status-derived CATEGORY, never the release
		// BUCKET. The two were one field until 6fb9cd8814 (2026-09-05) split
		// them: `bucket` is now the directory the spec sits in (after,
		// immediate, pre-release, internal/le/spec/specpath) and `category` is
		// the backlog / idea / other split the sections print
		// (specstatus.Category). Reading the section off `bucket` asks the
		// wrong record for the answer.
		wantSection := map[string]string{
			"backlog": "Committed backlog",
			"idea":    "Idea capture",
			"other":   "Other",
		}[category]
		if wantSection == "" || !strings.Contains(section, wantSection) {
			return uiLeSpecStatusAnswersFailf("the page files %q under %q, want category %q", name, section, category)
		}
		if !slices.Contains([]string{"after", "immediate", "pre-release"}, bucket) {
			return uiLeSpecStatusAnswersFailf("record %q carries bucket %q, which names no release bucket", name, bucket)
		}
		for key, value := range map[string]string{
			fieldName:    name,
			fieldStatus:  status,
			"bucket":     bucket,
			fieldUpdated: updated,
		} {
			if value != "" && !strings.Contains(lines[row], value) {
				return uiLeSpecStatusAnswersFailf("the page row for %q does not render %s %q: %q", name, key, value, lines[row])
			}
		}
		for _, key := range []string{"phase", "depends"} {
			raw, ok := record[key]
			if !ok {
				continue
			}
			values, err := textualValues(raw)
			if err != nil {
				return uiLeSpecStatusAnswersFailf("record %q has an invalid %s value: %v", name, key, err)
			}
			for _, value := range values {
				if value != "" && !strings.Contains(lines[row], value) {
					return uiLeSpecStatusAnswersFailf("the page row for %q does not render %s value %q: %q", name, key, value, lines[row])
				}
			}
		}
	}
	return nil
}

func stringField(record map[string]json.RawMessage, key string) (string, error) {
	var value string
	if err := json.Unmarshal(record[key], &value); err != nil {
		return "", err
	}
	return value, nil
}

func textualValues(raw json.RawMessage) ([]string, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	var out []string
	var visit func(any)
	visit = func(v any) {
		switch v := v.(type) {
		case string:
			out = append(out, v)
		case []any:
			for _, item := range v {
				visit(item)
			}
		}
	}
	visit(value)
	return out, nil
}

func decodeRecords(answer []byte) ([]map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(answer))
	var records []map[string]json.RawMessage
	if err := decoder.Decode(&records); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("extra value after the record array")
		}
		return nil, err
	}
	return records, nil
}

func uiLeSpecStatusAnswersRunCommand(ctx context.Context, dir string, env []string, name string, args ...string) (uiLeSpecStatusAnswersCommandAnswer, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Dir = dir
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	answer := uiLeSpecStatusAnswersCommandAnswer{stdout: stdout.Bytes(), stderr: stderr.Bytes(), code: 0}
	if err == nil {
		return answer, nil
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		answer.code = exit.ExitCode()
		return answer, nil
	}
	return uiLeSpecStatusAnswersCommandAnswer{}, uiLeSpecStatusAnswersFailf("execute %s: %v", name, err)
}

func lineDifference(a, b []byte, limit int) string {
	as := strings.Split(string(a), "\n")
	bs := strings.Split(string(b), "\n")
	var out textbuf.Buffer
	length := len(as)
	length = max(length, len(bs))
	for i := 0; i < length && out.Len() < limit; i++ {
		var av, bv string
		if i < len(as) {
			av = as[i]
		}
		if i < len(bs) {
			bv = bs[i]
		}
		if av != bv {
			fmt.Fprintf(&out, "  first[%d]:  %q\n  second[%d]: %q\n", i, av, i, bv)
		}
	}
	text := out.String()
	if len(text) > limit {
		text = text[:limit]
	}
	return text
}

func uiLeSpecStatusAnswersFailf(format string, args ...any) error {
	return fmt.Errorf("FAIL: "+format, args...)
}

// specStatusSettleAttempts bounds how many times the four readings are retaken
// when the checkout moved between them.
const specStatusSettleAttempts = 3

// specStatusOverAStillTree answers the page and the two structured readings
// `le spec status` writes, taken over a tree that did not change between the
// two page readings.
//
// The second structured reading is returned with them, because the case
// compares the two against each other and against the page.
//
// This checkout is shared: a spec file written between two readings moves a
// count, and the case then reports two renderings disagreeing when what
// disagreed was the tree. Retaking is not retry-until-green -- the comparisons
// the caller makes are unchanged, and a tree that will not settle is named as
// that, which is a different statement from the property being false.
func specStatusOverAStillTree(ctx context.Context, root, here, binary string) (
	page, structured, structuredAgain uiLeSpecStatusAnswersCommandAnswer, err error,
) {
	var lastFirst, lastSecond uiLeSpecStatusAnswersCommandAnswer
	for attempt := range specStatusSettleAttempts {
		page1, runErr := uiLeSpecStatusAnswersRunCommand(ctx, here, os.Environ(), binary, "spec status")
		if runErr != nil {
			return page, structured, structuredAgain, runErr
		}
		json1, runErr := uiLeSpecStatusAnswersRunCommand(ctx, root, os.Environ(), binary, "spec status", "|", "json")
		if runErr != nil {
			return page, structured, structuredAgain, runErr
		}
		page2, runErr := uiLeSpecStatusAnswersRunCommand(ctx, here, os.Environ(), binary, "spec status")
		if runErr != nil {
			return page, structured, structuredAgain, runErr
		}
		json2, runErr := uiLeSpecStatusAnswersRunCommand(ctx, root, os.Environ(), binary, "spec status", "|", "json")
		if runErr != nil {
			return page, structured, structuredAgain, runErr
		}

		lastFirst, lastSecond = page1, page2
		if page2.code == page1.code &&
			bytes.Equal(page2.stderr, page1.stderr) && bytes.Equal(page2.stdout, page1.stdout) {
			return page1, json1, json2, nil
		}
		fmt.Fprintf(os.Stderr, "the checkout moved during reading %d of %d; retaking\n",
			attempt+1, specStatusSettleAttempts)
	}

	return page, structured, structuredAgain, uiLeSpecStatusAnswersFailf(
		"the checkout would not hold still across %d readings, so the renderings could not be compared:\n%s",
		specStatusSettleAttempts, lineDifference(lastFirst.stdout, lastSecond.stdout, 2000))
}
