<p align="center">
  <br>
</p>

<pre align="center">
███████╗ ██████╗ ███╗   ███╗██████╗ ██╗███████╗
╚══███╔╝██╔═══██╗████╗ ████║██╔══██╗██║██╔════╝
  ███╔╝ ██║   ██║██╔████╔██║██████╔╝██║█████╗
 ███╔╝  ██║   ██║██║╚██╔╝██║██╔══██╗██║██╔══╝
███████╗╚██████╔╝██║ ╚═╝ ██║██████╔╝██║███████╗
╚══════╝ ╚═════╝ ╚═╝     ╚═╝╚═════╝ ╚═╝╚══════╝

      ZOMBIE HTTP REQUEST MANAGER
      raise dead requests

[🌱] living requests: 128
[🧟] undead history: 542
</pre>

<p align="center">
  <strong>Terminal HTTP Request Manager</strong>
  <br>
  <em>Lightweight · File-based · Git-friendly · Terminal-first</em>
  <br><br>
  <a href="#installation">Installation</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#usage">Usage</a> •
  <a href="#request-format">Request Format</a> •
  <a href="#contributing">Contributing</a>
</p>

---

**zombie** is a minimalistic TUI tool for managing and executing HTTP requests stored as plain text files. Think of it as a terminal-native, git-friendly alternative to Postman or Insomnia — no cloud, no accounts, no bloat.

Requests live as files. Responses are saved automatically. Everything is versionable with Git.

## Features

- 🧟 **File-based workflow** — Requests are `.curl` files containing raw curl commands
- 🔍 **Fuzzy search** — Find requests instantly with fzf-style filtering
- ⚡ **Fast execution** — Powered by [xh](https://github.com/ducaale/xh) under the hood
- 💾 **Auto-save** — Responses and history stored automatically as files
- 📜 **Rich history** — Browse past requests with natural-language timestamps, HTTP methods, endpoints, and request names
- 📋 **Request info** — Inspect curl/xh translations and past responses without executing
- 🎨 **Syntax highlighting** — JSON pretty-printing, color-coded HTTP methods and status codes
- 📝 **Paste & run** — Paste curl commands directly, execute or save them as `.curl` files
- 📦 **Response viewer** — Pretty, raw, headers, and meta views with scrolling and copy support
- 🔧 **Git-friendly** — Plain text files, no binary formats, no databases

## Prerequisites

**zombie** uses [xh](https://github.com/ducaale/xh) as its HTTP execution engine. Install it first:

```bash
# macOS
brew install xh

# Arch Linux
pacman -S xh

# Cargo (any platform)
cargo install xh

# Ubuntu
snap install xh
```

## Installation

### From source

```bash
go install github.com/jpastorm/zombie/cmd/zombie@latest
```

### Clone and build

```bash
git clone https://github.com/jpastorm/http-zombie.git
cd http-zombie
make build
```

## Quick Start

```bash
# 1. Create a project directory
mkdir my-api && cd my-api

# 2. Run zombie (creates requests/, responses/, history/ automatically)
zombie

# 3. Add request files
mkdir -p requests/github
echo 'curl https://api.github.com/users/octocat' > requests/github/get-user.curl

# 4. Run zombie again — your request is ready to fire
zombie
```

## Usage

```bash
# Run from current directory
zombie

# Run from a specific directory
zombie /path/to/project
```

### Keybindings

#### Request List

| Key | Action |
|-----|--------|
| `Enter` | View request info (curl/xh/responses) |
| `r` | Execute selected request |
| `/` | Fuzzy search |
| `c` | Paste curl command |
| `h` | View history |
| `j/k` `↓/↑` | Navigate |
| `q` | Quit |

#### Response View

| Key | Action |
|-----|--------|
| `1` | Pretty view (syntax-highlighted JSON) |
| `2` | Raw view (headers + body) |
| `3` | Headers view |
| `4` | Meta view (status, duration, command) |
| `d` | View request details (curl/xh) |
| `r` | Rerun request |
| `s` | Save request as `.curl` file |
| `y` | Copy current view to clipboard |
| `j/k` | Scroll |
| `Esc` | Back |

#### Request Info

| Key | Action |
|-----|--------|
| `1` | Curl view |
| `2` | Xh view |
| `3` | Saved responses |
| `Tab` | Cycle tabs |
| `r` | Execute request |
| `Enter` | View selected response (responses tab) |
| `y` | Copy curl/xh to clipboard |
| `Esc` | Back |

#### Curl Paste Mode

| Key | Action |
|-----|--------|
| `Ctrl+X` | Execute curl command |
| `Ctrl+G` | Save as `.curl` file |
| `Esc` | Cancel |

#### History

| Key | Action |
|-----|--------|
| `Enter` | View response detail |
| `1-4` | Switch view modes (in detail) |
| `y` | Copy (in detail) |
| `j/k` | Navigate / scroll |
| `Esc` | Back |

## Project Structure

zombie expects this directory layout:

```
my-project/
├── requests/          # Your request files (you create these)
│   ├── github/
│   │   ├── get-user.curl
│   │   └── repos.curl
│   └── auth/
│       └── login.curl
├── responses/         # Saved responses (auto-generated)
│   └── github/
│       └── get-user-2026-03-11_12-01-22.json
└── history/           # Execution history (auto-generated)
    ├── 2026-03-11_12-01-22.request
    ├── 2026-03-11_12-01-22.response.json
    └── 2026-03-11_12-01-22.name
```

> **Tip:** Add `responses/` and `history/` to your `.gitignore` — only `requests/` should be versioned.

## Request Format

Requests are `.curl` files containing raw curl commands. Copy them straight from browser DevTools or documentation.

### Simple GET

```bash
curl https://api.github.com/users/octocat
```

### POST with JSON

```bash
curl -X POST https://httpbin.org/post \
  -H "Content-Type: application/json" \
  -d '{"username": "zombie", "password": "braaaains"}'
```

### With authentication

```bash
curl -u admin:secret https://api.example.com/admin
```

### With multiple headers

```bash
curl -X GET https://api.example.com/data \
  -H "Accept: application/json" \
  -H "Authorization: Bearer token123" \
  -L --compressed
```

### Just a URL (no curl prefix)

```
https://httpbin.org/ip
```

## How It Works

1. **Scan** — zombie recursively discovers all `.curl` files in `requests/`
2. **Display** — Files are listed in a navigable TUI with fuzzy search
3. **Translate** — curl command is translated to `xh` arguments automatically
4. **Execute** — Request is executed via `xh`
5. **Store** — Response saved to `responses/`, full request+response to `history/`

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | [Go](https://go.dev) |
| TUI Framework | [Bubble Tea](https://github.com/charmbracelet/bubbletea) |
| Styling | [Lip Gloss](https://github.com/charmbracelet/lipgloss) |
| Components | [Bubbles](https://github.com/charmbracelet/bubbles) |
| Search | [sahilm/fuzzy](https://github.com/sahilm/fuzzy) |
| HTTP Engine | [xh](https://github.com/ducaale/xh) |
| Clipboard | [atotto/clipboard](https://github.com/atotto/clipboard) |

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

[MIT](LICENSE) © [jpastorm](https://github.com/jpastorm)