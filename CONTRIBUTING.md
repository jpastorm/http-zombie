# Contributing to zombie 🧟

Thanks for wanting to help zombie rise! Here's how to contribute.

## Getting Started

1. Fork the repo
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/zombie.git
   cd zombie
   ```
3. Install dependencies:
   ```bash
   go mod download
   ```
4. Build:
   ```bash
   go build -o zombie ./cmd/zombie/
   ```

## Development

### Project Layout

```
cmd/zombie/          → Entry point
internal/
  parser/            → .req file parser
  executor/          → xh execution engine
  scanner/           → Request file discovery
  storage/           → Response & history persistence
  tui/               → Bubble Tea TUI (views, styles)
requests/            → Sample request files
```

### Running

```bash
go run ./cmd/zombie/
```

### Testing

```bash
go test ./...
```

## Pull Requests

1. Create a branch from `main`:
   ```bash
   git checkout -b feat/my-feature
   ```
2. Make your changes
3. Run `go vet ./...` and fix any issues
4. Commit with a clear message:
   ```
   feat: add environment variable support
   fix: correct header parsing for multiline values
   ```
5. Push and open a PR

## Conventions

- Follow standard Go formatting (`gofmt`)
- Keep it minimal — zombie's philosophy is simplicity
- No external HTTP libraries — we use `xh` via exec
- TUI changes should maintain the zombie theme 🧟

## Ideas for Contributions

- [ ] Environment variable support (`{{ENV_VAR}}` in requests)
- [ ] Request templating
- [ ] Response JSON formatting/pretty-print
- [ ] Response diffing between runs
- [ ] Auth helpers (Bearer, Basic)
- [ ] Request editing inside TUI
- [ ] HTTP headers toggle view
- [ ] Export to curl command
- [ ] Import from curl command
- [ ] Request collections/groups

## Reporting Issues

Open an issue at [github.com/jpastorm/zombie/issues](https://github.com/jpastorm/zombie/issues) with:

- What you expected
- What happened instead
- Steps to reproduce
- Your OS and Go version

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
