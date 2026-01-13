package visualizer

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log-interleaver/internal/config"
	"log-interleaver/internal/parser"
	"log-interleaver/pkg/pattern"
	"os"
	"sort"
	"strings"
	"time"
)

// ExportData exports time series data to CSV format
func ExportData(lines []*parser.LogLine, configPath, outputPath string) error {
	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Convert config patterns to pattern matcher format
	patternConfigs := make([]pattern.PatternConfig, len(cfg.Patterns))
	for i, p := range cfg.Patterns {
		patternConfigs[i] = pattern.PatternConfig{
			Name:                    p.Name,
			Regex:                   p.Regex,
			TagFilter:               p.TagFilter,
			ValueGroup:              p.ValueGroup,
			StateGroup:              p.StateGroup,
			DeviceGroup:             p.DeviceGroup,
			StateMapping:            p.StateMapping,
			ValueMultiplier:         p.ValueMultiplier,
			ConvertNanosecondOffset: p.ConvertNanosecondOffset,
			Color:                   p.Color,
			LineStyle:               p.LineStyle,
			Marker:                  p.Marker,
			YAxisLabel:              p.YAxisLabel,
			YAxisIndex:              p.YAxisIndex,
		}
	}

	// Create pattern matcher
	matcher, err := pattern.NewPatternMatcher(patternConfigs)
	if err != nil {
		return fmt.Errorf("failed to create pattern matcher: %w", err)
	}

	// Extract metrics
	metrics, err := matcher.ExtractMetrics(lines)
	if err != nil {
		return fmt.Errorf("failed to extract metrics: %w", err)
	}

	// Create CSV file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Find earliest timestamp to use as reference
	var earliestTime *time.Time
	for _, points := range metrics {
		for _, pt := range points {
			if earliestTime == nil || pt.Time.Before(*earliestTime) {
				t := pt.Time
				earliestTime = &t
			}
		}
	}

	if earliestTime == nil {
		return fmt.Errorf("no timestamps found in data")
	}

	// Collect all unique timestamps
	timeSet := make(map[time.Time]bool)
	for _, points := range metrics {
		for _, pt := range points {
			timeSet[pt.Time] = true
		}
	}

	// Convert to sorted slice
	times := make([]time.Time, 0, len(timeSet))
	for t := range timeSet {
		times = append(times, t)
	}
	sort.Slice(times, func(i, j int) bool {
		return times[i].Before(times[j])
	})

	// Sort each series by time
	for seriesName := range metrics {
		sort.Slice(metrics[seriesName], func(i, j int) bool {
			return metrics[seriesName][i].Time.Before(metrics[seriesName][j].Time)
		})
	}

	// Write header
	header := []string{"Time", "TimeOffsetSeconds"}
	for _, pattern := range cfg.Patterns {
		// Include pattern name if it exists, and also all device-based series
		if _, ok := metrics[pattern.Name]; ok {
			header = append(header, pattern.Name)
		}
		// Find all device-based series for this pattern
		for seriesName := range metrics {
			if strings.HasPrefix(seriesName, pattern.Name+" ") {
				header = append(header, seriesName)
			}
		}
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Create index maps for quick lookup
	seriesIndices := make(map[string]int)
	for seriesName := range metrics {
		seriesIndices[seriesName] = 0
	}

	// Write data rows
	for _, t := range times {
		row := []string{
			t.Format(time.RFC3339Nano),
			fmt.Sprintf("%.6f", t.Sub(*earliestTime).Seconds()),
		}

		// Add value for each series at this timestamp
		for _, pattern := range cfg.Patterns {
			// Handle exact pattern match
			seriesName := pattern.Name
			if points, ok := metrics[seriesName]; ok {
				// Find value at this timestamp (or closest)
				var value string
				idx := seriesIndices[seriesName]
				if idx < len(points) {
					// Check if we have an exact match or need to advance
					for idx < len(points) && points[idx].Time.Before(t) {
						idx++
					}
					seriesIndices[seriesName] = idx

					if idx < len(points) && points[idx].Time.Equal(t) {
						value = fmt.Sprintf("%.6f", points[idx].Value)
					} else {
						value = "" // No data at this timestamp
					}
				}
				row = append(row, value)
			}
			// Handle device-based series
			for seriesName, points := range metrics {
				if strings.HasPrefix(seriesName, pattern.Name+" ") {
					// Find value at this timestamp (or closest)
					var value string
					idx := seriesIndices[seriesName]
					if idx < len(points) {
						// Check if we have an exact match or need to advance
						for idx < len(points) && points[idx].Time.Before(t) {
							idx++
						}
						seriesIndices[seriesName] = idx

						if idx < len(points) && points[idx].Time.Equal(t) {
							value = fmt.Sprintf("%.6f", points[idx].Value)
						} else {
							value = "" // No data at this timestamp
						}
					}
					row = append(row, value)
				}
			}
		}

		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return nil
}

// SeriesData represents a time series for JSON/HTML export
type SeriesData struct {
	Name         string             `json:"name"`
	X            []float64          `json:"x"` // Time offsets in seconds
	Y            []float64          `json:"y"` // Values
	Color        string             `json:"color,omitempty"`
	Marker       string             `json:"marker,omitempty"`
	LineStyle    string             `json:"line_style,omitempty"`
	Mode         string             `json:"mode"`                  // "lines+markers", "lines", "markers"
	Step         bool               `json:"step,omitempty"`        // If true, use step plot (hold value between points)
	YAxisLabel   string             `json:"yaxis_label,omitempty"` // Y-axis label for this series
	StateMapping map[string]float64 `json:"state_mapping,omitempty"`
}

// ExportJSON exports time series data to JSON format
func ExportJSON(lines []*parser.LogLine, configPath, outputPath string) error {
	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Convert config patterns to pattern matcher format
	patternConfigs := make([]pattern.PatternConfig, len(cfg.Patterns))
	for i, p := range cfg.Patterns {
		patternConfigs[i] = pattern.PatternConfig{
			Name:                    p.Name,
			Regex:                   p.Regex,
			TagFilter:               p.TagFilter,
			ValueGroup:              p.ValueGroup,
			StateGroup:              p.StateGroup,
			DeviceGroup:             p.DeviceGroup,
			StateMapping:            p.StateMapping,
			ValueMultiplier:         p.ValueMultiplier,
			ConvertNanosecondOffset: p.ConvertNanosecondOffset,
			Color:                   p.Color,
			LineStyle:               p.LineStyle,
			Marker:                  p.Marker,
			YAxisLabel:              p.YAxisLabel,
			YAxisIndex:              p.YAxisIndex,
		}
	}

	// Create pattern matcher
	matcher, err := pattern.NewPatternMatcher(patternConfigs)
	if err != nil {
		return fmt.Errorf("failed to create pattern matcher: %w", err)
	}

	// Extract metrics
	metrics, err := matcher.ExtractMetrics(lines)
	if err != nil {
		return fmt.Errorf("failed to extract metrics: %w", err)
	}

	// Find earliest timestamp
	var earliestTime *time.Time
	for _, points := range metrics {
		for _, pt := range points {
			if earliestTime == nil || pt.Time.Before(*earliestTime) {
				t := pt.Time
				earliestTime = &t
			}
		}
	}

	if earliestTime == nil {
		return fmt.Errorf("no timestamps found in data")
	}

	// Build series data
	// For device-based series, we need to iterate over all metrics and match them to patterns
	seriesList := make([]SeriesData, 0)

	// First, collect all series names that match each pattern
	for _, pattern := range cfg.Patterns {
		// Find all series that match this pattern (including device-based ones)
		for seriesName, points := range metrics {
			if seriesName == pattern.Name || strings.HasPrefix(seriesName, pattern.Name+" ") {
				if len(points) == 0 {
					continue
				}

				// Sort points by time
				sort.Slice(points, func(i, j int) bool {
					return points[i].Time.Before(points[j].Time)
				})

				// Extract X and Y arrays
				var x []float64
				var y []float64
				// For step plots with a single point, extend it to the end of the time range
				if pattern.Step && len(points) == 1 {
					// Find the maximum time across all metrics to extend the step
					maxTime := points[0].Time
					for _, otherPoints := range metrics {
						if len(otherPoints) > 0 {
							lastPoint := otherPoints[len(otherPoints)-1]
							if lastPoint.Time.After(maxTime) {
								maxTime = lastPoint.Time
							}
						}
					}
					// Create two points: one at the actual time, one at the max time
					x = []float64{
						points[0].Time.Sub(*earliestTime).Seconds(),
						maxTime.Sub(*earliestTime).Seconds(),
					}
					y = []float64{points[0].Value, points[0].Value}
				} else {
					x = make([]float64, len(points))
					y = make([]float64, len(points))
					for i, pt := range points {
						x[i] = pt.Time.Sub(*earliestTime).Seconds()
						y[i] = pt.Value
					}
				}

				// Determine mode based on marker and line style
				mode := "lines+markers"
				if pattern.LineStyle == "none" {
					// Markers only
					mode = "markers"
				} else if pattern.Marker == "" {
					// Lines only
					mode = "lines"
				} else if pattern.LineStyle == "" {
					// If marker is set but no line style, default to markers only
					mode = "markers"
				} else {
					// Both lines and markers
					mode = "lines+markers"
				}

				series := SeriesData{
					Name:       seriesName,
					X:          x,
					Y:          y,
					Color:      pattern.Color,
					Marker:     pattern.Marker,
					LineStyle:  pattern.LineStyle,
					Mode:       mode,
					Step:       pattern.Step,
					YAxisLabel: pattern.YAxisLabel,
				}

				if pattern.StateMapping != nil {
					series.StateMapping = pattern.StateMapping
				}

				seriesList = append(seriesList, series)
			}
		}
	}

	// Create output structure
	output := map[string]interface{}{
		"title":       cfg.Title,
		"xaxis_label": cfg.XAxisLabel,
		"yaxis_label": cfg.YAxisLabel,
		"start_time":  earliestTime.Format(time.RFC3339Nano),
		"series":      seriesList,
	}

	// Add Y-axis range if configured
	if cfg.YRange != nil {
		output["y_range"] = *cfg.YRange
	} else {
		if cfg.YMin != nil {
			output["y_min"] = *cfg.YMin
		}
		if cfg.YMax != nil {
			output["y_max"] = *cfg.YMax
		}
	}

	// Add Y-axis tick spacing/count if configured
	if cfg.YTickSpacing != nil {
		output["y_tick_spacing"] = *cfg.YTickSpacing
	}
	if cfg.YTickCount != nil {
		output["y_tick_count"] = *cfg.YTickCount
	}

	// Write JSON
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create JSON file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}
