// Design: docs/architecture/web-interface.md -- source-derived browser capability accounting.
// Related: cli_controls_test.go -- editor help declarations reach the HTTP answer.
// Related: ../cli/view_browser_test.go -- every registered view declares browser controls.
// Related: ../../../cmd/ze/hub/surface_parity_test.go -- offered and served command coverage.

package web

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
)

// TestWebCapabilityLocalOperatorsReachPrimaryCLI checks offered-control
// evidence only: the primary CLI page must offer a native caller-side control
// for every operator the command catalog declares caller-local. It does not
// prove download execution, output completeness, authorization or cancellation;
// those require browser journeys over actual command results.
func TestWebCapabilityLocalOperatorsReachPrimaryCLI(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)
	response := httptest.NewRecorder()
	HandleCLIPageHTTP(renderer, false, nil)(response, authedRequest(t, http.MethodGet, "/cli", nil))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	page := parseFinderHTML(t, response.Body.String())
	finderElement(t, page, "id", "terminal-input")
	finderElement(t, page, "id", "cli-output")

	operators := command.PipeOperatorCatalog()
	controls := finderAttributeValues(page, "data-cli-local-operator")
	require.NotEmpty(t, webCapabilityLocalOperatorNames(operators), "the producer must contain caller-local capabilities")
	assert.Empty(t, webCapabilityLocalOperatorGaps(operators, controls), "browser controls must account for the live operator catalog")
	for _, operator := range operators {
		if !operator.LocalOnly {
			continue
		}
		t.Run(operator.Name, func(t *testing.T) {
			control := finderElement(t, page, "data-cli-local-operator", operator.Name)
			assert.Equal(t, "button", control.Data, "a native control must remain keyboard accessible")
			assert.Equal(t, "button", finderAttr(control, "type"), "caller-local output must not submit a daemon form")
			assert.Equal(t, operator.Description, finderAttr(control, "title"), "the capability description belongs to its command owner")
			assert.NotEmpty(t, finderText(control), "the caller needs a visible action")
		})
	}
}

// TestWebCapabilityLocalOperatorGuard checks the same verdict function with a
// complete catalog-derived control population, then removes coverage, adds a
// future producer capability, duplicates a control and invents a control. No
// command names or production registration exceptions are maintained here.
func TestWebCapabilityLocalOperatorGuard(t *testing.T) {
	operators := command.PipeOperatorCatalog()
	controls := webCapabilityLocalOperatorNames(operators)
	require.NotEmpty(t, controls)
	assert.Empty(t, webCapabilityLocalOperatorGaps(operators, controls))

	t.Run("missing-control", func(t *testing.T) {
		assert.Equal(t, []string{controls[0] + ": caller-local control missing"},
			webCapabilityLocalOperatorGaps(operators, controls[1:]))
	})
	t.Run("new-producer-capability", func(t *testing.T) {
		expanded := append(slices.Clone(operators), command.PipeOperator{
			Name: "synthetic-caller-local-result", LocalOnly: true,
		})
		assert.Equal(t, []string{"synthetic-caller-local-result: caller-local control missing"},
			webCapabilityLocalOperatorGaps(expanded, controls))
	})
	t.Run("duplicate-control", func(t *testing.T) {
		doubled := append(slices.Clone(controls), controls[0])
		assert.Equal(t, []string{controls[0] + ": caller-local control duplicated"},
			webCapabilityLocalOperatorGaps(operators, doubled))
	})
	t.Run("undeclared-control", func(t *testing.T) {
		expanded := append(slices.Clone(controls), "synthetic-undeclared-control")
		assert.Equal(t, []string{"synthetic-undeclared-control: no caller-local operator declaration"},
			webCapabilityLocalOperatorGaps(operators, expanded))
	})
}

func webCapabilityLocalOperatorNames(operators []command.PipeOperator) []string {
	var names []string
	for _, operator := range operators {
		if operator.LocalOnly {
			names = append(names, operator.Name)
		}
	}
	return names
}

func webCapabilityLocalOperatorGaps(operators []command.PipeOperator, controls []string) []string {
	counts := make(map[string]int, len(controls))
	for _, control := range controls {
		counts[control]++
	}
	declared := make(map[string]bool)
	var gaps []string
	for _, name := range webCapabilityLocalOperatorNames(operators) {
		declared[name] = true
		if counts[name] == 0 {
			gaps = append(gaps, name+": caller-local control missing")
			continue
		}
		if counts[name] > 1 {
			gaps = append(gaps, name+": caller-local control duplicated")
		}
	}
	for _, control := range controls {
		if !declared[control] {
			gaps = append(gaps, control+": no caller-local operator declaration")
		}
	}
	return gaps
}
