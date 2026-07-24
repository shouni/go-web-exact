# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project overview

Go Web Exact (`github.com/shouni/go-web-exact/v2`) is a Go library for high-precision main-content extraction from web pages, combined with a rate-limited concurrent scraping/retry pipeline. It is a library (no `main` package) intended to be imported by consuming applications, which supply their own `Fetcher` implementation.

## Commands

```bash
go build ./...                  # build
go vet ./...                    # vet
gofmt -l .                      # must print nothing (CI enforces this)
go test -race ./...             # run all tests with race detector (as CI does)
go test -race ./extract/...     # run a single package's tests
go test -race -run TestName ./extract/...   # run a single test
golangci-lint run                # lint (config: .golangci.yml, version pinned in CI: v2.12.2)
govulncheck ./...                # vulnerability scan
```

CI (`.github/workflows/ci.yml`) runs three jobs on push/PR to `main`/`develop`: build+vet+gofmt+race-tests, golangci-lint, and govulncheck. Match these locally before pushing.

## Architecture

The library is split into five packages connected through interfaces defined in `ports/interface.go`, forming a layered pipeline. Understanding data flow across these packages (not any single file) is key to working on this codebase:

```
Fetcher (caller-supplied) -> Scraper (scraper/) -> ScrapeRunner (runner/, uses Extractor from extract/)
                                                          ^
                                              Builder (builder/) wires all of the above
```

- **`ports`**: Defines the contracts everything else depends on — `Fetcher` (raw byte + Content-Type retrieval by URL), `Extractor` (HTML -> text, both from a reader and by fetching a URL), `Scraper`/`ScrapeRunner` (run over a list of URLs, return `[]URLResult`). `URLResult` carries `URL`, `Content`, `ContentType`, `Error`. Consumers implement `Fetcher` themselves; this library does not perform HTTP requests.

- **`extract`**: `Extractor.ExtractText` parses HTML with `goquery`, locates main content via `mainContentSelectors` (falling back to excluding `header/footer/nav/aside/.sidebar/script/style/form` if no match), strips `noiseSelectors`, then walks `p, h1-h6, li, blockquote, table, pre` in DOM order. A custom recursive walk (`processGeneralElement`) excludes nested `pre`/`table` text to avoid duplication when building each element's text. Tables and titles get special prefixes (`【表題】`, `【記事タイトル】`). Short paragraphs/headings below `MinParagraphLength`/`MinHeadingLength` are dropped as noise. `FetchAndExtractText` (fetch + parse in one call, ignoring Content-Type) is only used internally by `runner`'s sequential retry path — the main pipeline calls `ExtractText` directly on already-fetched bytes.

- **`scraper`**: `Concurrent.Run` is fetch-only (no HTML parsing) — it fans out over URLs with `errgroup` (`SetLimit` = `maxConcurrency`, default 10) plus a `golang.org/x/time/rate` token-bucket limiter (default 200ms interval) shared across all goroutines, calling `Fetcher.FetchBytes` and returning the raw body + Content-Type in `URLResult`. Each URL failure is captured as a `URLResult.Error`, not a fatal error — `Run` always returns one result per URL. Configured via functional options (`WithMaxConcurrency`, `WithRateLimit`) in `options.go`. This concurrency knob governs network-fetch parallelism only; parsing parallelism is a separate knob, see `runner` below.

- **`runner`**: `ScrapeRunner.Run` is the orchestrator on top of `Scraper`: (1) initial parallel fetch pass, (2) parallel HTML-to-text extraction via a worker pool sized by `htmlWorkerCount` (default `GOMAXPROCS`) — deliberately a separate concurrency knob from the scraper's `maxConcurrency` since fetching is network-bound and parsing is CPU-bound, (3) a delay (`initialScrapeDelay`, default 5s) to ease load before retrying, (4) sequential retry (`retryScrapeDelay`, default 3s) of anything that failed or came back empty, honoring `ctx.Done()` throughout. This two-phase (parallel-then-sequential-retry) design exists to recover transient failures/empty extractions without hammering the target server. `isHTMLContentType` gates whether a result is worth running through the extractor at all (only `text/html`/`application/xhtml+xml` get parsed; other content types pass through as-is) — this gate only does something useful if the `Fetcher` implementation actually populates `ContentType` (see below).

- **`builder`**: `builder.New(fetcher, scraperOpts, runnerOpts...)` is the intended entry point for consumers — it wires `extract.NewExtractor` -> `scraper.New(fetcher, ...)` -> `runner.NewScrapeRunner(scraper, extractor, ...)` and exposes the resulting `ports.ScrapeRunner` via `Builder.ScrapeRunner()`. New wiring changes should go here rather than expecting callers to assemble the layers manually.

### `ports.Fetcher` and Content-Type

`Fetcher.FetchBytes` returns `(body []byte, contentType string, err error)`. The `contentType` is what makes `runner`'s HTML-vs-other-content gating (`isHTMLContentType`) actually work — a `Fetcher` implementation that always returns `""` silently disables that gate (every result gets treated as non-HTML and passed through unparsed). When implementing `Fetcher` against an HTTP client, populate `contentType` from the response's `Content-Type` header rather than leaving it empty.

## Conventions specific to this repo

- Source comments, log messages, and error strings are written in Japanese; keep new ones consistent with this.
- Concurrency-heavy code (`scraper`, `runner`) must remain `-race` clean; run `go test -race` when touching either package.
- Functional options (`type Option func(*T)`) are the established pattern for configuring `Concurrent` and `ScrapeRunner` — follow it for new configuration knobs rather than adding constructor parameters.
- `URLResult.Error` is how per-URL failures propagate through the pipeline (not returned/panicked) — preserve this when adding new failure paths so partial results still come back to the caller.
