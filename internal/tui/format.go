package tui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// JSON syntax highlighting colors
var (
	jsonKeyColor    = lipgloss.Color("#56B6C2") // cyan
	jsonStringColor = lipgloss.Color("#98C379") // green
	jsonNumberColor = lipgloss.Color("#C678DD") // magenta
	jsonBoolColor   = lipgloss.Color("#E5C07B") // yellow/orange
	jsonNullColor   = lipgloss.Color("#E5C07B") // yellow/orange
	jsonBraceColor  = lipgloss.Color("#ABB2BF") // light gray
	jsonCommaColor  = lipgloss.Color("#ABB2BF") // light gray
)

var (
	jKey   = lipgloss.NewStyle().Foreground(jsonKeyColor)
	jStr   = lipgloss.NewStyle().Foreground(jsonStringColor)
	jNum   = lipgloss.NewStyle().Foreground(jsonNumberColor)
	jBool  = lipgloss.NewStyle().Foreground(jsonBoolColor)
	jNull  = lipgloss.NewStyle().Foreground(jsonNullColor)
	jBrace = lipgloss.NewStyle().Foreground(jsonBraceColor)
	jPlain = lipgloss.NewStyle().Foreground(boneWhite)
)

// prettyJSON formats and syntax-highlights JSON content.
func prettyJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// Try to parse and re-indent
	var obj interface{}
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return raw // not valid JSON, return as-is
	}

	indented, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return raw
	}

	return highlightJSON(string(indented))
}

// prettyJSONWidth formats, wraps to maxWidth, and syntax-highlights JSON content.
func prettyJSONWidth(raw string, maxWidth int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	var obj interface{}
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return wrapContent(raw, maxWidth)
	}

	indented, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return wrapContent(raw, maxWidth)
	}

	wrapped := wrapContent(string(indented), maxWidth)
	return highlightJSON(wrapped)
}

// highlightJSON applies syntax coloring to pretty-printed JSON.
func highlightJSON(s string) string {
	var out strings.Builder
	lines := strings.Split(s, "\n")

	for i, line := range lines {
		out.WriteString(highlightJSONLine(line))
		if i < len(lines)-1 {
			out.WriteString("\n")
		}
	}
	return out.String()
}

func highlightJSONLine(line string) string {
	trimmed := strings.TrimSpace(line)
	indent := line[:len(line)-len(trimmed)]

	if trimmed == "" {
		return ""
	}

	var out strings.Builder
	out.WriteString(indent)

	// Handle structural characters at start/end
	if trimmed == "{" || trimmed == "}" || trimmed == "}," ||
		trimmed == "[" || trimmed == "]" || trimmed == "]," {
		out.WriteString(jBrace.Render(trimmed))
		return out.String()
	}

	// Key-value line: "key": value
	if strings.HasPrefix(trimmed, "\"") {
		colonIdx := strings.Index(trimmed, "\": ")
		if colonIdx > 0 {
			key := trimmed[:colonIdx+1]
			rest := trimmed[colonIdx+2:] // ": " → skip ": "

			out.WriteString(jKey.Render(key))
			out.WriteString(jPlain.Render(": "))
			out.WriteString(highlightValue(rest))
			return out.String()
		}

		// Key with object/array value: "key": { or "key": [
		colonBrace := strings.Index(trimmed, "\": {")
		if colonBrace < 0 {
			colonBrace = strings.Index(trimmed, "\": [")
		}
		if colonBrace > 0 {
			key := trimmed[:colonBrace+1]
			rest := trimmed[colonBrace+2:]
			out.WriteString(jKey.Render(key))
			out.WriteString(jPlain.Render(": "))
			out.WriteString(jBrace.Render(rest))
			return out.String()
		}
	}

	// Bare value (in array)
	out.WriteString(highlightValue(trimmed))
	return out.String()
}

func highlightValue(v string) string {
	trailing := ""
	if strings.HasSuffix(v, ",") {
		v = v[:len(v)-1]
		trailing = jBrace.Render(",")
	}

	switch {
	case v == "null":
		return jNull.Render(v) + trailing
	case v == "true" || v == "false":
		return jBool.Render(v) + trailing
	case strings.HasPrefix(v, "\""):
		return jStr.Render(v) + trailing
	case v == "{" || v == "}" || v == "[" || v == "]":
		return jBrace.Render(v) + trailing
	default:
		// Try number
		if _, err := strconv.ParseFloat(v, 64); err == nil {
			return jNum.Render(v) + trailing
		}
		return jPlain.Render(v) + trailing
	}
}

// formatHeaders applies syntax highlighting to HTTP headers.
func formatHeaders(raw string) string {
	if raw == "" {
		return ""
	}
	lines := strings.Split(raw, "\n")
	var out strings.Builder

	for i, line := range lines {
		if strings.HasPrefix(line, "HTTP/") {
			// Status line
			out.WriteString(dimStyle.Render(line))
		} else if idx := strings.Index(line, ": "); idx > 0 {
			key := line[:idx]
			val := line[idx+2:]
			out.WriteString(headerKeyStyle.Render(key))
			out.WriteString(dimStyle.Render(": "))
			out.WriteString(headerValStyle.Render(val))
		} else {
			out.WriteString(dimStyle.Render(line))
		}
		if i < len(lines)-1 {
			out.WriteString("\n")
		}
	}
	return out.String()
}

// statusColor returns the appropriate style for an HTTP status code.
func statusColor(code string) lipgloss.Style {
	if len(code) == 0 {
		return dimStyle
	}
	switch code[0] {
	case '2':
		return lipgloss.NewStyle().Foreground(status2xx).Bold(true)
	case '3':
		return lipgloss.NewStyle().Foreground(status3xx).Bold(true)
	case '4':
		return lipgloss.NewStyle().Foreground(status4xx).Bold(true)
	case '5':
		return lipgloss.NewStyle().Foreground(status5xx).Bold(true)
	default:
		return dimStyle
	}
}

// methodColor returns a style for HTTP methods.
func methodColor(method string) lipgloss.Style {
	switch strings.ToUpper(method) {
	case "GET":
		return lipgloss.NewStyle().Foreground(status2xx).Bold(true)
	case "POST":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#E5C07B")).Bold(true)
	case "PUT", "PATCH":
		return lipgloss.NewStyle().Foreground(status3xx).Bold(true)
	case "DELETE":
		return lipgloss.NewStyle().Foreground(status4xx).Bold(true)
	default:
		return dimStyle
	}
}

// formatCurlPretty formats a raw curl command for display (multi-line, colored).
func formatCurlPretty(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return dimStyle.Render("<no request>")
	}

	// Normalize: join backslash continuations, then re-split nicely
	raw = strings.ReplaceAll(raw, "\\\n", " ")
	raw = strings.ReplaceAll(raw, "\\\r\n", " ")

	if !strings.HasPrefix(raw, "curl ") {
		raw = "curl " + raw
	}

	curlCmd := lipgloss.NewStyle().Foreground(lipgloss.Color("#E06C75")).Bold(true)
	curlFlag := lipgloss.NewStyle().Foreground(lipgloss.Color("#56B6C2"))
	curlVal := lipgloss.NewStyle().Foreground(lipgloss.Color("#98C379"))
	curlURL := lipgloss.NewStyle().Foreground(lipgloss.Color("#E5C07B")).Bold(true).Underline(true)

	tokens := tokenizeForDisplay(raw)
	var lines []string
	var current strings.Builder

	current.WriteString(curlCmd.Render("curl"))

	for i := 1; i < len(tokens); i++ {
		t := tokens[i]
		if strings.HasPrefix(t, "-") {
			// Start new line for flags
			if current.Len() > 0 {
				lines = append(lines, current.String())
				current.Reset()
				current.WriteString("  ")
			}
			current.WriteString(curlFlag.Render(t))
			// Consume the next token as value
			if i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") {
				i++
				current.WriteString(" ")
				current.WriteString(curlVal.Render(quoteIfNeeded(tokens[i])))
			}
		} else if looksLikeURL(t) || strings.Contains(t, "://") {
			current.WriteString(" ")
			current.WriteString(curlURL.Render(t))
		} else {
			current.WriteString(" ")
			current.WriteString(curlVal.Render(t))
		}
	}
	if current.Len() > 0 {
		lines = append(lines, current.String())
	}

	return strings.Join(lines, " \\\n")
}

// formatXhPretty formats xh command args for display.
func formatXhPretty(args []string, body string) string {
	if len(args) == 0 {
		return dimStyle.Render("<no request>")
	}

	xhCmd := lipgloss.NewStyle().Foreground(lipgloss.Color("#61AFEF")).Bold(true)
	xhMethod := lipgloss.NewStyle().Foreground(lipgloss.Color("#C678DD")).Bold(true)
	xhURL := lipgloss.NewStyle().Foreground(lipgloss.Color("#E5C07B")).Bold(true).Underline(true)
	xhHeader := lipgloss.NewStyle().Foreground(lipgloss.Color("#56B6C2"))
	xhFlag := lipgloss.NewStyle().Foreground(lipgloss.Color("#98C379"))

	var lines []string

	// First line: xh METHOD URL
	cmdLine := xhCmd.Render("xh")
	skipNext := false
	for i, a := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if i == 0 {
			if looksLikeURL(a) {
				cmdLine += " " + xhURL.Render(a)
			} else {
				cmdLine += " " + xhMethod.Render(a)
			}
		} else if i == 1 && looksLikeURL(a) {
			cmdLine += " " + xhURL.Render(a)
		} else if strings.HasPrefix(a, "--") {
			// Flag: put on its own line
			if len(lines) == 0 {
				lines = append(lines, cmdLine)
			}
			flagLine := "  " + xhFlag.Render(a)
			// Consume value if next arg isn't a flag or header
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") && !strings.Contains(args[i+1], ":") {
				flagLine += " " + xhFlag.Render(args[i+1])
				skipNext = true
			}
			lines = append(lines, flagLine)
		} else if strings.Contains(a, ":") && !strings.HasPrefix(a, "http") {
			// Header
			if len(lines) == 0 {
				lines = append(lines, cmdLine)
			}
			if idx := strings.Index(a, ":"); idx > 0 {
				k := a[:idx]
				v := a[idx+1:]
				lines = append(lines, "  "+xhHeader.Render(k+": ")+xhFlag.Render(v))
			} else {
				lines = append(lines, "  "+xhHeader.Render(a))
			}
		} else {
			cmdLine += " " + xhFlag.Render(a)
		}
	}
	if len(lines) == 0 {
		lines = append(lines, cmdLine)
	}

	if body != "" {
		lines = append(lines, "")
		lines = append(lines, dimStyle.Render("body:"))
		var obj interface{}
		if err := json.Unmarshal([]byte(body), &obj); err == nil {
			indented, _ := json.MarshalIndent(obj, "", "  ")
			for _, l := range strings.Split(string(indented), "\n") {
				lines = append(lines, jStr.Render(l))
			}
		} else {
			lines = append(lines, jStr.Render(body))
		}
	}

	return strings.Join(lines, "\n")
}

func quoteIfNeeded(s string) string {
	if strings.Contains(s, " ") || strings.Contains(s, "{") || strings.Contains(s, "}") {
		return fmt.Sprintf("'%s'", s)
	}
	return s
}

// wrapLine wraps a single line of text to fit within maxWidth.
func wrapLine(line string, maxWidth int) []string {
	if maxWidth <= 0 || lipgloss.Width(line) <= maxWidth {
		return []string{line}
	}
	var result []string
	for len(line) > 0 {
		if lipgloss.Width(line) <= maxWidth {
			result = append(result, line)
			break
		}
		// Find a reasonable break point
		cut := maxWidth
		if cut > len(line) {
			cut = len(line)
		}
		// Try to break at space
		best := -1
		for i := cut; i > maxWidth/2; i-- {
			if i < len(line) && line[i] == ' ' {
				best = i
				break
			}
		}
		if best > 0 {
			result = append(result, line[:best])
			line = "    " + line[best+1:] // indent continuation
		} else {
			result = append(result, line[:cut])
			line = "    " + line[cut:]
		}
	}
	return result
}

// wrapContent wraps all lines in content to fit within maxWidth.
func wrapContent(content string, maxWidth int) string {
	if maxWidth <= 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	var result []string
	for _, line := range lines {
		result = append(result, wrapLine(line, maxWidth)...)
	}
	return strings.Join(result, "\n")
}

// truncate cuts a string to maxLen characters, adding "…" if truncated.
func truncate(s string, maxLen int) string {
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}
	if maxLen < 4 {
		return s[:maxLen]
	}
	return s[:maxLen-1] + "…"
}

func looksLikeURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "://")
}

// tokenizeForDisplay splits a command preserving quotes for display only.
func tokenizeForDisplay(input string) []string {
	input = strings.ReplaceAll(input, "\\\n", " ")
	input = strings.ReplaceAll(input, "\\\r\n", " ")

	var tokens []string
	var current strings.Builder
	inSingle := false
	inDouble := false

	for _, ch := range input {
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if (ch == ' ' || ch == '\t') && !inSingle && !inDouble {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(ch)
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

// isJSON checks if a string looks like JSON.
func isJSON(s string) bool {
	s = strings.TrimSpace(s)
	return (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]"))
}

// formatXhCommandPretty formats a stored xh command string for display.
func formatXhCommandPretty(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return dimStyle.Render("<no command>")
	}

	// The stored command is "xh arg1 arg2 ... --print=hb"
	// Strip --print=hb for cleaner display
	command = strings.ReplaceAll(command, " --print=hb", "")
	command = strings.ReplaceAll(command, " --print hb", "")

	// Tokenize and reuse formatXhPretty logic
	tokens := tokenizeForDisplay(command)
	if len(tokens) == 0 {
		return dimStyle.Render("<no command>")
	}

	// Skip "xh" prefix, pass the rest as args
	args := tokens[1:]
	return formatXhPretty(args, "")
}

// curlPreview holds parsed parts of a curl command for live preview.
type curlPreview struct {
	method  string
	url     string
	headers []string
	body    string
	flags   []string
}

// parseCurlPreview extracts method, url, headers, body from raw curl text.
func parseCurlPreview(raw string) curlPreview {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return curlPreview{}
	}

	raw = strings.ReplaceAll(raw, "\\\n", " ")
	raw = strings.ReplaceAll(raw, "\\\r\n", " ")

	cmd := raw
	if strings.HasPrefix(cmd, "curl ") {
		cmd = cmd[5:]
	}

	tokens := tokenizeForDisplay(cmd)

	var p curlPreview
	i := 0
	for i < len(tokens) {
		t := tokens[i]
		switch {
		case t == "-X" || t == "--request":
			i++
			if i < len(tokens) {
				p.method = strings.ToUpper(tokens[i])
			}
		case t == "-H" || t == "--header":
			i++
			if i < len(tokens) {
				p.headers = append(p.headers, tokens[i])
			}
		case t == "-d" || t == "--data" || t == "--data-raw" || t == "--data-binary":
			i++
			if i < len(tokens) {
				p.body = tokens[i]
			}
		case t == "-F" || t == "--form":
			i++
			if i < len(tokens) {
				p.flags = append(p.flags, "form: "+tokens[i])
			}
		case t == "-u" || t == "--user":
			i++
			if i < len(tokens) {
				p.flags = append(p.flags, "auth: "+tokens[i])
			}
		case t == "-L" || t == "--location":
			p.flags = append(p.flags, "follow redirects")
		case t == "-k" || t == "--insecure":
			p.flags = append(p.flags, "insecure")
		case t == "-v" || t == "--verbose":
			p.flags = append(p.flags, "verbose")
		case strings.HasPrefix(t, "-"):
			// unknown flag, skip value if present
			if i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") && !looksLikeURL(tokens[i+1]) {
				i++
			}
		default:
			if p.url == "" {
				p.url = t
			}
		}
		i++
	}

	if p.method == "" {
		if p.body != "" {
			p.method = "POST"
		} else {
			p.method = "GET"
		}
	}

	return p
}

// extractContentType extracts content-type from headers string.
func extractContentType(headers string) string {
	for _, line := range strings.Split(strings.ToLower(headers), "\n") {
		if strings.HasPrefix(line, "content-type:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "content-type:"))
		}
	}
	return ""
}

// scrollbar builds a vertical scrollbar indicator.
func scrollbar(offset, total, viewHeight int) string {
	if total <= viewHeight {
		return ""
	}

	barHeight := viewHeight
	thumbSize := max(1, barHeight*viewHeight/total)
	thumbPos := barHeight * offset / total
	if thumbPos+thumbSize > barHeight {
		thumbPos = barHeight - thumbSize
	}

	var sb strings.Builder
	for i := 0; i < barHeight; i++ {
		if i >= thumbPos && i < thumbPos+thumbSize {
			sb.WriteString(lipgloss.NewStyle().Foreground(zombieGreen).Render("┃"))
		} else {
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#333333")).Render("│"))
		}
		if i < barHeight-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// timeAgo formats a timestamp string (2006-01-02_15-04-05) as natural language relative time.
func timeAgo(ts string) string {
	t, err := time.ParseInLocation("2006-01-02_15-04-05", ts, time.Local)
	if err != nil {
		return ts
	}
	now := time.Now()
	d := now.Sub(t)

	// Same calendar day
	y1, m1, d1 := now.Date()
	y2, m2, d2 := t.Date()
	sameDay := y1 == y2 && m1 == m2 && d1 == d2

	if sameDay {
		switch {
		case d < time.Minute:
			return "just now"
		case d < time.Hour:
			m := int(d.Minutes())
			if m == 1 {
				return "1 min ago"
			}
			return fmt.Sprintf("%d min ago", m)
		default:
			h := int(d.Hours())
			if h == 1 {
				return "1 hour ago"
			}
			return fmt.Sprintf("%dh ago", h)
		}
	}

	// Yesterday (calendar day)
	yesterday := now.AddDate(0, 0, -1)
	yy, ym, yd := yesterday.Date()
	if y2 == yy && m2 == ym && d2 == yd {
		return "yesterday " + t.Format("Jan 02")
	}

	// This week
	if d < 7*24*time.Hour {
		return fmt.Sprintf("%dd ago · %s", int(d.Hours()/24), t.Format("Jan 02"))
	}

	return t.Format("Jan 02, 2006")
}

// extractEndpoint extracts the HTTP method, host, and URL path from a command string.
func extractEndpoint(cmd string) (method, host, endpoint string) {
	cmd = strings.TrimSpace(cmd)
	fields := strings.Fields(cmd)

	// Skip leading "xh" or "curl"
	start := 0
	if len(fields) > 0 && (fields[0] == "xh" || fields[0] == "curl") {
		start = 1
	}

	for i := start; i < len(fields); i++ {
		f := fields[i]
		// Skip flags
		if strings.HasPrefix(f, "-") {
			// skip flag value if it's a flag that takes an argument
			if !strings.Contains(f, "=") && i+1 < len(fields) && !strings.HasPrefix(fields[i+1], "-") && !looksLikeURL(fields[i+1]) {
				i++ // skip the value
			}
			continue
		}
		// Check for HTTP method
		upper := strings.ToUpper(f)
		if method == "" && (upper == "GET" || upper == "POST" || upper == "PUT" || upper == "PATCH" || upper == "DELETE" || upper == "HEAD" || upper == "OPTIONS") {
			method = upper
			continue
		}
		// Check for URL
		if looksLikeURL(f) {
			url := f
			if idx := strings.Index(url, "://"); idx >= 0 {
				url = url[idx+3:]
			}
			if idx := strings.Index(url, "/"); idx >= 0 {
				host = url[:idx]
				endpoint = url[idx:]
			} else {
				host = url
				endpoint = "/"
			}
			// Trim query string for display
			if idx := strings.Index(endpoint, "?"); idx >= 0 {
				endpoint = endpoint[:idx]
			}
			break
		}
	}
	if method == "" {
		method = "GET"
	}
	return method, host, endpoint
}
