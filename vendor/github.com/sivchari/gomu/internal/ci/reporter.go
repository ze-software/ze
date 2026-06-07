package ci

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sivchari/gomu/internal/report"
)

const noQualityGateMessage = "No quality gate configured"

// Reporter generates CI-specific reports.
type Reporter struct {
	outputDir    string
	outputFormat string
}

// NewReporter creates a new Reporter.
func NewReporter(outputDir, outputFormat string) *Reporter {
	return &Reporter{
		outputDir:    outputDir,
		outputFormat: outputFormat,
	}
}

// Report represents a CI-specific mutation testing report.
type Report struct {
	Summary            *report.Summary     `json:"summary"`
	QualityGate        *QualityGateResult  `json:"qualityGate"`
	ChangedFiles       []ChangedFileReport `json:"changedFiles"`
	Recommendations    []string            `json:"recommendations"`
	MutationScore      float64             `json:"mutationScore"`
	TotalMutants       int                 `json:"totalMutants"`
	Killed             int                 `json:"killed"`
	Survived           int                 `json:"survived"`
	KilledPercentage   float64             `json:"killedPercentage"`
	SurvivedPercentage float64             `json:"survivedPercentage"`
	Duration           string              `json:"duration"`
	ProcessedFiles     int                 `json:"processedFiles"`
	TotalFiles         int                 `json:"totalFiles"`
	Timestamp          time.Time           `json:"timestamp"`
	Metadata           Metadata            `json:"metadata"`
}

// ChangedFileReport represents a report for a changed file.
type ChangedFileReport struct {
	Path   string  `json:"path"`
	Score  float64 `json:"score"`
	Status string  `json:"status"`
}

// Metadata contains CI-specific metadata.
type Metadata struct {
	Provider   string `json:"provider"`
	Repository string `json:"repository"`
	Branch     string `json:"branch"`
	PRNumber   int    `json:"prNumber,omitempty"`
	Actor      string `json:"actor"`
	RunID      string `json:"runId"`
	EventName  string `json:"eventName"`
}

// Generate generates and outputs CI-specific reports.
func (r *Reporter) Generate(summary *report.Summary, qualityGate *QualityGateEvaluator) error {
	// Evaluate quality gate
	var qualityResult *QualityGateResult
	if qualityGate != nil {
		qualityResult = qualityGate.Evaluate(summary)
	}

	switch r.outputFormat {
	case "json":
		return r.generateJSONReport(summary, qualityResult)
	case "html":
		return r.generateHTMLReport(summary, qualityResult)
	case "console":
		r.generateConsoleReport(summary, qualityResult)

		return nil
	default:
		return r.generateJSONReport(summary, qualityResult)
	}
}

// generateJSONReport generates a JSON report.
func (r *Reporter) generateJSONReport(summary *report.Summary, qualityResult *QualityGateResult) error {
	data := map[string]any{
		"totalMutants":  summary.TotalMutants,
		"killedMutants": summary.KilledMutants,
		"files":         summary.Files,
		"timestamp":     time.Now(),
	}

	// Add quality gate results if available
	if qualityResult != nil {
		data["mutationScore"] = qualityResult.MutationScore
		data["qualityGatePassed"] = qualityResult.Pass
		data["qualityGateReason"] = qualityResult.Reason
	} else {
		// Calculate mutation score from summary
		score := 0.0
		if summary.TotalMutants > 0 {
			score = float64(summary.KilledMutants) / float64(summary.TotalMutants) * 100
		}

		data["mutationScore"] = score
		data["qualityGatePassed"] = "N/A"
		data["qualityGateReason"] = noQualityGateMessage
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	filename := filepath.Join(r.outputDir, "mutation-report.json")

	if err := os.WriteFile(filename, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write JSON report: %w", err)
	}

	return nil
}

// generateHTMLReport generates an HTML report.
func (r *Reporter) generateHTMLReport(summary *report.Summary, qualityResult *QualityGateResult) error {
	// Build file details HTML
	fileDetailsHTML := ""

	for _, file := range summary.Files {
		score := 0.0
		if file.TotalMutants > 0 {
			score = float64(file.KilledMutants) / float64(file.TotalMutants) * 100
		}

		fileDetailsHTML += fmt.Sprintf(`
        <tr>
            <td>%s</td>
            <td>%d</td>
            <td>%d</td>
            <td>%.1f%%</td>
        </tr>`, file.FilePath, file.TotalMutants, file.KilledMutants, score)
	}

	// Handle nil quality result
	mutationScore := 0.0

	var qualityGateStatus string

	if qualityResult != nil {
		mutationScore = qualityResult.MutationScore
		if qualityResult.Pass {
			qualityGateStatus = "PASSED"
		} else {
			qualityGateStatus = fmt.Sprintf("FAILED - %s", qualityResult.Reason)
		}
	} else {
		// Calculate score from summary
		if summary.TotalMutants > 0 {
			mutationScore = float64(summary.KilledMutants) / float64(summary.TotalMutants) * 100
		}

		qualityGateStatus = noQualityGateMessage
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Mutation Testing Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .header { background: #f5f5f5; padding: 20px; border-radius: 5px; }
        .score { font-size: 24px; font-weight: bold; color: %s; }
        table { border-collapse: collapse; width: 100%%; margin-top: 20px; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Mutation Testing Report</h1>
        <div class="score">Overall Score: %.1f%%</div>
        <p>Quality Gate: %s</p>
    </div>
    <h2>Summary</h2>
    <p>Total Mutants: %d</p>
    <p>Killed: %d</p>
    <p>Files Analyzed: %d</p>
    <h2>File Details</h2>
    <table>
        <tr>
            <th>File</th>
            <th>Total Mutants</th>
            <th>Killed</th>
            <th>Score</th>
        </tr>%s
    </table>
</body>
</html>`,
		r.getScoreColor(mutationScore),
		mutationScore,
		qualityGateStatus,
		summary.TotalMutants,
		summary.KilledMutants,
		len(summary.Files),
		fileDetailsHTML,
	)

	filename := filepath.Join(r.outputDir, "mutation-report.html")

	if err := os.WriteFile(filename, []byte(html), 0644); err != nil {
		return fmt.Errorf("failed to write HTML report: %w", err)
	}

	return nil
}

// generateConsoleReport prints a console report.
func (r *Reporter) generateConsoleReport(summary *report.Summary, qualityResult *QualityGateResult) {
	fmt.Printf("Mutation Score: %.1f%%\n", qualityResult.MutationScore)
	fmt.Printf("Quality Gate: %s\n", map[bool]string{true: "PASSED", false: "FAILED"}[qualityResult.Pass])
	fmt.Printf("Total Mutants: %d\n", summary.TotalMutants)
	fmt.Printf("Killed: %d\n", summary.KilledMutants)
}

// getScoreColor returns color based on score.
func (r *Reporter) getScoreColor(score float64) string {
	if score >= 80 {
		return "#28a745"
	}

	if score >= 60 {
		return "#ffc107"
	}

	return "#dc3545"
}
