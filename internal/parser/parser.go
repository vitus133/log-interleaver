package parser

import (
	"fmt"
	"log-interleaver/pkg/timestamp"
	"time"
)

// LogLine represents a parsed log line with its timestamp information
type LogLine struct {
	OriginalLine string
	Tag          string // Derived from filename (e.g., "daemon", "e825", "e830")
	Timestamp    *timestamp.Timestamp
	UptimeSec    float64 // For uptime lines, store the uptime value
	LineNumber   int
}

// GetTimestamp returns the timestamp, or nil if not available
func (l *LogLine) GetTimestamp() *timestamp.Timestamp {
	return l.Timestamp
}

// Parser parses log lines and extracts timestamp information
type Parser struct {
	tag string
}

// NewParser creates a new parser for a specific log file tag
func NewParser(tag string) *Parser {
	return &Parser{tag: tag}
}

// ParseLine parses a single log line and extracts timestamp information
func (p *Parser) ParseLine(line string, lineNum int) *LogLine {
	logLine := &LogLine{
		OriginalLine: line,
		Tag:          p.tag,
		LineNumber:   lineNum,
	}

	// Try to parse different timestamp formats
	// 1. Try absolute format (I0111 14:03:55.976211)
	if ts, err := timestamp.ParseAbsolute(line); err == nil {
		logLine.Timestamp = ts
		return logLine
	}

	// 2. Try full date-time format (2026-01-11 09:04:29)
	if ts, err := timestamp.ParseFullDateTime(line); err == nil {
		logLine.Timestamp = ts
		return logLine
	}

	// 3. Try Linux/Unix timestamp format (T-BC[1768140305]:)
	if ts, err := timestamp.ParseLinux(line); err == nil {
		logLine.Timestamp = ts
		return logLine
	}

	// 4. Try uptime format (ptp4l[275313.748]:)
	if uptime, ok := timestamp.ParseUptime(line); ok {
		logLine.UptimeSec = uptime
		// Timestamp will be resolved later using nearest absolute timestamp
		return logLine
	}

	// No timestamp found - this line will need to inherit from previous line
	return logLine
}

// ResolveUptimeTimestamps resolves uptime timestamps by finding the nearest absolute timestamp
// This function processes all log lines and converts uptime timestamps to absolute timestamps
func ResolveUptimeTimestamps(lines []*LogLine) error {
	// First pass: collect all absolute timestamps with their line numbers and uptimes
	type absTimestamp struct {
		lineNum   int
		time      time.Time
		uptime    float64
		hasUptime bool
	}

	var absTimestamps []absTimestamp
	for i, line := range lines {
		if line.Timestamp != nil && line.Timestamp.Type == timestamp.TypeAbsolute {
			abs := absTimestamp{
				lineNum: i,
				time:    line.Timestamp.Time,
			}
			// Check if there's an uptime timestamp nearby (within a few lines)
			// Look backward for uptime
			for j := i - 1; j >= 0 && j >= i-5; j-- {
				if lines[j].UptimeSec > 0 {
					abs.uptime = lines[j].UptimeSec
					abs.hasUptime = true
					break
				}
			}
			// If not found backward, look forward
			if !abs.hasUptime {
				for j := i + 1; j < len(lines) && j <= i+5; j++ {
					if lines[j].UptimeSec > 0 {
						abs.uptime = lines[j].UptimeSec
						abs.hasUptime = true
						break
					}
				}
			}
			absTimestamps = append(absTimestamps, abs)
		}
	}

	if len(absTimestamps) == 0 {
		return fmt.Errorf("no absolute timestamps found to resolve uptime timestamps")
	}

	// Second pass: resolve uptime timestamps
	for i, line := range lines {
		if line.UptimeSec > 0 && line.Timestamp == nil {
			// Find the nearest absolute timestamp
			// Prefer forward-looking (as in the example: uptime line followed by absolute timestamp)
			var nearestAbs *absTimestamp
			var forwardAbs *absTimestamp
			var backwardAbs *absTimestamp
			minDist := len(lines)
			forwardDist := len(lines)
			backwardDist := len(lines)

			// Check all absolute timestamps to find the nearest
			for j := range absTimestamps {
				dist := abs(absTimestamps[j].lineNum - i)
				if dist < minDist {
					minDist = dist
					nearestAbs = &absTimestamps[j]
				}
				// Track forward and backward separately
				if absTimestamps[j].lineNum > i && dist < forwardDist {
					forwardDist = dist
					forwardAbs = &absTimestamps[j]
				}
				if absTimestamps[j].lineNum < i && dist < backwardDist {
					backwardDist = dist
					backwardAbs = &absTimestamps[j]
				}
			}

			// Prefer forward absolute timestamp if available and close
			// Otherwise use backward, or fallback to nearest
			var refAbs *absTimestamp
			if forwardAbs != nil && forwardDist <= 10 {
				refAbs = forwardAbs
			} else if backwardAbs != nil && backwardDist <= 10 {
				refAbs = backwardAbs
			} else if nearestAbs != nil {
				refAbs = nearestAbs
			}

			if refAbs == nil {
				continue
			}

			// If the reference absolute timestamp has an associated uptime, calculate the offset
			if refAbs.hasUptime {
				uptimeDiff := line.UptimeSec - refAbs.uptime
				// Convert uptime difference to time difference
				// Uptime is in seconds, so we can directly add the difference
				resolvedTime := refAbs.time.Add(time.Duration(uptimeDiff * float64(time.Second)))
				line.Timestamp = &timestamp.Timestamp{
					Time:      resolvedTime,
					Type:      timestamp.TypeAbsolute,
					UptimeSec: line.UptimeSec,
				}
			} else {
				// Fallback: use the reference absolute timestamp directly
				// This matches the example where uptime 275401.719 uses absolute time 14:05:54.000549
				line.Timestamp = &timestamp.Timestamp{
					Time:      refAbs.time,
					Type:      timestamp.TypeAbsolute,
					UptimeSec: line.UptimeSec,
				}
			}
		}
	}

	return nil
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
