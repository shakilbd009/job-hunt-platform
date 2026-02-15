# Spec: Configure SQLite Connection Pool Limits

## Overview

Prevent file descriptor exhaustion by configuring Go's `sql.DB` connection pool limits in the job-hunt-platform. Currently `sql.Open()` at `db.go:29` uses Go's default unlimited pool, which combined with SQLite's OS-level file descriptor limit can cause "too many open files" crashes under concurrent load. Also adds ID format validation to handler endpoints to reject invalid IDs before they hit the database.

We chose explicit pool configuration over single-connection mode because SQLite WAL mode supports concurrent readers, and the existing `busy_timeout=5000` handles write contention. See deliberation on task ad88aedc.

## User Stories

- As an operator, I want the server to handle concurrent requests without crashing from file descriptor exhaustion
- As a developer, I want invalid IDs rejected at the handler layer with a clear 400 error, not a misleading 404 from the database

## Requirements

### Functional

1. **Configure connection pool** in `NewStore()` (db.go, after line 32 `sql.Open`):
   ```go
   db.SetMaxOpenConns(25)
   db.SetMaxIdleConns(5)
   db.SetConnMaxLifetime(5 * time.Minute)
   ```

2. **Add ID format validation helper** — Create a function `isValidID(id string) bool` that validates the 8-character hex format used by the application (IDs are generated as 8-char hex strings). Pattern: `/^[0-9a-f]{8}$/`.

3. **Validate ID in `GetApplication`** (handler.go line 65): After extracting `chi.URLParam(r, "id")`, check `isValidID(id)`. If invalid, return HTTP 400 with JSON error `{"error": "invalid application ID format"}`.

4. **Validate ID in `UpdateApplication`** (handler.go line 105): Same validation as above.

5. **Validate ID in `DeleteApplication`** (handler.go line 155): Same validation as above.

6. **Add tests for ID validation:**
   - Valid 8-char hex ID passes
   - Empty string rejected
   - Too short / too long rejected
   - Non-hex characters rejected
   - SQL injection attempt rejected (e.g., `'; DROP TABLE`)

### Non-Functional

- No new dependencies — `regexp` is in Go standard library
- Pool limits are reasonable defaults for SQLite WAL mode with moderate concurrency
- ID validation adds negligible latency (regex match on 8 chars)

## Edge Cases

1. **Uppercase hex IDs:** The current ID generation uses lowercase hex. Validation should accept only lowercase (`[0-9a-f]`) to match what the system generates. If the DB contains uppercase IDs from a different source, they'll get 400 errors — this is acceptable since the system only generates lowercase.
2. **ConnMaxLifetime with SQLite:** Unlike network databases, SQLite connections don't go stale. The 5-minute lifetime is a safety net to prevent leaked connections, not a freshness concern.
3. **MaxOpenConns under heavy load:** If all 25 connections are busy, new requests will block (not fail) until a connection is available. This is the desired behavior — backpressure instead of crash.

## Non-Goals

- Adding connection pool metrics or monitoring
- Configuring pool limits via environment variables (hardcoded is fine for this scale)
- Adding rate limiting at the HTTP layer
- Changing the ID generation format or length

## Success Criteria

1. `NewStore()` configures `SetMaxOpenConns(25)`, `SetMaxIdleConns(5)`, `SetConnMaxLifetime(5 * time.Minute)`
2. `isValidID` function exists and validates 8-char lowercase hex
3. All 3 handler endpoints (Get, Update, Delete) validate ID format before DB access
4. Invalid IDs return HTTP 400 with descriptive error JSON
5. New tests cover ID validation (minimum 5 test cases)
6. `go test ./...` passes with 0 failures
7. Existing API behavior unchanged for valid IDs

## Delivery

- Feature branch pushed to remote
- PR opened via `gh pr create` against main
- Task updated with branch name and PR URL
