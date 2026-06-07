// Package report provides report generation functionality.
package report

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/sivchari/gomu/internal/mutation"
)

// Generator handles report generation.
type Generator struct {
	outputFormat string
}

// Summary contains the complete results of a mutation testing run.
type Summary struct {
	TotalFiles     int                    `json:"totalFiles"`
	ProcessedFiles int                    `json:"processedFiles"`
	TotalMutants   int                    `json:"totalMutants"`
	KilledMutants  int                    `json:"killedMutants"`
	Results        []mutation.Result      `json:"results"`
	Files          map[string]*FileReport `json:"files"`
	Duration       time.Duration          `json:"duration"`
	Statistics     Statistics             `json:"statistics"`
	Timestamp      time.Time              `json:"timestamp"`
	Version        string                 `json:"version,omitempty"`
}

// FileReport represents a report for a single file.
type FileReport struct {
	FilePath      string  `json:"filePath"`
	TotalMutants  int     `json:"totalMutants"`
	KilledMutants int     `json:"killedMutants"`
	MutationScore float64 `json:"mutationScore"`
}

// Statistics contains aggregated mutation testing statistics.
type Statistics struct {
	Killed        int                       `json:"killed"`
	Survived      int                       `json:"survived"`
	TimedOut      int                       `json:"timedOut"`
	Errors        int                       `json:"errors"`
	NotViable     int                       `json:"notViable"`
	Score         float64                   `json:"mutationScore"`
	Coverage      float64                   `json:"lineCoverage,omitempty"`
	MutationTypes map[string]TypeStatistics `json:"mutationTypes,omitempty"`
}

// TypeStatistics contains statistics for a specific mutation type.
type TypeStatistics struct {
	Total    int `json:"total"`
	Killed   int `json:"killed"`
	Survived int `json:"survived"`
}

// New creates a new report generator with the specified output format.
// Supported formats: "json", "html", "text", "console".
func New(outputFormat string) (*Generator, error) {
	if outputFormat == "" {
		outputFormat = "console"
	}

	return &Generator{
		outputFormat: outputFormat,
	}, nil
}

const gomuVersion = "0.1.0"

// Generate creates and outputs the mutation testing report.
func (g *Generator) Generate(summary *Summary) error {
	// Calculate statistics
	summary.Statistics = g.calculateStatistics(summary.Results)
	summary.Timestamp = time.Now()
	summary.Version = gomuVersion

	// Generate only the requested format
	switch g.outputFormat {
	case "json":
		return g.generateJSON(summary)
	case "html":
		return g.generateHTML(summary)
	case "text":
		return g.generateText(summary)
	case "console":
		return g.generateConsole(summary)
	default:
		// Default to console for unknown formats
		return g.generateConsole(summary)
	}
}

func (g *Generator) calculateStatistics(results []mutation.Result) Statistics {
	stats := Statistics{
		MutationTypes: make(map[string]TypeStatistics),
	}

	for _, result := range results {
		switch result.Status {
		case mutation.StatusKilled:
			stats.Killed++
		case mutation.StatusSurvived:
			stats.Survived++
		case mutation.StatusTimedOut:
			stats.TimedOut++
		case mutation.StatusError:
			stats.Errors++
		case mutation.StatusNotViable:
			stats.NotViable++
		}

		// Track mutation type statistics
		mutationType := result.Mutant.Type
		if mutationType == "" {
			mutationType = "unknown"
		}

		typeStats := stats.MutationTypes[mutationType]
		typeStats.Total++

		switch result.Status {
		case mutation.StatusKilled:
			typeStats.Killed++
		case mutation.StatusSurvived:
			typeStats.Survived++
		}

		stats.MutationTypes[mutationType] = typeStats
	}

	// Calculate mutation score excluding NOT_VIABLE mutants
	validMutants := len(results) - stats.NotViable
	if validMutants > 0 {
		stats.Score = float64(stats.Killed) / float64(validMutants) * 100
	}

	return stats
}

func (g *Generator) generateJSON(summary *Summary) error {
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal summary: %w", err)
	}

	outputFile := "mutation-report.json"
	if err := os.WriteFile(outputFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Printf("Report written to %s\n", outputFile)

	return nil
}

func (g *Generator) generateText(summary *Summary) error {
	output := g.formatTextReport(summary)

	outputFile := "mutation-report.txt"
	if err := os.WriteFile(outputFile, []byte(output), 0600); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Printf("Report written to %s\n", outputFile)

	return nil
}

func (g *Generator) formatTextReport(summary *Summary) string {
	stats := summary.Statistics

	report := fmt.Sprintf(`
Mutation Testing Report
=======================

Summary:
  Files processed: %d/%d
  Total mutants:   %d
  Duration:        %v

Results:
  Killed:     %d (%.1f%%)
  Survived:   %d (%.1f%%)
  Timed out:  %d (%.1f%%)
  Errors:     %d (%.1f%%)
  Not viable: %d (%.1f%%)

Mutation Score: %.1f%%

`,
		summary.ProcessedFiles, summary.TotalFiles,
		summary.TotalMutants,
		summary.Duration,
		stats.Killed, percentage(stats.Killed, summary.TotalMutants),
		stats.Survived, percentage(stats.Survived, summary.TotalMutants),
		stats.TimedOut, percentage(stats.TimedOut, summary.TotalMutants),
		stats.Errors, percentage(stats.Errors, summary.TotalMutants),
		stats.NotViable, percentage(stats.NotViable, summary.TotalMutants),
		stats.Score,
	)

	// Add details for survived mutants
	if stats.Survived > 0 {
		report += "\nSurvived Mutants:\n"
		report += "=================\n"

		for _, result := range summary.Results {
			if result.Status == mutation.StatusSurvived {
				report += fmt.Sprintf("  %s:%d:%d - %s (%s -> %s)\n",
					result.Mutant.FilePath,
					result.Mutant.Line,
					result.Mutant.Column,
					result.Mutant.Description,
					result.Mutant.Original,
					result.Mutant.Mutated,
				)
			}
		}
	}

	return report
}

func (g *Generator) generateHTML(summary *Summary) error {
	funcMap := template.FuncMap{
		"percentage": percentage,
	}

	tmpl, err := template.New("html_report").Funcs(funcMap).Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse HTML template: %w", err)
	}

	var output strings.Builder
	if err := tmpl.Execute(&output, summary); err != nil {
		return fmt.Errorf("failed to execute HTML template: %w", err)
	}

	outputFile := "mutation-report.html"
	if err := os.WriteFile(outputFile, []byte(output.String()), 0600); err != nil {
		return fmt.Errorf("failed to write HTML output file: %w", err)
	}

	fmt.Printf("Report written to %s\n", outputFile)

	return nil
}

// htmlTemplate is the template for HTML reports.
const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Gomu Mutation Testing Report</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            margin: 0;
            padding: 20px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
        }
        .container {
            max-width: 1400px;
            margin: 0 auto;
            background: white;
            border-radius: 12px;
            box-shadow: 0 10px 30px rgba(0,0,0,0.2);
            padding: 0;
            overflow: hidden;
        }
        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 30px;
            text-align: center;
            position: relative;
        }
        .header::before {
            content: '';
            position: absolute;
            top: -50%;
            left: -50%;
            width: 200%;
            height: 200%;
            background: repeating-linear-gradient(
                45deg,
                transparent,
                transparent 10px,
                rgba(255,255,255,0.1) 10px,
                rgba(255,255,255,0.1) 20px
            );
            animation: move 20s linear infinite;
        }
        @keyframes move {
            0% { transform: translate(-50%, -50%) rotate(0deg); }
            100% { transform: translate(-50%, -50%) rotate(360deg); }
        }
        h1 {
            margin: 0;
            font-size: 2.5em;
            font-weight: 700;
            text-shadow: 2px 2px 4px rgba(0,0,0,0.3);
            z-index: 1;
            position: relative;
        }
        .subtitle {
            margin-top: 10px;
            font-size: 1.1em;
            opacity: 0.9;
            z-index: 1;
            position: relative;
        }
        .content {
            padding: 30px;
        }
        .summary-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
            gap: 20px;
            margin-bottom: 30px;
        }
        .summary-card {
            background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%);
            padding: 25px;
            border-radius: 8px;
            border-left: 4px solid #3498db;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
            transition: transform 0.3s ease;
        }
        .summary-card:hover {
            transform: translateY(-2px);
            box-shadow: 0 8px 15px rgba(0,0,0,0.15);
        }
        .summary-card h3 {
            margin: 0 0 15px 0;
            color: #2c3e50;
            font-size: 16px;
            font-weight: 600;
        }
        .summary-card .value {
            font-size: 28px;
            font-weight: bold;
            color: #2c3e50;
        }
        .mutation-score {
            text-align: center;
            margin: 30px 0;
            padding: 30px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            border-radius: 12px;
            box-shadow: 0 8px 20px rgba(102, 126, 234, 0.3);
        }
        .mutation-score h2 {
            margin: 0 0 15px 0;
            font-size: 20px;
            font-weight: 600;
        }
        .score-value {
            font-size: 64px;
            font-weight: bold;
            margin: 0;
            text-shadow: 2px 2px 4px rgba(0,0,0,0.3);
        }
        .score-indicator {
            display: inline-block;
            width: 20px;
            height: 20px;
            border-radius: 50%;
            margin-left: 10px;
            vertical-align: middle;
        }
        .score-excellent { background: #27ae60; }
        .score-good { background: #f39c12; }
        .score-poor { background: #e74c3c; }
        .statistics {
            margin-bottom: 30px;
        }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
            gap: 20px;
        }
        .stat-item {
            background: white;
            padding: 20px;
            border-radius: 8px;
            border: 1px solid #e0e0e0;
            text-align: center;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
            transition: transform 0.3s ease;
        }
        .stat-item:hover {
            transform: translateY(-2px);
            box-shadow: 0 8px 15px rgba(0,0,0,0.15);
        }
        .stat-item.killed { border-left: 4px solid #27ae60; }
        .stat-item.survived { border-left: 4px solid #e74c3c; }
        .stat-item.timed-out { border-left: 4px solid #f39c12; }
        .stat-item.error { border-left: 4px solid #e67e22; }
        .stat-item.not-viable { border-left: 4px solid #8e44ad; }
        .stat-number {
            font-size: 32px;
            font-weight: bold;
            margin-bottom: 8px;
        }
        .stat-label {
            font-size: 14px;
            color: #7f8c8d;
            text-transform: uppercase;
            letter-spacing: 0.8px;
            font-weight: 600;
        }
        .mutation-breakdown {
            margin-bottom: 30px;
        }
        .mutation-breakdown h2 {
            color: #2c3e50;
            border-bottom: 2px solid #3498db;
            padding-bottom: 10px;
            margin-bottom: 20px;
        }
        .mutation-types {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 20px;
            margin-bottom: 30px;
        }
        .mutation-type-card {
            background: #f8f9fa;
            padding: 20px;
            border-radius: 8px;
            border-left: 4px solid #9b59b6;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
        }
        .mutation-type-card h3 {
            margin: 0 0 10px 0;
            color: #2c3e50;
            font-size: 18px;
            text-transform: capitalize;
        }
        .type-stats {
            display: flex;
            justify-content: space-between;
            margin-bottom: 10px;
        }
        .type-stat {
            text-align: center;
        }
        .type-stat .number {
            font-size: 20px;
            font-weight: bold;
            color: #2c3e50;
        }
        .type-stat .label {
            font-size: 12px;
            color: #7f8c8d;
            text-transform: uppercase;
        }
        .progress-bar {
            background: #e0e0e0;
            border-radius: 10px;
            height: 8px;
            overflow: hidden;
            margin-top: 10px;
        }
        .progress-fill {
            height: 100%;
            background: linear-gradient(90deg, #27ae60 0%, #2ecc71 100%);
            transition: width 0.3s ease;
        }
        .file-reports {
            margin-bottom: 30px;
        }
        .file-reports h2 {
            color: #2c3e50;
            border-bottom: 2px solid #3498db;
            padding-bottom: 10px;
            margin-bottom: 20px;
        }
        .file-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(400px, 1fr));
            gap: 20px;
        }
        .file-card {
            background: white;
            border-radius: 8px;
            border: 1px solid #e0e0e0;
            overflow: hidden;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
            transition: transform 0.3s ease;
        }
        .file-card:hover {
            transform: translateY(-2px);
            box-shadow: 0 8px 15px rgba(0,0,0,0.15);
        }
        .file-header {
            background: #f8f9fa;
            padding: 15px;
            border-bottom: 1px solid #e0e0e0;
        }
        .file-path {
            font-family: 'Monaco', 'Consolas', monospace;
            font-size: 14px;
            color: #2c3e50;
            font-weight: 600;
            word-break: break-all;
        }
        .file-stats {
            padding: 15px;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .file-score {
            font-size: 24px;
            font-weight: bold;
            color: #2c3e50;
        }
        .file-mutants {
            color: #7f8c8d;
            font-size: 14px;
        }
        .detailed-analysis {
            margin-top: 30px;
        }
        .detailed-analysis h2 {
            color: #2c3e50;
            border-bottom: 2px solid #3498db;
            padding-bottom: 10px;
            margin-bottom: 20px;
        }
        .mutant-badges {
            display: flex;
            gap: 8px;
            align-items: center;
        }
        .mutant-status {
            padding: 4px 8px;
            border-radius: 4px;
            font-size: 12px;
            font-weight: 600;
            text-transform: uppercase;
        }
        .mutant-status.KILLED {
            background: #d4edda;
            color: #155724;
        }
        .mutant-status.SURVIVED {
            background: #f8d7da;
            color: #721c24;
        }
        .mutant-status.TIMED_OUT {
            background: #fff3cd;
            color: #856404;
        }
        .mutant-status.ERROR {
            background: #e2e3e5;
            color: #383d41;
        }
        .mutant-status.NOT_VIABLE {
            background: #e8d5f0;
            color: #5a2d6e;
        }
        .mutant-item.KILLED {
            border-left-color: #28a745;
        }
        .mutant-item.SURVIVED {
            border-left-color: #dc3545;
        }
        .mutant-item.TIMED_OUT {
            border-left-color: #ffc107;
        }
        .mutant-item.ERROR {
            border-left-color: #6c757d;
        }
        .mutant-item.NOT_VIABLE {
            border-left-color: #8e44ad;
        }
        .filters {
            margin-bottom: 20px;
            display: flex;
            gap: 10px;
            flex-wrap: wrap;
            align-items: center;
        }
        .filter-btn {
            padding: 8px 16px;
            border: 2px solid #3498db;
            background: white;
            color: #3498db;
            border-radius: 20px;
            cursor: pointer;
            transition: all 0.3s ease;
            font-size: 14px;
            font-weight: 600;
        }
        .filter-btn:hover,
        .filter-btn.active {
            background: #3498db;
            color: white;
        }
        .mutant-list {
            background: #fff5f5;
            border-radius: 8px;
            padding: 20px;
            margin-top: 15px;
        }
        .mutant-item {
            background: white;
            margin-bottom: 20px;
            padding: 20px;
            border-radius: 8px;
            border-left: 4px solid #e74c3c;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
            transition: transform 0.3s ease;
        }
        .mutant-item:hover {
            transform: translateY(-2px);
            box-shadow: 0 8px 15px rgba(0,0,0,0.15);
        }
        .mutant-item:last-child {
            margin-bottom: 0;
        }
        .mutant-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 15px;
        }
        .mutant-location {
            font-family: 'Monaco', 'Consolas', monospace;
            background: #f8f9fa;
            padding: 6px 12px;
            border-radius: 4px;
            font-size: 14px;
            color: #e74c3c;
            font-weight: 600;
        }
        .mutant-type {
            background: #9b59b6;
            color: white;
            padding: 4px 8px;
            border-radius: 4px;
            font-size: 12px;
            font-weight: 600;
            text-transform: uppercase;
        }
        .mutant-description {
            margin: 10px 0;
            color: #2c3e50;
            font-size: 16px;
        }
        .mutant-change {
            font-family: 'Monaco', 'Consolas', monospace;
            background: #f8f9fa;
            padding: 12px;
            border-radius: 6px;
            font-size: 14px;
            margin-top: 10px;
            border: 1px solid #e0e0e0;
        }
        .mutant-function {
            margin: 10px 0;
            color: #6c757d;
            font-size: 14px;
        }
        .mutant-function code {
            background: #f8f9fa;
            padding: 2px 6px;
            border-radius: 3px;
            font-family: 'Monaco', 'Consolas', monospace;
            color: #495057;
        }
        .mutant-context {
            margin-top: 15px;
            border-top: 1px solid #e0e0e0;
            padding-top: 15px;
        }
        .mutant-context h4 {
            margin: 0 0 10px 0;
            color: #495057;
            font-size: 14px;
        }
        .mutant-context pre {
            background: #f8f9fa;
            padding: 12px;
            border-radius: 6px;
            border: 1px solid #e0e0e0;
            overflow-x: auto;
            margin: 0;
        }
        .mutant-context code {
            font-family: 'Monaco', 'Consolas', monospace;
            font-size: 13px;
            color: #495057;
            line-height: 1.4;
        }
        .test-execution {
            margin-top: 15px;
            border-top: 1px solid #e0e0e0;
            padding-top: 15px;
        }
        .test-execution h4 {
            margin: 0 0 12px 0;
            color: #495057;
            font-size: 14px;
        }
        .test-summary {
            display: flex;
            gap: 15px;
            margin-bottom: 15px;
            flex-wrap: wrap;
        }
        .test-stat {
            background: #e9ecef;
            padding: 4px 8px;
            border-radius: 4px;
            font-size: 12px;
            font-weight: 600;
        }
        .test-stat.failed {
            background: #f8d7da;
            color: #721c24;
        }
        .test-details {
            background: #f8f9fa;
            border-radius: 6px;
            padding: 12px;
            border: 1px solid #e0e0e0;
        }
        .test-item {
            background: white;
            margin-bottom: 10px;
            padding: 10px;
            border-radius: 4px;
            border-left: 3px solid #6c757d;
        }
        .test-item:last-child {
            margin-bottom: 0;
        }
        .test-item.PASS {
            border-left-color: #28a745;
        }
        .test-item.FAIL {
            border-left-color: #dc3545;
        }
        .test-item.SKIP {
            border-left-color: #ffc107;
        }
        .test-name {
            font-weight: 600;
            font-size: 13px;
            color: #495057;
            margin-bottom: 5px;
        }
        .test-info {
            display: flex;
            gap: 10px;
            align-items: center;
            margin-bottom: 5px;
        }
        .test-package {
            font-size: 11px;
            color: #6c757d;
            background: #e9ecef;
            padding: 2px 6px;
            border-radius: 3px;
            font-family: 'Monaco', 'Consolas', monospace;
        }
        .test-status {
            font-size: 11px;
            font-weight: 600;
            padding: 2px 6px;
            border-radius: 3px;
            text-transform: uppercase;
        }
        .test-status.PASS {
            background: #d4edda;
            color: #155724;
        }
        .test-status.FAIL {
            background: #f8d7da;
            color: #721c24;
        }
        .test-status.SKIP {
            background: #fff3cd;
            color: #856404;
        }
        .test-duration {
            font-size: 11px;
            color: #6c757d;
            font-family: 'Monaco', 'Consolas', monospace;
        }
        .test-output {
            font-family: 'Monaco', 'Consolas', monospace;
            font-size: 11px;
            color: #6c757d;
            background: #f8f9fa;
            padding: 8px;
            border-radius: 3px;
            margin-top: 5px;
            white-space: pre-wrap;
            word-break: break-all;
        }
        .original { 
            color: #e74c3c; 
            font-weight: bold;
            background: #ffeaea;
            padding: 2px 4px;
            border-radius: 3px;
        }
        .mutated { 
            color: #27ae60; 
            font-weight: bold;
            background: #eafbea;
            padding: 2px 4px;
            border-radius: 3px;
        }
        .timestamp {
            text-align: center;
            color: #7f8c8d;
            font-size: 12px;
            margin-top: 40px;
            padding-top: 20px;
            border-top: 1px solid #e0e0e0;
        }
        .footer {
            background: #f8f9fa;
            padding: 20px;
            text-align: center;
            color: #7f8c8d;
            font-size: 14px;
            border-top: 1px solid #e0e0e0;
        }
        .gomu-badge {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 6px 12px;
            border-radius: 20px;
            font-size: 12px;
            font-weight: 600;
            text-decoration: none;
            display: inline-block;
            margin-top: 10px;
        }
        @media (max-width: 768px) {
            .summary-grid, .stats-grid, .mutation-types, .file-grid {
                grid-template-columns: 1fr;
            }
            .score-value {
                font-size: 48px;
            }
            .filters {
                justify-content: center;
            }
            .mutant-header {
                flex-direction: column;
                align-items: flex-start;
                gap: 10px;
            }
        }
    </style>
    <script>
        document.addEventListener('DOMContentLoaded', function() {
            // Filter functionality
            const filterBtns = document.querySelectorAll('.filter-btn');
            const mutantItems = document.querySelectorAll('.mutant-item');
            
            filterBtns.forEach(btn => {
                btn.addEventListener('click', function() {
                    const filter = this.dataset.filter;
                    
                    // Update active button
                    filterBtns.forEach(b => b.classList.remove('active'));
                    this.classList.add('active');
                    
                    // Filter mutants
                    mutantItems.forEach(item => {
                        const type = item.dataset.type;
                        const status = item.dataset.status;
                        
                        let shouldShow = false;
                        
                        if (filter === 'all') {
                            shouldShow = true;
                        } else if (filter === 'SURVIVED' || filter === 'KILLED' || filter === 'TIMED_OUT' || filter === 'ERROR' || filter === 'NOT_VIABLE') {
                            shouldShow = status === filter;
                        } else {
                            shouldShow = type === filter;
                        }
                        
                        if (shouldShow) {
                            item.style.display = 'block';
                        } else {
                            item.style.display = 'none';
                        }
                    });
                });
            });
            
            // Animate progress bars
            const progressBars = document.querySelectorAll('.progress-fill');
            progressBars.forEach(bar => {
                const width = bar.dataset.width;
                setTimeout(() => {
                    bar.style.width = width + '%';
                }, 100);
            });
            
            // Add score indicator
            const scoreValue = parseFloat(document.querySelector('.score-value').textContent);
            const scoreIndicator = document.createElement('span');
            scoreIndicator.className = 'score-indicator';
            
            if (scoreValue >= 80) {
                scoreIndicator.classList.add('score-excellent');
            } else if (scoreValue >= 60) {
                scoreIndicator.classList.add('score-good');
            } else {
                scoreIndicator.classList.add('score-poor');
            }
            
            document.querySelector('.score-value').appendChild(scoreIndicator);
        });
    </script>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🧬 Gomu Mutation Testing Report</h1>
            <div class="subtitle">Advanced Go Mutation Testing Analysis</div>
        </div>
        
        <div class="content">
            <div class="summary-grid">
                <div class="summary-card">
                    <h3>📁 Files Processed</h3>
                    <div class="value">{{.ProcessedFiles}}/{{.TotalFiles}}</div>
                </div>
                <div class="summary-card">
                    <h3>🧪 Total Mutants</h3>
                    <div class="value">{{.TotalMutants}}</div>
                </div>
                <div class="summary-card">
                    <h3>⏱️ Duration</h3>
                    <div class="value">{{.Duration}}</div>
                </div>
                <div class="summary-card">
                    <h3>💀 Killed Rate</h3>
                    <div class="value">{{printf "%.1f" (percentage .Statistics.Killed .TotalMutants)}}%</div>
                </div>
            </div>
            
            <div class="mutation-score">
                <h2>🎯 Mutation Score</h2>
                <div class="score-value">{{printf "%.1f" .Statistics.Score}}%</div>
            </div>
            
            <div class="statistics">
                <div class="stats-grid">
                    <div class="stat-item killed">
                        <div class="stat-number">{{.Statistics.Killed}}</div>
                        <div class="stat-label">Killed ({{printf "%.1f" (percentage .Statistics.Killed .TotalMutants)}}%)</div>
                    </div>
                    <div class="stat-item survived">
                        <div class="stat-number">{{.Statistics.Survived}}</div>
                        <div class="stat-label">Survived ({{printf "%.1f" (percentage .Statistics.Survived .TotalMutants)}}%)</div>
                    </div>
                    <div class="stat-item timed-out">
                        <div class="stat-number">{{.Statistics.TimedOut}}</div>
                        <div class="stat-label">Timed Out ({{printf "%.1f" (percentage .Statistics.TimedOut .TotalMutants)}}%)</div>
                    </div>
                    <div class="stat-item error">
                        <div class="stat-number">{{.Statistics.Errors}}</div>
                        <div class="stat-label">Errors ({{printf "%.1f" (percentage .Statistics.Errors .TotalMutants)}}%)</div>
                    </div>
                    <div class="stat-item not-viable">
                        <div class="stat-number">{{.Statistics.NotViable}}</div>
                        <div class="stat-label">Not Viable ({{printf "%.1f" (percentage .Statistics.NotViable .TotalMutants)}}%)</div>
                    </div>
                </div>
            </div>
            
            {{if .Statistics.MutationTypes}}
            <div class="mutation-breakdown">
                <h2>🔬 Mutation Type Analysis</h2>
                <div class="mutation-types">
                    {{range $type, $stats := .Statistics.MutationTypes}}
                    <div class="mutation-type-card">
                        <h3>{{$type}}</h3>
                        <div class="type-stats">
                            <div class="type-stat">
                                <div class="number">{{$stats.Total}}</div>
                                <div class="label">Total</div>
                            </div>
                            <div class="type-stat">
                                <div class="number">{{$stats.Killed}}</div>
                                <div class="label">Killed</div>
                            </div>
                            <div class="type-stat">
                                <div class="number">{{$stats.Survived}}</div>
                                <div class="label">Survived</div>
                            </div>
                        </div>
                        <div class="progress-bar">
                            <div class="progress-fill" data-width="{{printf "%.1f" (percentage $stats.Killed $stats.Total)}}"></div>
                        </div>
                    </div>
                    {{end}}
                </div>
            </div>
            {{end}}
            
            {{if .Files}}
            <div class="file-reports">
                <h2>📊 File-level Analysis</h2>
                <div class="file-grid">
                    {{range $path, $file := .Files}}
                    <div class="file-card">
                        <div class="file-header">
                            <div class="file-path">{{$path}}</div>
                        </div>
                        <div class="file-stats">
                            <div class="file-score">{{printf "%.1f" $file.MutationScore}}%</div>
                            <div class="file-mutants">{{$file.KilledMutants}}/{{$file.TotalMutants}} killed</div>
                        </div>
                    </div>
                    {{end}}
                </div>
            </div>
            {{end}}
            
            <div class="detailed-analysis">
                <h2>🔍 Detailed Mutant Analysis</h2>
                <div class="filters">
                    <button class="filter-btn active" data-filter="all">All Status</button>
                    <button class="filter-btn" data-filter="SURVIVED">Survived</button>
                    <button class="filter-btn" data-filter="KILLED">Killed</button>
                    <button class="filter-btn" data-filter="NOT_VIABLE">Not Viable</button>
                    <button class="filter-btn" data-filter="arithmetic">Arithmetic</button>
                    <button class="filter-btn" data-filter="conditional">Conditional</button>
                    <button class="filter-btn" data-filter="logical">Logical</button>
                </div>
                <div class="mutant-list">
                    {{range .Results}}
                    <div class="mutant-item {{.Status}}" data-type="{{.Mutant.Type}}" data-status="{{.Status}}">
                        <div class="mutant-header">
                            <div class="mutant-location">{{.Mutant.FilePath}}:{{.Mutant.Line}}:{{.Mutant.Column}}</div>
                            <div class="mutant-badges">
                                <div class="mutant-type">{{.Mutant.Type}}</div>
                                <div class="mutant-status {{.Status}}">{{.Status}}</div>
                            </div>
                        </div>
                        {{if .Mutant.Function}}
                        <div class="mutant-function">📍 Function: <code>{{.Mutant.Function}}</code></div>
                        {{end}}
                        <div class="mutant-description">{{.Mutant.Description}}</div>
                        <div class="mutant-change">
                            <span class="original">{{.Mutant.Original}}</span> → <span class="mutated">{{.Mutant.Mutated}}</span>
                        </div>
                        {{if .Mutant.Context}}
                        <div class="mutant-context">
                            <h4>📝 Code Context</h4>
                            <pre><code>{{.Mutant.Context}}</code></pre>
                        </div>
                        {{end}}
                        {{if .TestOutput}}
                        <div class="test-execution">
                            <h4>🧪 Test Execution Details</h4>
                            <div class="test-summary">
                                <span class="test-stat">⏱️ {{.ExecutionTime}}ms</span>
                                <span class="test-stat">🏃 {{.TestsRun}} tests run</span>
                                {{if .TestsFailed}}<span class="test-stat failed">❌ {{.TestsFailed}} failed</span>{{end}}
                            </div>
                            <div class="test-details">
                                {{range .TestOutput}}
                                <div class="test-item {{.Status}}">
                                    <div class="test-name">{{.Name}}</div>
                                    <div class="test-info">
                                        <span class="test-package">{{.Package}}</span>
                                        <span class="test-status {{.Status}}">{{.Status}}</span>
                                        {{if .Duration}}<span class="test-duration">{{.Duration}}ms</span>{{end}}
                                    </div>
                                    {{if .Output}}
                                    <div class="test-output">{{.Output}}</div>
                                    {{end}}
                                </div>
                                {{end}}
                            </div>
                        </div>
                        {{end}}
                    </div>
                    {{end}}
                </div>
            </div>
        </div>
        
        <div class="footer">
            <div class="timestamp">
                Generated on {{.Timestamp.Format "2006-01-02 15:04:05 MST"}}
            </div>
            <a href="https://github.com/sivchari/gomu" class="gomu-badge">
                Powered by Gomu v{{.Version}}
            </a>
        </div>
    </div>
</body>
</html>`

func (g *Generator) generateConsole(summary *Summary) error {
	stats := summary.Statistics

	fmt.Println()
	fmt.Println("Mutation Testing Results")
	fmt.Println("========================")
	fmt.Printf("Files: %d/%d processed\n", summary.ProcessedFiles, summary.TotalFiles)
	fmt.Printf("Mutants: %d total\n", summary.TotalMutants)
	fmt.Printf("Duration: %v\n", summary.Duration)
	fmt.Println()
	fmt.Printf("Killed:     %d (%.1f%%)\n", stats.Killed, percentage(stats.Killed, summary.TotalMutants))
	fmt.Printf("Survived:   %d (%.1f%%)\n", stats.Survived, percentage(stats.Survived, summary.TotalMutants))
	fmt.Printf("Timed out:  %d (%.1f%%)\n", stats.TimedOut, percentage(stats.TimedOut, summary.TotalMutants))
	fmt.Printf("Errors:     %d (%.1f%%)\n", stats.Errors, percentage(stats.Errors, summary.TotalMutants))
	fmt.Printf("Not viable: %d (%.1f%%)\n", stats.NotViable, percentage(stats.NotViable, summary.TotalMutants))
	fmt.Println()
	fmt.Printf("Mutation Score: %.1f%%\n", stats.Score)

	return nil
}

func percentage(part, total int) float64 {
	if total == 0 {
		return 0
	}

	return float64(part) / float64(total) * 100
}
