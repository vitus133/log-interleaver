package visualizer

import (
	"fmt"
	"image/color"
	"log-interleaver/internal/config"
	"log-interleaver/internal/parser"
	"log-interleaver/pkg/pattern"
	"os"
	"sort"
	"strings"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
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
			Name:         p.Name,
			Regex:        p.Regex,
			TagFilter:    p.TagFilter,
			ValueGroup:   p.ValueGroup,
			StateGroup:   p.StateGroup,
			StateMapping: p.StateMapping,
			Color:        p.Color,
			LineStyle:    p.LineStyle,
			Marker:       p.Marker,
			YAxisLabel:   p.YAxisLabel,
			YAxisIndex:   p.YAxisIndex,
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

	// Group series by Y-axis index
	seriesByAxis := make(map[int][]string)
	for _, pattern := range v.config.Patterns {
		axisIdx := pattern.YAxisIndex
		if axisIdx < 0 {
			axisIdx = 0
		}
		if _, ok := seriesByAxis[axisIdx]; !ok {
			seriesByAxis[axisIdx] = make([]string, 0)
		}
		seriesByAxis[axisIdx] = append(seriesByAxis[axisIdx], pattern.Name)
	}
	

	// Create secondary Y-axis if needed
	var rightAxis *plot.Axis
	if len(seriesByAxis) > 1 {
		rightAxis = &plot.Axis{}
		p.Y.Label.Text = v.config.YAxisLabel // Left axis label
	}

	// Plot each series
	colors := []color.Color{
		color.RGBA{R: 31, G: 119, B: 180, A: 255}, // blue
		color.RGBA{R: 255, G: 127, B: 14, A: 255},  // orange
		color.RGBA{R: 44, G: 160, B: 44, A: 255},  // green
		color.RGBA{R: 214, G: 39, B: 40, A: 255},  // red
		color.RGBA{R: 148, G: 103, B: 189, A: 255}, // purple
		color.RGBA{R: 140, G: 86, B: 75, A: 255},  // brown
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
			var patternCfg *config.PatternConfig
			for i := range v.config.Patterns {
				if v.config.Patterns[i].Name == seriesName {
					patternCfg = &v.config.Patterns[i]
					break
				}
			}

			// Sort points by time
			sort.Slice(points, func(i, j int) bool {
				return points[i].Time.Before(points[j].Time)
			})

			// Convert to plotter.XYs
			xy := make(plotter.XYs, len(points))
			startTime := points[0].Time
			for i, pt := range points {
				xy[i].X = pt.Time.Sub(startTime).Seconds()
				xy[i].Y = pt.Value
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
			if patternCfg != nil && patternCfg.Marker != "" {
				// Adjust marker size based on type
				switch patternCfg.Marker {
				case "x", "+":
					markerRadius = vg.Points(4)
				case "o":
					markerRadius = vg.Points(3)
				}
			}

			// Create scatter plot (dots)
			scatter, err := plotter.NewScatter(xy)
			if err != nil {
				return fmt.Errorf("failed to create scatter plot: %w", err)
			}
			scatter.GlyphStyle.Radius = markerRadius
			scatter.GlyphStyle.Color = plotColor

			// Create line plot
			lineStyle := plotter.DefaultLineStyle
			if patternCfg != nil && patternCfg.LineStyle != "" {
				switch patternCfg.LineStyle {
				case "-":
					lineStyle = plotter.DefaultLineStyle
				case "--":
					lineStyle.Dashes = []vg.Length{vg.Points(5), vg.Points(5)}
				case ":":
					lineStyle.Dashes = []vg.Length{vg.Points(2), vg.Points(2)}
				case "-.":
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

			// Add to plot
			p.Add(scatter, line)
			p.Legend.Add(seriesName, scatter, line)

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
