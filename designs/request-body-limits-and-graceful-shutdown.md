# Design: Request Body Limits and Graceful Shutdown

## Overview
Harden the HTTP server with body size limits, content-type validation, graceful shutdown, and encode error logging. Pure defensive changes — no new API surface.

## Architecture

Two files change:

1. **`internal/handler/handler.go`** — Add content-type validation middleware, body size limiting in POST/PUT handlers, and encode error logging in `respondJSON`.
2. **`cmd/server/main.go`** — Replace `http.ListenAndServe` with `http.Server` + signal-based graceful shutdown.

No new files. No new packages. No new dependencies.

## Technical Decisions

### Content-Type validation: Chi middleware (not per-handler)
The spec prefers middleware. Implement as a Chi middleware function `requireJSON` that checks `Content-Type` header on POST and PUT methods only. Uses `strings.HasPrefix` for prefix matching (accepts `application/json; charset=utf-8`). Returns 415 with JSON error body for non-matching requests. GET and DELETE pass through unchecked.

### Body size limit: per-handler wrapping (not middleware)
`http.MaxBytesReader` must wrap `r.Body` before the JSON decoder reads it. This is done inline in `CreateApplication` and `UpdateApplication` — two lines each. When the body exceeds 1MB, `json.Decoder.Decode` returns a `*http.MaxBytesError`. Detect this with `errors.As` and return 413 instead of the generic 400.

Why not middleware? `MaxBytesReader` needs the `ResponseWriter` and must be applied before any body read. Doing it per-handler right before `json.NewDecoder` is the clearest pattern and avoids replacing `r.Body` globally for routes that don't read bodies.

### Graceful shutdown: standard pattern
Replace `http.ListenAndServe` with `http.Server{Addr, Handler}`. Start `ListenAndServe` in a goroutine. Block on `signal.Notify(SIGTERM, SIGINT)`. On signal, create a 10-second timeout context, call `srv.Shutdown(ctx)`, log the event. The existing `defer store.Close()` runs after `main()` returns.

### Encode error logging: minimal change
Capture `json.NewEncoder(w).Encode(data)` return value. If error, `log.Printf("failed to encode response: %v", err)`. No status code change (already written).

## Implementation Plan

### Task 1 (P1): Handler hardening — body limits, content-type, encode logging
Files: `internal/handler/handler.go`, `internal/handler/handler_test.go`

Changes to `handler.go`:
- Add imports: `"errors"`, `"log"`, `"net/http"`, `"strings"` (some already present)
- Add `requireJSON` middleware function: checks `r.Method` is POST or PUT, then checks `Content-Type` starts with `application/json`. Returns 415 JSON error if not. Wire into `Routes()` with `r.Use(requireJSON)`.
- In `CreateApplication`: add `r.Body = http.MaxBytesReader(w, r.Body, 1<<20)` before the decoder. After `Decode`, check `errors.As(&maxBytesError)` → respond 413.
- In `UpdateApplication`: same MaxBytesReader + 413 pattern.
- In `respondJSON`: capture encode error, log with `log.Printf`.

Changes to `handler_test.go`:
- Update all POST/PUT test requests to set `req.Header.Set("Content-Type", "application/json")`. This is required because the new middleware rejects missing content-type. This is not a behavior change — real clients already send this header.
- Add test: `TestCreateApplication_OversizedBody` — send >1MB body, expect 413.
- Add test: `TestCreateApplication_MissingContentType` — POST without header, expect 415.
- Add test: `TestCreateApplication_WrongContentType` — POST with `text/plain`, expect 415.

### Task 2 (P2): Graceful shutdown
File: `cmd/server/main.go`

Changes:
- Add imports: `"context"`, `"os/signal"`, `"syscall"`, `"time"`
- Replace `http.ListenAndServe(addr, r)` block with:
  - `srv := &http.Server{Addr: addr, Handler: r}`
  - Goroutine: `go func() { if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed { log.Fatalf(...) } }()`
  - `quit := make(chan os.Signal, 1)` + `signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)`
  - `<-quit` blocks until signal
  - `ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)` + `defer cancel()`
  - `srv.Shutdown(ctx)` + log shutdown message

## Dependencies
None new. All stdlib: `net/http`, `os/signal`, `syscall`, `context`, `time`, `errors`, `log`, `strings`.

## Risks
- **Existing test breakage from content-type middleware:** Mitigated by updating test requests to set the header. This is the only tricky part — Task 1 description calls it out explicitly.
- **MaxBytesReader error detection:** `json.Decoder` wraps the underlying error. Need `errors.As` (not type assertion) to unwrap `*http.MaxBytesError`. If detection fails, falls through to existing 400 "invalid JSON body" — acceptable degraded behavior.
