package visualizer

import (
	"fmt"
	"image/color"
	"log-interleaver/internal/config"
	"log-interleaver/internal/parser"
	"log-interleaver/pkg/pattern"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// Visualizer creates plots from log data
type Visualizer struct {
	config *config.VisualizationConfig
}

// NewVisualizer creates a new visualizer with the given configuration
func NewVisualizer(cfg *config.VisualizationConfig) *Visualizer {
	return &Visualizer{config: cfg}
}

// GeneratePlot generates a plot from log lines and saves it to a file
func (v *Visualizer) GeneratePlot(lines []*parser.LogLine, outputPath string) error {
	// Convert config patterns to pattern matcher format
	patternConfigs := make([]pattern.PatternConfig, len(v.config.Patterns))
	for i, p := range v.config.Patterns {
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

	// Create plot
	p := plot.New()
	p.Title.Text = v.config.Title
	p.X.Label.Text = v.config.XAxisLabel
	p.Y.Label.Text = v.config.YAxisLabel

	// Set Y-axis range if configured
	var yMin, yMax float64
	if v.config.YRange != nil {
		// Symmetric range: +/- value
		rangeVal := *v.config.YRange
		yMin = -rangeVal
		yMax = rangeVal
		p.Y.Min = yMin
		p.Y.Max = yMax
	} else {
		// Individual min/max if specified
		if v.config.YMin != nil {
			yMin = *v.config.YMin
			p.Y.Min = yMin
		}
		if v.config.YMax != nil {
			yMax = *v.config.YMax
			p.Y.Max = yMax
		}
	}

	// Set Y-axis tick spacing - generate ticks if range is set or custom spacing is configured
	actualMin := p.Y.Min
	actualMax := p.Y.Max
	hasExplicitRange := actualMin != 0 || actualMax != 0 || v.config.YRange != nil || v.config.YMin != nil || v.config.YMax != nil

	if v.config.YTickSpacing != nil {
		// Use custom tick marker with specified spacing
		spacing := *v.config.YTickSpacing
		if hasExplicitRange {
			tickValues := generateTickValues(actualMin, actualMax, spacing)
			if len(tickValues) > 0 {
				ticks := make([]plot.Tick, len(tickValues))
				for i, val := range tickValues {
					ticks[i] = plot.Tick{Value: val, Label: fmt.Sprintf("%.0f", val)}
				}
				p.Y.Tick.Marker = plot.ConstantTicks(ticks)
			}
		}
	} else if v.config.YTickCount != nil {
		// Use specified number of ticks
		count := *v.config.YTickCount
		if count > 0 && hasExplicitRange {
			spacing := (actualMax - actualMin) / float64(count-1)
			tickValues := generateTickValues(actualMin, actualMax, spacing)
			if len(tickValues) > 0 {
				ticks := make([]plot.Tick, len(tickValues))
				for i, val := range tickValues {
					ticks[i] = plot.Tick{Value: val, Label: fmt.Sprintf("%.0f", val)}
				}
				p.Y.Tick.Marker = plot.ConstantTicks(ticks)
			}
		}
	} else if hasExplicitRange {
		// Automatic tick generation when range is set but no custom spacing
		// Calculate reasonable tick spacing for ~8-10 ticks
		rangeVal := actualMax - actualMin
		if rangeVal > 0 {
			targetSpacing := rangeVal / 10.0
			// Round to nice numbers (powers of 1, 2, 5, 10, etc.)
			magnitude := math.Pow(10, math.Floor(math.Log10(targetSpacing)))
			normalized := targetSpacing / magnitude
			var multiplier float64
			if normalized <= 1 {
				multiplier = 1
			} else if normalized <= 2 {
				multiplier = 2
			} else if normalized <= 5 {
				multiplier = 5
			} else {
				multiplier = 10
			}
			spacing := multiplier * magnitude

			tickValues := generateTickValues(actualMin, actualMax, spacing)
			if len(tickValues) > 0 && len(tickValues) <= 20 {
				ticks := make([]plot.Tick, len(tickValues))
				for i, val := range tickValues {
					ticks[i] = plot.Tick{Value: val, Label: fmt.Sprintf("%.0f", val)}
				}
				p.Y.Tick.Marker = plot.ConstantTicks(ticks)
			}
		}
	}

	// Find the earliest timestamp across ALL metrics to use as the reference point
	// This ensures all series are plotted relative to the same start time
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
		return fmt.Errorf("no timestamps found in metrics")
	}

	// Group series by Y-axis index
	// For device-based series, we need to find all series that start with the pattern name
	seriesByAxis := make(map[int][]string)
	for _, pattern := range v.config.Patterns {
		axisIdx := pattern.YAxisIndex
		if axisIdx < 0 {
			axisIdx = 0
		}
		if _, ok := seriesByAxis[axisIdx]; !ok {
			seriesByAxis[axisIdx] = make([]string, 0)
		}
		// Find all series that match this pattern (including device-based ones)
		for seriesName := range metrics {
			if seriesName == pattern.Name || strings.HasPrefix(seriesName, pattern.Name+" ") {
				seriesByAxis[axisIdx] = append(seriesByAxis[axisIdx], seriesName)
			}
		}
	}

	// Create secondary Y-axis if needed
	var rightAxis *plot.Axis
	if len(seriesByAxis) > 1 {
		rightAxis = &plot.Axis{}
		p.Y.Label.Text = v.config.YAxisLabel // Left axis label
	}

	// Plot each series
	colors := []color.Color{
		color.RGBA{R: 31, G: 119, B: 180, A: 255},  // blue
		color.RGBA{R: 255, G: 127, B: 14, A: 255},  // orange
		color.RGBA{R: 44, G: 160, B: 44, A: 255},   // green
		color.RGBA{R: 214, G: 39, B: 40, A: 255},   // red
		color.RGBA{R: 148, G: 103, B: 189, A: 255}, // purple
		color.RGBA{R: 140, G: 86, B: 75, A: 255},   // brown
		color.RGBA{R: 227, G: 119, B: 194, A: 255}, // pink
		color.RGBA{R: 127, G: 127, B: 127, A: 255}, // gray
	}
	colorIdx := 0

	for axisIdx, seriesNames := range seriesByAxis {
		for _, seriesName := range seriesNames {
			points, ok := metrics[seriesName]
			if !ok || len(points) == 0 {
				continue
			}

			// Find pattern config for styling
			// Match by exact name or by prefix (for device-based series)
			var patternCfg *config.PatternConfig
			for i := range v.config.Patterns {
				if v.config.Patterns[i].Name == seriesName || strings.HasPrefix(seriesName, v.config.Patterns[i].Name+" ") {
					patternCfg = &v.config.Patterns[i]
					break
				}
			}

			// Sort points by time
			sort.Slice(points, func(i, j int) bool {
				return points[i].Time.Before(points[j].Time)
			})

			// Convert to plotter.XYs
			var xy plotter.XYs

			// Check if step plot is requested
			useStep := patternCfg != nil && patternCfg.Step

			if useStep {
				if len(points) > 1 {
					// For step plots with multiple points, create horizontal-vertical steps
					// Each point needs a horizontal segment to the next x value
					xy = make(plotter.XYs, 0, len(points)*2-1)
					for i, pt := range points {
						x := pt.Time.Sub(*earliestTime).Seconds()
						y := pt.Value

						// Add the point
						xy = append(xy, plotter.XY{X: x, Y: y})

						// Add horizontal segment to next point (if not last point)
						if i < len(points)-1 {
							nextX := points[i+1].Time.Sub(*earliestTime).Seconds()
							xy = append(xy, plotter.XY{X: nextX, Y: y})
						}
					}
				} else if len(points) == 1 {
					// For step plots with a single point, extend it to the end of the time range
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
					x1 := points[0].Time.Sub(*earliestTime).Seconds()
					x2 := maxTime.Sub(*earliestTime).Seconds()
					y := points[0].Value
					xy = plotter.XYs{
						{X: x1, Y: y},
						{X: x2, Y: y},
					}
				}
			} else {
				// Normal linear plot
				xy = make(plotter.XYs, len(points))
				for i, pt := range points {
					xy[i].X = pt.Time.Sub(*earliestTime).Seconds()
					xy[i].Y = pt.Value
				}
			}

			// Build legend label with state mapping if available
			legendLabel := seriesName
			if patternCfg != nil && patternCfg.StateMapping != nil && len(patternCfg.StateMapping) > 0 {
				// Create mapping string for legend
				mappingParts := make([]string, 0, len(patternCfg.StateMapping))
				for state, value := range patternCfg.StateMapping {
					mappingParts = append(mappingParts, fmt.Sprintf("%s=%.0f", state, value))
				}
				// Sort for consistent display
				sort.Strings(mappingParts)
				legendLabel = fmt.Sprintf("%s (%s)", seriesName, strings.Join(mappingParts, ", "))
			}

			// Create line/scatter plot
			var line *plotter.Line
			var scatter *plotter.Scatter

			// Determine color
			plotColor := colors[colorIdx%len(colors)]
			if patternCfg != nil && patternCfg.Color != "" {
				if parsedColor := parseColor(patternCfg.Color); parsedColor != nil {
					plotColor = parsedColor
				}
			}

			// Determine marker style
			markerRadius := vg.Points(3)
			var markerShape draw.GlyphDrawer
			if patternCfg != nil && patternCfg.Marker != "" {
				// Adjust marker size and shape based on type
				switch patternCfg.Marker {
				case "x", "X":
					markerRadius = vg.Points(4)
					markerShape = draw.CrossGlyph{}
				case "+":
					markerRadius = vg.Points(4)
					markerShape = draw.PlusGlyph{}
				case "o", "O", "circle":
					markerRadius = vg.Points(3)
					markerShape = draw.CircleGlyph{}
				case "s", "S", "square":
					markerRadius = vg.Points(3)
					markerShape = draw.SquareGlyph{}
				case "d", "D", "diamond":
					markerRadius = vg.Points(4)
					markerShape = draw.RingGlyph{} // Diamond not directly available, use ring as alternative
				case ".", "point":
					markerRadius = vg.Points(2)
					markerShape = draw.CircleGlyph{}
				default:
					markerShape = draw.CircleGlyph{} // Default to circle
				}
			} else {
				// Default marker if none specified
				markerShape = draw.CircleGlyph{}
			}

			// Create scatter plot (dots)
			scatter, err := plotter.NewScatter(xy)
			if err != nil {
				return fmt.Errorf("failed to create scatter plot: %w", err)
			}
			scatter.GlyphStyle.Radius = markerRadius
			scatter.GlyphStyle.Color = plotColor
			if markerShape != nil {
				scatter.GlyphStyle.Shape = markerShape
			}

			// Determine if we should draw lines
			drawLines := true
			if patternCfg != nil {
				// If line_style is explicitly "none", don't draw lines
				if patternCfg.LineStyle == "none" {
					drawLines = false
				}
				// Otherwise, always draw lines (even if marker is set)
				// User can set line_style: "none" explicitly for markers-only
			}

			// Create line plot (if needed)
			if drawLines {
				lineStyle := plotter.DefaultLineStyle
				if patternCfg != nil && patternCfg.LineStyle != "" {
					switch patternCfg.LineStyle {
					case "-", "solid":
						lineStyle = plotter.DefaultLineStyle
					case "--", "dashed":
						lineStyle.Dashes = []vg.Length{vg.Points(5), vg.Points(5)}
					case ":", "dotted":
						lineStyle.Dashes = []vg.Length{vg.Points(2), vg.Points(2)}
					case "-.", "dashdot":
						lineStyle.Dashes = []vg.Length{vg.Points(5), vg.Points(2), vg.Points(2), vg.Points(2)}
					}
				}
				lineStyle.Color = plotColor
				lineStyle.Width = vg.Points(1)

				line, err = plotter.NewLine(xy)
				if err != nil {
					return fmt.Errorf("failed to create line plot: %w", err)
				}
				line.LineStyle = lineStyle
			}

			// Add to plot
			if drawLines && line != nil {
				p.Add(scatter, line)
				p.Legend.Add(legendLabel, scatter, line)
			} else {
				// Markers only
				p.Add(scatter)
				p.Legend.Add(legendLabel, scatter)
			}

			// Use right axis if specified
			if axisIdx == 1 && rightAxis != nil {
				// Note: gonum/plot doesn't directly support dual Y-axes easily
				// For now, we'll use the same axis but could enhance this later
			}

			colorIdx++
		}
	}

	// Set legend position
	p.Legend.Top = true
	p.Legend.Left = true

	// Save plot
	if err := p.Save(vg.Length(v.config.Width)*vg.Inch, vg.Length(v.config.Height)*vg.Inch, outputPath); err != nil {
		return fmt.Errorf("failed to save plot: %w", err)
	}

	return nil
}

// generateTickValues generates tick values from min to max with specified spacing
func generateTickValues(min, max, spacing float64) []float64 {
	if spacing <= 0 {
		return nil
	}
	var values []float64
	// Start from the first tick >= min
	start := math.Ceil(min/spacing) * spacing
	for val := start; val <= max+spacing*0.001; val += spacing {
		values = append(values, val)
	}
	return values
}

// GeneratePlotFromFile generates a plot from an interleaved log file
func GeneratePlotFromFile(logPath, configPath, outputPath string) error {
	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Read log file
	file, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Parse log file (simplified - we'll reuse the parser)
	// For now, we'll need to parse the interleaved format
	// This is a simplified version - in production, you'd want to reuse the existing parser
	lines, err := parseInterleavedLog(file)
	if err != nil {
		return fmt.Errorf("failed to parse log file: %w", err)
	}

	// Create visualizer
	viz := NewVisualizer(cfg)

	// Generate plot
	return viz.GeneratePlot(lines, outputPath)
}

// parseInterleavedLog parses an interleaved log file
// This is a simplified parser - in production, you might want to reuse the existing parser
func parseInterleavedLog(file *os.File) ([]*parser.LogLine, error) {
	// This would need to parse the interleaved format
	// For now, returning empty - we'll implement this or reuse existing parsing logic
	return nil, fmt.Errorf("not implemented - use Process() from interleaver instead")
}

// parseColor parses a color string (e.g., "blue", "red", "#FF0000") and returns a color.Color
func parseColor(colorStr string) color.Color {
	colorStr = strings.ToLower(strings.TrimSpace(colorStr))

	// Named colors
	switch colorStr {
	case "black":
		return color.RGBA{R: 0, G: 0, B: 0, A: 255}
	case "white":
		return color.RGBA{R: 255, G: 255, B: 255, A: 255}
	case "red":
		return color.RGBA{R: 255, G: 0, B: 0, A: 255}
	case "green":
		return color.RGBA{R: 0, G: 255, B: 0, A: 255}
	case "blue":
		return color.RGBA{R: 0, G: 0, B: 255, A: 255}
	case "yellow":
		return color.RGBA{R: 255, G: 255, B: 0, A: 255}
	case "cyan":
		return color.RGBA{R: 0, G: 255, B: 255, A: 255}
	case "magenta":
		return color.RGBA{R: 255, G: 0, B: 255, A: 255}
	case "orange":
		return color.RGBA{R: 255, G: 165, B: 0, A: 255}
	case "purple":
		return color.RGBA{R: 128, G: 0, B: 128, A: 255}
	case "brown":
		return color.RGBA{R: 165, G: 42, B: 42, A: 255}
	case "pink":
		return color.RGBA{R: 255, G: 192, B: 203, A: 255}
	case "gray", "grey":
		return color.RGBA{R: 128, G: 128, B: 128, A: 255}
	}

	// Try parsing as hex color (#RRGGBB)
	if strings.HasPrefix(colorStr, "#") && len(colorStr) == 7 {
		var r, g, b uint8
		if _, err := fmt.Sscanf(colorStr, "#%02x%02x%02x", &r, &g, &b); err == nil {
			return color.RGBA{R: r, G: g, B: b, A: 255}
		}
	}

	return nil // Return nil if color cannot be parsed
}
