package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jpastorm/zombie/internal/executor"
)

// SaveResponse writes the response body to the responses/ directory.
func SaveResponse(baseDir string, requestName string, result *executor.Result) (string, error) {
	ts := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("%s-%s.json", requestName, ts)
	filename = filepath.FromSlash(filename)

	fullPath := filepath.Join(baseDir, "responses", filename)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return "", fmt.Errorf("cannot create response directory: %w", err)
	}

	content := result.Body
	if result.Headers != "" {
		content = result.Headers + "\n\n" + result.Body
	}

	// Save duration alongside the response
	if result.Duration > 0 {
		durPath := fullPath + ".duration"
		os.WriteFile(durPath, []byte(result.Duration.String()), 0o644)
	}

	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("cannot save response: %w", err)
	}

	return fullPath, nil
}

// SaveHistory writes both the executed command and the response to history/.
func SaveHistory(baseDir string, requestName string, result *executor.Result, rawCurl string) (string, string, error) {
	ts := time.Now().Format("2006-01-02_15-04-05")
	histDir := filepath.Join(baseDir, "history")

	if err := os.MkdirAll(histDir, 0o755); err != nil {
		return "", "", fmt.Errorf("cannot create history directory: %w", err)
	}

	reqPath := filepath.Join(histDir, ts+".request")
	if err := os.WriteFile(reqPath, []byte(result.Command), 0o644); err != nil {
		return "", "", fmt.Errorf("cannot save history request: %w", err)
	}

	// Save original curl command
	if rawCurl != "" {
		curlPath := filepath.Join(histDir, ts+".curl")
		os.WriteFile(curlPath, []byte(rawCurl), 0o644)
	}

	respPath := filepath.Join(histDir, ts+".response.json")
	respContent := result.Body
	if result.Headers != "" {
		respContent = result.Headers + "\n\n" + result.Body
	}
	if err := os.WriteFile(respPath, []byte(respContent), 0o644); err != nil {
		return reqPath, "", fmt.Errorf("cannot save history response: %w", err)
	}

	// Save request name for display in history
	if requestName != "" {
		namePath := filepath.Join(histDir, ts+".name")
		os.WriteFile(namePath, []byte(requestName), 0o644)
	}

	// Save response time duration
	if result.Duration > 0 {
		durPath := filepath.Join(histDir, ts+".duration")
		os.WriteFile(durPath, []byte(result.Duration.String()), 0o644)
	}

	return reqPath, respPath, nil
}

// HistoryEntry represents one item from history.
type HistoryEntry struct {
	Timestamp   string
	ReqPath     string
	RespPath    string
	CurlPath    string
	RequestName string
	Duration    time.Duration
}

// ListHistory returns all history entries sorted by timestamp (newest first).
func ListHistory(baseDir string) ([]HistoryEntry, error) {
	histDir := filepath.Join(baseDir, "history")
	entries, err := os.ReadDir(histDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	grouped := make(map[string]*HistoryEntry)
	for _, e := range entries {
		name := e.Name()
		var ts string

		if idx := len(name) - len(".name"); idx > 0 && name[idx:] == ".name" {
			ts = name[:idx]
		} else if idx := len(name) - len(".duration"); idx > 0 && name[idx:] == ".duration" {
			ts = name[:idx]
		} else if idx := len(name) - len(".curl"); idx > 0 && name[idx:] == ".curl" {
			ts = name[:idx]
		} else if idx := len(name) - len(".request"); idx > 0 && name[idx:] == ".request" {
			ts = name[:idx]
		} else if idx := len(name) - len(".response.json"); idx > 0 && name[idx:] == ".response.json" {
			ts = name[:idx]
		} else {
			continue
		}

		entry, ok := grouped[ts]
		if !ok {
			entry = &HistoryEntry{Timestamp: ts}
			grouped[ts] = entry
		}

		if strings.HasSuffix(name, ".name") {
			data, err := os.ReadFile(filepath.Join(histDir, name))
			if err == nil {
				entry.RequestName = strings.TrimSpace(string(data))
			}
		} else if strings.HasSuffix(name, ".duration") {
			data, err := os.ReadFile(filepath.Join(histDir, name))
			if err == nil {
				if d, err := time.ParseDuration(strings.TrimSpace(string(data))); err == nil {
					entry.Duration = d
				}
			}
		} else if strings.HasSuffix(name, ".curl") {
			entry.CurlPath = filepath.Join(histDir, name)
		} else if strings.HasSuffix(name, ".request") {
			entry.ReqPath = filepath.Join(histDir, name)
		} else {
			entry.RespPath = filepath.Join(histDir, name)
		}
	}

	var result []HistoryEntry
	for _, e := range grouped {
		result = append(result, *e)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp > result[j].Timestamp
	})

	return result, nil
}

// ReadFile reads a file and returns its content.
func ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ResponseEntry represents a saved response for a specific request.
type ResponseEntry struct {
	Timestamp string
	Path      string
}

// CountHistory returns the number of history entries.
func CountHistory(baseDir string) int {
	histDir := filepath.Join(baseDir, "history")
	entries, err := os.ReadDir(histDir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".request") {
			count++
		}
	}
	return count
}

// ListRequestResponses returns saved responses for a given request name, newest first.
func ListRequestResponses(baseDir, requestName string) []ResponseEntry {
	respDir := filepath.Join(baseDir, "responses")
	prefix := filepath.FromSlash(requestName) + "-"

	var entries []ResponseEntry

	filepath.Walk(respDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(respDir, path)
		rel = filepath.ToSlash(rel)
		name := strings.TrimSuffix(rel, ".json")
		if strings.HasPrefix(name, prefix) {
			ts := strings.TrimPrefix(name, prefix)
			entries = append(entries, ResponseEntry{
				Timestamp: ts,
				Path:      path,
			})
		}
		return nil
	})

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp > entries[j].Timestamp
	})

	return entries
}

// DeleteHistoryEntry removes all files belonging to a single history entry
// (identified by its timestamp prefix).
func DeleteHistoryEntry(baseDir string, timestamp string) error {
	histDir := filepath.Join(baseDir, "history")
	for _, suffix := range []string{".name", ".request", ".response.json", ".curl", ".duration"} {
		p := filepath.Join(histDir, timestamp+suffix)
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("cannot delete %s: %w", p, err)
		}
	}
	return nil
}

// DeleteRecord removes the .curl request file, all its saved responses, and
// all history entries whose .name file matches the request name.
func DeleteRecord(baseDir string, requestName string, requestPath string) error {
	// 1. Remove the .curl file itself
	if err := os.Remove(requestPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot delete request file: %w", err)
	}

	// Remove empty parent dirs up to requests/
	requestsDir := filepath.Join(baseDir, "requests")
	dir := filepath.Dir(requestPath)
	for dir != requestsDir && dir != "." && dir != "/" {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			break
		}
		os.Remove(dir)
		dir = filepath.Dir(dir)
	}

	// 2. Remove saved responses matching this request name
	respDir := filepath.Join(baseDir, "responses")
	prefix := filepath.FromSlash(requestName) + "-"
	filepath.Walk(respDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(respDir, path)
		rel = filepath.ToSlash(rel)
		name := strings.TrimSuffix(rel, ".json")
		if strings.HasPrefix(name, prefix) {
			os.Remove(path)
		}
		return nil
	})

	// 3. Remove history entries whose .name matches the request name
	histDir := filepath.Join(baseDir, "history")
	entries, err := os.ReadDir(histDir)
	if err != nil {
		return nil // history dir may not exist
	}

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".name") {
			continue
		}
		namePath := filepath.Join(histDir, e.Name())
		data, err := os.ReadFile(namePath)
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(data)) != requestName {
			continue
		}
		// This history group matches — remove all files with the same timestamp
		ts := strings.TrimSuffix(e.Name(), ".name")
		for _, suffix := range []string{".name", ".request", ".response.json", ".curl", ".duration"} {
			os.Remove(filepath.Join(histDir, ts+suffix))
		}
	}

	return nil
}
