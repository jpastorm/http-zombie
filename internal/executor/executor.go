package executor

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Result holds the output of an executed HTTP request.
type Result struct {
	Command    string
	StatusCode string
	Headers    string
	Body       string
	Duration   time.Duration
	Error      string
}

// CheckXh verifies that xh is installed and reachable.
func CheckXh() (string, error) {
	path, err := exec.LookPath("xh")
	if err != nil {
		return "", fmt.Errorf(
			"xh is not installed or not in PATH\n\n" +
				"  🧟 zombie needs xh to rise from the grave!\n\n" +
				"  Install it:\n" +
				"    macOS:   brew install xh\n" +
				"    Linux:   cargo install xh\n" +
				"    Arch:    pacman -S xh\n" +
				"    Ubuntu:  snap install xh\n\n" +
				"  More info: https://github.com/ducaale/xh",
		)
	}

	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("xh found but failed to get version: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}

// Run executes xh with the given args and optional body via stdin.
func Run(args []string, body string) (*Result, error) {
	args = append(args, "--print=hb")

	cmdStr := "xh " + strings.Join(args, " ")

	start := time.Now()
	cmd := exec.Command("xh", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if body != "" {
		cmd.Stdin = strings.NewReader(body)
	}

	err := cmd.Run()
	duration := time.Since(start)

	result := &Result{
		Command:  cmdStr,
		Duration: duration,
	}

	if err != nil {
		result.Error = stderr.String()
		if result.Error == "" {
			result.Error = err.Error()
		}
		result.Body = stdout.String()
		return result, nil
	}

	output := stdout.String()
	result.Body = output

	// Try to extract status code from the first line
	if idx := strings.Index(output, "\n"); idx > 0 {
		firstLine := output[:idx]
		if strings.HasPrefix(firstLine, "HTTP/") {
			parts := strings.Fields(firstLine)
			if len(parts) >= 2 {
				result.StatusCode = parts[1]
			}
			if sepIdx := strings.Index(output, "\n\n"); sepIdx > 0 {
				result.Headers = output[:sepIdx]
				result.Body = output[sepIdx+2:]
			}
		}
	}

	return result, nil
}
