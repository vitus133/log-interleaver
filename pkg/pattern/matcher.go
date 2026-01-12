package pattern

import (
	"fmt"
	"log-interleaver/internal/parser"
	"regexp"
	"strconv"
	"time"
)

// MetricPoint represents a single data point extracted from a log line
type MetricPoint struct {
	Time      time.Time
	Value     float64
	State     string // Optional state value (e.g., "s0", "s2")
	SeriesName string
}

// PatternMatcher extracts metrics from log lines based on regex patterns
type PatternMatcher struct {
	patterns []CompiledPattern
}

// CompiledPattern is a compiled regex pattern with metadata
type CompiledPattern struct {
	Name           string
	Regex          *regexp.Regexp
	TagFilter      string
	ValueGroup     int
	StateGroup     int
	DeviceGroup    int
	StateMapping   map[string]float64
	ValueMultiplier float64
	ConvertNanosecondOffset bool
	Color          string
	LineStyle      string
	Marker         string
	YAxisLabel     string
	YAxisIndex     int
}

// NewPatternMatcher creates a new pattern matcher from configuration
func NewPatternMatcher(patterns []PatternConfig) (*PatternMatcher, error) {
	compiled := make([]CompiledPattern, 0, len(patterns))

	for _, p := range patterns {
		regex, err := regexp.Compile(p.Regex)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern '%s': %w", p.Regex, err)
		}

		compiled = append(compiled, CompiledPattern{
			Name:           p.Name,
			Regex:          regex,
			TagFilter:      p.TagFilter,
			ValueGroup:     p.ValueGroup,
			StateGroup:     p.StateGroup,
			DeviceGroup:    p.DeviceGroup,
			StateMapping:   p.StateMapping,
			ValueMultiplier: p.ValueMultiplier,
			ConvertNanosecondOffset: p.ConvertNanosecondOffset,
			Color:          p.Color,
			LineStyle:      p.LineStyle,
			Marker:         p.Marker,
			YAxisLabel:     p.YAxisLabel,
			YAxisIndex:     p.YAxisIndex,
		})
	}

	return &PatternMatcher{patterns: compiled}, nil
}

// PatternConfig is the configuration for a pattern (imported from config package)
type PatternConfig struct {
	Name           string
	Regex          string
	TagFilter      string
	ValueGroup     int
	StateGroup     int
	DeviceGroup    int
	StateMapping   map[string]float64
	ValueMultiplier float64
	ConvertNanosecondOffset bool
	Color          string
	LineStyle      string
	Marker         string
	YAxisLabel     string
	YAxisIndex     int
}

// ExtractMetrics processes log lines and extracts metrics based on patterns
func (pm *PatternMatcher) ExtractMetrics(lines []*parser.LogLine) (map[string][]MetricPoint, error) {
	metrics := make(map[string][]MetricPoint)

	for _, line := range lines {
		// Skip lines without timestamps
		if line.Timestamp == nil {
			continue
		}

		// Try each pattern
		for _, pattern := range pm.patterns {
			// Check tag filter
			if pattern.TagFilter != "" && line.Tag != pattern.TagFilter {
				continue
			}

			// Match pattern
			matches := pattern.Regex.FindStringSubmatch(line.OriginalLine)
			if len(matches) == 0 {
				continue
			}

			// Extract value
			if pattern.ValueGroup >= len(matches) {
				continue
			}

			valueStr := matches[pattern.ValueGroup]
			
			// Extract state if configured
			state := ""
			if pattern.StateGroup > 0 && pattern.StateGroup < len(matches) {
				state = matches[pattern.StateGroup]
			}
			
			// Extract device if configured
			device := ""
			if pattern.DeviceGroup > 0 && pattern.DeviceGroup < len(matches) {
				device = matches[pattern.DeviceGroup]
			}
			
			var value float64
			var valueParsed bool
			
			// If this is a state series (state_group is set and matches value_group), handle state mapping first
			if pattern.StateGroup > 0 && pattern.StateGroup == pattern.ValueGroup {
				// This is a state series - use state mapping or extract from state string
				if pattern.StateMapping != nil {
					if mappedValue, ok := pattern.StateMapping[valueStr]; ok {
						value = mappedValue
						valueParsed = true
					} else {
						// Fallback: try to extract numeric part from state string (e.g., "s0" -> 0)
						if len(valueStr) > 1 && valueStr[0] == 's' {
							if stateVal, err := strconv.ParseFloat(valueStr[1:], 64); err == nil {
								value = stateVal
								valueParsed = true
							}
						}
					}
				} else {
					// No mapping configured, try to extract numeric part (e.g., "s0" -> 0)
					if len(valueStr) > 1 && valueStr[0] == 's' {
						if stateVal, err := strconv.ParseFloat(valueStr[1:], 64); err == nil {
							value = stateVal
							valueParsed = true
						}
					}
				}
				
				if !valueParsed {
					continue // Skip if we can't map/parse the state
				}
			} else {
				// Regular numeric value - try to parse as float/int
				// Special handling for nanosecond offset conversion: pad fractional nanoseconds to 9 digits
				if pattern.ConvertNanosecondOffset {
					// Pad the fractional part to 9 digits (nanoseconds)
					for len(valueStr) < 9 {
						valueStr = valueStr + "0"
					}
					if len(valueStr) > 9 {
						valueStr = valueStr[:9]
					}
				}
				
				var err error
				value, err = strconv.ParseFloat(valueStr, 64)
				if err != nil {
					// Try parsing as integer first
					if intVal, err2 := strconv.ParseInt(valueStr, 10, 64); err2 == nil {
						value = float64(intVal)
						valueParsed = true
					} else {
						continue // Skip if we can't parse the value
					}
				} else {
					valueParsed = true
				}
			}

			// Convert nanosecond offset if configured (for fractional nanoseconds >= 500000000)
			if pattern.ConvertNanosecondOffset {
				// If value is >= 500000000 (half a second in nanoseconds), subtract 1000000000 to get negative offset
				if value >= 500000000 && value < 1000000000 {
					value = value - 1000000000
				}
			}

			// Apply value multiplier if configured (e.g., convert ps to ns)
			// Apply multiplier if it's set (not zero and not identity)
			// Note: 0.001 is used to convert picoseconds to nanoseconds
			if pattern.ValueMultiplier != 0 && pattern.ValueMultiplier != 1.0 {
				value = value * pattern.ValueMultiplier
			}

			// Determine series name: if device is extracted, append it to the pattern name
			seriesName := pattern.Name
			if device != "" {
				seriesName = fmt.Sprintf("%s %s", pattern.Name, device)
			}

			point := MetricPoint{
				Time:       line.Timestamp.Time,
				Value:      value,
				State:      state,
				SeriesName: seriesName,
			}

			metrics[seriesName] = append(metrics[seriesName], point)
		}
	}

	return metrics, nil
}
