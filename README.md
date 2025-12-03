# GoFindADomain

> Go find a domain! 🔍

A fast, concurrent domain availability checker with both CLI and TUI modes. Built in Go.

## Installation

### Homebrew (macOS/Linux)

```bash
brew install james-see/tap/gofindadomain
```

### Go Install

```bash
go install github.com/james-see/gofindadomain/cmd/gofindadomain@latest
```

### Download Binary

Download from [GitHub Releases](https://github.com/james-see/gofindadomain/releases).

## Prerequisites

- `whois` command must be installed on your system

## Usage

### Interactive TUI Mode

```bash
gofindadomain -i
```

Launch an interactive terminal UI where you can:
- Enter keywords
- Select TLDs from a list
- See results in real-time
- Filter to show only available domains

### CLI Mode

```bash
# Check a single TLD
gofindadomain -k mycompany -e .com

# Check multiple TLDs from file
gofindadomain -k mycompany -E tlds.txt

# Only show available domains
gofindadomain -k mycompany -E top-12.txt -x

# Update TLD list from IANA
gofindadomain --update-tld
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--keyword` | `-k` | Keyword to check (required for CLI mode) |
| `--tld` | `-e` | Single TLD to check (e.g., `.com`) |
| `--tld-file` | `-E` | File containing TLDs to check |
| `--not-registered` | `-x` | Only show available domains |
| `--interactive` | `-i` | Launch interactive TUI mode |
| `--concurrency` | `-c` | Number of concurrent checks (default: 30) |
| `--update-tld` | | Update TLD list from IANA |

## TLD Files

Two TLD files are included:

- `tlds.txt` - Full list of all TLDs from IANA (~1400 TLDs)
- `top-12.txt` - Top 12 most popular TLDs

Update the TLD list anytime:

```bash
gofindadomain --update-tld
```

## Development

```bash
# Build
go build ./cmd/gofindadomain

# Run tests
go test ./...

# Run locally
go run ./cmd/gofindadomain -k example -e .com
```

## Creating a Release

1. Tag the release:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

2. GitHub Actions will automatically:
   - Build binaries for all platforms
   - Create a GitHub Release
   - Update the Homebrew formula

## License

MIT
