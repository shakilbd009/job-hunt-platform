# Design: Configure SQLite Connection Pool Limits

## Overview
Configure Go's `sql.DB` connection pool limits in `NewStore()` and add ID format validation to all 3 handler endpoints that accept an application ID. Prevents file descriptor exhaustion and rejects invalid IDs before they hit the database.

## Architecture

### Changes

**db.go — Pool configuration (3 lines after `sql.Open`):**
```go
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(5 * time.Minute)
```

**handler.go — ID validation:**
- Add `isValidID(id string) bool` using `regexp.MustCompile("^[0-9a-f]{8}$")` (compiled once at package level)
- Apply in `GetApplication`, `UpdateApplication`, `DeleteApplication` before any DB access
- Return 400 with `{"error": "invalid application ID format"}` on failure

## Technical Decisions

### DECISION: MaxOpenConns(25) for SQLite
**Chosen:** 25 max connections. Allows concurrent reads in WAL mode while preventing file descriptor exhaustion.

**Rejected:** `MaxOpenConns(1)` — eliminates read concurrency. `Unlimited` (current) — risks fd exhaustion under load.

### DECISION: Compile-time regex vs inline validation
**Chosen:** `regexp.MustCompile` at package level (compiled once).

**Rejected:** `regexp.Match` per-request — recompiles regex every call, unnecessary overhead.

## Implementation Plan

Single task — 3 pool config lines + validation helper + 3 handler checks + tests.

## Risks
1. **Pool limits too restrictive** — 25 connections is generous for SQLite. If too few, requests block (not crash) — safe default.
