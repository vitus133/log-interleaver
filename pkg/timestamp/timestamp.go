package timestamp

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// Timestamp represents a parsed timestamp with its type
type Timestamp struct {
	Time      time.Time
	Type      Type
	UptimeSec float64 // For uptime timestamps, store the uptime value
}

// Type represents the type of timestamp
type Type int

const (
	TypeUnknown Type = iota
	TypeAbsolute
	TypeUptime
	TypeLinux // Unix timestamp
)

// ParseAbsolute parses absolute timestamp format: "I0111 14:03:55.976211" or "E0111 14:03:55.976211"
// Format: [IEWD][MMDD HH:MM:SS.microseconds]
func ParseAbsolute(line string) (*Timestamp, error) {
	// Pattern: I/E/W/D followed by MMDD HH:MM:SS.microseconds
	re := regexp.MustCompile(`^[IEWD](\d{4})\s+(\d{2}):(\d{2}):(\d{2})\.(\d{6})`)
	matches := re.FindStringSubmatch(line)
	if len(matches) != 6 {
		return nil, fmt.Errorf("invalid absolute timestamp format")
	}

	month, _ := strconv.Atoi(matches[1][:2])
	day, _ := strconv.Atoi(matches[1][2:])
	hour, _ := strconv.Atoi(matches[2])
	min, _ := strconv.Atoi(matches[3])
	sec, _ := strconv.Atoi(matches[4])
	micro, _ := strconv.Atoi(matches[5])

	// Assume current year (or we could parse from context)
	now := time.Now()
	t := time.Date(now.Year(), time.Month(month), day, hour, min, sec, micro*1000, time.UTC)

	return &Timestamp{
		Time: t,
		Type: TypeAbsolute,
	}, nil
}

// ParseUptime parses uptime timestamp format: "ptp4l[275313.748]:"
// Returns the uptime value in seconds
func ParseUptime(line string) (float64, bool) {
	// Pattern: [number.number]:
	re := regexp.MustCompile(`\[(\d+)\.(\d+)\]:`)
	matches := re.FindStringSubmatch(line)
	if len(matches) != 3 {
		return 0, false
	}

	sec, err1 := strconv.Atoi(matches[1])
	msec, err2 := strconv.Atoi(matches[2])
	if err1 != nil || err2 != nil {
		return 0, false
	}

	uptime := float64(sec) + float64(msec)/1000.0
	return uptime, true
}

// ParseLinux parses Linux/Unix timestamp format: "T-BC[1768140305]:"
func ParseLinux(line string) (*Timestamp, error) {
	// Pattern: [unix_timestamp]:
	re := regexp.MustCompile(`\[(\d+)\]:`)
	matches := re.FindStringSubmatch(line)
	if len(matches) != 2 {
		return nil, fmt.Errorf("invalid linux timestamp format")
	}

	unix, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid unix timestamp: %w", err)
	}

	return &Timestamp{
		Time: time.Unix(unix, 0),
		Type: TypeLinux,
	}, nil
}

// ParseFullDateTime parses full date-time format: "2026-01-11 09:04:29"
func ParseFullDateTime(line string) (*Timestamp, error) {
	// Pattern: YYYY-MM-DD HH:MM:SS
	re := regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})\s+(\d{2}):(\d{2}):(\d{2})`)
	matches := re.FindStringSubmatch(line)
	if len(matches) != 7 {
		return nil, fmt.Errorf("invalid full date-time format")
	}

	year, _ := strconv.Atoi(matches[1])
	month, _ := strconv.Atoi(matches[2])
	day, _ := strconv.Atoi(matches[3])
	hour, _ := strconv.Atoi(matches[4])
	min, _ := strconv.Atoi(matches[5])
	sec, _ := strconv.Atoi(matches[6])

	t := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.UTC)

	return &Timestamp{
		Time: t,
		Type: TypeAbsolute,
	}, nil
}

// ParseJSONTimestamp parses JSON ISO timestamp format: "timestamp":"2026-01-12T11:36:14.788270397Z"
func ParseJSONTimestamp(line string) (*Timestamp, error) {
	// Pattern: "timestamp":"YYYY-MM-DDTHH:MM:SS.nanosecondsZ"
	re := regexp.MustCompile(`"timestamp"\s*:\s*"(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})\.(\d+)Z"`)
	matches := re.FindStringSubmatch(line)
	if len(matches) != 8 {
		return nil, fmt.Errorf("invalid JSON timestamp format")
	}

	year, _ := strconv.Atoi(matches[1])
	month, _ := strconv.Atoi(matches[2])
	day, _ := strconv.Atoi(matches[3])
	hour, _ := strconv.Atoi(matches[4])
	min, _ := strconv.Atoi(matches[5])
	sec, _ := strconv.Atoi(matches[6])
	nanosStr := matches[7]

	// Parse nanoseconds (can be variable length, pad to 9 digits)
	nanos := 0
	if len(nanosStr) > 0 {
		// Pad or truncate to 9 digits
		if len(nanosStr) > 9 {
			nanosStr = nanosStr[:9]
		}
		// Pad with zeros if needed
		for len(nanosStr) < 9 {
			nanosStr += "0"
		}
		nanos, _ = strconv.Atoi(nanosStr)
	}

	t := time.Date(year, time.Month(month), day, hour, min, sec, nanos, time.UTC)

	return &Timestamp{
		Time: t,
		Type: TypeAbsolute,
	}, nil
}

// FormatTimestamp formats a timestamp for output: "14:05:54.000549"
func FormatTimestamp(t time.Time) string {
	return t.Format("15:04:05.000000")
}
