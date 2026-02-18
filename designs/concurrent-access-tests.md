# Design: Concurrent Access and Error Path Tests

## Overview

Add targeted tests for concurrent goroutine access and error/edge-case paths that the existing 56 tests don't cover. All new tests follow the project's existing patterns: stdlib testing, table-driven cases, `t.Helper()` setup functions, in-memory SQLite.

## Architecture

New tests added to existing test files — no new files needed.

```
internal/
├── db/
│   └── db_test.go          ← +2 concurrent tests, +1 connection failure test
├── handler/
│   └── handler_test.go     ← +4 error path tests (malformed ID, large payload, content-type, empty body)
└── model/
    └── model_test.go       ← (no changes)
```

## Technical Decisions

### 1. Concurrent tests in db_test.go (not handler_test.go)
Concurrent access tests target the DB layer directly because that's where the real concurrency concern is — SQLite's WAL mode and busy timeout. Testing at the HTTP layer would add httptest overhead without testing anything new (chi's router is already thread-safe). The DB layer is where races could corrupt data.

### 2. File-based SQLite for concurrent tests (not `:memory:`)
In-memory SQLite databases are per-connection and don't support concurrent access from multiple goroutines sharing a `*sql.DB` pool. Concurrent tests must use a file-based DB in `t.TempDir()` with WAL mode to exercise the real locking behavior.

### 3. Table-driven tests for malformed IDs
Matches existing pattern in `model_test.go`. Each malformed ID case is a struct with `name`, `id`, and `wantStatus`. This is extensible — adding more adversarial inputs later is trivial.

### 4. Last-write-wins assertion for concurrent updates
SQLite serializes writes. When 5 goroutines update the same record, all 5 should succeed (WAL + busy timeout), and the final record should contain a valid value for each updated field. We assert no errors and valid final state — not which goroutine "won," since ordering is non-deterministic.

## Considered Alternatives

**Decision 1: DB-layer vs. HTTP-layer concurrent tests**
Chose DB-layer because the concurrency concern is SQLite locking, not HTTP routing. Chi's router dispatches to handlers synchronously — there's no HTTP-level concurrency bug possible that isn't also a DB-level bug. HTTP-layer concurrent tests were rejected because they'd test the same thing with more setup and slower execution (httptest + JSON encoding/decoding per request).

**Decision 2: File-based vs. in-memory SQLite for concurrency**
Chose file-based because `:memory:` databases don't support WAL mode and behave differently under concurrent access. The whole point is to test WAL + busy timeout behavior. In-memory was rejected because it wouldn't test the actual concurrency model.

**Decision 3: Deterministic assertions vs. timing assertions**
Chose final-state assertions (all creates succeeded, count is correct, record is valid) over timing assertions (operation took <Xms). Timing-based tests are inherently flaky across different machines. The spec explicitly requires deterministic assertions.

## Implementation Plan

### Task 1 (P1): Add concurrent DB tests and error path tests

**In `internal/db/db_test.go`:**

1. `TestConcurrentCreate` — Create a file-based store in `t.TempDir()`. Launch 10 goroutines via `sync.WaitGroup`, each calling `store.Create(...)` with unique company names. After all complete, call `store.List("")` and assert count is 10 and no errors occurred. Collect errors via a `chan error` or `sync.Mutex`-protected slice.

2. `TestConcurrentUpdateSameRecord` — Create a file-based store and one application. Launch 5 goroutines, each updating a different field on the same record (goroutine 0 updates status to "applied", goroutine 1 updates notes, goroutine 2 updates location, etc.). Assert: all 5 updates succeed, final record has valid non-zero values for all updated fields.

3. `TestNewStoreInvalidPath` — Call `NewStore("/nonexistent/deeply/nested/dir/db.sqlite")`. Assert error is non-nil and contains "opening database" or similar.

**In `internal/handler/handler_test.go`:**

4. `TestMalformedIDs` — Table-driven test with cases:
   - `"too-long"`: 200-char string → expect 400 or 404
   - `"sql-injection"`: `"'; DROP TABLE applications;--"` → expect 400 or 404
   - `"unicode"`: `"🎉emoji🎉"` → expect 400 or 404
   - `"spaces"`: `"has spaces"` → expect 400 or 404
   For each case, send GET `/applications/{id}` and assert status is 400 or 404 (not 500, not a panic). Note: if the project adds the `isValidID` regex validation (from sqlite-pool-config design), these should return 400. Without it, they'll return 404 (ID not found). Both are acceptable — the test verifies no 500 or panic.

5. `TestOversizedPayload` — POST `/applications` with `Content-Type: application/json` and a body of 2MB (repeat `"x"` chars in a JSON string field). Assert 413 status code.

6. `TestInvalidContentType` — POST `/applications` with `Content-Type: text/plain` and valid JSON body. Assert 415 status code. (This may already be tested — check first. If so, skip.)

7. `TestEmptyJSONBody` — POST `/applications` with `Content-Type: application/json` and body `{}`. Assert 400 with validation error mentioning "company" (required field).

**Race detector note:** Add a comment at the top of the concurrent test functions: `// Run with: go test -race ./internal/db/`

## Dependencies

- `sync` (WaitGroup) — Go stdlib
- `strings` (Repeat for large payload) — Go stdlib
- No new dependencies

## Risks

1. **Concurrent SQLite busy timeout:** If 10 simultaneous writes exceed the 5-second busy timeout, some creates could fail with SQLITE_BUSY. Mitigation: 10 inserts should complete well within 5 seconds. If flaky, reduce to 5 goroutines.
2. **Malformed ID behavior depends on validation:** Without `isValidID`, malformed IDs hit the DB and return 404 (not found). With it, they return 400. The test should accept either 400 or 404 — both mean "handled correctly." It should fail only on 500 (server error) or panic.
3. **Oversized payload test assumes MaxBytesReader:** The handler uses `http.MaxBytesReader` with 1MB limit. If this was removed or changed, the test would fail. This is correct behavior — the test documents the contract.
