package interleaver

import (
	"bufio"
	"fmt"
	"log-interleaver/internal/parser"
	"log-interleaver/pkg/timestamp"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Interleaver merges and sorts log files by timestamp
type Interleaver struct {
	logDir      string
	fileOffsets map[string]time.Duration // Offset per file tag (in hours, converted to duration)
	autoAlign   bool                     // Whether to automatically align timezones
}

// NewInterleaver creates a new interleaver for the given log directory
func NewInterleaver(logDir string) *Interleaver {
	return &Interleaver{
		logDir:      logDir,
		fileOffsets: make(map[string]time.Duration),
		autoAlign:   true,
	}
}

// SetFileOffset sets a manual offset (in hours) for a specific file tag
func (i *Interleaver) SetFileOffset(tag string, hours float64) {
	i.fileOffsets[tag] = time.Duration(hours * float64(time.Hour))
}

// SetAutoAlign enables or disables automatic timezone alignment
func (i *Interleaver) SetAutoAlign(enabled bool) {
	i.autoAlign = enabled
}

// Process reads all log files, parses them, resolves timestamps, and returns sorted log lines
func (i *Interleaver) Process() ([]*parser.LogLine, error) {
	// Read all log files
	files, err := os.ReadDir(i.logDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read log directory: %w", err)
	}

	// Map to store lines by tag
	linesByTag := make(map[string][]*parser.LogLine)

	// Process each log file
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Skip non-text files
		if !strings.HasSuffix(file.Name(), ".txt") {
			continue
		}

		// Extract tag from filename (remove .txt extension)
		tag := strings.TrimSuffix(file.Name(), ".txt")

		filePath := filepath.Join(i.logDir, file.Name())
		lines, err := i.parseFile(filePath, tag)
		if err != nil {
			return nil, fmt.Errorf("failed to parse file %s: %w", file.Name(), err)
		}

		linesByTag[tag] = lines
	}

	// Resolve uptime timestamps for all tags that have uptime timestamps
	for _, lines := range linesByTag {
		// Check if this tag has any uptime timestamps
		hasUptime := false
		for _, line := range lines {
			if line.UptimeSec > 0 {
				hasUptime = true
				break
			}
		}
		if hasUptime {
			if err := parser.ResolveUptimeTimestamps(lines); err != nil {
				// Log warning but continue - some tags might not have absolute timestamps
				// This is okay if the tag doesn't need uptime resolution
				continue
			}
		}
	}

	// Calculate automatic offsets if enabled
	if i.autoAlign {
		if err := i.calculateAutoOffsets(linesByTag); err != nil {
			return nil, fmt.Errorf("failed to calculate auto offsets: %w", err)
		}
	}

	// Apply offsets to all lines
	var allLines []*parser.LogLine
	for tag, lines := range linesByTag {
		offset := i.fileOffsets[tag]
		for _, line := range lines {
			if line.Timestamp != nil {
				line.Timestamp.Time = line.Timestamp.Time.Add(offset)
			}
			allLines = append(allLines, line)
		}
	}

	// Sort by timestamp
	sort.Slice(allLines, func(i, j int) bool {
		tsI := allLines[i].GetTimestamp()
		tsJ := allLines[j].GetTimestamp()

		// Lines without timestamps go to the end
		if tsI == nil && tsJ == nil {
			return allLines[i].LineNumber < allLines[j].LineNumber
		}
		if tsI == nil {
			return false
		}
		if tsJ == nil {
			return true
		}

		return tsI.Time.Before(tsJ.Time)
	})

	return allLines, nil
}

// calculateAutoOffsets calculates timezone offsets automatically based on first timestamps
// Prefers daemon as reference, otherwise uses the file with the most timestamps
func (i *Interleaver) calculateAutoOffsets(linesByTag map[string][]*parser.LogLine) error {
	// Prefer daemon as reference, otherwise find the file with the most timestamps
	var referenceTime *time.Time
	var referenceTag string

	// First, try to use daemon as reference
	if daemonLines, ok := linesByTag["daemon"]; ok {
		for _, line := range daemonLines {
			if line.Timestamp != nil {
				if referenceTime == nil || line.Timestamp.Time.Before(*referenceTime) {
					refTime := line.Timestamp.Time
					referenceTime = &refTime
					referenceTag = "daemon"
				}
			}
		}
	}

	// If daemon not found or has no timestamps, use the file with most timestamps
	if referenceTime == nil {
		maxTimestampCount := 0
		for tag, lines := range linesByTag {
			count := 0
			var firstTime *time.Time
			for _, line := range lines {
				if line.Timestamp != nil {
					count++
					if firstTime == nil || line.Timestamp.Time.Before(*firstTime) {
						ft := line.Timestamp.Time
						firstTime = &ft
					}
				}
			}
			if count > maxTimestampCount && firstTime != nil {
				maxTimestampCount = count
				referenceTime = firstTime
				referenceTag = tag
			}
		}
	}

	if referenceTime == nil {
		// No timestamps found, nothing to align
		return nil
	}

	// Calculate offsets for each tag (skip reference tag)
	for tag, lines := range linesByTag {
		// Skip if manual offset already set
		if _, hasManual := i.fileOffsets[tag]; hasManual {
			continue
		}

		// Skip reference tag (no offset needed)
		if tag == referenceTag {
			continue
		}

		// Find first timestamp in this file
		var firstTime *time.Time
		for _, line := range lines {
			if line.Timestamp != nil {
				if firstTime == nil || line.Timestamp.Time.Before(*firstTime) {
					ft := line.Timestamp.Time
					firstTime = &ft
				}
			}
		}

		if firstTime != nil {
			// Calculate offset needed to align with reference
			offset := referenceTime.Sub(*firstTime)
			// Round to nearest hour for cleaner alignment
			offsetHours := offset.Hours()
			roundedHours := float64(int(offsetHours + 0.5))
			i.fileOffsets[tag] = time.Duration(roundedHours * float64(time.Hour))
		}
	}

	return nil
}

// parseFile reads and parses a single log file
func (i *Interleaver) parseFile(filePath, tag string) ([]*parser.LogLine, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	p := parser.NewParser(tag)
	var lines []*parser.LogLine

	scanner := bufio.NewScanner(file)
	lineNum := 1
	for scanner.Scan() {
		line := scanner.Text()
		logLine := p.ParseLine(line, lineNum)
		lines = append(lines, logLine)
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return lines, nil
}

// FormatLine formats a log line for output with timestamp prefix
func FormatLine(line *parser.LogLine) string {
	ts := line.GetTimestamp()
	if ts == nil {
		// Lines without timestamps keep original format
		return line.OriginalLine
	}

	timeStr := timestamp.FormatTimestamp(ts.Time)
	return fmt.Sprintf("%s %s %s", timeStr, line.Tag, line.OriginalLine)
}
