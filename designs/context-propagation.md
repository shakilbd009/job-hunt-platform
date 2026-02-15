# Design: Context Propagation to DB Operations

## Overview

Mechanical refactor: add `context.Context` as first parameter to all 6 Store methods, switch to `*Context` DB call variants, pass `r.Context()` from handlers. ~30 lines changed across 4 files. 34+ existing tests validate correctness.

## Architecture

No new components. Changes are purely signature additions:

```
Handler (r.Context()) → Store methods (ctx) → sql.DB *Context variants
```

The Update method's transaction switches from `db.Begin()` to `db.BeginTx(ctx, nil)`, and internal tx calls use `tx.QueryRowContext`/`tx.ExecContext`.

## Technical Decisions

**Single subtask (not per-file):** The changes are tightly coupled — updating a Store signature requires updating all callers (handlers + tests) simultaneously. Splitting by file would create non-compiling intermediate states.

**No new tests:** Context cancellation behavior is provided by `database/sql`, not our code. The spec explicitly marks cancellation tests as a non-goal. Existing tests validate the signature change works.

## Considered Alternatives

**Split into 2 tasks (Store + Handlers):** Rejected because changing Store signatures without updating handlers breaks compilation. The changes must ship atomically.

**Add cancellation integration test:** Rejected per spec non-goals. The `database/sql` library handles `BeginTx` cancellation/rollback — testing their implementation isn't our job.

## Implementation Plan

1. Single task: Add context parameter to all Store methods, switch to *Context DB calls, update handlers and tests

### Files to modify:
- `internal/db/db.go` — 6 method signatures + ~10 DB call replacements
- `internal/handler/handler.go` — 5 handler methods add `r.Context()` to Store calls
- `internal/db/db_test.go` — All Store call sites add `context.Background()`
- `internal/handler/handler_test.go` — No changes (tests use `httptest.NewRequest` which provides context automatically)

### Specific changes in db.go:
- `Count(status)` → `Count(ctx context.Context, status string)`, `s.db.QueryRow` → `s.db.QueryRowContext(ctx, ...)`
- `List(status, limit, offset)` → `List(ctx context.Context, status string, limit, offset int)`, `s.db.Query` → `s.db.QueryContext(ctx, ...)`
- `Get(id)` → `Get(ctx context.Context, id string)`, `s.db.QueryRow` → `s.db.QueryRowContext(ctx, ...)`
- `Create(req)` → `Create(ctx context.Context, req model.CreateRequest)`, `s.db.Exec` → `s.db.ExecContext(ctx, ...)`, internal `Get()` call → `Get(ctx, ...)`
- `Update(id, fields)` → `Update(ctx context.Context, id string, fields map[string]interface{})`, `s.db.Begin()` → `s.db.BeginTx(ctx, nil)`, `tx.QueryRow` → `tx.QueryRowContext(ctx, ...)`, `tx.Exec` → `tx.ExecContext(ctx, ...)`
- `Delete(id)` → `Delete(ctx context.Context, id string)`, `s.db.Exec` → `s.db.ExecContext(ctx, ...)`

### Import additions:
- `db.go`: add `"context"` to imports
- `handler.go`: no new imports (`net/http` already imported, `r.Context()` is a method on `*http.Request`)
- `db_test.go`: add `"context"` to imports

## Risks

- **Minimal:** Purely mechanical change. 34+ tests catch any missed call sites. `go vet` catches context misuse.
- **Compile-time safety:** Go's type system ensures every caller is updated — a missed site won't compile.
