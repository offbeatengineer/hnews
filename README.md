# HN

A minimalist, premium Hacker News client.

![HN](./screenshot.png)

> ### Built with AI
> This entire project was written using [pi](https://pi.dev) and [Qwen3.6-27B](https://huggingface.co/unsloth/Qwen3.6-27B-GGUF) (the [UD-Q8_K_XL](https://huggingface.co/unsloth/Qwen3.6-27B-GGUF) quantization by Unsloth). The full session transcript is available at [offbeatengineer.com/labs/session-hnews](https://offbeatengineer.com/labs/session-hnews).

## Features

- **Single binary** — everything bundled, zero runtime dependencies
- **Premium dark UI** — warm amber tones, optimized for reading
- **Full comment threads** — nested, collapsible discussions
- **Instant search** — powered by Algolia's HN index
- **Keyboard shortcuts** — `/` to search, `Esc` to go back, `n` to refresh
- **Infinite scroll** — stories load as you read

## Install

### Homebrew

```bash
brew tap offbeatengineer/tap
brew install hnews
```

### From source

```bash
go install github.com/offbeatengineer/hnews@latest
```

## Usage

```bash
hnews
```

That's it. It starts a local server and opens your browser.

### Options

```bash
# Use a custom port
HNEWS_PORT=9000 hnews
```

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `/` | Focus search |
| `Enter` | Search (when in search input) |
| `Esc` | Go back / Focus search |
| `n` | Refresh current feed |

## Architecture

```
┌─────────────────────────────────────────────┐
│                 Browser                      │
│  ┌──────────┐  ┌──────────┐  ┌───────────┐ │
│  │  HTML     │  │   CSS    │  │    JS     │ │
│  │  (embed)  │  │  (embed) │  │  (embed)  │ │
│  └─────┬─────┘  └─────┬────┘  └─────┬─────┘ │
│        └──────────────┴─────────────┘       │
│              fetch() API calls               │
└──────────────────┬──────────────────────────┘
                   │ localhost:8666
┌──────────────────▼──────────────────────────┐
│              Go Binary                       │
│  ┌─────────────┐    ┌────────────────────┐  │
│  │  Web Server  │    │   API Handlers     │  │
│  │  (net/http)  │◄──►│  stories/search/   │  │
│  │              │    │  comments          │  │
│  └─────────────┘    └────────┬───────────┘  │
│                              │              │
│                     ┌────────▼───────────┐  │
│                     │  HN Firebase API   │  │
│                     │  Algolia HN API    │  │
│                     └────────────────────┘  │
└─────────────────────────────────────────────┘
```

- Frontend assets (HTML/CSS/JS) are embedded via Go's `embed.FS`
- The binary serves the frontend and proxies API calls to HN
- Comments are fetched from Algolia's HN index (full nested tree)
- Search uses Algolia's search API

## Development

```bash
# Run
go run .

# Build
go build -o hnews .

# Build optimized binary
go build -ldflags="-s -w" -o hnews .
```

## License

MIT
