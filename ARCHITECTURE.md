# ARCHITECTURE.md — Job Hunt Platform

> Last updated: 2026-02-19 by Metis (Strategic Architect) — Cycle 3, sorting/filtering complete

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
│  │  Query param parsing, validation        ││
│  └──────────────┬──────────────────────────┘│
│                 │                            │
│  ┌──────────────▼──────────────────────────┐│
│  │ internal/db                             ││
│  │  Store struct — CRUD, List(), Count()   ││
│  │  Dynamic WHERE + ORDER BY builders      ││
│  └──────────────┬──────────────────────────┘│
│                 │                            │
│  ┌──────────────▼──────────────────────────┐│
│  │ internal/model                          ││
│  │  Application, CreateRequest,            ││
│  │  ListOptions, ValidSortColumns          ││
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
| `internal/handler` | HTTP handlers for 5 REST endpoints. Query param parsing for filtering/sorting. Content-type enforcement. |
| `internal/db` | SQLite Store. `List()` and `Count()` accept `ListOptions` for dynamic query building. Shared `buildWhere()` helper. |
| `internal/model` | Domain types: `Application`, `CreateRequest`, `ListOptions`. `ValidStatuses` and `ValidSortColumns` allowlists. |

## API Surface

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/applications` | Paginated, sortable, filterable list |
| GET | `/applications/{id}` | Get by ID |
| POST | `/applications` | Create (requires company + role) |
| PUT | `/applications/{id}` | Partial update |
| DELETE | `/applications/{id}` | Delete |
| GET | `/applications/stats` | Aggregate metrics (by status, salary range, recent activity) |
| GET | `/health` | Health check with DB connectivity |

### Pagination + Sorting + Filtering (GET /applications)

**Query Parameters:**

| Param | Type | Description |
|-------|------|-------------|
| `limit` | int | Page size (default 20, max 100) |
| `offset` | int | Pagination offset |
| `sort_by` | string | Column to sort by (8-column allowlist) |
| `sort_order` | string | `asc` or `desc` (default `desc` for dates, `asc` for text) |
| `status` | string | Filter by exact status match |
| `company` | string | Substring filter (case-insensitive LIKE) |
| `role` | string | Substring filter (case-insensitive LIKE) |
| `location` | string | Substring filter (case-insensitive LIKE) |
| `applied_after` | date (YYYY-MM-DD) | Date range filter (inclusive) |
| `applied_before` | date (YYYY-MM-DD) | Date range filter (inclusive) |
| `salary_min_gte` | int | Salary range filter (>=) |
| `salary_max_lte` | int | Salary range filter (<=) |

**Response envelope:**
```json
{
  "data": [...],
  "pagination": {
    "total": 42,
    "limit": 20,
    "offset": 0,
    "has_more": true
  }
}
```

Error responses: `{"error": "message"}` with appropriate HTTP status codes (400, 404, 413, 415, 500).

## Data Model

Single table `applications` with 12 columns:
- ID: 8-char truncated UUID
- Timestamps: RFC3339 UTC
- Status: 9 valid values via `ValidStatuses` map
- Salary: min/max integers (0 = unspecified)
- `applied_at`: ISO date (YYYY-MM-DD) separate from `created_at`

## Technical Decisions

1. **Pure Go SQLite (`modernc.org/sqlite`)** — No CGO dependency. Simplifies cross-compilation.

2. **Chi router** — Lightweight, stdlib-compatible.

3. **No ORM** — Raw SQL with parameterized queries. Schema is stable; ORM would add complexity.

4. **Transaction wrapping on Update** — `db.Update()` uses `sql.Tx` with deferred Rollback + explicit Commit.

5. **WAL mode** — Enables concurrent reads during writes.

6. **Short UUIDs** — First 8 chars of UUID v4. Collision risk negligible at expected scale.

7. **ListOptions struct** — Bundles filter/sort/pagination params. Avoids 12+ function arguments. Cleaner than individual params or `map[string]interface{}`.

8. **ValidSortColumns allowlist** — Column names validated against hard-coded map before SQL construction. Defense-in-depth against column injection. Mirrors `ValidStatuses` pattern.

9. **Shared `buildWhere()` helper** — Single source of truth for WHERE clause construction. Used by both `List()` and `Count()` to ensure filtered count matches returned results.

10. **Boolean flags for zero-value disambiguation** — `HasSalaryMinGTE` distinguishes "not set" from "set to 0". Alternative (`*int` pointers) is more idiomatic but boolean flags are more explicit.

11. **LOWER() for case-insensitive LIKE** — Explicit over `COLLATE NOCASE`. Trade-off: prevents index usage on those columns. Acceptable at current scale; add comment if dataset grows.

## Query Building Architecture

```go
// internal/db/db.go
func (s *Store) buildWhere(opts model.ListOptions) (string, []interface{}) {
    // Base clause for uniform AND-chaining
    where := "WHERE 1=1"
    args := []interface{}{}
    
    if opts.Status != "" {
        where += " AND status = ?"
        args = append(args, opts.Status)
    }
    if opts.Company != "" {
        where += " AND LOWER(company) LIKE LOWER(?)"
        args = append(args, "%"+opts.Company+"%")
    }
    // ... additional filters
    return where, args
}
```

**Ordering:** Dynamic ORDER BY with column allowlist validation:
```go
orderBy := "updated_at DESC" // default
if opts.SortBy != "" && model.ValidSortColumns[opts.SortBy] {
    order := "ASC"
    if opts.SortOrder == "desc" {
        order = "DESC"
    }
    orderBy = fmt.Sprintf("%s %s", opts.SortBy, order)
}
```

## Testing

| Suite | File | Coverage |
|-------|------|----------|
| Model | `model_test.go` | Validation logic, ListOptions edge cases |
| Handler | `handler_test.go` | HTTP integration, query param parsing, validation |
| DB | `db_test.go` | Store operations, concurrent access, error paths |

All use stdlib `testing`. No external test frameworks.

**Key test patterns:**
- Concurrent DB tests verify WAL mode behavior
- Error path tests verify graceful handling of connection failures
- Handler tests verify query param validation (invalid sort columns, malformed dates)

## Configuration

| Env Var | Default | Purpose |
|---------|---------|---------|
| `PORT` | `8081` | HTTP listen port |
| `DB_PATH` | `./data/tracker.db` | SQLite database file path |

## Cross-Project Notes

- **Timestamps:** RFC3339 UTC (`Z` suffix). Consistent with ISO 8601. Matches mcp-pantheon-suite convention.
- **SQLite pattern:** WAL mode + parameterized queries matches mcp-pantheon-suite and clawd-aed conventions.
- **ValidXxx allowlists:** Pattern borrowed from mcp-pantheon-suite's column allowlist approach. Cross-project consistency for injection defense.
- **No shared dependencies** with other Forge projects. Fully standalone.

## Architecture Decision Records

### ADR-001: Column Allowlist for Sorting (Feb 2026)

**Decision:** Validate `sort_by` against `ValidSortColumns` map before SQL construction.

**Alternatives considered:**
- Schema introspection (overkill for single table)
- Pointer types for zero-value disambiguation (more idiomatic but less explicit)

**Trade-offs:** Hard-coded allowlist requires manual update if schema changes. Acceptable: schema is stable, single table.

### ADR-002: ListOptions Struct (Feb 2026)

**Decision:** Bundle filter/sort/pagination into single struct passed to Store methods.

**Alternatives considered:**
- Individual function parameters (12+ args unmaintainable)
- `map[string]interface{}` (loses type safety, harder to validate)

**Trade-offs:** Struct adds ceremony but enables clean validation layer in handler before DB call.

### ADR-003: Shared buildWhere Helper (Feb 2026)

**Decision:** Extract WHERE clause construction to shared helper used by both List() and Count().

**Rationale:** Prevents filter/count mismatch bugs. Ensures pagination total reflects applied filters.
