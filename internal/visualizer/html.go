package visualizer

import (
	"fmt"
	"html/template"
	"log-interleaver/internal/config"
	"log-interleaver/internal/parser"
	"os"
)

// GenerateInteractiveHTML generates an interactive HTML plot using Plotly.js
func GenerateInteractiveHTML(lines []*parser.LogLine, configPath, outputPath string) error {
	// Export to JSON first to get the data structure
	jsonPath := outputPath + ".tmp.json"
	if err := ExportJSON(lines, configPath, jsonPath); err != nil {
		return fmt.Errorf("failed to export JSON data: %w", err)
	}
	defer os.Remove(jsonPath)

	// Load configuration for metadata
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Read JSON data
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("failed to read JSON data: %w", err)
	}

	// Generate HTML template
	htmlTemplate := `<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}}</title>
    <script src="https://cdn.plot.ly/plotly-2.27.0.min.js"></script>
    <style>
        body {
            font-family: Arial, sans-serif;
            margin: 20px;
            background-color: #f5f5f5;
        }
        #plotly-div {
            width: 100%;
            height: 800px;
            background-color: white;
            border: 1px solid #ddd;
            border-radius: 5px;
            padding: 10px;
        }
        .controls {
            margin-bottom: 20px;
            padding: 15px;
            background-color: white;
            border-radius: 5px;
            border: 1px solid #ddd;
        }
        .info {
            margin-top: 20px;
            padding: 15px;
            background-color: #e8f4f8;
            border-radius: 5px;
            border-left: 4px solid #2196F3;
        }
        h1 {
            color: #333;
        }
        button {
            background-color: #4CAF50;
            color: white;
            padding: 10px 20px;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            margin-right: 10px;
            font-size: 14px;
        }
        button:hover {
            background-color: #45a049;
        }
    </style>
</head>
<body>
    <h1>{{.Title}}</h1>
    
    <div class="controls">
        <button onclick="resetZoom()">Reset Zoom</button>
        <button onclick="toggleSeries()">Toggle Series Visibility</button>
        <button onclick="exportData()">Export Data (CSV)</button>
    </div>
    
    <div id="plotly-div"></div>
    
    <div class="info">
        <h3>Interactive Features:</h3>
        <ul>
            <li><strong>Zoom:</strong> Click and drag to select a region, or use the zoom buttons in the toolbar</li>
            <li><strong>Pan:</strong> Click and drag to pan around the plot</li>
            <li><strong>Reset:</strong> Double-click to reset zoom, or use the "Reset Zoom" button</li>
            <li><strong>Toggle Series:</strong> Click on series names in the legend to show/hide them</li>
            <li><strong>Hover:</strong> Hover over data points to see exact values</li>
        </ul>
    </div>

    <script>
        const data = {{.JSONData}};
        const series = data.series;
        
        // Prepare Plotly traces
        const traces = series.map((s, idx) => {
            // Build legend name with state mapping
            let legendName = s.name;
            if (s.state_mapping) {
                const mappingStr = Object.entries(s.state_mapping)
                    .sort((a, b) => a[1] - b[1])
                    .map(([k, v]) => k + '=' + v)
                    .join(', ');
                legendName += ' (' + mappingStr + ')';
            }
            
            // Format hover template to show both X and Y values
            // %{x} = X value, %{y} = Y value, %{fullData.name} = series name
            // Use series-specific Y-axis label if available, otherwise use global label
            const yLabel = s.yaxis_label || data.yaxis_label || 'Value';
            const hoverTemplate = '<b>%{fullData.name}</b><br>' +
                data.xaxis_label + ': %{x:.6f}<br>' +
                yLabel + ': %{y:.6f}<extra></extra>';
            
            // Format Y values to use ASCII minus sign
            const formattedY = s.y.map(val => val.toFixed(6).replace(/\u2212/g, '-'));
            
            const trace = {
                x: s.x,
                y: s.y,
                customdata: formattedY.map((val, idx) => [val]),  // Store formatted Y values for hover
                name: legendName,
                type: 'scatter',
                mode: s.mode || 'lines+markers',
                hovertemplate: '<b>%{fullData.name}</b><br>' +
                    data.xaxis_label + ': %{x:.6f}<br>' +
                    yLabel + ': %{customdata[0]}<extra></extra>',
                hoverlabel: {
                    namelength: -1  // Don't truncate series names
                },
                marker: s.mode && s.mode.includes('markers') ? {
                    size: 5,
                    symbol: s.marker === 'o' || s.marker === 'O' || s.marker === 'circle' ? 'circle' : 
                            s.marker === 'x' || s.marker === 'X' ? 'x' : 
                            s.marker === 's' || s.marker === 'S' || s.marker === 'square' ? 'square' : 
                            s.marker === 'd' || s.marker === 'D' || s.marker === 'diamond' ? 'diamond' : 
                            s.marker === '+' ? 'cross' :
                            s.marker === '.' || s.marker === 'point' ? 'circle' : 'circle'
                } : undefined,
                line: s.mode && s.mode.includes('lines') ? {
                    width: 2,
                    dash: s.line_style === '--' || s.line_style === 'dashed' ? 'dash' : 
                          s.line_style === ':' || s.line_style === 'dotted' ? 'dot' : 
                          s.line_style === '-.' || s.line_style === 'dashdot' ? 'dashdot' : 'solid',
                    shape: s.step ? 'hv' : 'linear'  // 'hv' = horizontal-vertical step, 'linear' = normal line
                } : undefined
            };
            
            // Set color if specified
            if (s.color) {
                const colorMap = {
                    'blue': 'rgb(31, 119, 180)',
                    'red': 'rgb(214, 39, 40)',
                    'green': 'rgb(44, 160, 44)',
                    'orange': 'rgb(255, 127, 14)',
                    'purple': 'rgb(148, 103, 189)',
                    'brown': 'rgb(140, 86, 75)',
                    'cyan': 'rgb(0, 255, 255)',
                    'magenta': 'rgb(255, 0, 255)',
                    'teal': 'rgb(0, 128, 128)',
                    'black': 'rgb(0, 0, 0)',
                    'pink': 'rgb(227, 119, 194)',
                    'gray': 'rgb(127, 127, 127)'
                };
                const color = colorMap[s.color.toLowerCase()] || s.color;
                if (trace.marker) {
                    trace.marker.color = color;
                }
                if (trace.line) {
                    trace.line.color = color;
                }
            }
            
            return trace;
        });
        
        // Set Y-axis configuration
        const yaxisConfig = {
            title: data.yaxis_label,
            showgrid: true,
            gridcolor: '#e0e0e0',
            tickmode: 'linear'
        };
        
        // Set Y-axis range if configured
        let min = null, max = null;
        if (data.y_range !== undefined && data.y_range !== null) {
            // Symmetric range: +/- value
            min = -data.y_range;
            max = data.y_range;
            yaxisConfig.range = [min, max];
        } else if (data.y_min !== undefined || data.y_max !== undefined) {
            // Individual min/max if specified
            min = data.y_min !== undefined ? data.y_min : null;
            max = data.y_max !== undefined ? data.y_max : null;
            yaxisConfig.range = [min, max];
        }
        
        // Fix negative number display: Plotly uses Unicode minus (U+2212) by default
        // Use custom formatter to replace with ASCII minus
        yaxisConfig.tickformat = function(d) {
            return d.toFixed(0).replace(/\u2212/g, '-');
        };
        
        // Check if custom tick options are configured
        const hasTickSpacing = data.y_tick_spacing !== undefined && data.y_tick_spacing !== null && !isNaN(data.y_tick_spacing);
        const hasTickCount = data.y_tick_count !== undefined && data.y_tick_count !== null && !isNaN(data.y_tick_count);
        const hasRange = min !== null && max !== null;
        
        // Determine the effective range for tick generation
        let effectiveMin = min, effectiveMax = max;
        if (effectiveMin === null || effectiveMax === null) {
            // If range not explicitly set, determine from data
            let dataMin = Infinity, dataMax = -Infinity;
            series.forEach(s => {
                if (s.y && s.y.length > 0) {
                    const seriesMin = Math.min(...s.y);
                    const seriesMax = Math.max(...s.y);
                    if (seriesMin < dataMin) dataMin = seriesMin;
                    if (seriesMax > dataMax) dataMax = seriesMax;
                }
            });
            if (isFinite(dataMin) && isFinite(dataMax)) {
                effectiveMin = dataMin;
                effectiveMax = dataMax;
            }
        }
        
        // Generate ticks if we have a valid range
        if (effectiveMin !== null && effectiveMax !== null && effectiveMax > effectiveMin) {
            let spacing = null;
            
            if (hasTickSpacing) {
                // Use custom spacing
                spacing = Number(data.y_tick_spacing);
            } else if (hasTickCount) {
                // Use custom count
                spacing = (effectiveMax - effectiveMin) / (Number(data.y_tick_count) - 1);
            } else {
                // Calculate automatic spacing for ~8-10 ticks
                const dataRange = effectiveMax - effectiveMin;
                let targetSpacing = dataRange / 10;
                // Round to nice numbers (powers of 1, 2, 5, 10, etc.)
                if (targetSpacing > 0) {
                    const magnitude = Math.pow(10, Math.floor(Math.log10(targetSpacing)));
                    const normalized = targetSpacing / magnitude;
                    let multiplier = 1;
                    if (normalized <= 1) multiplier = 1;
                    else if (normalized <= 2) multiplier = 2;
                    else if (normalized <= 5) multiplier = 5;
                    else multiplier = 10;
                    spacing = multiplier * magnitude;
                }
            }
            
            // Generate tick values if we have valid spacing
            if (spacing !== null && !isNaN(spacing) && spacing > 0 && isFinite(spacing)) {
                const tickvals = [];
                const ticktext = [];
                // Calculate starting value aligned to spacing
                let startVal = Math.floor(effectiveMin / spacing) * spacing;
                if (startVal > effectiveMin) {
                    startVal -= spacing;
                }
                // Generate ticks from start to max
                let val = startVal;
                const maxVal = effectiveMax + spacing * 0.001; // Add small buffer
                while (val <= maxVal) {
                    tickvals.push(val);
                    // Format with ASCII minus sign
                    const formatted = val.toFixed(0);
                    ticktext.push(formatted.replace(/\u2212/g, '-'));
                    val += spacing;
                }
                
                // Use explicit ticks if we have a reasonable number (2-20 ticks)
                if (tickvals.length >= 2 && tickvals.length <= 20) {
                    yaxisConfig.tickmode = 'array';
                    yaxisConfig.tickvals = tickvals;
                    yaxisConfig.ticktext = ticktext;
                } else {
                    // Fallback to dtick for spacing
                    yaxisConfig.dtick = spacing;
                }
            }
        }
        
        const layout = {
            title: data.title,
            xaxis: {
                title: data.xaxis_label,
                showgrid: true,
                gridcolor: '#e0e0e0'
            },
            yaxis: yaxisConfig,
            hovermode: 'closest',
            hoverlabel: {
                namelength: -1,  // Don't truncate series names in hover
                bgcolor: 'rgba(255, 255, 255, 0.95)',
                bordercolor: '#333',
                font: {
                    size: 12
                }
            },
            legend: {
                x: 0,
                y: 1,
                bgcolor: 'rgba(255, 255, 255, 0.8)',
                bordercolor: '#ccc',
                borderwidth: 1
            },
            margin: {
                l: 60,
                r: 20,
                t: 60,
                b: 60
            }
        };
        
        const config = {
            responsive: true,
            displayModeBar: true,
            modeBarButtonsToAdd: ['select2d', 'lasso2d'],
            displaylogo: false
        };
        
        Plotly.newPlot('plotly-div', traces, layout, config);
        
        let currentLayout = layout;
        
        function resetZoom() {
            Plotly.relayout('plotly-div', {
                'xaxis.range': null,
                'yaxis.range': null
            });
        }
        
        function toggleSeries() {
            // This is handled by clicking on legend items in Plotly
            alert('Click on series names in the legend to toggle visibility');
        }
        
        function exportData() {
            // Convert data to CSV format
            let csv = 'TimeOffsetSeconds';
            series.forEach(s => {
                csv += ',' + s.name;
            });
            csv += '\n';
            
            // Find all unique timestamps
            const allTimes = new Set();
            series.forEach(s => {
                s.x.forEach(t => allTimes.add(t));
            });
            const sortedTimes = Array.from(allTimes).sort((a, b) => a - b);
            
            // Build CSV rows
            sortedTimes.forEach(time => {
                csv += time;
                series.forEach(s => {
                    const idx = s.x.indexOf(time);
                    csv += ',' + (idx >= 0 ? s.y[idx] : '');
                });
                csv += '\n';
            });
            
            // Download CSV
            const blob = new Blob([csv], { type: 'text/csv' });
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = 'plot_data.csv';
            a.click();
            window.URL.revokeObjectURL(url);
        }
        
        // Store layout on zoom/pan for reset functionality
        document.getElementById('plotly-div').on('plotly_relayout', function(eventData) {
            if (eventData['xaxis.range[0]'] !== undefined) {
                currentLayout = Object.assign({}, layout, {
                    xaxis: { range: [eventData['xaxis.range[0]'], eventData['xaxis.range[1]']] },
                    yaxis: { range: [eventData['yaxis.range[0]'], eventData['yaxis.range[1]']] }
                });
            }
        });
    </script>
</body>
</html>`

	// Create template
	tmpl, err := template.New("plot").Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse HTML template: %w", err)
	}

	// Prepare template data
	templateData := struct {
		Title    string
		JSONData template.JS
	}{
		Title:    cfg.Title,
		JSONData: template.JS(string(jsonData)),
	}

	// Write HTML file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create HTML file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, templateData); err != nil {
		return fmt.Errorf("failed to execute HTML template: %w", err)
	}

	return nil
}
