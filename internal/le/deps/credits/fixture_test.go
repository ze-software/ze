package credits

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fixture is a checkout with one vendored module and one work that is not
// a module, each with its licence file, declared once, and one package that
// declares the work present, beside the toolchain work every declaration
// holds, which no package declares. The toolchain work shares the group of
// the other work, so the fixture still declares two groups. Every test
// changes one thing of it.
const (
	goWork        = "Go"
	goLicence     = "third_party/go/LICENSE"
	fixtureModule = "example.com/ze"
	widget        = "github.com/acme/widget"
	gadget        = "github.com/acme/gadget"
	thing         = "thing"
	webPackage    = fixtureModule + "/internal/component/web"
	webFile       = "internal/component/web/web.go"
	widgetLicence = "vendor/" + widget + "/LICENSE"
	thingLicence  = "third_party/thing/LICENSE"
	thingBundle   = "third_party/thing/thing.js.LICENSE.txt"
)

const mitText = `MIT License

Copyright (c) 2020 Acme Widgets

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction.
`

const bsdText = `Copyright 2019 The Thing Authors. All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

   * Neither the name of the copyright holder nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.
`

const webSource = `package web

import "example.com/ze/internal/core/credits"

func init() {
	credits.DeclarePresent("thing")
}
`

const vendorModules = "# github.com/acme/widget v1.0.0\n## explicit\ngithub.com/acme/widget\n" +
	"# github.com/acme/gadget v1.0.0\n## explicit\ngithub.com/acme/gadget\n"

// declarationJSON is the fixture's declaration; copyright is widget's holder.
func declarationJSON(copyright string) string {
	return `{
  "categories": [
    {"name": "Widgets", "modules": [
      {"module": "github.com/acme/widget", "why": "Widgets.", "licence": "MIT", "copyright": "` + copyright + `", "source": "https://github.com/acme/widget"}
    ]}
  ],
  "linked": [],
  "works": [
    {"name": "thing", "version": "1.0", "licence": "BSD-3-Clause", "copyright": "The Thing Authors",
     "source": "https://example.org/thing", "why": "Things.", "group": "Web assets",
     "files": ["third_party/thing/LICENSE"], "carried-by": ["example.com/ze/internal/component/web"]},
    {"name": "` + goWork + `", "version": "", "toolchain": true, "licence": "BSD-3-Clause", "copyright": "The Go Authors",
     "source": "https://github.com/golang/go", "why": "The language.", "group": "Web assets",
     "files": ["` + goLicence + `"]}
  ]
}
`
}

// goText is the toolchain work's licence: the fixture's BSD text under the Go
// authors' name, so the check finds its holder and its licence.
var goText = strings.Replace(bsdText, "The Thing Authors", "The Go Authors", 1)

// fixtureFiles is the valid fixture, path to content.
func fixtureFiles() map[string]string {
	return map[string]string{
		"go.mod":                        "module " + fixtureModule + "\n\ngo 1.24\n",
		"vendor/modules.txt":            vendorModules,
		widgetLicence:                   mitText,
		"vendor/" + widget + "/w.go":    "package widget\n",
		"vendor/" + gadget + "/LICENSE": mitText,
		"vendor/" + gadget + "/g.go":    "package gadget\n",
		thingLicence:                    bsdText,
		goLicence:                       goText,
		webFile:                         webSource,
		DeclarationRel:                  declarationJSON("Acme Widgets"),
	}
}

// writeTree writes files under a new temporary checkout and answers its root.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		writeFile(t, root, rel, content)
	}
	return root
}

// writeFile writes one checkout-relative file.
func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// fixtureInputs answers the given linked modules, all on distro/linux unless
// a test names other flavors, and the fixture's one Go file.
func fixtureInputs(links ...flavorLinks) inputs {
	if len(links) == 0 {
		links = []flavorLinks{{Flavor: "distro/linux", Modules: []string{widget}}}
	}
	return inputs{
		linked:  func(string) ([]flavorLinks, error) { return links, nil },
		goFiles: func(string) ([]string, error) { return []string{webFile}, nil },
	}
}

// generate runs Update over root and fails the test on any refusal.
func generate(t *testing.T, root string, in inputs) UpdateReport {
	t.Helper()
	report, refusals, err := Update(root, in)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(refusals) != 0 {
		t.Fatalf("update refused the fixture: %+v", refusals)
	}
	return report
}

// check runs Check over root.
func check(t *testing.T, root string, in inputs) Report {
	t.Helper()
	report, err := Check(root, in)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	return report
}

// requireRefusal fails unless report holds a refusal of work whose problem
// contains problem, and whose rendering ends with the fix command alone on
// its last line.
func requireRefusal(t *testing.T, report Report, work, problem string) {
	t.Helper()
	for _, refusal := range report.Refusals {
		if refusal.Work != work {
			continue
		}
		if !strings.Contains(refusal.Problem, problem) {
			continue
		}
		if refusal.Fix != updateCommand {
			t.Errorf("refusal of %s ends with %q, want %q", work, refusal.Fix, updateCommand)
		}
		rendered := Report{Refusals: []Refusal{refusal}}.Text()
		lines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
		if strings.TrimSpace(lines[len(lines)-1]) != updateCommand {
			t.Errorf("refusal of %s renders its last line as %q, want the command alone:\n%s", work, lines[len(lines)-1], rendered)
		}
		return
	}
	t.Fatalf("no refusal of %s mentioning %q; got:\n%s", work, problem, report.Text())
}
