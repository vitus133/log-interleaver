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

### Capturing the logs

To capture the logs, you can use the following commands:
On the monitor machine:
```bash
ptp4l -f ptp4l.mon -i ens2f0 -m --slaveOnly=1 --free_running=1 | awk '{ print strftime("%Y-%m-%d %H:%M:%S"), "E825", $0; fflush(); }'  |tee e825.txt
ptp4l -f ptp4l.mon -i ens2f2 -m --slaveOnly=1 --free_running=1 | awk '{ print strftime("%Y-%m-%d %H:%M:%S"), "E830", $0; fflush(); }'  |tee e830.txt
```
On the cluster:
```bash
 oc -c linuxptp-daemon-container logs ds/linuxptp-daemon > daemon.txt
 ```
 
### Using the tool

```bash
# Build the tool
go build ./cmd/log-interleaver

# Interleave logs from the logs directory (output to stdout)
./log-interleaver -logs logs

# Save output to a file
./log-interleaver -logs logs -output interleaved.log

# Run with analysis
./log-interleaver -logs logs -analyze -output interleaved.log

# Generate visualization plot
./log-interleaver -logs logs -visualize -config config.yaml -plot-output plot.png

# Combine interleaving and visualization
./log-interleaver -logs logs -output interleaved.log -visualize -config config.yaml  -export-html plot.html
```

## Command-line Options

- `-logs <directory>`: Directory containing log files (default: `logs`)
- `-output <file>`: Output file path (default: stdout)
- `-analyze`: Run basic stats on the interleaved logs
- `-no-auto-align`: Disable automatic timezone alignment (default: auto-align enabled)
- `-offset <spec>`: Manual timezone offsets in format `tag:hours,tag:hours` (e.g., `e825:5,e830:5`). Manual offsets override automatic alignment for specified files.
- `-visualize`: Generate visualization plot from interleaved logs
- `-config <file>`: Path to visualization configuration file (YAML format, default: `config.yaml`)
- `-plot-output <file>`: Output path for plot image (default: `plot.png`)
- `-export-csv <file>`: Export time series data to CSV format (for use in Excel, Python pandas, etc.)
- `-export-json <file>`: Export time series data to JSON format
- `-export-html <file>`: Export interactive HTML plot using Plotly.js (allows zooming, panning, and interactive exploration)

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

## Visualization

The tool can generate time-series plots from log data using configurable regex patterns. Each pattern extracts specific metrics (like offset, delay, state) and displays them as separate series on the plot.

### Example Plot

![T-BC Log Analysis Plot](plot.png)

*Note: For an interactive version with zooming, panning, and hover details, generate the HTML plot (see [Interactive Visualization](#interactive-visualization) section below) and open it in a web browser.*

### Configuration File Format

Create a YAML configuration file (see `config.example.yaml` for a complete example):

```yaml
title: "T-BC Log Analysis"
xaxis_label: "Time (seconds from start)"
yaxis_label: "Value"
width: 16
height: 10
dpi: 100

patterns:
  - name: "E830 offset"
    regex: 'E830 ptp4l\[.*\]: master offset\s+(-?\d+)\s+s\d+\s+freq'
    tag_filter: "e830"
    value_group: 1
    color: "blue"
    marker: "."
    line_style: "-"
    yaxis_label: "Offset (ns)"
    yaxis_index: 0
```

### Pattern Configuration Fields

- `name`: Series name displayed in the legend
- `regex`: Regular expression pattern to match log lines (use capture groups for values)
- `tag_filter`: Optional filter by log file tag (e.g., "e830", "e825", "daemon")
- `value_group`: Capture group index (1-based) containing the numeric value to extract
- `state_group`: Optional capture group for state values (e.g., "s0", "s2") - if same as value_group, uses state mapping
- `state_mapping`: Optional map of state strings to numeric values (e.g., `{"s0": 10, "s1": 20, "s2": 30, "s3": 40}`). Required when extracting non-numeric state values. If not provided and state_group matches value_group, will attempt to extract numeric part from state string (e.g., "s0" -> 0).
- `color`: Plot color (named colors like "blue", "red", "green", "orange", "purple", "brown", "cyan", "magenta", "teal", "black", "pink", "gray", or hex like "#FF0000")
- `marker`: Marker style. Supported values:
  - `"."` or `"point"`: Small dot
  - `"o"` or `"circle"`: Circle
  - `"x"` or `"X"`: Cross/X mark
  - `"s"` or `"square"`: Square
  - `"d"` or `"diamond"`: Diamond
  - `"+"`: Plus sign
  - Empty string: No marker (lines only)
- `line_style`: Line style. Supported values:
  - `"-"` or `"solid"`: Solid line (default)
  - `"--"` or `"dashed"`: Dashed line
  - `":"` or `"dotted"`: Dotted line
  - `"-."` or `"dashdot"`: Dash-dot line
  - `"none"`: No line (markers only)
  - Empty string or omitted: Defaults to solid line (even if marker is set)
- `step`: Boolean (optional). If `true`, creates a step plot that holds the Y value horizontally until the next data point, then steps vertically. Useful for discrete state changes or constant values between measurements. Default: `false`
- `yaxis_label`: Y-axis label for this series
- `yaxis_index`: Which Y-axis to use (0=left, 1=right)

### Display Modes

The combination of `marker` and `line_style` determines how the series is displayed:

- **Lines + Markers**: Set both `marker` and `line_style` (or just `line_style` with default marker)
- **Markers Only**: Set `marker` and explicitly set `line_style: "none"`
- **Lines Only**: Set `line_style` and omit `marker` (or set `marker: ""`)

Examples:
```yaml
# Markers only (no connecting lines)
- name: "My Series"
  marker: "o"
  line_style: "none"  # or omit line_style

# Lines only (no markers)
- name: "My Series"
  marker: ""  # or omit marker
  line_style: "--"

# Both lines and markers
- name: "My Series"
  marker: "x"
  line_style: "-"

# Step plot (holds value between points)
- name: "State Series"
  marker: "o"
  line_style: "-"
  step: true  # Creates horizontal-vertical steps
```

### Step Plots

Step plots are useful for visualizing discrete state changes or values that remain constant between measurements. When `step: true` is set:

- The line holds its Y value horizontally until the next data point
- Then it steps vertically to the new value
- This creates a "staircase" effect showing when values change

**Use cases:**
- State machines (e.g., PTP states: s0, s1, s2, s3)
- Lock status changes (unlocked → locked → holdover)
- Discrete configuration changes
- Any metric that represents a constant value between measurements

**Example:**
```yaml
- name: "TR state"
  regex: 'ptp4l\[.*\]: \[ptp4l\.\d+\.config:\d+\] master offset\s+-?\d+\s+(s\d+)\s+freq'
  tag_filter: "daemon"
  value_group: 1
  state_group: 1
  state_mapping:
    s0: 10
    s1: 20
    s2: 30
    s3: 40
  color: "red"
  marker: "o"
  line_style: "-"
  step: true  # Show state transitions as steps
```

### State Mapping Example

For patterns that extract state values (like "s0", "s1", "s2", "s3"), use `state_mapping` to convert them to numeric values:

```yaml
- name: "TR state"
  regex: 'ptp4l\[.*\]: \[ptp4l\.\d+\.config:\d+\] master offset\s+-?\d+\s+(s\d+)\s+freq'
  tag_filter: "daemon"
  value_group: 1
  state_group: 1
  state_mapping:
    s0: 10
    s1: 20
    s2: 30
    s3: 40
  color: "black"
  marker: "x"
  line_style: "--"
```

This will map state "s0" to 10, "s1" to 20, "s2" to 30, and "s3" to 40 in the plot.

### Display Modes: Markers, Lines, or Both

You can control how data points are displayed:

**Markers Only (no connecting lines):**
```yaml
- name: "My Series"
  marker: "o"
  line_style: "none"  # Must explicitly set to "none"
```

**Lines Only (no markers):**
```yaml
- name: "My Series"
  marker: ""  # or omit marker entirely
  line_style: "--"
```

**Both Lines and Markers (default):**
```yaml
- name: "My Series"
  marker: "x"
  line_style: "-"
```

### Additional Pattern Examples

The example configuration includes several additional patterns:

**DPLL Lock Status:**
```yaml
- name: "eno5 lockStatus"
  regex: '"lockStatus":"([^"]+)"[^}]*\}\s+eno5'
  tag_filter: "daemon"
  value_group: 1
  state_group: 1
  state_mapping:
    unlocked: 10
    locked: 20
    "locked-ho-acquired": 30
    holdover: 40
```

**T-BC State:**
```yaml
- name: "T-BC state"
  regex: 'T-BC\[.*\]:\[ts2phc\.\d+\.config\]\s+\w+\s+offset\s+\d+\s+T-BC-STATUS\s+(s[0-2])'
  tag_filter: "daemon"
  value_group: 1
  state_group: 1
  state_mapping:
    s0: 10
    s1: 20
    s2: 30
```

**DPLL Phase Offset:**
```yaml
- name: "eno5 DPLL offset"
  regex: 'dpll\.go:\d+\]\s+setting phase offset to\s+(-?\d+)\s+ns for clock id\s+\d+\s+iface\s+eno5'
  tag_filter: "daemon"
  value_group: 1
```

### Legend Display

When a pattern has `state_mapping` configured, the legend will automatically display the mapping. For example:
- "TR state (s0=10, s1=20, s2=30, s3=40)"
- "eno5 lockStatus (holdover=40, locked=20, locked-ho-acquired=30, unlocked=10)"

This makes it easy to understand what the numeric values represent on the plot.

### Example Usage

```bash
# Generate plot using example configuration
./log-interleaver -logs logs -visualize -config config.example.yaml -plot-output ptp_analysis.png
```

The visualization extracts metrics based on the configured patterns and displays them as time series, making it easy to analyze PTP performance over time.

## Interactive Visualization

For interactive exploration with zooming, panning, and data selection capabilities, use the HTML export option:

```bash
# Generate interactive HTML plot
./log-interleaver -logs logs -export-html plot.html -config config.yaml
```

The HTML file uses Plotly.js and provides:
- **Zoom**: Click and drag to select a region, or use zoom buttons
- **Pan**: Click and drag to move around the plot
- **Reset**: Double-click or use "Reset Zoom" button
- **Toggle Series**: Click on legend items to show/hide series
- **Hover**: Hover over data points to see exact X and Y values
- **Export Data**: Download CSV directly from the browser

### Viewing the Interactive Plot

**Option 1: Local Viewing (Recommended)**
Simply open the generated `plot.html` file in any modern web browser. The file is self-contained and includes all necessary JavaScript libraries via CDN.

**Option 2: GitHub Pages (Optional)**
If you want to view the interactive plot directly from GitHub:
1. Enable GitHub Pages in your repository settings
2. Commit the `plot.html` file to your repository
3. Access it via: `https://<username>.github.io/<repo>/plot.html`

*Note: GitHub README files cannot execute JavaScript, so the static PNG image is shown above. The interactive HTML version must be opened separately.*

## Data Export

You can also export the time series data for use in external tools:

```bash
# Export to CSV (for Excel, Python pandas, etc.)
./log-interleaver -logs logs -export-csv data.csv -config config.yaml

# Export to JSON (for programmatic access)
./log-interleaver -logs logs -export-json data.json -config config.yaml
```

The CSV format includes:
- `Time`: RFC3339 timestamp
- `TimeOffsetSeconds`: Time offset in seconds from the first data point
- One column per series with values at each timestamp

The JSON format includes:
- Metadata (title, axis labels, start time)
- Array of series with X (time offsets) and Y (values) arrays
- State mappings for series that use them

You can load these files into:
- **Python**: Use pandas (`pd.read_csv()`) or json module
- **Excel**: Open CSV directly
- **R**: Use `read.csv()` or `jsonlite`
- **MATLAB**: Use `readtable()` or `jsondecode()`
- **Any plotting library**: Matplotlib, Plotly, D3.js, etc.
