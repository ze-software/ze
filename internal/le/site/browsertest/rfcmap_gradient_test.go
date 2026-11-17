// Design: website/AI.md -- connected RFC cards share the mandatory result bar's colors and proportions.
package browsertest

import (
	"testing"

	"github.com/chromedp/chromedp"
)

func TestRFCMapNodeGradientsMatchRecordedProof(t *testing.T) {
	base, _, responses := rfcMapBrowserSite(t)
	ctx := newRFCBrowser(t)
	for _, mode := range []int32{0, 1, 2, 3} {
		responses.Figure.Store(mode)
		rfcMapBrowserOpen(ctx, t, base+"/quality/rfc-map/?rfc=rfc4271")
		rfcMapBrowserWait(ctx, t, `document.querySelector('.rfc-map-proof-value') !== null && !document.querySelector('.rfc-map-node-selected').textContent.includes('Loading')`)
		problem, err := chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
            const card = document.querySelector('.rfc-map-node-selected');
            const gradient = card.style.getPropertyValue('--rfc-map-node-gradient');
            const rectangles = Array.from(document.querySelectorAll('.rfc-map-proof-card svg rect'));
            if (!rectangles.length) return gradient ? 'Unknown or empty evidence has a completion gradient' : '';
            if (!getComputedStyle(card).backgroundImage.includes('to right')) return 'The card has no visible left-to-right gradient';
            const stops = rectangles.map(rect => {
                const key = rect.getAttribute('class').replace('rfc-map-proof-', '');
                const start = Number(rect.getAttribute('x'));
                const end = start + Number(rect.getAttribute('width'));
                const token = '--rfc-map-proof-' + key;
                const color = getComputedStyle(card).getPropertyValue(token).trim();
                const probe = document.createElement('span');
                probe.style.color = color;
                card.append(probe);
                const matches = getComputedStyle(probe).color === getComputedStyle(rect).fill;
                probe.remove();
                if (!matches) throw new Error('The card and bar colors differ for ' + key);
                return 'var(' + token + ') ' + start + '% ' + end + '%';
            });
            const expected = 'linear-gradient(to right, ' + stops.join(', ') + ')';
            return gradient === expected ? '' : 'Card gradient differs from the result bar: ' + gradient + ' expected ' + expected;
        })()`))
		if err != nil {
			t.Fatal(err)
		}
		if problem != "" {
			t.Fatalf("figure mode %d: %s", mode, problem)
		}
	}
}
