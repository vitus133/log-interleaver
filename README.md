# Log Interleaver

A Go-based tool for interleaving and analyzing PTP (Precision Time Protocol) logs from multiple sources with different timestamp formats.

## Features

- **Multi-format timestamp parsing**: Supports uptime, absolute, Linux/Unix timestamp, and full date-time formats
- **Automatic timestamp resolution**: Resolves uptime timestamps to absolute timestamps by finding the nearest absolute timestamp
- **Log interleaving**: Merges logs from multiple files and sorts them chronologically
- **Tag-based identification**: Each log line is tagged with its source filename
- **Basic analysis**: Provides statistics about log coverage and distribution

## Supported Timestamp Formats

1. **Uptime format**: `ptp4l[275401.719]: ...` or `ts2phc[275401.719]: ...`
   - Resolved using the nearest absolute timestamp

2. **Absolute format**: `I0111 14:05:54.000549  644511 stats.go:65] ...`
   - Format: `[IEWD][MMDD HH:MM:SS.microseconds]`

3. **Linux/Unix timestamp**: `T-BC[1768140354]:[ts2phc.1.config] ...`
   - Unix epoch timestamp in brackets

4. **Full date-time**: `2026-01-11 09:04:29 E825-NAC ptp4l[1138494.080]: ...`
   - Format: `YYYY-MM-DD HH:MM:SS`

## Usage

```bash
# Build the tool
go build ./cmd/log-interleaver

# Interleave logs from the logs directory (output to stdout)
./log-interleaver -logs logs

# Save output to a file
./log-interleaver -logs logs -output interleaved.log

# Run with analysis
./log-interleaver -logs logs -analyze -output interleaved.log
```

## Command-line Options

- `-logs <directory>`: Directory containing log files (default: `logs`)
- `-output <file>`: Output file path (default: stdout)
- `-analyze`: Run basic analysis on the interleaved logs
- `-no-auto-align`: Disable automatic timezone alignment (default: auto-align enabled)
- `-offset <spec>`: Manual timezone offsets in format `tag:hours,tag:hours` (e.g., `e825:5,e830:5`). Manual offsets override automatic alignment for specified files.

## Output Format

Each line in the interleaved output follows this format:

```
[absolute_timestamp] [tag] [original_line]
```

Example:
```
14:05:54.000549 daemon ts2phc[275401.719]: [ts2phc.1.config:6] eno16495 offset          0 s2 freq      -0
```

Where:
- `14:05:54.000549` is the resolved absolute timestamp
- `daemon` is the tag derived from the source filename (`daemon.txt`)
- The rest is the original log line

## Architecture

The tool follows Clean Architecture principles:

- `cmd/log-interleaver/`: Application entrypoint
- `internal/parser/`: Log parsing and timestamp resolution logic
- `internal/interleaver/`: Log merging and sorting logic
- `pkg/timestamp/`: Timestamp parsing utilities

## How Uptime Resolution Works

Uptime timestamps are resolved by:

1. Finding all absolute timestamps in the log file
2. For each uptime timestamp, locating the nearest absolute timestamp (preferring forward-looking)
3. If the reference absolute timestamp has an associated uptime, calculating the time difference
4. Otherwise, using the reference absolute timestamp directly

This matches the pattern where uptime timestamps are typically followed by absolute timestamps in the same log stream.

## Timezone Alignment

The tool automatically aligns timezones across log files by:

1. Using the `daemon` log file as the reference (or the file with the most timestamps if daemon is not available)
2. Finding the first timestamp in each log file
3. Calculating the offset needed to align all files to the reference timezone
4. Applying the offset (rounded to the nearest hour) to all timestamps in each file

You can disable automatic alignment with `-no-auto-align` and manually specify offsets using `-offset`:

```bash
# Manually set 5-hour offset for e825 and e830 files
./log-interleaver -logs logs -no-auto-align -offset e825:5,e830:5
```
