# Spec: Context Propagation to DB Operations

## Overview

Add `context.Context` as the first parameter to all Store methods and switch from `Query`/`QueryRow`/`Exec` to their `*Context` variants. Handlers pass `r.Context()` so DB queries cancel when a client disconnects. This is the Go-idiomatic pattern and a structural improvement — no behavior change for the current single-user deployment.

**Deliberation:** We chose full propagation (all 6 methods) over selective (writes-only) for API consistency. The change is mechanical (~30 lines across 4 files) with 34+ existing tests as a safety net.

## User Stories

- As a developer, I want Store methods to accept context so that DB operations respect request cancellation.
- As a future multi-user deployment, I want long-running queries to abort when the requesting client disconnects.

## Requirements

### Functional

1. **Store method signatures** — All 6 public methods gain `ctx context.Context` as their first parameter:
   - `Count(ctx context.Context, status string) (int, error)`
   - `List(ctx context.Context, status string, limit, offset int) ([]model.Application, error)`
   - `Get(ctx context.Context, id string) (*model.Application, error)`
   - `Create(ctx context.Context, req model.CreateRequest) (model.Application, error)`
   - `Update(ctx context.Context, id string, fields map[string]interface{}) (model.Application, error)`
   - `Delete(ctx context.Context, id string) (bool, error)`

2. **Context-aware DB calls** — Every `db.Query`, `db.QueryRow`, `db.Exec`, `tx.QueryRow`, and `tx.Exec` call is replaced with its `*Context` variant (`QueryContext`, `QueryRowContext`, `ExecContext`), passing `ctx` as the first argument.

3. **Transaction context** — In `Update()`, the transaction must use `db.BeginTx(ctx, nil)` instead of `db.Begin()`. All operations within the transaction use `tx.QueryRowContext(ctx, ...)` and `tx.ExecContext(ctx, ...)`.

4. **Handler context passing** — All handler methods pass `r.Context()` as the first argument to Store calls:
   - `ListApplications`: `h.store.List(r.Context(), ...)` and `h.store.Count(r.Context(), ...)`
   - `GetApplication`: `h.store.Get(r.Context(), ...)`
   - `CreateApplication`: `h.store.Create(r.Context(), ...)`
   - `UpdateApplication`: `h.store.Update(r.Context(), ...)`
   - `DeleteApplication`: `h.store.Delete(r.Context(), ...)`

5. **Test updates** — All test call sites pass `context.Background()` as the first argument. No new test cases required — this is a signature change, not a behavior change.

6. **scanApplication unchanged** — The `scanApplication` helper method does not need a context parameter (it only calls `Scan()`, which is not context-aware).

### Non-Functional

- **Zero behavior change** — All 34+ existing tests must pass without modification beyond adding `context.Background()`.
- **No new dependencies** — `context` is in the standard library.

## Edge Cases

1. **Context cancellation mid-transaction** — If context is cancelled between `BeginTx` and `Commit`, the transaction rolls back automatically (Go's `database/sql` handles this). No explicit rollback-on-cancel code needed.
2. **Nil context** — Never pass `nil` context. Tests use `context.Background()`, handlers use `r.Context()`. Both are always non-nil.
3. **scanApplication with cancelled context** — `Scan()` reads from an already-fetched row buffer, so it succeeds even if context was cancelled after the query completed. No issue.

## Non-Goals

- Adding request timeouts or deadline propagation — that's a separate concern
- Adding context to `NewStore()` or `Close()` — these are lifecycle methods, not request-scoped
- Adding cancellation tests — the behavior is provided by `database/sql`, not our code
- Middleware for request timeouts — out of scope

## Success Criteria

1. All 6 Store methods accept `ctx context.Context` as first parameter
2. All DB calls use `*Context` variants
3. All handlers pass `r.Context()`
4. All tests pass with `context.Background()`
5. `go vet ./...` and `go test ./...` clean
