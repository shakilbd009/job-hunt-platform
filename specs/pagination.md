# Spec: Add Pagination to GET /applications

## Overview

The `GET /applications` endpoint currently returns all matching records with no LIMIT. As data grows, this causes unbounded memory usage, large JSON payloads, and slow responses. This spec adds offset-based pagination with metadata, and adds HTTP server timeouts to prevent slow-client resource exhaustion.

**Approach:** Offset-based pagination (not cursor-based) — appropriate for this scale. See deliberation on task for rationale.

## User Stories

- As an API consumer, I want to paginate through applications so that responses are fast and bounded regardless of dataset size
- As a server operator, I want timeouts on HTTP connections so that slow clients cannot exhaust server resources

## Requirements

### Functional

1. **Query parameters:** `GET /applications` accepts optional `limit` (int, default 50, max 500) and `offset` (int, default 0, min 0) query parameters alongside existing `status` filter.

2. **Parameter validation:**
   - `limit` must be a positive integer between 1 and 500. Invalid values return 400 with descriptive error.
   - `offset` must be a non-negative integer. Invalid values return 400 with descriptive error.
   - Non-numeric values for either return 400.

3. **SQL query update:** `db.List()` signature changes to accept pagination params. SQL gains `LIMIT ? OFFSET ?` clause. The `ORDER BY updated_at DESC` is preserved.

4. **Response envelope:** Response shape changes from a bare array to an object:
   ```json
   {
     "data": [...applications...],
     "pagination": {
       "total": 142,
       "limit": 50,
       "offset": 0,
       "has_more": true
     }
   }
   ```
   The `total` field requires a separate `SELECT COUNT(*)` query with the same WHERE clause.

5. **Backward compatibility:** The response shape change is intentional and breaking. Since this is an internal API with no external consumers, no compatibility shim is needed. Existing handler tests must be updated.

6. **HTTP server timeouts:** Add to `http.Server` in `main.go`:
   - `ReadTimeout: 10 * time.Second`
   - `WriteTimeout: 30 * time.Second`
   - `IdleTimeout: 60 * time.Second`

7. **Existing tests updated:** All handler tests for `ListApplications` must be updated to parse the new response envelope. Add new test cases for pagination.

### Non-Functional

- `GET /applications` with default pagination must respond in under 100ms for datasets up to 10,000 rows
- COUNT query should not add significant overhead for the expected dataset size

## Edge Cases

1. **Offset beyond total:** Return empty `data: []` with correct `total` count and `has_more: false` — not an error
2. **limit=0:** Return 400 error (minimum is 1)
3. **Negative offset:** Return 400 error
4. **No query params:** Defaults apply (limit=50, offset=0) — existing behavior returns first 50 results
5. **Combined with status filter:** Pagination applies to filtered results, `total` reflects filtered count

## Non-Goals

- Cursor-based pagination — overkill for this scale
- Sorting options beyond `updated_at DESC` — future enhancement
- Caching/ETags — not needed yet
- Rate limiting — separate concern

## Success Criteria

1. `GET /applications` returns paginated response envelope with `data` and `pagination` fields
2. `limit` and `offset` query params work correctly with validation
3. `total` count is accurate with and without status filter
4. `has_more` is true when more results exist beyond current page
5. HTTP server has ReadTimeout, WriteTimeout, and IdleTimeout configured
6. All existing tests pass after updating for new response shape
7. New test cases cover: default pagination, custom limit/offset, invalid params, offset beyond total, combined with status filter
