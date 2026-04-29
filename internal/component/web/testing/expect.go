// Design: docs/architecture/testing/ci-format.md -- web browser expectation checking
// Related: parser.go -- .wb file parsing

package webtesting

import (
	"fmt"
	"strings"
)

// checkExpectation validates a single expectation against the current browser state.
func checkExpectation(b *Browser, e *WBExpectation) error {
	switch e.Kind {
	case "element":
		return checkElement(b, e)
	case "breadcrumb":
		return checkBreadcrumb(b, e)
	case "url":
		return checkURL(b, e)
	case "title":
		return checkTitle(b, e)
	}
	return fmt.Errorf("unknown expectation kind %q", e.Kind)
}

func checkElement(b *Browser, e *WBExpectation) error {
	snap, err := b.Snapshot()
	if err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}

	if id, ok := e.Values["id"]; ok {
		html, htmlErr := b.GetHTML()
		if htmlErr != nil {
			return fmt.Errorf("html: %w", htmlErr)
		}
		if !strings.Contains(html, "id=\""+id+"\"") && !strings.Contains(html, "id='"+id+"'") {
			return fmt.Errorf("expected element with id %q not found in DOM; snapshot:\n%s", id, snap)
		}
	}

	if id, ok := e.Values["not-id"]; ok {
		html, htmlErr := b.GetHTML()
		if htmlErr != nil {
			return fmt.Errorf("html: %w", htmlErr)
		}
		if strings.Contains(html, "id=\""+id+"\"") || strings.Contains(html, "id='"+id+"'") {
			return fmt.Errorf("unexpected element with id %q found", id)
		}
	}

	if text, ok := e.Values["text"]; ok {
		fullSnap, textErr := b.FullSnapshot()
		if textErr != nil {
			return fmt.Errorf("full snapshot: %w", textErr)
		}
		if !strings.Contains(strings.ToLower(fullSnap), strings.ToLower(text)) {
			return fmt.Errorf("expected element with text %q not found in snapshot:\n%s", text, fullSnap)
		}
	}

	if text, ok := e.Values["not-text"]; ok {
		fullSnap, textErr := b.FullSnapshot()
		if textErr != nil {
			return fmt.Errorf("full snapshot: %w", textErr)
		}
		if strings.Contains(strings.ToLower(fullSnap), strings.ToLower(text)) {
			return fmt.Errorf("unexpected element with text %q found in snapshot:\n%s", text, fullSnap)
		}
	}

	return nil
}

func checkBreadcrumb(b *Browser, e *WBExpectation) error {
	snap, err := b.Snapshot()
	if err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}

	snapLower := strings.ToLower(snap)

	if csv, ok := e.Values["contains"]; ok {
		for seg := range strings.SplitSeq(csv, ",") {
			seg = strings.TrimSpace(seg)
			if !strings.Contains(snapLower, strings.ToLower(seg)) {
				return fmt.Errorf("breadcrumb missing segment %q in snapshot:\n%s", seg, snap)
			}
		}
	}

	if csv, ok := e.Values["not-contains"]; ok {
		for seg := range strings.SplitSeq(csv, ",") {
			seg = strings.TrimSpace(seg)
			if strings.Contains(snapLower, "\""+strings.ToLower(seg)+"\"") ||
				strings.Contains(snapLower, " "+strings.ToLower(seg)+" ") {
				return fmt.Errorf("breadcrumb has unexpected segment %q", seg)
			}
		}
	}

	return nil
}

func checkURL(b *Browser, e *WBExpectation) error {
	snap, err := b.Snapshot()
	if err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}

	if sub, ok := e.Values["contains"]; ok {
		if !strings.Contains(snap, sub) {
			return fmt.Errorf("URL does not contain %q", sub)
		}
	}

	return nil
}

func checkTitle(b *Browser, e *WBExpectation) error {
	text, err := b.FullSnapshot()
	if err != nil {
		return fmt.Errorf("full snapshot: %w", err)
	}

	if sub, ok := e.Values["contains"]; ok {
		if !strings.Contains(strings.ToLower(text), strings.ToLower(sub)) {
			return fmt.Errorf("page text does not contain %q", sub)
		}
	}

	return nil
}
