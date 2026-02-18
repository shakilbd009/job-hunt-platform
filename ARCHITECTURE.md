# ARCHITECTURE.md — Job Hunt Platform

> Last updated: 2026-02-12 by Metis (Strategic Architect)

## Overview

Self-contained REST API for tracking job applications. Go binary with embedded SQLite — no external services, no runtime dependencies beyond the compiled binary.

## System Diagram

```
┌─────────────────────────────────────────────┐
│  cmd/server/main.go                         │
│  ┌─────────────────────────────────────────┐│
│  │ Chi Router + Middleware Stack            ││
│  │  Logger → Recoverer → requireJSON       ││
│  └──────────────┬──────────────────────────┘│
│                 │                            │
│  ┌──────────────▼──────────────────────────┐│
│  │ internal/handler                        ││
│  │  GET/POST/PUT/DELETE /applications      ││
│  │  Input validation, error responses      ││
│  └──────────────┬──────────────────────────┘│
│                 │                            │
│  ┌──────────────▼──────────────────────────┐│
│  │ internal/db                             ││
│  │  Store struct — CRUD, migrations, txns  ││
│  └──────────────┬──────────────────────────┘│
│                 │                            │
│  ┌──────────────▼──────────────────────────┐│
│  │ internal/model                          ││
│  │  Application, CreateRequest, validation ││
│  └─────────────────────────────────────────┘│
│                 │                            │
│         ┌───────▼───────┐                    │
│         │ data/tracker.db│                   │
│         │ SQLite (WAL)   │                   │
│         └───────────────┘                    │
└─────────────────────────────────────────────┘
```

## Package Structure

| Package | Responsibility |
|---------|---------------|
| `cmd/server` | Entry point. Initializes Store, mounts router, starts HTTP with graceful shutdown (SIGTERM/SIGINT, 10s drain). |
| `internal/handler` | HTTP handlers for 5 REST endpoints. Enforces content-type, body size limits (1 MB), status validation. |
| `internal/db` | SQLite Store wrapping `*sql.DB`. Auto-migrates schema on init. CRUD with transaction safety on updates. `Count()` for pagination totals, `List()` supports limit/offset. |
| `internal/model` | Domain types: `Application` (12 fields), `CreateRequest`. Status validation against 9 valid values. Salary cross-validation. |

## API Surface

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/applications` | Paginated list (`?status=`, `?limit=`, `?offset=`) |
| GET | `/applications/{id}` | Get by ID |
| POST | `/applications` | Create (requires company + role) |
| PUT | `/applications/{id}` | Partial update |
| DELETE | `/applications/{id}` | Delete |

**Pagination envelope** (GET /applications):
```json
{"data": [...], "pagination": {"total": 42, "limit": 20, "offset": 0, "has_more": true}}
```
Default limit: 20, max limit: 100. Offset beyond total returns empty data with correct total.

Error responses: `{"error": "message"}` with appropriate HTTP status codes (400, 404, 413, 415, 500).

## Data Model

Single table `applications` with 12 columns. ID is truncated UUID (first 8 chars). Timestamps in RFC3339. Status is one of: `wishlist`, `applied`, `phone_screen`, `interview`, `offer`, `accepted`, `rejected`, `withdrawn`, `ghosted`. Default status: `wishlist`.

## Technical Decisions

1. **Pure Go SQLite (`modernc.org/sqlite`)** — No CGO dependency. Simplifies cross-compilation and deployment. Trade-off: slightly slower than `mattn/go-sqlite3` but eliminates C toolchain requirement.

2. **Chi router** — Lightweight, stdlib-compatible. No framework lock-in — handlers use standard `http.ResponseWriter`/`*http.Request` signatures.

3. **No ORM** — Raw SQL with parameterized queries. Schema is stable (single table), query patterns are simple. ORM would add complexity without benefit.

4. **Transaction wrapping on Update** — `db.Update()` uses `sql.Tx` with deferred Rollback + explicit Commit. Prevents partial updates on concurrent access.

5. **WAL mode** — Enables concurrent reads during writes. Appropriate for single-user tool that may have concurrent API requests.

6. **Short UUIDs** — First 8 chars of UUID v4. Collision risk is negligible at expected scale (hundreds of applications, not millions).

7. **Middleware-based content-type enforcement** — `requireJSON` middleware rejects POST/PUT without `application/json` with 415 before handler logic runs.

## Testing

Three test files covering all layers:
- `model_test.go` — Validation logic (unit, no DB)
- `handler_test.go` — HTTP integration tests (17+ cases, including pagination edge cases)
- `db_test.go` — Store integration tests with in-memory SQLite (20+ cases)

All use stdlib `testing` only. No external test frameworks.

## Configuration

| Env Var | Default | Purpose |
|---------|---------|---------|
| `PORT` | `8081` | HTTP listen port |
| `DB_PATH` | `./data/tracker.db` | SQLite database file path |

## Cross-Project Notes

- **Timestamps:** Uses RFC3339 (Go `time.RFC3339`), which is UTC with `Z` suffix. Consistent with ISO 8601. No drift risk with other projects.
- **No shared dependencies** with clawd-aed or other Forge projects. Fully standalone.
- **SQLite pattern:** WAL mode + parameterized queries matches clawd-aed convention. Different driver (pure Go vs better-sqlite3/Node) but same access patterns.
