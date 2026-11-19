// Design: docs/contributing/gh-pages.md -- rendered proof uses the canonical publication.
package browsertest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestPublishedBFDDetailCardContrast checks the actual generated BFD page with
// the real stylesheet in both themes and at desktop and phone widths. Every
// visible text node in every metadata card must have readable contrast.
func TestPublishedBFDDetailCardContrast(t *testing.T) {
	base, ctx := markupBrowser(t)
	for _, width := range []int64{1440, 390} {
		for _, theme := range []string{"light", "dark"} {
			t.Run(strconv.FormatInt(width, 10)+"/"+theme, func(t *testing.T) {
				if _, err := chromedp.Run(ctx, chromedp.EmulateViewport(width, 900)); err != nil {
					t.Fatal(err)
				}
				if _, err := chromedp.Run(ctx, chromedp.Navigate(base+"/reference/plugins/bfd/")); err != nil {
					t.Fatal(err)
				}
				if _, err := chromedp.Run(ctx, chromedp.Evaluate[bool](`document.documentElement.setAttribute('data-theme', `+strconv.Quote(theme)+`); true`)); err != nil {
					t.Fatal(err)
				}
				if _, err := chromedp.Run(ctx, chromedp.Poll[bool](`document.fonts.status === 'loaded' && document.querySelectorAll('.plugin-detail-panel').length >= 6`)); err != nil {
					t.Fatal(err)
				}
				issues, err := chromedp.Run(ctx, chromedp.Evaluate[[]string](pluginCardContrast))
				if err != nil {
					t.Fatal(err)
				}
				if len(issues) != 0 {
					t.Fatal(strings.Join(issues, "; "))
				}
			})
		}
	}
}

// TestPublishedGuideTablesAreTables reads both original defective guides in
// Chrome. Headers and data cells must exist, and no pipe-delimited row may
// survive as a prose paragraph after the complete docs publishing pipeline.
func TestPublishedGuideTablesAreTables(t *testing.T) {
	base, ctx := markupBrowser(t)
	for _, guide := range []string{"firewall", "command-reference"} {
		t.Run(guide, func(t *testing.T) {
			if _, err := chromedp.Run(ctx, chromedp.Navigate(base+"/guides/"+guide+"/")); err != nil {
				t.Fatal(err)
			}
			issues, err := chromedp.Run(ctx, chromedp.Evaluate[[]string](`(() => {
                const body = document.querySelector('.md-content');
                if (!body) return ['guide body is missing'];
                const issues = [];
                if (!body.querySelector('table thead th')) issues.push('table headers are missing');
                if (!body.querySelector('table tbody td')) issues.push('table data cells are missing');
                for (const paragraph of body.querySelectorAll('p')) {
                    const text = paragraph.textContent.trim();
                    if (text.startsWith('|') && text.endsWith('|')) issues.push('raw table row: ' + text.slice(0, 200));
                }
                return issues;
            })()`))
			if err != nil {
				t.Fatal(err)
			}
			if len(issues) != 0 {
				t.Fatal(strings.Join(issues, "; "))
			}
		})
	}
}

// markupBrowser serves only the canonical artifact; the main thread regenerates
// that artifact before these tests. The test owns and closes its HTTP listener.
func markupBrowser(t *testing.T) (string, context.Context) {
	t.Helper()
	repository, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.FileServer(http.Dir(recordedBrowserSite(repository))))
	t.Cleanup(server.Close)
	options := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(findChrome(t)))
	allocator, cancelAllocator := chromedp.NewExecAllocator(t.Context(), options...)
	t.Cleanup(cancelAllocator)
	browser, cancelBrowser := chromedp.NewContext(allocator)
	t.Cleanup(cancelBrowser)
	ctx, cancel := context.WithTimeout(browser, 90*time.Second)
	t.Cleanup(cancel)
	return server.URL, ctx
}

const pluginCardContrast = `(() => {
    const issues = [];
    const rgb = value => (value.match(/[\d.]+/g) || []).map(Number);
    const luminance = color => color.slice(0, 3).map(value => {
        value /= 255;
        return value <= 0.04045 ? value / 12.92 : Math.pow((value + 0.055) / 1.055, 2.4);
    }).reduce((sum, value, index) => sum + value * [0.2126, 0.7152, 0.0722][index], 0);
    for (const panel of document.querySelectorAll('.plugin-detail-panel')) {
        const background = rgb(getComputedStyle(panel).backgroundColor);
        if (background.length < 3 || (background.length === 4 && background[3] !== 1)) {
            issues.push('metadata panel has no opaque theme surface');
            continue;
        }
        const walker = document.createTreeWalker(panel, NodeFilter.SHOW_TEXT);
        while (walker.nextNode()) {
            const text = walker.currentNode.textContent.trim();
            if (!text) continue;
            const element = walker.currentNode.parentElement;
            const style = getComputedStyle(element);
            const foreground = rgb(style.color);
            const front = luminance(foreground), back = luminance(background);
            const ratio = (Math.max(front, back) + 0.05) / (Math.min(front, back) + 0.05);
            if (ratio < 4.5) issues.push(text.slice(0, 70) + ' contrast=' + ratio.toFixed(2));
        }
    }
    if (document.documentElement.scrollWidth > innerWidth + 1) issues.push('plugin page overflows');
    return issues;
})()`
