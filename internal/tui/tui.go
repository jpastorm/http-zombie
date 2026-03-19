package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"github.com/jpastorm/zombie/internal/executor"
	"github.com/jpastorm/zombie/internal/parser"
	"github.com/jpastorm/zombie/internal/scanner"
	"github.com/jpastorm/zombie/internal/storage"
)

// view modes
type viewMode int

const (
	viewList viewMode = iota
	viewSearch
	viewRunning
	viewResponse
	viewHistory
	viewHistoryDetail
	viewCurl
	viewCurlSave
	viewRequestModal
	viewRequestInfo
	viewResponseSave
	viewDeleteConfirm
	viewHistoryDeleteConfirm
)

// response body display mode
type bodyMode int

const (
	bodyPretty bodyMode = iota
	bodyRaw
	bodyHeaders
	bodyMeta
	bodyCommand
	bodyResponse
	bodyEditor
)

// messages
type requestDoneMsg struct {
	result      *executor.Result
	requestName string
	respPath    string
	rawCurl     string
	xhArgs      []string
	xhBody      string
}

type requestErrMsg struct {
	err error
}

type clearCopyMsg struct{}

type spinnerTickMsg struct{}

// Model is the main TUI model.
type Model struct {
	baseDir  string
	requests []scanner.RequestEntry
	filtered []scanner.RequestEntry

	cursor int
	mode   viewMode
	search string
	width  int
	height int

	// execution state
	lastResult   *executor.Result
	lastReqName  string
	lastRespPath string
	statusMsg    string
	lastRawCurl  string   // original curl command
	lastXhArgs   []string // translated xh args
	lastXhBody   string   // body passed to xh

	// response view state
	respBodyMode bodyMode

	// history
	history       []storage.HistoryEntry
	historyCursor int

	// scroll for response viewer
	scroll    int
	maxScroll int

	// request modal state
	modalPane   int // 0=curl, 1=xh
	modalScroll [2]int

	// history detail state
	historyResult     *executor.Result
	historyCurl       string
	historyReqName    string
	historyBodyMode   bodyMode
	historyScroll     int
	historyDetailBack viewMode

	// request info state
	reqInfoEntry     scanner.RequestEntry
	reqInfoRawCurl   string
	reqInfoXhArgs    []string
	reqInfoXhBody    string
	reqInfoResponses []storage.ResponseEntry
	reqInfoPane      int // 0=curl, 1=xh, 2=responses
	reqInfoCursor    int
	reqInfoScroll    int

	// save from response
	respSaveName string

	// curl paste mode
	curlTextarea      textarea.Model
	curlSaveName      string
	curlTab           int // 0=editor, 1=preview
	curlPreviewScroll int

	// copy feedback
	copyFeedback string

	// delete confirmation
	deleteTarget       scanner.RequestEntry
	deleteHistoryEntry storage.HistoryEntry

	// host replace mode
	hostReplace      bool
	hostReplaceInput string
	hostReplaceOld   string

	// running state
	cancelFunc context.CancelFunc
	spinnerIdx int
}

// setCopyFeedback sets a temporary feedback message and returns a command to clear it.
func (m *Model) setCopyFeedback(label string) tea.Cmd {
	m.copyFeedback = successStyle.Render("✓ copied " + label)
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearCopyMsg{} })
}

// New creates a new TUI model.
func New(baseDir string, requests []scanner.RequestEntry) Model {
	ta := newCurlTextarea()
	return Model{
		baseDir:      baseDir,
		requests:     requests,
		filtered:     requests,
		curlTextarea: ta,
	}
}

func newCurlTextarea() textarea.Model {
	ta := textarea.New()
	ta.Placeholder = "curl -X POST https://api.example.com/users \\\n  -H 'Content-Type: application/json' \\\n  -H 'Authorization: Bearer token' \\\n  -d '{\"name\": \"zombie\", \"type\": \"undead\"}'"
	ta.CharLimit = 0 // no limit
	ta.SetWidth(90)
	ta.SetHeight(12)
	ta.FocusedStyle.Base = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(zombieGreen).
		Padding(0, 1)
	ta.BlurredStyle.Base = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#333333")).
		Padding(0, 1)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(ghostGray)
	ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(boneWhite)
	ta.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(zombieGreen)
	ta.FocusedStyle.EndOfBuffer = lipgloss.NewStyle().Foreground(lipgloss.Color("#333333"))
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#444444"))
	ta.BlurredStyle.Text = lipgloss.NewStyle().Foreground(ghostGray)
	ta.BlurredStyle.Prompt = lipgloss.NewStyle().Foreground(ghostGray)
	ta.BlurredStyle.EndOfBuffer = lipgloss.NewStyle().Foreground(lipgloss.Color("#333333"))
	ta.ShowLineNumbers = false
	ta.Focus()
	return ta
}

func (m Model) Init() tea.Cmd {
	return tea.SetWindowTitle("🧟 zombie")
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Resize textarea to fill terminal
		// outer border=2 + outer padding=2 + textarea border=2 + textarea padding=2 = 8
		w := msg.Width - 8
		if w < 30 {
			w = 30
		}
		m.curlTextarea.SetWidth(w)
		// outer border=2 + title+tabs=4 + actions=2 + statusbar=3 = ~11 lines of chrome
		h := msg.Height - 11
		if h < 4 {
			h = 4
		}
		m.curlTextarea.SetHeight(h)
		return m, nil

	case clearCopyMsg:
		m.copyFeedback = ""
		return m, nil

	case spinnerTickMsg:
		if m.mode != viewRunning {
			return m, nil
		}
		m.spinnerIdx++
		return m, tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return spinnerTickMsg{} })

	case requestDoneMsg:
		if m.mode != viewRunning {
			return m, nil
		}
		m.lastResult = msg.result
		m.lastReqName = msg.requestName
		m.lastRespPath = msg.respPath
		m.lastRawCurl = msg.rawCurl
		m.lastXhArgs = msg.xhArgs
		m.lastXhBody = msg.xhBody
		m.mode = viewResponse
		m.scroll = 0
		m.respBodyMode = bodyResponse
		m.statusMsg = ""
		// Pre-fill editor with last curl for editing
		if msg.rawCurl != "" {
			m.curlTextarea.SetValue(msg.rawCurl)
			for m.curlTextarea.Line() > 0 {
				m.curlTextarea.CursorUp()
			}
			m.curlTextarea.CursorStart()
		}
		return m, nil

	case requestErrMsg:
		if m.mode != viewRunning {
			return m, nil
		}
		m.statusMsg = errorStyle.Render("☠ Error: " + msg.err.Error())
		m.mode = viewList
		m.resetSearch()
		return m, nil

	case tea.KeyMsg:
		// Paste events take priority (Cmd+V / bracket paste)
		if msg.Paste {
			// Host replace mode intercepts paste before editor modes
			if m.hostReplace {
				m.hostReplaceInput += strings.TrimSpace(string(msg.Runes))
				return m, nil
			}
			// In textarea-backed editor modes, let the textarea handle paste
			if m.mode == viewCurl && m.curlTab == 0 {
				return m.handleCurlKey(msg)
			}
			if m.mode == viewResponse && m.respBodyMode == bodyEditor {
				return m.handleResponseEditorKey(msg)
			}
			if m.mode == viewRequestInfo && m.reqInfoPane == 0 {
				return m.handleRequestInfoKey(msg)
			}
			return m.handlePaste(string(msg.Runes))
		}
		// Host replace mode takes priority over all editor modes
		if m.hostReplace {
			return m.handleHostReplaceKey(msg)
		}
		// In curl mode, delegate most keys to the textarea
		if m.mode == viewCurl {
			return m.handleCurlKey(msg)
		}
		// In response editor mode, delegate to textarea
		if m.mode == viewResponse && m.respBodyMode == bodyEditor {
			return m.handleResponseEditorKey(msg)
		}
		if m.mode == viewCurlSave {
			return m.handleCurlSaveKey(msg)
		}
		if m.mode == viewRequestModal {
			return m.handleModalKey(msg)
		}
		if m.mode == viewRequestInfo {
			return m.handleRequestInfoKey(msg)
		}
		if m.mode == viewResponseSave {
			return m.handleResponseSaveKey(msg)
		}
		if m.mode == viewDeleteConfirm {
			return m.handleDeleteConfirmKey(msg)
		}
		if m.mode == viewHistoryDeleteConfirm {
			return m.handleHistoryDeleteConfirmKey(msg)
		}
		return m.handleKey(msg)
	}

	// Let textarea handle other messages (blink cursor, etc.) when in editor modes
	if (m.mode == viewCurl && m.curlTab == 0) ||
		(m.mode == viewResponse && m.respBodyMode == bodyEditor) ||
		(m.mode == viewRequestInfo && m.reqInfoPane == 0) {
		var cmd tea.Cmd
		m.curlTextarea, cmd = m.curlTextarea.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) handlePaste(text string) (tea.Model, tea.Cmd) {
	if m.hostReplace {
		m.hostReplaceInput += strings.TrimSpace(text)
		return m, nil
	}
	switch m.mode {
	case viewCurlSave:
		m.curlSaveName += text
	case viewResponseSave:
		m.respSaveName += text
	case viewSearch:
		m.search += text
		m.applyFilter()
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case viewList:
		return m.handleListKey(msg)
	case viewSearch:
		return m.handleSearchKey(msg)
	case viewResponse:
		return m.handleResponseKey(msg)
	case viewRunning:
		switch msg.String() {
		case "esc", "q", "ctrl+c":
			if m.cancelFunc != nil {
				m.cancelFunc()
				m.cancelFunc = nil
			}
			m.mode = viewList
			m.statusMsg = dimStyle.Render("🪦 request cancelled")
			m.resetSearch()
		}
		return m, nil
	case viewHistory:
		return m.handleHistoryKey(msg)
	case viewHistoryDetail:
		return m.handleHistoryDetailKey(msg)
	case viewRequestInfo:
		return m.handleRequestInfoKey(msg)
	case viewResponseSave:
		return m.handleResponseSaveKey(msg)
	case viewCurl:
		return m.handleCurlKey(msg)
	case viewCurlSave:
		return m.handleCurlSaveKey(msg)
	case viewDeleteConfirm:
		return m.handleDeleteConfirmKey(msg)
	case viewHistoryDeleteConfirm:
		return m.handleHistoryDeleteConfirmKey(msg)
	}
	return m, nil
}

func (m Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
	case "pgup":
		pageSize := m.height - 12
		if pageSize < 5 {
			pageSize = 5
		}
		m.cursor -= pageSize
		if m.cursor < 0 {
			m.cursor = 0
		}
	case "pgdown":
		pageSize := m.height - 12
		if pageSize < 5 {
			pageSize = 5
		}
		m.cursor += pageSize
		if m.cursor >= len(m.filtered) {
			m.cursor = len(m.filtered) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
	case "home":
		m.cursor = 0
	case "end":
		if len(m.filtered) > 0 {
			m.cursor = len(m.filtered) - 1
		}
	case "/":
		m.mode = viewSearch
		m.search = ""
	case "enter":
		if len(m.filtered) > 0 {
			return m.openRequestInfo(m.filtered[m.cursor])
		}
	case "r":
		if len(m.filtered) > 0 {
			return m.executeRequest(m.filtered[m.cursor])
		}
	case "d":
		if len(m.filtered) > 0 {
			m.deleteTarget = m.filtered[m.cursor]
			m.mode = viewDeleteConfirm
		}
	case "h":
		return m.showHistory()
	case "c":
		m.mode = viewCurl
		m.curlTab = 0
		m.curlTextarea.Reset()
		m.curlTextarea.Focus()
		m.curlSaveName = ""
		return m, m.curlTextarea.Focus()
	}
	return m, nil
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = viewList
		m.search = ""
		m.filtered = m.requests
		m.cursor = 0
	case "enter":
		m.mode = viewList
		if len(m.filtered) > 0 && m.cursor >= len(m.filtered) {
			m.cursor = len(m.filtered) - 1
		}
	case "backspace":
		if len(m.search) > 0 {
			m.search = m.search[:len(m.search)-1]
			m.applyFilter()
		}
	case "ctrl+c":
		return m, tea.Quit
	default:
		if len(msg.String()) == 1 {
			m.search += msg.String()
			m.applyFilter()
		}
	}
	return m, nil
}

func (m Model) handleResponseKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.mode = viewList
		m.scroll = 0
		m.resetSearch()
	case "r":
		// rerun: if it was a curl-quick-run, re-execute from stored curl
		if m.lastReqName == "curl-quick-run" && m.lastRawCurl != "" {
			return m.rerunCurl()
		}
		for _, req := range m.requests {
			if req.Name == m.lastReqName {
				return m.executeRequest(req)
			}
		}
	case "d":
		// open request modal
		m.mode = viewRequestModal
		m.modalPane = 0
		m.modalScroll = [2]int{0, 0}
	case "1":
		m.respBodyMode = bodyEditor
		m.scroll = 0
		return m, m.curlTextarea.Focus()
	case "2":
		m.respBodyMode = bodyResponse
		m.scroll = 0
	case "3":
		m.respBodyMode = bodyRaw
		m.scroll = 0
	case "4":
		m.respBodyMode = bodyHeaders
		m.scroll = 0
	case "5":
		m.respBodyMode = bodyMeta
		m.scroll = 0
	case "6":
		m.respBodyMode = bodyCommand
		m.scroll = 0
	case "up", "k":
		if m.scroll > 0 {
			m.scroll--
		}
	case "down", "j":
		m.scroll++
	case "pgup":
		m.scroll -= 10
		if m.scroll < 0 {
			m.scroll = 0
		}
	case "pgdown":
		m.scroll += 10
	case "y":
		if label := m.copyCurrentView(); label != "" {
			return m, m.setCopyFeedback(label)
		}
	case "c":
		if m.respBodyMode == bodyCommand && m.lastRawCurl != "" {
			clipboard.WriteAll(m.lastRawCurl)
			return m, m.setCopyFeedback("raw curl")
		}
	case "s":
		if m.lastRawCurl != "" {
			m.mode = viewResponseSave
			m.respSaveName = ""
		}
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.mode = viewResponse
	case "up", "k":
		if m.modalScroll[0] > 0 {
			m.modalScroll[0]--
		}
	case "down", "j":
		m.modalScroll[0]++
	case "pgup":
		m.modalScroll[0] -= 10
		if m.modalScroll[0] < 0 {
			m.modalScroll[0] = 0
		}
	case "pgdown":
		m.modalScroll[0] += 10
	case "y":
		if m.lastRawCurl != "" {
			clipboard.WriteAll(m.lastRawCurl)
			return m, m.setCopyFeedback("curl")
		}
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleHistoryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.mode = viewList
		m.resetSearch()
	case "up", "k":
		if m.historyCursor > 0 {
			m.historyCursor--
		}
	case "down", "j":
		if m.historyCursor < len(m.history)-1 {
			m.historyCursor++
		}
	case "enter":
		if len(m.history) > 0 {
			entry := m.history[m.historyCursor]

			// Build a Result from the stored files
			var command, headers, body, rawCurl string
			if entry.ReqPath != "" {
				c, err := storage.ReadFile(entry.ReqPath)
				if err == nil {
					command = c
				}
			}
			if entry.CurlPath != "" {
				c, err := storage.ReadFile(entry.CurlPath)
				if err == nil {
					rawCurl = c
				}
			}
			if entry.RespPath != "" {
				raw, err := storage.ReadFile(entry.RespPath)
				if err == nil {
					// The response file stores headers\n\nbody
					if idx := strings.Index(raw, "\n\n"); idx > 0 {
						headers = raw[:idx]
						body = raw[idx+2:]
					} else {
						body = raw
					}
				}
			}

			// Extract status from header line
			statusCode := ""
			if headers != "" {
				firstLine := strings.SplitN(headers, "\n", 2)[0]
				// HTTP/1.1 200 OK → extract "200 OK"
				parts := strings.SplitN(firstLine, " ", 3)
				if len(parts) >= 2 {
					statusCode = strings.Join(parts[1:], " ")
				}
			}

			m.historyResult = &executor.Result{
				Command:    command,
				StatusCode: statusCode,
				Headers:    headers,
				Body:       body,
			}
			m.historyCurl = rawCurl
			m.historyReqName = entry.Timestamp
			m.historyBodyMode = bodyResponse
			m.historyScroll = 0
			m.historyDetailBack = viewHistory
			m.mode = viewHistoryDetail
		}
	case "d":
		if len(m.history) > 0 {
			m.deleteHistoryEntry = m.history[m.historyCursor]
			m.mode = viewHistoryDeleteConfirm
		}
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleHistoryDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		if m.historyDetailBack == viewRequestInfo {
			m.mode = viewRequestInfo
		} else {
			m.mode = viewHistory
		}
		m.historyScroll = 0
	case "1":
		m.historyBodyMode = bodyCommand
		m.historyScroll = 0
	case "2":
		m.historyBodyMode = bodyResponse
		m.historyScroll = 0
	case "3":
		m.historyBodyMode = bodyRaw
		m.historyScroll = 0
	case "4":
		m.historyBodyMode = bodyHeaders
		m.historyScroll = 0
	case "5":
		m.historyBodyMode = bodyMeta
		m.historyScroll = 0
	case "up", "k":
		if m.historyScroll > 0 {
			m.historyScroll--
		}
	case "down", "j":
		m.historyScroll++
	case "pgup":
		m.historyScroll -= 10
		if m.historyScroll < 0 {
			m.historyScroll = 0
		}
	case "pgdown":
		m.historyScroll += 10
	case "y":
		if label := m.copyHistoryView(); label != "" {
			return m, m.setCopyFeedback(label)
		}
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleCurlKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.curlTab == 1 {
			// go back to editor
			m.curlTab = 0
			m.curlTextarea.Focus()
			return m, nil
		}
		m.mode = viewList
		m.resetSearch()
		m.curlTextarea.Reset()
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "ctrl+x":
		input := strings.TrimSpace(m.curlTextarea.Value())
		if input == "" {
			return m, nil
		}
		return m.executeCurl()
	case "ctrl+g":
		input := strings.TrimSpace(m.curlTextarea.Value())
		if input == "" {
			return m, nil
		}
		m.mode = viewCurlSave
		m.curlSaveName = ""
		return m, nil
	case "tab", "ctrl+p":
		if m.curlTab == 0 {
			m.curlTab = 1
			m.curlPreviewScroll = 0
			m.curlTextarea.Blur()
		} else {
			m.curlTab = 0
			m.curlTextarea.Focus()
		}
		return m, nil
	case "up", "k":
		if m.curlTab == 1 {
			if m.curlPreviewScroll > 0 {
				m.curlPreviewScroll--
			}
			return m, nil
		}
	case "down", "j":
		if m.curlTab == 1 {
			m.curlPreviewScroll++
			return m, nil
		}
	case "pgup":
		if m.curlTab == 1 {
			m.curlPreviewScroll -= 10
			if m.curlPreviewScroll < 0 {
				m.curlPreviewScroll = 0
			}
			return m, nil
		}
	case "pgdown":
		if m.curlTab == 1 {
			m.curlPreviewScroll += 10
			return m, nil
		}
	case "ctrl+y":
		raw := strings.TrimSpace(m.curlTextarea.Value())
		if raw != "" {
			clipboard.WriteAll(raw)
			return m, m.setCopyFeedback("curl")
		}
		return m, nil
	case "ctrl+h":
		if m.startHostReplace() {
			return m, nil
		}
	}
	// In editor mode, delegate to textarea; in preview, ignore
	if m.curlTab == 0 {
		var cmd tea.Cmd
		m.curlTextarea, cmd = m.curlTextarea.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) handleCurlSaveKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = viewCurl
		m.curlTab = 0
		m.curlTextarea.Focus()
	case "ctrl+c":
		return m, tea.Quit
	case "enter":
		name := strings.TrimSpace(m.curlSaveName)
		if name == "" {
			return m, nil
		}
		// Save the curl input as a .curl file
		if err := m.saveCurlFile(name, m.curlTextarea.Value()); err != nil {
			m.statusMsg = errorStyle.Render("☠ Save failed: " + err.Error())
			m.mode = viewList
			m.resetSearch()
			return m, nil
		}
		m.statusMsg = successStyle.Render("✓ Saved as requests/" + name + ".curl")
		// Rescan requests
		m.rescanRequests()
		m.mode = viewList
	case "backspace":
		if len(m.curlSaveName) > 0 {
			m.curlSaveName = m.curlSaveName[:len(m.curlSaveName)-1]
		}
	default:
		s := msg.String()
		if len(s) >= 1 && s != "ctrl+v" {
			m.curlSaveName += s
		}
	}
	return m, nil
}

func (m Model) executeCurl() (Model, tea.Cmd) {
	m.mode = viewRunning
	m.spinnerIdx = 0

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFunc = cancel

	curlInput := strings.TrimSpace(m.curlTextarea.Value())
	return m, tea.Batch(
		tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return spinnerTickMsg{} }),
		func() tea.Msg {
			req, err := parser.ParseString(curlInput)
			if err != nil {
				return requestErrMsg{err: err}
			}

			args, body := parser.CurlToXhArgs(req.RawCurl)
			result, err := executor.RunCtx(ctx, args, body)
			if err != nil {
				return requestErrMsg{err: err}
			}

			storage.SaveHistory(m.baseDir, "curl-quick-run", result, curlInput)

			return requestDoneMsg{
				result:      result,
				requestName: "curl-quick-run",
				rawCurl:     curlInput,
				xhArgs:      args,
				xhBody:      body,
			}
		},
	)
}

func (m *Model) saveCurlFile(name string, content string) error {
	curlPath := filepath.Join(m.baseDir, "requests", name+".curl")
	if err := os.MkdirAll(filepath.Dir(curlPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(curlPath, []byte(content+"\n"), 0o644)
}

func (m *Model) rescanRequests() {
	requestsDir := filepath.Join(m.baseDir, "requests")
	entries, err := scanner.Scan(requestsDir)
	if err == nil {
		m.requests = entries
		m.filtered = entries
		m.cursor = 0
	}
}

// resetSearch clears any active search filter so the full list is shown.
func (m *Model) resetSearch() {
	m.search = ""
	m.filtered = m.requests
}

func (m *Model) applyFilter() {
	if m.search == "" {
		m.filtered = m.requests
		m.cursor = 0
		return
	}

	// Build string list for fuzzy matching
	names := make([]string, len(m.requests))
	for i, r := range m.requests {
		names[i] = r.Name
	}

	matches := fuzzy.Find(m.search, names)
	m.filtered = make([]scanner.RequestEntry, len(matches))
	for i, match := range matches {
		m.filtered[i] = m.requests[match.Index]
	}
	m.cursor = 0
}

func (m Model) rerunCurl() (Model, tea.Cmd) {
	m.mode = viewRunning
	m.spinnerIdx = 0
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFunc = cancel
	curlInput := m.lastRawCurl
	return m, tea.Batch(
		tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return spinnerTickMsg{} }),
		func() tea.Msg {
			req, err := parser.ParseString(curlInput)
			if err != nil {
				return requestErrMsg{err: err}
			}
			args, body := parser.CurlToXhArgs(req.RawCurl)
			result, err := executor.RunCtx(ctx, args, body)
			if err != nil {
				return requestErrMsg{err: err}
			}
			storage.SaveHistory(m.baseDir, "curl-quick-run", result, curlInput)
			return requestDoneMsg{
				result:      result,
				requestName: "curl-quick-run",
				rawCurl:     curlInput,
				xhArgs:      args,
				xhBody:      body,
			}
		},
	)
}

// handleResponseEditorKey handles keys when the response view editor tab is active.
func (m Model) handleResponseEditorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = viewList
		m.scroll = 0
		m.resetSearch()
		m.curlTextarea.Blur()
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "ctrl+x":
		input := strings.TrimSpace(m.curlTextarea.Value())
		if input == "" {
			return m, nil
		}
		return m.executeFromResponseEditor()
	case "tab", "ctrl+p":
		m.respBodyMode = bodyResponse
		m.scroll = 0
		m.curlTextarea.Blur()
		return m, nil
	case "ctrl+y":
		raw := strings.TrimSpace(m.curlTextarea.Value())
		if raw != "" {
			clipboard.WriteAll(raw)
			return m, m.setCopyFeedback("curl")
		}
		return m, nil
	case "ctrl+g":
		input := strings.TrimSpace(m.curlTextarea.Value())
		if input != "" {
			m.lastRawCurl = input
			m.mode = viewResponseSave
			m.respSaveName = ""
		}
		return m, nil
	case "ctrl+h":
		if m.startHostReplace() {
			return m, nil
		}
	}
	// Delegate to textarea
	var cmd tea.Cmd
	m.curlTextarea, cmd = m.curlTextarea.Update(msg)
	return m, cmd
}

func (m Model) executeFromReqInfoEditor() (Model, tea.Cmd) {
	m.mode = viewRunning
	m.spinnerIdx = 0
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFunc = cancel
	curlInput := strings.TrimSpace(m.curlTextarea.Value())
	entryName := m.reqInfoEntry.Name
	baseDir := m.baseDir
	return m, tea.Batch(
		tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return spinnerTickMsg{} }),
		func() tea.Msg {
			req, err := parser.ParseString(curlInput)
			if err != nil {
				return requestErrMsg{err: err}
			}
			args, body := parser.CurlToXhArgs(req.RawCurl)
			result, err := executor.RunCtx(ctx, args, body)
			if err != nil {
				return requestErrMsg{err: err}
			}
			respPath, _ := storage.SaveResponse(baseDir, entryName, result)
			storage.SaveHistory(baseDir, entryName, result, curlInput)
			return requestDoneMsg{
				result:      result,
				requestName: entryName,
				respPath:    respPath,
				rawCurl:     curlInput,
				xhArgs:      args,
				xhBody:      body,
			}
		},
	)
}

func (m Model) executeFromResponseEditor() (Model, tea.Cmd) {
	m.mode = viewRunning
	m.spinnerIdx = 0
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFunc = cancel
	curlInput := strings.TrimSpace(m.curlTextarea.Value())
	reqName := m.lastReqName
	baseDir := m.baseDir
	return m, tea.Batch(
		tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return spinnerTickMsg{} }),
		func() tea.Msg {
			req, err := parser.ParseString(curlInput)
			if err != nil {
				return requestErrMsg{err: err}
			}
			args, body := parser.CurlToXhArgs(req.RawCurl)
			result, err := executor.RunCtx(ctx, args, body)
			if err != nil {
				return requestErrMsg{err: err}
			}
			var respPath string
			if reqName != "curl-quick-run" {
				respPath, _ = storage.SaveResponse(baseDir, reqName, result)
			}
			storage.SaveHistory(baseDir, reqName, result, curlInput)
			return requestDoneMsg{
				result:      result,
				requestName: reqName,
				respPath:    respPath,
				rawCurl:     curlInput,
				xhArgs:      args,
				xhBody:      body,
			}
		},
	)
}

// copyCurrentView copies response content based on the active tab.
func (m Model) copyCurrentView() string {
	if m.lastResult == nil {
		return ""
	}
	r := m.lastResult
	var text, label string
	switch m.respBodyMode {
	case bodyResponse:
		label = "response"
		text = r.Body
	case bodyRaw:
		label = "raw response"
		if r.Headers != "" {
			text = r.Headers + "\n\n" + r.Body
		} else {
			text = r.Body
		}
	case bodyHeaders:
		label = "headers"
		text = r.Headers
	case bodyMeta:
		label = "meta"
		text = fmt.Sprintf("Request: %s\nCommand: %s\nStatus: %s\nDuration: %s",
			m.lastReqName, r.Command, r.StatusCode, r.Duration.Round(1_000_000))
	case bodyCommand:
		label = "request"
		text = m.formatCurlRequestPlain()
	}
	if text != "" {
		clipboard.WriteAll(text)
		return label
	}
	return ""
}

// copyHistoryView copies history content based on the active tab.
func (m Model) copyHistoryView() string {
	if m.historyResult == nil {
		return ""
	}
	r := m.historyResult
	var text, label string
	switch m.historyBodyMode {
	case bodyCommand:
		label = "curl"
		if m.historyCurl != "" {
			text = m.historyCurl
		} else {
			text = r.Command
		}
	case bodyResponse:
		label = "response"
		text = r.Body
	case bodyRaw:
		label = "raw response"
		if r.Headers != "" {
			text = r.Headers + "\n\n" + r.Body
		} else {
			text = r.Body
		}
	case bodyHeaders:
		label = "headers"
		text = r.Headers
	case bodyMeta:
		label = "meta"
		text = fmt.Sprintf("Timestamp: %s\nCommand: %s\nStatus: %s",
			m.historyReqName, r.Command, r.StatusCode)
	}
	if text != "" {
		clipboard.WriteAll(text)
		return label
	}
	return ""
}

func (m Model) executeRequest(entry scanner.RequestEntry) (Model, tea.Cmd) {
	m.mode = viewRunning
	m.spinnerIdx = 0
	m.statusMsg = dimStyle.Render("🧟 zombie is fetching... braaaains...")

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFunc = cancel

	return m, tea.Batch(
		tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return spinnerTickMsg{} }),
		func() tea.Msg {
			req, err := parser.ParseFile(entry.Path)
			if err != nil {
				return requestErrMsg{err: err}
			}

			args, body := parser.CurlToXhArgs(req.RawCurl)
			result, err := executor.RunCtx(ctx, args, body)
			if err != nil {
				return requestErrMsg{err: err}
			}

			// Save response
			respPath, _ := storage.SaveResponse(m.baseDir, entry.Name, result)

			// Save history
			storage.SaveHistory(m.baseDir, entry.Name, result, req.RawCurl)

			return requestDoneMsg{
				result:      result,
				requestName: entry.Name,
				respPath:    respPath,
				rawCurl:     req.RawCurl,
				xhArgs:      args,
				xhBody:      body,
			}
		},
	)
}

func (m Model) showHistory() (Model, tea.Cmd) {
	history, err := storage.ListHistory(m.baseDir)
	if err != nil {
		m.statusMsg = errorStyle.Render("☠ Cannot read history: " + err.Error())
		return m, nil
	}
	m.history = history
	m.historyCursor = 0
	m.mode = viewHistory
	return m, nil
}

// View renders the TUI
func (m Model) View() string {
	switch m.mode {
	case viewList, viewSearch:
		return m.viewList()
	case viewRunning:
		return m.viewRunning()
	case viewResponse:
		return m.viewResponse()
	case viewRequestModal:
		return m.viewRequestModal()
	case viewRequestInfo:
		return m.viewRequestInfo()
	case viewHistory:
		return m.viewHistory()
	case viewHistoryDetail:
		return m.viewHistoryDetail()
	case viewCurl:
		return m.viewCurl()
	case viewCurlSave:
		return m.viewCurlSave()
	case viewResponseSave:
		return m.viewResponseSave()
	case viewDeleteConfirm:
		return m.viewDeleteConfirm()
	case viewHistoryDeleteConfirm:
		return m.viewHistoryDeleteConfirm()
	}
	return ""
}

func (m Model) viewList() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(zombieBanner()))
	b.WriteString("\n")

	// Counters
	living := len(m.requests)
	undead := storage.CountHistory(m.baseDir)
	counters := dimStyle.Render(fmt.Sprintf("  [🌱] living requests: %d", living)) +
		"  " + dimStyle.Render(fmt.Sprintf("[🧟] undead history: %d", undead))
	b.WriteString(counters)
	b.WriteString("\n\n")

	if m.mode == viewSearch {
		b.WriteString(searchPromptStyle.Render("search: "))
		b.WriteString(normalStyle.Render(m.search))
		b.WriteString(selectedStyle.Render("█"))
		b.WriteString("\n\n")
	}

	if len(m.filtered) == 0 {
		b.WriteString(dimStyle.Render("  no requests found... the graveyard is empty 🪦"))
		b.WriteString("\n")
	} else {
		visibleHeight := m.height - 12
		if visibleHeight < 5 {
			visibleHeight = 5
		}

		start := 0
		if m.cursor >= visibleHeight {
			start = m.cursor - visibleHeight + 1
		}
		end := start + visibleHeight
		if end > len(m.filtered) {
			end = len(m.filtered)
		}

		// Scroll indicator: items above
		if start > 0 {
			b.WriteString(scrollIndicator.Render(fmt.Sprintf("  ↑ %d more above", start)))
			b.WriteString("\n")
		}

		for i := start; i < end; i++ {
			entry := m.filtered[i]
			if i == m.cursor {
				b.WriteString(selectedStyle.Render("  ▸ " + entry.Name))
			} else {
				b.WriteString(normalStyle.Render("    " + entry.Name))
			}
			b.WriteString("\n")
		}

		// Scroll indicator: items below
		remaining := len(m.filtered) - end
		if remaining > 0 {
			b.WriteString(scrollIndicator.Render(fmt.Sprintf("  ↓ %d more below", remaining)))
			b.WriteString("\n")
		}

		// Position counter
		if len(m.filtered) > visibleHeight {
			pos := dimStyle.Render(fmt.Sprintf("  %d/%d", m.cursor+1, len(m.filtered)))
			if m.mode == viewSearch {
				pos += dimStyle.Render(fmt.Sprintf(" (filtered from %d)", len(m.requests)))
			}
			b.WriteString(pos)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	if m.statusMsg != "" {
		b.WriteString(m.statusMsg)
		b.WriteString("\n")
	}

	var help string
	if m.mode == viewSearch {
		help = "[enter] select  [esc] cancel  type to filter"
	} else if len(m.filtered) == 0 {
		help = "[/] search  [c] curl  [h] history  [q] quit"
	} else {
		help = "[enter] view  [r] run  [d] delete  [/] search  [c] curl  [h] history  [q] quit"
	}
	b.WriteString(helpStyle.Render(help))

	return b.String()
}

func (m Model) viewRunning() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(zombieBanner()))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  🧟 zombie is fetching... braaaains...\n"))
	b.WriteString("\n")

	// Smooth gradient wave progress bar (ping-pong)
	barWidth := 32
	wave := []rune{'░', '▒', '▓', '█', '▓', '▒', '░'}
	waveLen := len(wave)

	// Ping-pong: go forward then backward across the bar
	cycle := 2 * barWidth
	raw := m.spinnerIdx % cycle
	center := raw
	if raw > barWidth {
		center = cycle - raw
	}

	bar := make([]rune, barWidth)
	for i := range bar {
		bar[i] = '░'
	}
	// Paint the wave centered at 'center'
	for j, ch := range wave {
		offset := j - waveLen/2
		pos := center + offset
		if pos >= 0 && pos < barWidth {
			bar[pos] = ch
		}
	}

	barStr := string(bar)
	barStyled := lipgloss.NewStyle().Foreground(zombieGreen).Render(barStr)
	b.WriteString("  " + dimStyle.Render("[") + barStyled + dimStyle.Render("]") + "\n")
	b.WriteString("\n")

	// Zombie icon + message (both change together every ~25 ticks = ~3s)
	zombies := []string{"🧟", "💀", "🦴", "🧠", "🪦"}
	msgs := []string{
		"digging up endpoints...",
		"shambling through packets...",
		"eating response headers...",
		"crawling to the server...",
		"moaning at the API...",
		"dragging bytes back...",
		"reanimating the connection...",
		"gnawing on TCP handshakes...",
		"lurching toward the host...",
		"decomposing the payload...",
		"unearthing status codes...",
		"stumbling over firewalls...",
		"haunting the DNS resolver...",
		"feasting on JSON brains...",
		"rising from the socket grave...",
		"infecting the request pipeline...",
		"groaning at slow latency...",
		"chewing through SSL certs...",
		"shuffling past load balancers...",
		"collecting severed headers...",
	}
	step := m.spinnerIdx / 25
	b.WriteString("  " + zombies[step%len(zombies)] + " ")
	msg := msgs[step%len(msgs)]
	b.WriteString(dimStyle.Render(msg) + "\n")
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  [esc/q] cancel"))

	return b.String()
}

func (m Model) viewResponse() string {
	if m.lastResult == nil {
		return dimStyle.Render("  no response data")
	}

	r := m.lastResult
	w := m.width
	if w < 40 {
		w = 80
	}
	innerW := w - 6

	var b strings.Builder

	// ─── STATUS BAR ───
	statusLine := ""
	if r.StatusCode != "" {
		statusLine = statusColor(r.StatusCode).Render("  " + r.StatusCode + "  ")
	} else if r.Error != "" {
		statusLine = errorStyle.Render("  ERROR  ")
	} else {
		statusLine = dimStyle.Render("  ???  ")
	}
	timeLine := dimStyle.Render(fmt.Sprintf("Time: %s", r.Duration.Round(1_000_000)))
	ct := extractContentType(r.Headers)
	if ct == "" {
		ct = "unknown"
	}
	typeLine := dimStyle.Render(fmt.Sprintf("Type: %s", ct))

	metaBar := lipgloss.JoinHorizontal(lipgloss.Center,
		statusLine, "  ", timeLine, "  ", typeLine)
	if m.lastReqName != "" {
		metaBar += "  " + dimStyle.Render("← "+m.lastReqName)
	}

	metaBox := sectionBorder.Width(innerW).Render(metaBar)
	b.WriteString(metaBox)
	b.WriteString("\n")

	// ─── VIEW MODE TABS ───
	tabs := []struct {
		label string
		mode  bodyMode
	}{
		{"1:editor", bodyEditor},
		{"2:response", bodyResponse},
		{"3:response raw", bodyRaw},
		{"4:headers", bodyHeaders},
		{"5:meta", bodyMeta},
		{"6:request", bodyCommand},
	}
	var tabParts []string
	for _, t := range tabs {
		if m.respBodyMode == t.mode {
			tabParts = append(tabParts, activeTabStyle.Render(t.label))
		} else {
			tabParts = append(tabParts, inactiveTabStyle.Render(t.label))
		}
	}
	tabBar := "  " + strings.Join(tabParts, " ")
	b.WriteString(tabBar)
	b.WriteString("\n\n")

	// ─── BODY CONTENT ───
	contentWrapW := innerW - 2
	if contentWrapW < 40 {
		contentWrapW = 40
	}
	var content string
	switch m.respBodyMode {
	case bodyEditor:
		// Editor is rendered separately below
		content = ""
	case bodyResponse:
		if strings.TrimSpace(r.Body) == "" {
			content = dimStyle.Render("  <empty response body>")
		} else if isJSON(r.Body) {
			content = prettyJSONWidth(r.Body, contentWrapW)
		} else {
			content = wrapContent(r.Body, contentWrapW)
		}
	case bodyRaw:
		full := ""
		if r.Headers != "" {
			full = r.Headers + "\n\n"
		}
		full += r.Body
		if full == "" {
			full = "<empty>"
		}
		content = wrapContent(full, contentWrapW)
	case bodyHeaders:
		if r.Headers != "" {
			content = formatHeaders(wrapContent(r.Headers, contentWrapW))
		} else {
			content = dimStyle.Render("  <no headers captured>")
		}
	case bodyMeta:
		var meta strings.Builder
		meta.WriteString(metaKeyStyle.Render("  Request:   ") + metaValStyle.Render(m.lastReqName) + "\n")
		meta.WriteString(metaKeyStyle.Render("  Command:   ") + dimStyle.Render(r.Command) + "\n")
		meta.WriteString(metaKeyStyle.Render("  Status:    ") + statusColor(r.StatusCode).Render(r.StatusCode) + "\n")
		meta.WriteString(metaKeyStyle.Render("  Duration:  ") + metaValStyle.Render(r.Duration.Round(1_000_000).String()) + "\n")
		meta.WriteString(metaKeyStyle.Render("  Type:      ") + metaValStyle.Render(ct) + "\n")
		if m.lastRespPath != "" {
			meta.WriteString(metaKeyStyle.Render("  Saved:     ") + dimStyle.Render(m.lastRespPath) + "\n")
		}
		if r.Error != "" {
			meta.WriteString(metaKeyStyle.Render("  Error:     ") + errorStyle.Render(r.Error) + "\n")
		}
		content = meta.String()
	case bodyCommand:
		content = m.formatCurlRequest(contentWrapW)
	}

	if m.respBodyMode == bodyEditor {
		// Editor mode - render textarea directly
		b.WriteString("  " + m.curlTextarea.View())
		b.WriteString("\n")
		if m.hostReplace {
			b.WriteString(m.hostReplaceBar())
			b.WriteString("\n")
		}
		b.WriteString("\n")
		actions := []string{
			dimStyle.Render("[ctrl+x] run"),
			dimStyle.Render("[ctrl+g] save"),
			dimStyle.Render("[ctrl+h] host"),
			dimStyle.Render("[tab] switch tab"),
			dimStyle.Render("[ctrl+y] copy"),
			dimStyle.Render("[esc] back"),
		}
		b.WriteString("  " + strings.Join(actions, "  "))
		if m.copyFeedback != "" {
			b.WriteString("  " + m.copyFeedback)
		}
		return b.String()
	}

	lines := strings.Split(content, "\n")
	totalLines := len(lines)
	viewHeight := m.height - 12
	if viewHeight < 5 {
		viewHeight = 5
	}

	// Clamp scroll
	maxOff := totalLines - viewHeight
	if maxOff < 0 {
		maxOff = 0
	}
	if m.scroll > maxOff {
		m.scroll = maxOff
	}

	start := m.scroll
	end := start + viewHeight
	if end > totalLines {
		end = totalLines
	}

	// Scroll indicator top
	if m.scroll > 0 {
		b.WriteString(scrollIndicator.Render(fmt.Sprintf("  ↑ more (%d lines above)", m.scroll)))
		b.WriteString("\n")
	}

	// Render visible lines with scrollbar
	visibleLines := lines[start:end]
	bodyContent := strings.Join(visibleLines, "\n")

	if totalLines > viewHeight {
		bar := scrollbar(m.scroll, totalLines, viewHeight)
		barLines := strings.Split(bar, "\n")
		contentLines := strings.Split(bodyContent, "\n")

		// Pad to same length
		for len(contentLines) < len(barLines) {
			contentLines = append(contentLines, "")
		}
		for len(barLines) < len(contentLines) {
			barLines = append(barLines, " ")
		}

		for i := 0; i < len(contentLines); i++ {
			b.WriteString("  " + contentLines[i])
			// Pad to width then scrollbar
			padding := innerW - lipgloss.Width(contentLines[i]) - 2
			if padding < 1 {
				padding = 1
			}
			b.WriteString(strings.Repeat(" ", padding))
			if i < len(barLines) {
				b.WriteString(barLines[i])
			}
			b.WriteString("\n")
		}
	} else {
		for _, line := range visibleLines {
			b.WriteString("  " + line + "\n")
		}
	}

	// Scroll indicator bottom
	remaining := totalLines - end
	if remaining > 0 {
		posInfo := fmt.Sprintf("line %d/%d", end, totalLines)
		b.WriteString(scrollIndicator.Render(fmt.Sprintf("  ↓ more (%d lines below)          %s", remaining, posInfo)))
		b.WriteString("\n")
	}

	// ─── ACTION BAR ───
	b.WriteString("\n")
	actions := []string{
		dimStyle.Render("[d] request"),
		dimStyle.Render("[r] rerun"),
	}
	if m.respBodyMode == bodyEditor {
		actions = append(actions, dimStyle.Render("[s] save"))
	}
	actions = append(actions,
		dimStyle.Render("[y] copy"),
	)
	if m.respBodyMode == bodyCommand {
		actions = append(actions, dimStyle.Render("[c] copy curl"))
	}
	actions = append(actions,
		dimStyle.Render("[j/k] scroll"),
		dimStyle.Render("[esc] back"),
	)
	b.WriteString("  " + strings.Join(actions, "  "))
	if m.copyFeedback != "" {
		b.WriteString("  " + m.copyFeedback)
	}

	return b.String()
}

func (m Model) viewRequestModal() string {
	w := m.width - 4
	if w < 40 {
		w = 40
	}
	if w > 100 {
		w = 100
	}
	contentH := m.height - 10
	if contentH < 5 {
		contentH = 5
	}

	var b strings.Builder

	// Title
	b.WriteString(responseHeaderStyle.Render("  ☠ REQUEST"))
	b.WriteString("\n\n")

	// Content: curl only
	wrapW := w - 6
	content := wrapContent(formatCurlPretty(m.lastRawCurl), wrapW)

	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	// Clamp scroll
	scr := m.modalScroll[0]
	maxOff := totalLines - contentH
	if maxOff < 0 {
		maxOff = 0
	}
	if scr > maxOff {
		scr = maxOff
	}

	start := scr
	end := start + contentH
	if end > totalLines {
		end = totalLines
	}

	if scr > 0 {
		b.WriteString(scrollIndicator.Render(fmt.Sprintf("  ↑ %d lines above", scr)))
		b.WriteString("\n")
	}

	for i := start; i < end; i++ {
		b.WriteString("  " + lines[i] + "\n")
	}

	remaining := totalLines - end
	if remaining > 0 {
		b.WriteString(scrollIndicator.Render(fmt.Sprintf("  ↓ %d lines below", remaining)))
		b.WriteString("\n")
	}

	// Footer
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  [y] copy curl  [j/k] scroll  [esc] back"))
	if m.copyFeedback != "" {
		b.WriteString("  " + m.copyFeedback)
	}

	return b.String()
}

func (m Model) viewHistory() string {
	var b strings.Builder

	b.WriteString(responseHeaderStyle.Render("  📜 HISTORY"))
	b.WriteString("  " + dimStyle.Render(fmt.Sprintf("[🧟] %d entries", len(m.history))))
	b.WriteString("\n\n")

	if len(m.history) == 0 {
		b.WriteString(dimStyle.Render("  no history yet... zombie has not risen 🪦"))
		b.WriteString("\n")
	} else {
		visibleHeight := m.height - 8
		if visibleHeight < 5 {
			visibleHeight = 5
		}

		start := 0
		if m.historyCursor >= visibleHeight {
			start = m.historyCursor - visibleHeight + 1
		}
		end := start + visibleHeight
		if end > len(m.history) {
			end = len(m.history)
		}

		for i := start; i < end; i++ {
			entry := m.history[i]
			// Format: "2h ago  POST /api/users  host or request-name"
			ts := timeAgo(entry.Timestamp)
			var method, host, endpoint string
			// Try .curl file first (has original curl with -X, -d flags)
			if entry.CurlPath != "" {
				if cmd, err := storage.ReadFile(entry.CurlPath); err == nil {
					method, host, endpoint = extractEndpoint(strings.TrimSpace(cmd))
				}
			}
			// Fallback to .request file (xh command)
			if method == "" && entry.ReqPath != "" {
				if cmd, err := storage.ReadFile(entry.ReqPath); err == nil {
					method, host, endpoint = extractEndpoint(strings.TrimSpace(cmd))
				}
			}
			if method == "" {
				method = "GET"
			}
			if endpoint == "" {
				endpoint = "/"
			}

			timePart := dimStyle.Render(fmt.Sprintf("%-18s", ts))
			methodPart := methodColor(method).Render(fmt.Sprintf("%-7s", method))
			fullEndpoint := endpoint
			if host != "" {
				fullEndpoint = host + endpoint
			}
			endpointPart := normalStyle.Render(fullEndpoint)
			namePart := ""
			if entry.RequestName != "" {
				namePart = "  " + dimStyle.Render("← "+entry.RequestName)
			}

			line := timePart + " " + methodPart + " " + endpointPart + namePart

			if i == m.historyCursor {
				b.WriteString(selectedStyle.Render("  ▸ ") + line)
			} else {
				b.WriteString(normalStyle.Render("    ") + line)
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  [enter] view  [d] delete  [esc] back  [j/k] scroll  [ctrl+c] quit"))

	return b.String()
}

func (m Model) viewHistoryDetail() string {
	r := m.historyResult
	if r == nil {
		return dimStyle.Render("  no data")
	}

	var b strings.Builder

	// ─── STATUS BAR ───
	b.WriteString(responseHeaderStyle.Render("  📜 HISTORY"))
	if r.StatusCode != "" {
		b.WriteString("  " + statusColor(r.StatusCode).Render(r.StatusCode))
	}
	b.WriteString("  " + dimStyle.Render(m.historyReqName))
	b.WriteString("\n\n")

	// ─── TABS ───
	tabs := []struct {
		label string
		mode  bodyMode
	}{
		{"1:request", bodyCommand},
		{"2:response", bodyResponse},
		{"3:response raw", bodyRaw},
		{"4:headers", bodyHeaders},
		{"5:meta", bodyMeta},
	}
	var tabParts []string
	for _, t := range tabs {
		if m.historyBodyMode == t.mode {
			tabParts = append(tabParts, activeTabStyle.Render(t.label))
		} else {
			tabParts = append(tabParts, inactiveTabStyle.Render(t.label))
		}
	}
	b.WriteString("  " + strings.Join(tabParts, " "))
	b.WriteString("\n\n")

	// ─── CONTENT ───
	var content string
	ct := extractContentType(r.Headers)
	wrapW := m.width - 8
	if wrapW < 40 {
		wrapW = 40
	}

	switch m.historyBodyMode {
	case bodyCommand:
		if m.historyCurl != "" {
			content = wrapContent(formatCurlPretty(m.historyCurl), wrapW)
		} else if r.Command != "" {
			content = wrapContent(formatXhCommandPretty(r.Command), wrapW)
		} else {
			content = dimStyle.Render("  <no request data>")
		}
	case bodyResponse:
		if strings.TrimSpace(r.Body) == "" {
			content = dimStyle.Render("  <empty response body>")
		} else if isJSON(r.Body) {
			content = prettyJSONWidth(r.Body, wrapW)
		} else {
			content = wrapContent(r.Body, wrapW)
		}
	case bodyRaw:
		full := ""
		if r.Headers != "" {
			full = r.Headers + "\n\n"
		}
		full += r.Body
		if full == "" {
			full = "<empty>"
		}
		content = wrapContent(full, wrapW)
	case bodyHeaders:
		if r.Headers != "" {
			content = formatHeaders(wrapContent(r.Headers, wrapW))
		} else {
			content = dimStyle.Render("  <no headers>")
		}
	case bodyMeta:
		var meta strings.Builder
		meta.WriteString(metaKeyStyle.Render("  Timestamp: ") + metaValStyle.Render(m.historyReqName) + "\n")
		meta.WriteString(metaKeyStyle.Render("  Command:   ") + dimStyle.Render(r.Command) + "\n")
		if r.StatusCode != "" {
			meta.WriteString(metaKeyStyle.Render("  Status:    ") + statusColor(r.StatusCode).Render(r.StatusCode) + "\n")
		}
		if ct != "" {
			meta.WriteString(metaKeyStyle.Render("  Type:      ") + metaValStyle.Render(ct) + "\n")
		}
		content = meta.String()
	}

	lines := strings.Split(content, "\n")
	totalLines := len(lines)
	viewHeight := m.height - 10
	if viewHeight < 5 {
		viewHeight = 5
	}

	// Clamp scroll
	scr := m.historyScroll
	maxOff := totalLines - viewHeight
	if maxOff < 0 {
		maxOff = 0
	}
	if scr > maxOff {
		scr = maxOff
	}

	start := scr
	end := start + viewHeight
	if end > totalLines {
		end = totalLines
	}

	if scr > 0 {
		b.WriteString(scrollIndicator.Render(fmt.Sprintf("  ↑ %d lines above", scr)))
		b.WriteString("\n")
	}

	for i := start; i < end; i++ {
		b.WriteString("  " + lines[i] + "\n")
	}

	remaining := totalLines - end
	if remaining > 0 {
		b.WriteString(scrollIndicator.Render(fmt.Sprintf("  ↓ %d lines below  (line %d/%d)", remaining, end, totalLines)))
		b.WriteString("\n")
	}

	// ─── FOOTER ───
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  [1-5] views  [y] copy  [j/k] scroll  [esc] back"))
	if m.copyFeedback != "" {
		b.WriteString("  " + m.copyFeedback)
	}

	return b.String()
}

func zombieBanner() string {
	return `  ╺━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━╸
       ▀███▀   ▄██▄   █▄ ▄█ █▀▀▄ ▀█▀ █▀▀▀
        ▄█▀   █▀  █▀  █ ▀ █ █▀▀▄  █  █▀▀
       ▄██▄▄ ▀█▄▄█▀  ▄█   █ █▄▄▀ ▄█▄ █▄▄▄
  ╺━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━╸
          ⚰  HTTP Request Manager`
}

func (m Model) openRequestInfo(entry scanner.RequestEntry) (Model, tea.Cmd) {
	raw, err := storage.ReadFile(entry.Path)
	if err != nil {
		m.statusMsg = errorStyle.Render("☠ Cannot read: " + err.Error())
		return m, nil
	}
	rawCurl := strings.TrimSpace(raw)
	args, body := parser.CurlToXhArgs(rawCurl)
	responses := storage.ListRequestResponses(m.baseDir, entry.Name)

	m.reqInfoEntry = entry
	m.reqInfoRawCurl = rawCurl
	m.reqInfoXhArgs = args
	m.reqInfoXhBody = body
	m.reqInfoResponses = responses
	m.reqInfoPane = 0
	m.reqInfoCursor = 0
	m.reqInfoScroll = 0
	m.curlTextarea.SetValue(rawCurl)
	for m.curlTextarea.Line() > 0 {
		m.curlTextarea.CursorUp()
	}
	m.curlTextarea.CursorStart()
	m.mode = viewRequestInfo
	return m, m.curlTextarea.Focus()
}

func (m Model) handleRequestInfoKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Pane 0: editor mode - delegate most keys to textarea
	if m.reqInfoPane == 0 {
		switch msg.String() {
		case "esc":
			m.mode = viewList
			m.resetSearch()
			m.curlTextarea.Blur()
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+x":
			input := strings.TrimSpace(m.curlTextarea.Value())
			if input == "" {
				return m, nil
			}
			return m.executeFromReqInfoEditor()
		case "ctrl+s":
			input := strings.TrimSpace(m.curlTextarea.Value())
			if input == "" {
				return m, nil
			}
			if err := os.WriteFile(m.reqInfoEntry.Path, []byte(input+"\n"), 0o644); err != nil {
				m.statusMsg = errorStyle.Render("☠ Save failed: " + err.Error())
				return m, nil
			}
			m.reqInfoRawCurl = input
			m.copyFeedback = successStyle.Render("✓ saved")
			return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearCopyMsg{} })
		case "tab":
			m.reqInfoPane = 1
			m.reqInfoResponses = storage.ListRequestResponses(m.baseDir, m.reqInfoEntry.Name)
			m.reqInfoScroll = 0
			m.reqInfoCursor = 0
			m.curlTextarea.Blur()
			return m, nil
		case "ctrl+y":
			raw := strings.TrimSpace(m.curlTextarea.Value())
			if raw != "" {
				clipboard.WriteAll(raw)
				return m, m.setCopyFeedback("curl")
			}
			return m, nil
		case "ctrl+h":
			if m.startHostReplace() {
				return m, nil
			}
		}
		// Delegate to textarea
		var cmd tea.Cmd
		m.curlTextarea, cmd = m.curlTextarea.Update(msg)
		return m, cmd
	}

	// Pane 1: responses
	switch msg.String() {
	case "esc", "q":
		m.mode = viewList
		m.resetSearch()
	case "tab", "1":
		m.reqInfoPane = 0
		m.reqInfoScroll = 0
		return m, m.curlTextarea.Focus()
	case "2":
		m.reqInfoPane = 1
		m.reqInfoScroll = 0
		m.reqInfoCursor = 0
	case "r":
		return m.executeRequest(m.reqInfoEntry)
	case "up", "k":
		if m.reqInfoCursor > 0 {
			m.reqInfoCursor--
		}
	case "down", "j":
		if m.reqInfoCursor < len(m.reqInfoResponses)-1 {
			m.reqInfoCursor++
		}
	case "enter":
		if len(m.reqInfoResponses) > 0 {
			resp := m.reqInfoResponses[m.reqInfoCursor]
			raw, err := storage.ReadFile(resp.Path)
			if err != nil {
				break
			}
			var headers, body string
			if idx := strings.Index(raw, "\n\n"); idx > 0 {
				headers = raw[:idx]
				body = raw[idx+2:]
			} else {
				body = raw
			}
			statusCode := ""
			if headers != "" {
				firstLine := strings.SplitN(headers, "\n", 2)[0]
				parts := strings.SplitN(firstLine, " ", 3)
				if len(parts) >= 2 {
					statusCode = strings.Join(parts[1:], " ")
				}
			}
			m.historyResult = &executor.Result{
				StatusCode: statusCode,
				Headers:    headers,
				Body:       body,
			}
			m.historyCurl = m.reqInfoRawCurl
			m.historyReqName = m.reqInfoEntry.Name + " · " + resp.Timestamp
			m.historyBodyMode = bodyResponse
			m.historyScroll = 0
			m.historyDetailBack = viewRequestInfo
			m.mode = viewHistoryDetail
		}
	case "y":
		if m.reqInfoRawCurl != "" {
			clipboard.WriteAll(m.reqInfoRawCurl)
			return m, m.setCopyFeedback("curl")
		}
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) viewRequestInfo() string {
	var b strings.Builder

	b.WriteString(responseHeaderStyle.Render("  📋 " + m.reqInfoEntry.Name))
	b.WriteString("\n\n")

	// Tabs
	labels := []struct {
		label string
		pane  int
	}{
		{"1:editor", 0},
		{"2:responses", 1},
	}
	var tabParts []string
	for _, t := range labels {
		if m.reqInfoPane == t.pane {
			tabParts = append(tabParts, activeTabStyle.Render(t.label))
		} else {
			tabParts = append(tabParts, inactiveTabStyle.Render(t.label))
		}
	}
	b.WriteString("  " + strings.Join(tabParts, " "))
	b.WriteString("\n\n")

	wrapW := m.width - 8
	if wrapW < 40 {
		wrapW = 40
	}

	if m.reqInfoPane == 0 {
		// Editor pane - render textarea directly
		b.WriteString("  " + m.curlTextarea.View())
		b.WriteString("\n")
		if m.hostReplace {
			b.WriteString(m.hostReplaceBar())
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("  [ctrl+x] run  [ctrl+h] host  [ctrl+s] save  [tab] responses  [ctrl+y] copy  [esc] back"))
		if m.copyFeedback != "" {
			b.WriteString("  " + m.copyFeedback)
		}
		return b.String()
	}

	// Responses pane
	var content string
	if len(m.reqInfoResponses) == 0 {
		content = dimStyle.Render("  no saved responses yet")
	} else {
		var sb strings.Builder
		sb.WriteString(dimStyle.Render(fmt.Sprintf("  %d saved responses", len(m.reqInfoResponses))))
		sb.WriteString("\n\n")
		for i, r := range m.reqInfoResponses {
			ts := timeAgo(r.Timestamp)

			// Read response file to extract status, content-type, body size
			statusBadge := dimStyle.Render("???")
			ctBadge := ""
			sizeBadge := ""
			if raw, err := storage.ReadFile(r.Path); err == nil {
				var headers, body string
				if idx := strings.Index(raw, "\n\n"); idx > 0 {
					headers = raw[:idx]
					body = raw[idx+2:]
				} else {
					body = raw
				}

				// Status from first header line: "HTTP/1.1 200 OK"
				if headers != "" {
					firstLine := strings.SplitN(headers, "\n", 2)[0]
					parts := strings.SplitN(firstLine, " ", 3)
					if len(parts) >= 2 {
						code := parts[1]
						statusBadge = statusColor(strings.Join(parts[1:], " ")).Render(code)
					}
				}

				// Content-Type
				ct := extractContentType(headers)
				if ct != "" {
					// Shorten common content types
					short := ct
					if idx := strings.Index(ct, ";"); idx > 0 {
						short = strings.TrimSpace(ct[:idx])
					}
					short = strings.TrimPrefix(short, "application/")
					short = strings.TrimPrefix(short, "text/")
					ctBadge = dimStyle.Render(short)
				}

				// Body size
				bodyLen := len(body)
				if bodyLen > 0 {
					if bodyLen < 1024 {
						sizeBadge = dimStyle.Render(fmt.Sprintf("%dB", bodyLen))
					} else if bodyLen < 1024*1024 {
						sizeBadge = dimStyle.Render(fmt.Sprintf("%.1fKB", float64(bodyLen)/1024))
					} else {
						sizeBadge = dimStyle.Render(fmt.Sprintf("%.1fMB", float64(bodyLen)/(1024*1024)))
					}
				}
			}

			timePart := fmt.Sprintf("%-16s", ts)
			detailParts := []string{statusBadge}
			if ctBadge != "" {
				detailParts = append(detailParts, ctBadge)
			}
			if sizeBadge != "" {
				detailParts = append(detailParts, sizeBadge)
			}
			details := strings.Join(detailParts, dimStyle.Render(" · "))

			if i == m.reqInfoCursor {
				sb.WriteString(selectedStyle.Render("  ▸ ") + normalStyle.Render(timePart) + " " + details)
			} else {
				sb.WriteString(normalStyle.Render("    ") + dimStyle.Render(timePart) + " " + details)
			}
			sb.WriteString("\n")
		}
		content = sb.String()
	}

	lines := strings.Split(content, "\n")
	totalLines := len(lines)
	viewHeight := m.height - 10
	if viewHeight < 5 {
		viewHeight = 5
	}

	scr := m.reqInfoScroll
	maxOff := totalLines - viewHeight
	if maxOff < 0 {
		maxOff = 0
	}
	if scr > maxOff {
		scr = maxOff
	}

	start := scr
	end := start + viewHeight
	if end > totalLines {
		end = totalLines
	}

	if scr > 0 {
		b.WriteString(scrollIndicator.Render(fmt.Sprintf("  ↑ %d lines above", scr)))
		b.WriteString("\n")
	}
	for i := start; i < end; i++ {
		b.WriteString("  " + lines[i] + "\n")
	}
	remaining := totalLines - end
	if remaining > 0 {
		b.WriteString(scrollIndicator.Render(fmt.Sprintf("  ↓ %d lines below", remaining)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  [tab/1-2] switch  [enter] view  [r] run  [j/k] navigate  [esc] back"))
	if m.copyFeedback != "" {
		b.WriteString("  " + m.copyFeedback)
	}

	return b.String()
}

func (m Model) handleResponseSaveKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = viewResponse
	case "ctrl+c":
		return m, tea.Quit
	case "enter":
		name := strings.TrimSpace(m.respSaveName)
		if name == "" {
			return m, nil
		}
		if err := m.saveCurlFile(name, m.lastRawCurl); err != nil {
			m.statusMsg = errorStyle.Render("☠ Save failed: " + err.Error())
			m.mode = viewList
			m.resetSearch()
			return m, nil
		}
		m.statusMsg = successStyle.Render("✓ Saved as requests/" + name + ".curl")
		m.rescanRequests()
		m.mode = viewList
	case "backspace":
		if len(m.respSaveName) > 0 {
			m.respSaveName = m.respSaveName[:len(m.respSaveName)-1]
		}
	default:
		s := msg.String()
		if len(s) >= 1 && s != "ctrl+v" {
			m.respSaveName += s
		}
	}
	return m, nil
}

func (m Model) viewResponseSave() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(zombieBanner()))
	b.WriteString("\n\n")
	b.WriteString(responseHeaderStyle.Render("  💾 SAVE REQUEST"))
	b.WriteString("\n\n")

	// Show curl content preview
	content := m.lastRawCurl
	lines := strings.Split(content, "\n")
	if len(lines) > 3 {
		content = strings.Join(lines[:3], "\n") + "\n..."
	}
	b.WriteString(boxStyle.Render(content))
	b.WriteString("\n\n")

	b.WriteString(normalStyle.Render("  Name (e.g. github/get-user):"))
	b.WriteString("\n")
	b.WriteString(searchPromptStyle.Render("  > "))
	b.WriteString(normalStyle.Render(m.respSaveName))
	b.WriteString(selectedStyle.Render("█"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  will save to: requests/" + m.respSaveName + ".curl"))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("[enter] save  [esc] back"))

	return b.String()
}

func (m Model) handleDeleteConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		err := storage.DeleteRecord(m.baseDir, m.deleteTarget.Name, m.deleteTarget.Path)
		if err != nil {
			m.statusMsg = errorStyle.Render("☠ Delete failed: " + err.Error())
		} else {
			m.statusMsg = successStyle.Render("✓ Deleted " + m.deleteTarget.Name + " and all its history/responses")
		}
		m.rescanRequests()
		if m.cursor >= len(m.filtered) && m.cursor > 0 {
			m.cursor--
		}
		m.mode = viewList
	case "n", "esc", "q":
		m.mode = viewList
		m.resetSearch()
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) viewDeleteConfirm() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(zombieBanner()))
	b.WriteString("\n\n")
	b.WriteString(errorStyle.Render("  ⚠ DELETE REQUEST"))
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render("  Are you sure you want to delete:"))
	b.WriteString("\n\n")
	b.WriteString(selectedStyle.Render("    " + m.deleteTarget.Name))
	b.WriteString("\n\n")

	responses := storage.ListRequestResponses(m.baseDir, m.deleteTarget.Name)
	b.WriteString(dimStyle.Render(fmt.Sprintf("  This will permanently remove:")))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("    • the .curl request file")))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("    • %d saved response(s)", len(responses))))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("    • all related history entries")))
	b.WriteString("\n\n")

	b.WriteString(errorStyle.Render("  [y] confirm delete") + "  " + helpStyle.Render("[n/esc] cancel"))

	return b.String()
}

func (m Model) handleHistoryDeleteConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		err := storage.DeleteHistoryEntry(m.baseDir, m.deleteHistoryEntry.Timestamp)
		if err != nil {
			m.statusMsg = errorStyle.Render("☠ Delete failed: " + err.Error())
		} else {
			m.statusMsg = successStyle.Render("✓ Deleted history entry " + m.deleteHistoryEntry.Timestamp)
		}
		// Refresh history list
		history, _ := storage.ListHistory(m.baseDir)
		m.history = history
		if m.historyCursor >= len(m.history) && m.historyCursor > 0 {
			m.historyCursor--
		}
		if len(m.history) == 0 {
			m.mode = viewList
			m.resetSearch()
		} else {
			m.mode = viewHistory
		}
	case "n", "esc", "q":
		m.mode = viewHistory
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) viewHistoryDeleteConfirm() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(zombieBanner()))
	b.WriteString("\n\n")
	b.WriteString(errorStyle.Render("  ⚠ DELETE HISTORY ENTRY"))
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render("  Are you sure you want to delete this history entry?"))
	b.WriteString("\n\n")

	entry := m.deleteHistoryEntry
	label := entry.Timestamp
	if entry.RequestName != "" {
		label += "  ← " + entry.RequestName
	}
	b.WriteString(selectedStyle.Render("    " + label))
	b.WriteString("\n\n")

	b.WriteString(dimStyle.Render("  This will permanently remove:"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("    • the executed command (.request)"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("    • the response (.response.json)"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("    • the curl backup (.curl)"))
	b.WriteString("\n\n")

	b.WriteString(errorStyle.Render("  [y] confirm delete") + "  " + helpStyle.Render("[n/esc] cancel"))

	return b.String()
}

func (m Model) viewCurl() string {
	var b strings.Builder
	innerW := m.width - 8
	if innerW < 40 {
		innerW = 40
	}

	// ─── TITLE ───
	title := responseHeaderStyle.Render("⚡ PASTE CURL")
	separator := dimStyle.Render(strings.Repeat("─", innerW-lipgloss.Width(title)-2))
	b.WriteString("  " + title + " " + separator)
	b.WriteString("\n\n")

	// ─── TABS ───
	editorTab := "1:editor"
	previewTab := "2:preview"
	if m.curlTab == 0 {
		editorTab = activeTabStyle.Render(editorTab)
		previewTab = inactiveTabStyle.Render(previewTab)
	} else {
		editorTab = inactiveTabStyle.Render(editorTab)
		previewTab = activeTabStyle.Render(previewTab)
	}
	b.WriteString("  " + editorTab + " " + previewTab)
	b.WriteString("\n\n")

	raw := strings.TrimSpace(m.curlTextarea.Value())
	preview := parseCurlPreview(raw)

	if m.curlTab == 0 {
		// ─── EDITOR TAB ───
		b.WriteString(m.curlTextarea.View())
		b.WriteString("\n")
		if m.hostReplace {
			b.WriteString(m.hostReplaceBar())
			b.WriteString("\n")
		}
	} else {
		// ─── PREVIEW TAB ───
		if raw == "" {
			emptyBox := curlPreviewBorder.Width(innerW - 2).Render(
				dimStyle.Render("  paste a curl command in the editor to see a preview"))
			b.WriteString("  " + emptyBox)
			b.WriteString("\n")
		} else {
			wrapW := innerW - 8 // content width inside preview box
			if wrapW < 20 {
				wrapW = 20
			}
			var pv strings.Builder

			// Method + URL
			badge := methodBadge(preview.method)
			if preview.url != "" {
				urlStr := preview.url
				if len(urlStr) > wrapW-10 {
					urlStr = urlStr[:wrapW-13] + "..."
				}
				pv.WriteString(badge + "  " + curlURLStyle.Render(urlStr))
			} else {
				pv.WriteString(badge + "  " + dimStyle.Render("<no url>"))
			}
			pv.WriteString("\n")

			// Divider
			pv.WriteString(dimStyle.Render(strings.Repeat("─", wrapW)))
			pv.WriteString("\n")

			// Headers
			if len(preview.headers) > 0 {
				pv.WriteString(curlSectionTitle.Render("HEADERS"))
				pv.WriteString(dimStyle.Render(fmt.Sprintf(" (%d)", len(preview.headers))))
				pv.WriteString("\n")
				for _, h := range preview.headers {
					var key, val string
					if idx := strings.Index(h, ": "); idx > 0 {
						key = h[:idx]
						val = h[idx+2:]
					} else if idx := strings.Index(h, ":"); idx > 0 {
						key = h[:idx]
						val = h[idx+1:]
					} else {
						pv.WriteString("  " + dimStyle.Render(truncate(h, wrapW-2)) + "\n")
						continue
					}
					maxVal := wrapW - len(key) - 4 // "  key: "
					if maxVal < 10 {
						maxVal = 10
					}
					pv.WriteString("  " + headerKeyStyle.Render(key) + dimStyle.Render(": ") + headerValStyle.Render(truncate(val, maxVal)) + "\n")
				}
			} else {
				pv.WriteString(dimStyle.Render("no headers") + "\n")
			}

			// Body
			if preview.body != "" {
				pv.WriteString("\n")
				pv.WriteString(curlSectionTitle.Render("BODY"))
				bodyLen := len(preview.body)
				pv.WriteString(dimStyle.Render(fmt.Sprintf(" (%d bytes)", bodyLen)))
				pv.WriteString("\n")
				if isJSON(preview.body) {
					pv.WriteString(wrapContent(prettyJSON(preview.body), wrapW))
				} else {
					pv.WriteString("  " + curlBodySnippet.Render(truncate(preview.body, wrapW-2)))
				}
				pv.WriteString("\n")
			}

			// Flags
			if len(preview.flags) > 0 {
				pv.WriteString("\n")
				pv.WriteString(curlSectionTitle.Render("OPTIONS") + "\n")
				for _, f := range preview.flags {
					pv.WriteString("  " + curlFlagStyle.Render(truncate("• "+f, wrapW-2)) + "\n")
				}
			}

			// Apply scroll
			pvContent := pv.String()
			pvLines := strings.Split(pvContent, "\n")
			viewH := m.height - 12
			if viewH < 5 {
				viewH = 5
			}
			maxScroll := len(pvLines) - viewH
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.curlPreviewScroll > maxScroll {
				m.curlPreviewScroll = maxScroll
			}
			start := m.curlPreviewScroll
			end := start + viewH
			if end > len(pvLines) {
				end = len(pvLines)
			}
			visible := strings.Join(pvLines[start:end], "\n")

			// Scroll indicator
			if maxScroll > 0 {
				if m.curlPreviewScroll > 0 {
					visible = scrollIndicator.Render(fmt.Sprintf("  ↑ %d lines above", m.curlPreviewScroll)) + "\n" + visible
				}
				remaining := len(pvLines) - end
				if remaining > 0 {
					visible += "\n" + scrollIndicator.Render(fmt.Sprintf("  ↓ %d lines below", remaining))
				}
			}

			previewBox := curlPreviewBorder.Width(innerW - 2).Render(visible)
			b.WriteString("  " + previewBox)
			b.WriteString("\n")
		}
	}

	// ─── COPY FEEDBACK ───
	if m.copyFeedback != "" {
		b.WriteString("\n  " + successStyle.Render(m.copyFeedback))
		b.WriteString("\n")
	}

	// ─── ACTIONS ───
	b.WriteString("\n")
	var actions []string
	if raw != "" {
		actions = append(actions, buttonStyle.Render("ctrl+x run"))
		actions = append(actions, buttonDimStyle.Render("ctrl+h host"))
		actions = append(actions, buttonDimStyle.Render("ctrl+g save"))
		actions = append(actions, buttonDimStyle.Render("ctrl+y copy"))
	} else {
		actions = append(actions, buttonDimStyle.Render("ctrl+x run"))
		actions = append(actions, buttonDimStyle.Render("ctrl+h host"))
		actions = append(actions, buttonDimStyle.Render("ctrl+g save"))
		actions = append(actions, buttonDimStyle.Render("ctrl+y copy"))
	}
	actions = append(actions, dimStyle.Render("[tab] preview"))
	actions = append(actions, dimStyle.Render("[esc] back"))
	b.WriteString("  " + strings.Join(actions, "  "))

	content := b.String()
	contentLines := strings.Count(content, "\n") + 1
	// Pad to fill terminal height (outer border uses ~4 lines for chrome)
	targetLines := m.height - 4
	if contentLines < targetLines {
		content += strings.Repeat("\n", targetLines-contentLines)
	}
	return curlOuterBorder.Width(m.width - 4).Render(content)
}

// formatCurlRequest renders the original curl command in a nice formatted view.
func (m Model) formatCurlRequest(wrapW int) string {
	raw := m.lastRawCurl
	if raw == "" {
		return dimStyle.Render("  <no curl command available>")
	}

	preview := parseCurlPreview(raw)
	var out strings.Builder

	// Method + URL
	badge := methodBadge(preview.method)
	if preview.url != "" {
		out.WriteString("  " + badge + "  " + curlURLStyle.Render(preview.url))
	} else {
		out.WriteString("  " + badge + "  " + dimStyle.Render("<no url>"))
	}
	out.WriteString("\n\n")

	// Divider
	out.WriteString("  " + dimStyle.Render(strings.Repeat("─", wrapW-4)))
	out.WriteString("\n\n")

	// Headers
	if len(preview.headers) > 0 {
		out.WriteString("  " + curlSectionTitle.Render("HEADERS"))
		out.WriteString(dimStyle.Render(fmt.Sprintf(" (%d)", len(preview.headers))))
		out.WriteString("\n")
		for _, h := range preview.headers {
			var key, val string
			if idx := strings.Index(h, ": "); idx > 0 {
				key = h[:idx]
				val = h[idx+2:]
			} else if idx := strings.Index(h, ":"); idx > 0 {
				key = h[:idx]
				val = h[idx+1:]
			} else {
				out.WriteString("    " + dimStyle.Render(h) + "\n")
				continue
			}
			out.WriteString("    " + headerKeyStyle.Render(key) + dimStyle.Render(": ") + headerValStyle.Render(val) + "\n")
		}
		out.WriteString("\n")
	}

	// Body
	if preview.body != "" {
		out.WriteString("  " + curlSectionTitle.Render("BODY"))
		out.WriteString(dimStyle.Render(fmt.Sprintf(" (%d bytes)", len(preview.body))))
		out.WriteString("\n")
		if isJSON(preview.body) {
			out.WriteString(prettyJSONWidth(preview.body, wrapW-4))
		} else {
			out.WriteString("    " + wrapContent(preview.body, wrapW-4))
		}
		out.WriteString("\n\n")
	}

	// Flags
	if len(preview.flags) > 0 {
		out.WriteString("  " + curlSectionTitle.Render("OPTIONS") + "\n")
		for _, f := range preview.flags {
			out.WriteString("    " + dimStyle.Render("• "+f) + "\n")
		}
		out.WriteString("\n")
	}

	// Raw curl
	out.WriteString("  " + curlSectionTitle.Render("RAW CURL") + "\n")
	out.WriteString("  " + dimStyle.Render(wrapContent(raw, wrapW-4)))

	return out.String()
}

// formatCurlRequestPlain builds a plain-text version of the request tab content for copying.
func (m Model) formatCurlRequestPlain() string {
	raw := m.lastRawCurl
	if raw == "" {
		if m.lastResult != nil {
			return m.lastResult.Command
		}
		return ""
	}

	preview := parseCurlPreview(raw)
	var out strings.Builder

	// Method + URL
	method := preview.method
	if method == "" {
		method = "GET"
	}
	if preview.url != "" {
		out.WriteString(method + "   " + preview.url)
	} else {
		out.WriteString(method + "   <no url>")
	}
	out.WriteString("\n\n")

	// Divider
	out.WriteString(strings.Repeat("─", 80))
	out.WriteString("\n\n")

	// Headers
	if len(preview.headers) > 0 {
		out.WriteString(fmt.Sprintf("HEADERS (%d)\n", len(preview.headers)))
		for _, h := range preview.headers {
			out.WriteString("  " + h + "\n")
		}
		out.WriteString("\n")
	}

	// Body
	if preview.body != "" {
		out.WriteString(fmt.Sprintf("BODY (%d bytes)\n", len(preview.body)))
		if isJSON(preview.body) {
			var obj interface{}
			if err := json.Unmarshal([]byte(preview.body), &obj); err == nil {
				indented, err2 := json.MarshalIndent(obj, "", "  ")
				if err2 == nil {
					out.WriteString(string(indented))
				} else {
					out.WriteString(preview.body)
				}
			} else {
				out.WriteString(preview.body)
			}
		} else {
			out.WriteString(preview.body)
		}
		out.WriteString("\n\n")
	}

	// Flags
	if len(preview.flags) > 0 {
		out.WriteString("OPTIONS\n")
		for _, f := range preview.flags {
			out.WriteString("  • " + f + "\n")
		}
		out.WriteString("\n")
	}

	// Raw curl
	out.WriteString("RAW CURL\n")
	out.WriteString(raw)

	return out.String()
}

// methodBadge returns a styled method badge.
func methodBadge(method string) string {
	method = strings.ToUpper(method)
	switch method {
	case "GET":
		return methodBadgeGET.Render(method)
	case "POST":
		return methodBadgePOST.Render(method)
	case "PUT", "PATCH":
		return methodBadgePUT.Render(method)
	case "DELETE":
		return methodBadgeDELETE.Render(method)
	default:
		if method == "" {
			method = "GET"
		}
		return methodBadgeDefault.Render(method)
	}
}

func (m Model) viewCurlSave() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(zombieBanner()))
	b.WriteString("\n\n")
	b.WriteString(responseHeaderStyle.Render("  💾 SAVE REQUEST"))
	b.WriteString("\n\n")

	// Show the curl content in a box
	content := m.curlTextarea.Value()
	lines := strings.Split(content, "\n")
	if len(lines) > 3 {
		content = strings.Join(lines[:3], "\n") + "\n..."
	}
	b.WriteString(boxStyle.Render(content))
	b.WriteString("\n\n")

	b.WriteString(normalStyle.Render("  Name (e.g. github/get-user):"))
	b.WriteString("\n")
	b.WriteString(searchPromptStyle.Render("  > "))
	b.WriteString(normalStyle.Render(m.curlSaveName))
	b.WriteString(selectedStyle.Render("█"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  will save to: requests/" + m.curlSaveName + ".curl"))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("[enter] save  [esc] back"))

	return b.String()
}

// extractURLOrigin finds the origin (scheme://host:port) from a curl command text.
// Detects: https://host, http://host, localhost, localhost:port, 127.0.0.1, IPs, etc.
func extractURLOrigin(text string) string {
	// First try URLs with scheme (http:// or https://)
	for _, scheme := range []string{"https://", "http://"} {
		idx := strings.Index(text, scheme)
		if idx < 0 {
			continue
		}
		rest := text[idx:]
		end := len(rest)
		for i, c := range rest {
			if i == 0 {
				continue
			}
			if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\'' || c == '"' || c == '\\' {
				end = i
				break
			}
		}
		fullURL := rest[:end]
		u, err := url.Parse(fullURL)
		if err == nil && u.Host != "" {
			return u.Scheme + "://" + u.Host
		}
	}

	// Then try bare hosts: localhost, 127.0.0.1, 0.0.0.0, 192.168.x.x, 10.x.x.x, etc.
	// Look for tokens that look like host or host:port followed by a path or space
	words := extractURLTokens(text)
	for _, w := range words {
		// Strip surrounding quotes
		w = strings.Trim(w, "'\"")
		if w == "" {
			continue
		}
		// Check if starts with a known bare host pattern
		host, ok := parseBareOrigin(w)
		if ok {
			return host
		}
	}
	return ""
}

// extractURLTokens pulls out tokens from a curl command that might be URLs.
// It looks for tokens after the curl command itself, skipping flags.
func extractURLTokens(text string) []string {
	var tokens []string
	// Split by whitespace and backslash-newlines
	normalized := strings.ReplaceAll(text, "\\\n", " ")
	normalized = strings.ReplaceAll(text, "\\\r\n", " ")
	parts := strings.Fields(normalized)
	for i, p := range parts {
		// Skip the "curl" command itself and flags
		if i == 0 && strings.EqualFold(p, "curl") {
			continue
		}
		if strings.HasPrefix(p, "-") {
			continue
		}
		tokens = append(tokens, p)
	}
	return tokens
}

// parseBareOrigin checks if a token starts with a bare host (no scheme) like
// localhost, localhost:8080, 127.0.0.1:3000, 0.0.0.0, 192.168.1.1, 10.0.0.1
// and returns the host:port portion.
func parseBareOrigin(token string) (string, bool) {
	token = strings.Trim(token, "'\"")

	// If it has a scheme, skip (handled by the main logic)
	if strings.HasPrefix(token, "http://") || strings.HasPrefix(token, "https://") {
		return "", false
	}

	// Extract host:port part (everything before the first /)
	hostPort := token
	if slashIdx := strings.Index(token, "/"); slashIdx >= 0 {
		hostPort = token[:slashIdx]
	}

	if hostPort == "" {
		return "", false
	}

	// Check for known bare host patterns
	lower := strings.ToLower(hostPort)

	// localhost or localhost:port
	if lower == "localhost" || strings.HasPrefix(lower, "localhost:") {
		return hostPort, true
	}

	// IP addresses: 127.x.x.x, 0.0.0.0, 192.168.x.x, 10.x.x.x, 172.x.x.x
	host := hostPort
	if colonIdx := strings.LastIndex(hostPort, ":"); colonIdx >= 0 {
		host = hostPort[:colonIdx]
	}
	if isIPAddress(host) {
		return hostPort, true
	}

	return "", false
}

// isIPAddress checks if a string looks like an IPv4 address.
func isIPAddress(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		if p == "" || len(p) > 3 {
			return false
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}

// handleHostReplaceKey handles keys during the host replace mini-mode.
func (m Model) handleHostReplaceKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.hostReplace = false
		m.hostReplaceInput = ""
		m.hostReplaceOld = ""
		return m, m.curlTextarea.Focus()
	case "enter":
		newHost := strings.TrimSpace(m.hostReplaceInput)
		if newHost != "" && m.hostReplaceOld != "" {
			old := m.curlTextarea.Value()
			replaced := strings.ReplaceAll(old, m.hostReplaceOld, newHost)
			m.curlTextarea.SetValue(replaced)
		}
		m.hostReplace = false
		m.hostReplaceInput = ""
		m.hostReplaceOld = ""
		return m, m.curlTextarea.Focus()
	case "backspace":
		if len(m.hostReplaceInput) > 0 {
			m.hostReplaceInput = m.hostReplaceInput[:len(m.hostReplaceInput)-1]
		}
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "ctrl+v":
		clip, err := clipboard.ReadAll()
		if err == nil && clip != "" {
			m.hostReplaceInput += strings.TrimSpace(clip)
		}
		return m, nil
	default:
		s := msg.String()
		if len(s) == 1 {
			m.hostReplaceInput += s
		}
		return m, nil
	}
}

// startHostReplace activates the host replace mini-mode.
func (m *Model) startHostReplace() bool {
	origin := extractURLOrigin(m.curlTextarea.Value())
	if origin == "" {
		return false
	}
	m.hostReplace = true
	m.hostReplaceOld = origin
	m.hostReplaceInput = ""
	m.curlTextarea.Blur()
	return true
}

// hostReplaceBar renders the host replace input bar.
func (m Model) hostReplaceBar() string {
	label := lipgloss.NewStyle().Foreground(zombieGreen).Bold(true).Render("  🔗 HOST ")
	arrow := lipgloss.NewStyle().Foreground(zombieGreen).Bold(true).Render(" → ")
	input := lipgloss.NewStyle().Foreground(boneWhite).Render(m.hostReplaceInput)
	cursor := selectedStyle.Render("█")
	hint := dimStyle.Render("  [enter] apply  [esc] cancel")

	// Truncate old host if it doesn't fit in the terminal width
	maxOldLen := m.width - 40 // leave room for label, arrow, cursor, hint
	if maxOldLen < 20 {
		maxOldLen = 20
	}
	oldHost := m.hostReplaceOld
	if len(oldHost) > maxOldLen {
		oldHost = oldHost[:maxOldLen-3] + "..."
	}
	old := lipgloss.NewStyle().Foreground(ghostGray).Strikethrough(true).Render(oldHost)

	line1 := label + old
	line2 := "       " + arrow + input + cursor + hint
	return line1 + "\n" + line2
}
