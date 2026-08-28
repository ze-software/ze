package fixture

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestVerifyPartialFault(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	path := filepath.Join(directory, "fault.json")
	rows := make([]map[string]any, 0, 11)
	for index := range 12 {
		if index != 6 {
			rows = append(rows, map[string]any{"index": index, "fill": "payload"})
		}
	}
	document := map[string]any{
		"rows": rows,
		"errors": []map[string]any{{
			"encoded-bytes": 16777217,
			"limit-bytes":   16777216,
			"message":       "answer record does not fit one wire message",
			"record":        7,
		}},
	}
	writeFixtureJSON(t, path, document)
	if err := verifyPartialFault(context.Background(), []string{path}); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyCommandStream(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "walk.ndjson")
	var content strings.Builder
	for index := range 300 {
		row, err := json.Marshal(map[string]any{"index": index, "fill": "payload"})
		if err != nil {
			t.Fatal(err)
		}
		content.Write(row)
		content.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyCommandStream(context.Background(), []string{path}); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyEngineAnswer(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "reading.json")
	writeFixtureJSON(t, path, map[string]any{
		"engine-answer": map[string]any{
			"type":    "map",
			"key":     "commands",
			"verdict": "done",
			"rows":    300,
			"first":   "show first",
			"last":    "show last",
		},
	})
	if err := verifyEngineAnswer(context.Background(), []string{path, strconv.Itoa(300)}); err != nil {
		t.Fatal(err)
	}
}

func TestEOFNoSpin(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := eofNoSpin(ctx, nil); err != nil {
		t.Fatal(err)
	}
}

func TestAliasConfigsUseNativeProvider(t *testing.T) {
	t.Parallel()
	for which := aliasCaseBasic; which <= aliasCaseShape; which++ {
		config := aliasConfig(which)
		if !strings.Contains(config, `run "ze-test fixture plugin/`) {
			t.Fatalf("case %d has no native provider: %s", which, config)
		}
	}
}

func writeFixtureJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}
