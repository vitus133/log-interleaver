package main

import (
	"flag"
	"fmt"
	"log-interleaver/internal/config"
	"log-interleaver/internal/interleaver"
	"log-interleaver/internal/parser"
	"log-interleaver/internal/visualizer"
	"os"
	"strconv"
	"strings"
)

func main() {
	var (
		logDir      = flag.String("logs", "logs", "Directory containing log files")
		output      = flag.String("output", "", "Output file (default: stdout)")
		analyze     = flag.Bool("analyze", false, "Run analysis on interleaved logs")
		noAutoAlign = flag.Bool("no-auto-align", false, "Disable automatic timezone alignment")
		offsets     = flag.String("offset", "", "Comma-separated file offsets in format tag:hours (e.g., e825:5,e830:5)")
		visualize   = flag.Bool("visualize", false, "Generate visualization plot")
		configPath  = flag.String("config", "config.yaml", "Path to visualization config file (YAML)")
		plotOutput  = flag.String("plot-output", "plot.png", "Output path for plot image")
		exportCSV   = flag.String("export-csv", "", "Export time series data to CSV file")
		exportJSON  = flag.String("export-json", "", "Export time series data to JSON file")
		exportHTML  = flag.String("export-html", "", "Export interactive HTML plot (uses Plotly.js)")
	)
	flag.Parse()

	// Create interleaver
	iv := interleaver.NewInterleaver(*logDir)
	iv.SetAutoAlign(!*noAutoAlign)

	// Parse manual offsets
	if *offsets != "" {
		offsetPairs := strings.Split(*offsets, ",")
		for _, pair := range offsetPairs {
			parts := strings.Split(strings.TrimSpace(pair), ":")
			if len(parts) != 2 {
				fmt.Fprintf(os.Stderr, "Warning: invalid offset format '%s', expected tag:hours\n", pair)
				continue
			}
			tag := strings.TrimSpace(parts[0])
			hours, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: invalid hours value '%s': %v\n", parts[1], err)
				continue
			}
			iv.SetFileOffset(tag, hours)
		}
	}

	// Process logs
	lines, err := iv.Process()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error processing logs: %v\n", err)
		os.Exit(1)
	}

	// Output results
	var outputFile *os.File
	if *output != "" {
		outputFile, err = os.Create(*output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
			os.Exit(1)
		}
		defer outputFile.Close()
	} else {
		outputFile = os.Stdout
	}

	// Write interleaved logs if output file is specified
	// (always write when -output is provided, regardless of -visualize flag)
	if *output != "" {
		for _, line := range lines {
			formatted := interleaver.FormatLine(line)
			fmt.Fprintln(outputFile, formatted)
		}
	} else if !*visualize {
		// Only write to stdout if not visualizing and no output file specified
		for _, line := range lines {
			formatted := interleaver.FormatLine(line)
			fmt.Fprintln(outputFile, formatted)
		}
	}

	if *analyze {
		// Run basic analysis
		analyzeLogs(lines, outputFile)
	}

	if *visualize {
		// Generate visualization
		if err := generateVisualization(lines, *configPath, *plotOutput); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating visualization: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Plot saved to: %s\n", *plotOutput)
	}

	if *exportCSV != "" {
		// Export to CSV
		if err := exportToCSV(lines, *configPath, *exportCSV); err != nil {
			fmt.Fprintf(os.Stderr, "Error exporting CSV: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "CSV data exported to: %s\n", *exportCSV)
	}

	if *exportJSON != "" {
		// Export to JSON
		if err := exportToJSON(lines, *configPath, *exportJSON); err != nil {
			fmt.Fprintf(os.Stderr, "Error exporting JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "JSON data exported to: %s\n", *exportJSON)
	}

	if *exportHTML != "" {
		// Export interactive HTML
		if err := exportToHTML(lines, *configPath, *exportHTML); err != nil {
			fmt.Fprintf(os.Stderr, "Error exporting HTML: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Interactive HTML plot saved to: %s\n", *exportHTML)
		fmt.Fprintf(os.Stderr, "Open in a web browser to view and interact with the plot\n")
	}
}

func generateVisualization(lines []*parser.LogLine, configPath, outputPath string) error {
	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create visualizer
	viz := visualizer.NewVisualizer(cfg)

	// Generate plot
	return viz.GeneratePlot(lines, outputPath)
}

func exportToCSV(lines []*parser.LogLine, configPath, outputPath string) error {
	return visualizer.ExportData(lines, configPath, outputPath)
}

func exportToJSON(lines []*parser.LogLine, configPath, outputPath string) error {
	return visualizer.ExportJSON(lines, configPath, outputPath)
}

func exportToHTML(lines []*parser.LogLine, configPath, outputPath string) error {
	return visualizer.GenerateInteractiveHTML(lines, configPath, outputPath)
}

func analyzeLogs(lines []*parser.LogLine, output *os.File) {
	// Basic statistics
	fmt.Fprintf(output, "\n=== Analysis ===\n")
	fmt.Fprintf(output, "Total log lines: %d\n", len(lines))

	// Count by tag
	tagCounts := make(map[string]int)
	for _, line := range lines {
		tagCounts[line.Tag]++
	}

	fmt.Fprintf(output, "\nLines by tag:\n")
	for tag, count := range tagCounts {
		fmt.Fprintf(output, "  %s: %d\n", tag, count)
	}

	// Count lines with/without timestamps
	withTimestamp := 0
	withoutTimestamp := 0
	for _, line := range lines {
		if line.GetTimestamp() != nil {
			withTimestamp++
		} else {
			withoutTimestamp++
		}
	}

	fmt.Fprintf(output, "\nTimestamp coverage:\n")
	fmt.Fprintf(output, "  With timestamp: %d\n", withTimestamp)
	fmt.Fprintf(output, "  Without timestamp: %d\n", withoutTimestamp)
}
