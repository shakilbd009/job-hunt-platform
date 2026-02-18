# Spec: Concurrent Access and Error Path Tests

## Overview

Add test coverage for concurrent access patterns and error paths that the existing 56 tests don't cover. The Go service uses SQLite with WAL mode and a 5-second busy timeout, but no tests verify behavior under concurrent goroutine access, malformed input at system boundaries, or DB error handling. This spec defines targeted tests to fill those gaps.

Approach chosen: Go standard library testing with goroutines + WaitGroup, matching existing table-driven test style. See deliberation on task.

## User Stories

- As a developer, I want concurrent access tests so I know the service handles simultaneous requests safely
- As a maintainer, I want error path tests so I know the service returns correct HTTP status codes for adversarial input

## Requirements

### Functional

1. **Concurrent CRUD test (DB layer):** Launch 10 goroutines that each create an application, then 10 goroutines that each list all applications. Use `sync.WaitGroup` to synchronize. Assert: all 10 creates succeed, final count is 10, no panics. Run with `-race` flag.

2. **Concurrent update test (DB layer):** Create one application. Launch 5 goroutines that each update a different field (status, notes, etc.) on the same record. Assert: no errors, final record has one of the valid states (last-write-wins is acceptable for SQLite).

3. **Malformed ID tests (handler layer):** Test these ID formats via HTTP request and assert 400 or 404:
   - Empty string (`/applications/`)
   - Too long (> 100 chars)
   - SQL injection attempt (`'; DROP TABLE applications;--`)
   - Unicode/emoji characters

4. **Large payload test (handler layer):** POST a request body exceeding 1MB. Assert 413 or 400 (the server has a 1MB body limit via `http.MaxBytesReader`).

5. **Invalid Content-Type test (handler layer):** POST/PUT with `Content-Type: text/plain`. Assert 415 Unsupported Media Type (existing `requireJSON` middleware).

6. **Missing required fields test (handler layer):** POST with empty JSON `{}`. Assert 400 with validation error message.

7. **DB connection failure test (DB layer):** Attempt `NewStore` with an invalid path (e.g., `/nonexistent/dir/that/cannot/be/created/db`). Assert error is returned and contains a meaningful message.

8. **Race detector CI script:** Add a comment or note in the test file explaining that concurrent tests should be run with `go test -race ./...` to detect data races.

### Non-Functional

- Tests must be deterministic — assert final state correctness, not timing or ordering
- Concurrent tests must complete in under 10 seconds
- Follow existing table-driven test patterns in the codebase

## Edge Cases

1. **SQLite busy timeout:** If concurrent writes exceed the 5-second busy timeout, the test should handle the error gracefully (not panic)
2. **Goroutine cleanup:** All goroutines must complete before test assertions run (WaitGroup ensures this)
3. **Test isolation:** Each concurrent test must use its own DB instance (`:memory:` or temp directory) to prevent cross-test interference

## Non-Goals

- Load testing or benchmarking (no performance assertions)
- Testing network-level failures (connection drops, timeouts)
- Fuzzing (separate concern)
- Testing SQLite internal concurrency guarantees (trust the DB)

## Success Criteria

1. `go test ./...` passes with all new tests
2. `go test -race ./...` passes with no data race warnings
3. Concurrent CRUD test verifies correct final state after parallel operations
4. Malformed ID tests verify correct HTTP status codes
5. Large payload test verifies body size limit enforcement
6. All new tests follow existing table-driven patterns
