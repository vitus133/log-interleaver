package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// PatternConfig defines a pattern for extracting metrics from log lines
type PatternConfig struct {
	Name         string            `yaml:"name"`          // Series name (e.g., "E830 offset")
	Regex        string            `yaml:"regex"`        // Regex pattern to match
	TagFilter    string            `yaml:"tag_filter"`   // Optional: filter by log tag (e.g., "e830", "daemon")
	ValueGroup   int               `yaml:"value_group"`  // Regex capture group index for the value
	StateGroup   int               `yaml:"state_group"`  // Optional: regex capture group for state (e.g., s0, s2)
	StateMapping map[string]float64 `yaml:"state_mapping"` // Optional: map state strings to numeric values (e.g., {"s0": 10, "s1": 20})
	Color        string            `yaml:"color"`         // Optional: matplotlib color
	LineStyle    string            `yaml:"line_style"`   // Optional: matplotlib line style (e.g., "-", "--", ".")
	Marker       string            `yaml:"marker"`       // Optional: matplotlib marker (e.g., ".", "o", "x")
	Step         bool              `yaml:"step"`         // Optional: if true, use step plot (hold value between points)
	YAxisLabel   string            `yaml:"yaxis_label"`  // Optional: Y-axis label for this series
	YAxisIndex   int               `yaml:"yaxis_index"`  // Optional: which Y-axis to use (0=left, 1=right)
}

// VisualizationConfig contains all pattern configurations
type VisualizationConfig struct {
	Title      string          `yaml:"title"`
	XAxisLabel string          `yaml:"xaxis_label"`
	YAxisLabel string          `yaml:"yaxis_label"`
	Width      int             `yaml:"width"`
	Height     int             `yaml:"height"`
	DPI        int             `yaml:"dpi"`
	Patterns   []PatternConfig `yaml:"patterns"`
}

// LoadConfig loads visualization configuration from a YAML file
func LoadConfig(configPath string) (*VisualizationConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config VisualizationConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if config.Title == "" {
		config.Title = "PTP Log Analysis"
	}
	if config.XAxisLabel == "" {
		config.XAxisLabel = "Time"
	}
	if config.YAxisLabel == "" {
		config.YAxisLabel = "Value"
	}
	if config.Width == 0 {
		config.Width = 12
	}
	if config.Height == 0 {
		config.Height = 8
	}
	if config.DPI == 0 {
		config.DPI = 100
	}

	return &config, nil
}
