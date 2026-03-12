package parser

import (
	"fmt"
	"os"
	"strings"
)

// Request holds a raw curl command loaded from a .curl file.
type Request struct {
	RawCurl string
}

// ParseFile reads a .curl file and returns a Request.
func ParseFile(path string) (*Request, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open request file: %w", err)
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return nil, fmt.Errorf("request file is empty")
	}
	return &Request{RawCurl: content}, nil
}

// ParseString parses a raw string (e.g. a pasted curl command) into a Request.
func ParseString(input string) (*Request, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty input")
	}
	return &Request{RawCurl: input}, nil
}

// CurlToXhArgs translates a curl command into xh CLI arguments.
// Returns (args, body) where body should be passed via stdin to xh.
func CurlToXhArgs(rawCurl string) ([]string, string) {
	cmd := strings.TrimSpace(rawCurl)
	if strings.HasPrefix(cmd, "curl ") {
		cmd = cmd[5:]
	}

	tokens := tokenize(cmd)

	var (
		method     string
		url        string
		headers    []string
		body       string
		follow     bool
		insecure   bool
		compressed bool
		verbose    bool
		auth       string
		output     string
		form       bool
		formFields []string
	)

	i := 0
	for i < len(tokens) {
		t := tokens[i]
		switch {
		case t == "-X" || t == "--request":
			i++
			if i < len(tokens) {
				method = strings.ToUpper(tokens[i])
			}
		case t == "-H" || t == "--header":
			i++
			if i < len(tokens) {
				headers = append(headers, tokens[i])
			}
		case t == "-d" || t == "--data" || t == "--data-raw" || t == "--data-binary":
			i++
			if i < len(tokens) {
				body = tokens[i]
			}
		case t == "--data-urlencode":
			i++
			if i < len(tokens) {
				body = tokens[i]
				form = true
			}
		case t == "-F" || t == "--form":
			i++
			if i < len(tokens) {
				form = true
				formFields = append(formFields, tokens[i])
			}
		case t == "-u" || t == "--user":
			i++
			if i < len(tokens) {
				auth = tokens[i]
			}
		case t == "-A" || t == "--user-agent":
			i++
			if i < len(tokens) {
				headers = append(headers, "User-Agent: "+tokens[i])
			}
		case t == "-e" || t == "--referer":
			i++
			if i < len(tokens) {
				headers = append(headers, "Referer: "+tokens[i])
			}
		case t == "-b" || t == "--cookie":
			i++
			if i < len(tokens) {
				headers = append(headers, "Cookie: "+tokens[i])
			}
		case t == "-o" || t == "--output":
			i++
			if i < len(tokens) {
				output = tokens[i]
			}
		case t == "-L" || t == "--location":
			follow = true
		case t == "-k" || t == "--insecure":
			insecure = true
		case t == "--compressed":
			compressed = true
		case t == "-v" || t == "--verbose":
			verbose = true
		case t == "-s" || t == "--silent" || t == "-S" || t == "--show-error" || t == "-sS" || t == "-Ss":
			// ignore, not relevant for xh
		case strings.HasPrefix(t, "-"):
			// unknown flag — skip its value if it looks like one
			if i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") && !looksLikeURL(tokens[i+1]) {
				i++
			}
		default:
			if url == "" {
				url = t
			}
		}
		i++
	}

	// Build xh args: METHOD URL [headers...] [options...]
	var args []string
	if method != "" {
		args = append(args, method)
	}
	if url != "" {
		args = append(args, url)
	}

	for _, h := range headers {
		// Convert "Key: Value" to "Key:Value" (xh request item format)
		if idx := strings.Index(h, ": "); idx > 0 {
			h = h[:idx] + ":" + h[idx+2:]
		}
		args = append(args, h)
	}

	if form {
		args = append(args, "--form")
		for _, f := range formFields {
			args = append(args, f)
		}
	}
	if follow {
		args = append(args, "--follow")
	}
	if insecure {
		args = append(args, "--verify", "no")
	}
	if compressed {
		args = append(args, "--compress")
	}
	if verbose {
		args = append(args, "--verbose")
	}
	if auth != "" {
		args = append(args, "--auth", auth)
	}
	if output != "" {
		args = append(args, "--output", output)
	}

	return args, body
}

func looksLikeURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// tokenize splits a shell-like command string respecting single/double quotes
// and backslash-continued lines.
func tokenize(input string) []string {
	// Join backslash-continued lines
	input = strings.ReplaceAll(input, "\\\n", " ")
	input = strings.ReplaceAll(input, "\\\r\n", " ")

	var tokens []string
	var current strings.Builder
	inSingle := false
	inDouble := false
	escaped := false

	for _, ch := range input {
		if escaped {
			current.WriteRune(ch)
			escaped = false
			continue
		}
		if ch == '\\' && !inSingle {
			escaped = true
			continue
		}
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if (ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r') && !inSingle && !inDouble {
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
