# Contributing to symvibe

Thank you for your interest in contributing to symvibe! This document provides guidelines and instructions for contributing.

## Prerequisites

- **Go 1.26+** — required for building and testing the core binary
- **Node.js** (optional) — only needed if you modify the web board (`web/` directory)
- **Swift toolchain** (optional) — for iOS/macOS client development (`client/` directory)
- **opencode** — runtime peer for running cycles (not required for building/testing)
- **gh** (optional) — GitHub CLI for issue/PR workflows

## Getting Started

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/<your-username>/symaira-vibecoder.git
   cd symaira-vibecoder
   ```
3. Create a feature branch:
   ```bash
   git checkout -b issue/<number>-<short-description>
   ```

## Development Workflow

### Building

```bash
make build        # builds ./symvibe binary
make install      # installs to GOBIN
```

### Testing

```bash
make test         # unit tests
make test-race    # tests with race detector (requires CGO_ENABLED=1)
```

### Linting

```bash
make lint         # runs gofmt + go vet
```

Before submitting a PR, ensure:
- `gofmt -l ./cmd ./internal ./web/embed.go` produces no output
- `go vet ./...` passes
- All tests pass

### Running the Board Locally

```bash
make run          # builds and starts the server
# or
make dev          # hot-reload development mode (no browser)
```

The board will be available at `http://127.0.0.1:4317`.

## Code Style

- **Go code**: Follow standard Go conventions
  - Use `gofmt` for formatting (run `make lint` to check)
  - Use `go vet` for static analysis
  - Write meaningful variable and function names
  - Add comments for exported functions and complex logic
- **Commit messages**: Write clear, concise commit messages describing the change
- **PR titles**: Should reference the issue number and describe the change (e.g., "Fix login crash #123")

## Submitting a Pull Request

1. Ensure your changes follow the code style guidelines above
2. Run the full test suite: `make test`
3. Run linting: `make lint`
4. Push your branch and create a pull request
5. In your PR description:
   - Reference the issue(s) this PR addresses (e.g., "Closes #123")
   - Describe what changed and why
   - Include any relevant screenshots or test results

## Reporting Issues

- Use the GitHub issue templates when available
- Include steps to reproduce for bug reports
- For feature requests, describe the use case and proposed solution

## Code of Conduct

Please be respectful and constructive in all interactions. See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) for details.

## License

By contributing to symvibe, you agree that your contributions will be licensed under the [Apache-2.0 License](LICENSE).
