# Spec: Deduplicate DB Scan Helpers and Extract Body-Limit Middleware

## Overview

Reduce code duplication in the job-hunt-platform Go backend. The same 12-field SELECT + Scan() call appears 3 times in `db.go`, MaxBytesReader wrapping is duplicated in 2 handlers, and status validation runs redundantly in both handler and model layers. This refactor consolidates each pattern into a single location, making the codebase easier to maintain when fields are added or validation rules change.

We chose inline refactoring over adding an ORM dependency because the codebase is small (224 lines in db.go) and the existing test suite (714 lines across 3 test files) will catch regressions. See deliberation on task a71d9b29.

## User Stories

- As a maintainer, I want a single `scanApplication` method so that adding a new column means updating one location, not three
- As a maintainer, I want body-size limiting applied once in middleware so that new handlers automatically get protection
- As a maintainer, I want status validation in one layer so that updating valid statuses happens in one place

## Requirements

### Functional

1. **Extract `scanApplication` method on Store** — Create a method `scanApplication(row)` (or similar, accepting `*sql.Row` or `*sql.Rows`) that encapsulates the 12-field SELECT column list and Scan call. The method returns `(model.Application, error)`.

2. **Define SELECT column constant** — Extract the repeated column list `id, company, role, url, salary_min, salary_max, location, status, notes, applied_at, created_at, updated_at` into a package-level constant (e.g., `applicationColumns`).

3. **Refactor `Get()` (db.go line 100-103)** — Replace inline QueryRow+Scan with `scanApplication`.

4. **Refactor `Update()` initial fetch (db.go lines 150-153)** — Replace inline QueryRow+Scan with `scanApplication`. Note: this uses `tx.QueryRow` not `s.db.QueryRow` — the helper must accept an interface that supports QueryRow (e.g., accept `*sql.Tx` or use an interface like `interface{ QueryRow(...) *sql.Row }`).

5. **Refactor `Update()` final fetch (db.go lines 199-202)** — Replace inline QueryRow+Scan with `scanApplication`.

6. **Extract body-limit middleware** — Create a middleware function (e.g., `maxBodySize(limit int64) func(http.Handler) http.Handler`) that wraps `http.MaxBytesReader`. Apply it in `Routes()` for POST and PUT routes. Remove inline MaxBytesReader calls from `CreateApplication` (handler.go line 79) and `UpdateApplication` (handler.go line 107).

7. **Remove redundant status validation from handler** — The handler at lines 119-126 manually validates status, but `model.ValidateStatus()` already does this. Remove the handler-level check; trust the model layer validation that's already called during request processing. If the handler currently validates status separately from the model's `Validate()`, ensure the model's validation covers the update path too.

8. **Define named constants:**
   - `maxBodySize = 1 << 20` (1 MB) — used in body-limit middleware
   - `shutdownTimeout = 10 * time.Second` — replace magic number in main.go line 60

### Non-Functional

- All existing tests must pass without modification (or with minimal adapter changes)
- No new dependencies — standard library only
- No changes to HTTP API behavior or response formats

## Edge Cases

1. **Transaction-scoped queries:** `scanApplication` must work with both `*sql.DB` and `*sql.Tx` query sources. Use a `querier` interface: `interface{ QueryRow(query string, args ...any) *sql.Row }`.
2. **MaxBytesReader error handling:** The middleware must produce the same `413 Request Entity Too Large` behavior as the current inline handling. Verify the existing MaxBytesError check (handler.go lines 82-86, 110-116) is preserved either in the middleware or in the handlers.
3. **List endpoint doesn't need body limit:** Ensure the body-limit middleware is only applied to routes with request bodies (POST, PUT), not GET or DELETE.

## Non-Goals

- Adding new fields to the Application model
- Changing the SQL schema or migration logic
- Adding pagination (separate task 8a0d91b6)
- Refactoring test files — only update tests if the refactor changes function signatures
- Adding new test coverage (separate task b27c4264)

## Success Criteria

1. `scanApplication` method exists and is used in all 3 locations (Get, Update initial, Update final)
2. `applicationColumns` constant exists — SELECT column list defined once
3. Body-limit middleware applied in Routes() — no inline MaxBytesReader in handlers
4. No redundant status validation in handler layer
5. Named constants `maxBodySize` and `shutdownTimeout` replace magic numbers
6. `go test ./...` passes with 0 failures
7. No changes to HTTP API behavior (same status codes, same response bodies)

## Delivery

- Feature branch pushed to remote
- PR opened via `gh pr create` against main
- Task updated with branch name and PR URL
