# Spec: Request Body Limits and Graceful Shutdown

## Overview
Harden the job-hunt-platform HTTP server against oversized payloads, missing content-type headers, and unclean shutdowns. These are defensive fixes — no new features, no API changes.

## User Stories
- As an operator, I want the server to reject oversized request bodies so that memory exhaustion attacks are prevented.
- As an operator, I want graceful shutdown so that in-flight requests complete and the DB closes cleanly on SIGTERM/SIGINT.
- As a client, I want a clear 415 error when I POST/PUT without `Content-Type: application/json`.

## Requirements

### Functional

1. **Request body limit (1 MB):** All POST/PUT handlers in `internal/handler/handler.go` must wrap `r.Body` with `http.MaxBytesReader(w, r.Body, 1<<20)` before calling `json.NewDecoder`. If the body exceeds 1 MB, the JSON decoder will return an error and the handler must respond with 413 Request Entity Too Large.

2. **Content-Type validation:** POST and PUT handlers must check that `Content-Type` is `application/json` (or starts with `application/json`). If not, respond with 415 Unsupported Media Type and a JSON error body. This can be implemented as Chi middleware or per-handler checks — implementer's choice, but middleware is preferred to avoid repetition.

3. **Graceful shutdown:** Replace `http.ListenAndServe` in `cmd/server/main.go` with:
   - Create an `http.Server` struct with the router and address.
   - Start `srv.ListenAndServe()` in a goroutine.
   - Use `signal.Notify` to catch `SIGTERM` and `SIGINT`.
   - On signal, call `srv.Shutdown(ctx)` with a 10-second timeout context.
   - After shutdown completes, `store.Close()` runs via existing defer.
   - Log the shutdown event.

4. **JSON encode error logging:** In `respondJSON()`, capture the return value of `json.NewEncoder(w).Encode(data)`. If it returns an error, log it with `log.Printf`. Do not attempt to change the HTTP status code (already written by `WriteHeader`).

### Non-Functional
- No new dependencies. All changes use stdlib (`net/http`, `os/signal`, `context`, `log`, `strings`).
- No API behavior changes for valid requests. All existing tests must continue to pass.

## Edge Cases
- Body exactly at 1 MB limit: should succeed (limit is exclusive at 1<<20 bytes).
- `Content-Type: application/json; charset=utf-8`: must be accepted (prefix match, not exact).
- DELETE and GET requests: no content-type check needed (no body expected).
- Multiple signals in rapid succession: second signal should force-exit (Go's default behavior after shutdown is called).

## Non-Goals
- Rate limiting (separate concern, not in scope).
- Request timeout middleware (can be added later).
- CORS headers (not needed for this API).
- Changing the 1 MB limit to be configurable.

## Success Criteria
1. POST/PUT with body > 1 MB returns 413 status.
2. POST/PUT without `Content-Type: application/json` returns 415 status.
3. Server shuts down cleanly on SIGTERM — logs shutdown, in-flight requests complete.
4. `respondJSON` logs encode errors to stderr/stdout.
5. All existing tests pass unchanged.
6. No new dependencies added to go.mod.

## Delivery
- Code pushed to GitHub remote on a feature branch.
- PR opened against main with passing CI (if configured).
- Task updated with branch name and PR link.
