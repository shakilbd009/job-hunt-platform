# Design: Add Pagination to GET /applications

## Overview
Add offset-based pagination to `GET /applications` with a response envelope (`data` + `pagination` metadata), parameter validation, and HTTP server timeouts. This prevents unbounded result sets as the applications table grows.

## Architecture

### Components Modified
1. **db.go** — `List()` gains `limit`, `offset` params + new `Count()` method
2. **handler.go** — `ListApplications` parses/validates query params, builds envelope response
3. **main.go** — `http.Server` gets timeout configuration

### Data Flow
```
Request → Handler (parse limit/offset, validate) → db.List(status, limit, offset)
                                                  → db.Count(status)
        → Build envelope { data, pagination { total, limit, offset, has_more } }
        → Response
```

## Technical Decisions

### DECISION: Two separate queries (List + Count) vs single query with window function
**Chosen:** Two queries — `SELECT ... LIMIT ? OFFSET ?` and `SELECT COUNT(*) ... WHERE ...`.

**Rationale:** SQLite supports `COUNT(*) OVER()` window functions, but that returns the total on every row (wastes bandwidth) and complicates the scan. Two queries is simpler, both are fast for the expected dataset size (<10K rows), and the WHERE clause logic stays DRY by extracting into a helper.

**Rejected:** Window function — adds complexity to the scan pattern for negligible performance gain at this scale.

### DECISION: Response envelope structure
**Chosen:** `{ "data": [...], "pagination": { "total", "limit", "offset", "has_more" } }`

**Rationale:** Standard REST pagination envelope. `has_more` is a convenience boolean (`offset + len(data) < total`). Breaking change is acceptable per spec since this is an internal API with no external consumers.

### DECISION: Defaults — limit=50, max=500
**Chosen:** Default limit 50, max 500, default offset 0.

**Rationale:** 50 is a reasonable page size for browsing. 500 cap prevents accidental huge fetches. These match the spec requirements.

## Implementation Plan

### Task 1 (P1): Update db.go — Add Count() and paginate List()
- Add `Count(status string) (int, error)` method — mirrors List() WHERE logic but returns `SELECT COUNT(*)`
- Change `List(status string)` → `List(status string, limit, offset int) ([]model.Application, error)`
- Add `LIMIT ? OFFSET ?` to the SQL query after existing `ORDER BY updated_at DESC`
- Append `limit`, `offset` to params slice

### Task 2 (P2): Update handler.go — Parse params, build envelope
- Parse `limit` and `offset` from `r.URL.Query()` using `strconv.Atoi`
- Validate: limit must be 1–500 (default 50), offset must be ≥0 (default 0)
- Return 400 on invalid values with descriptive error
- Call `store.List(status, limit, offset)` and `store.Count(status)`
- Build response envelope struct and return via `respondJSON`

### Task 3 (P3): Update main.go — HTTP server timeouts
- Change bare `http.Server` to include:
  - `ReadTimeout: 10 * time.Second`
  - `WriteTimeout: 30 * time.Second`
  - `IdleTimeout: 60 * time.Second`

### Task 4 (P4): Update handler_test.go — Fix existing + add pagination tests
- Update all `ListApplications` tests to parse new response envelope shape
- Add test cases: default pagination, custom limit/offset, invalid params (non-numeric, limit=0, negative offset), offset beyond total, combined with status filter

## Data Model
No schema changes. Existing `applications` table + `idx_applications_status` index is sufficient.

## API

### Before
```
GET /applications?status=applied
→ [{ id, company, ... }, ...]
```

### After
```
GET /applications?status=applied&limit=20&offset=40
→ {
    "data": [{ id, company, ... }, ...],
    "pagination": {
      "total": 142,
      "limit": 20,
      "offset": 40,
      "has_more": true
    }
  }
```

## Dependencies
None — standard library only.

## Risks
1. **COUNT query overhead** — Negligible for <10K rows with an indexed status column. Would need revisiting at 100K+ rows (unlikely for a job tracker).
2. **Breaking response shape** — Existing tests will fail until updated. No external consumers, so acceptable.
3. **Offset pagination drift** — If records are inserted between pages, items can be skipped or duplicated. Acceptable for this use case — not a real-time feed.
