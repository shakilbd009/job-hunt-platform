# Design: Deduplicate DB Scan Helpers and Extract Body-Limit Middleware

## Overview
Reduce duplication in the Go backend: extract `scanApplication` method (3 call sites), `applicationColumns` constant, body-limit middleware (2 handlers), and named constants. Pure refactor — zero behavior change.

## Architecture

### Changes by File

**db.go:**
- Add `applicationColumns` package-level constant: `"id, company, role, url, salary_min, salary_max, location, status, notes, applied_at, created_at, updated_at"`
- Add `scanApplication(row interface{ Scan(dest ...any) error }) (model.Application, error)` method on `Store` — accepts both `*sql.Row` and `*sql.Rows` via the Scan interface
- Refactor `Get()` (line 100-103): use `scanApplication`
- Refactor `Update()` initial fetch (line 150-153): pass `tx.QueryRow(...)` result to `scanApplication`
- Refactor `Update()` final fetch (line 199-202): use `scanApplication`
- Use `applicationColumns` in all SELECT queries

**handler.go:**
- Create `maxBodyMiddleware(limit int64) func(http.Handler) http.Handler` — wraps `http.MaxBytesReader` on request body, returns 413 on `MaxBytesError`
- Apply in `Routes()` for POST and PUT routes via `r.With(maxBodyMiddleware(maxBodyBytes))`
- Remove inline `MaxBytesReader` from `CreateApplication` (line 79) and `UpdateApplication` (line 107)
- Remove redundant status validation from `UpdateApplication` handler (lines 119-126) — `model.ValidateStatus()` already covers this in the update path
- Add constants: `const maxBodyBytes = 1 << 20` and `const shutdownTimeout = 10 * time.Second`

**main.go:**
- Replace magic number `10 * time.Second` on line 60 with `shutdownTimeout` (imported from handler or defined locally)

## Technical Decisions

### DECISION: scanApplication interface parameter
**Chosen:** Accept `interface{ Scan(dest ...any) error }` rather than separate methods for `*sql.Row` and `*sql.Rows`.

**Rationale:** Both `*sql.Row` and `*sql.Rows` implement `Scan(dest ...any) error`. A shared interface avoids two methods. Standard Go pattern for this exact situation.

### DECISION: MaxBytesError handling in middleware vs handlers
**Chosen:** Middleware applies `MaxBytesReader`. Error detection (`MaxBytesError` type assertion) stays in handler's JSON decode error handling, since the error surfaces during `json.Decode`, not during middleware execution.

**Rationale:** `MaxBytesReader` wraps the body but doesn't read it. The error only triggers when the handler tries to decode. So the middleware wraps, the handler's existing decode error path catches the size error. The difference is: remove the inline `MaxBytesReader` call, keep the `MaxBytesError` check in decode error handling.

**Alternative:** Actually, simpler approach: middleware reads and buffers the body up to the limit, returning 413 immediately if exceeded. This removes the need for `MaxBytesError` handling in handlers entirely. **But this changes the streaming behavior** — for 1MB limit this is fine. Let me reconsider...

**Final decision:** Keep it simple — middleware wraps `MaxBytesReader`, handlers keep their existing `MaxBytesError` checks in decode paths. The only change is removing the inline `http.MaxBytesReader(w, r.Body, maxBodyBytes)` lines from each handler. The error detection on decode stays.

## Implementation Plan

Single task — this is a small, cohesive refactor.

### Task 1 (P1): Extract scanApplication, body-limit middleware, named constants
All changes in one PR:
- `db.go`: `applicationColumns` const, `scanApplication` method, refactor 3 call sites
- `handler.go`: `maxBodyMiddleware`, apply in Routes(), remove inline MaxBytesReader, remove redundant status validation, add `maxBodyBytes`/`shutdownTimeout` constants
- `main.go`: use `shutdownTimeout` constant
- Run `go test ./...` — all 37+ tests must pass

## Risks
1. **Transaction QueryRow compatibility** — `tx.QueryRow()` returns `*sql.Row`, same as `db.QueryRow()`. Interface approach handles both. Verify in tests.
2. **Removing handler status validation** — Must verify the model's `Validate()` is called in the update path. If it's only called for creates, the handler validation is needed. Check `Update()` in db.go — it validates via `allowedFields` whitelist, not `model.Validate()`. So handler-level status validation may be needed. **Hephaestus: verify this before removing.**
