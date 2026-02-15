# Spec: Job Application Tracker API

## Overview
A REST API for tracking job applications through the hiring pipeline. Built as a lightweight Go service with local SQLite storage — no external dependencies, easy to run anywhere.

## User Stories
- As a job seeker, I want to record job applications so I can track what I've applied to
- As a job seeker, I want to update application status so I can track pipeline progress
- As a job seeker, I want to filter by status so I can focus on active applications
- As a job seeker, I want to delete old applications to keep my tracker clean

## Requirements

### Functional
1. **POST /applications** — Create a new application. Required: company, role. Optional: url, salary_min, salary_max, location, status, notes, applied_at. Returns 201 with created resource. Default status: `wishlist`.
2. **GET /applications** — List all applications, ordered by updated_at DESC. Optional `?status=` query param filters by status. Returns 200 with JSON array (empty array if none).
3. **GET /applications/{id}** — Get single application by ID. Returns 200 or 404.
4. **PUT /applications/{id}** — Partial update of any mutable field. Returns 200 with updated resource or 404.
5. **DELETE /applications/{id}** — Delete application. Returns 204 or 404.
6. Status validation: reject invalid status values with 400 on create, update, and list filter.
7. JSON error responses: `{"error": "message"}` format for all error cases.

### Non-Functional
- Pure Go SQLite driver (`modernc.org/sqlite`) — no CGO required
- SQLite WAL mode for concurrent read safety
- Chi v5 router with Logger and Recoverer middleware
- Configurable PORT (default 8081) and DB_PATH (default ./data/tracker.db) via env vars

## Data Model

| Field | Type | Required | Default | Notes |
|-------|------|----------|---------|-------|
| id | TEXT | auto | UUID[:8] | Short UUID primary key |
| company | TEXT | yes | — | Company name |
| role | TEXT | yes | — | Job title/role |
| url | TEXT | no | "" | Job posting URL |
| salary_min | INTEGER | no | 0 | Minimum salary |
| salary_max | INTEGER | no | 0 | Maximum salary |
| location | TEXT | no | "" | Job location |
| status | TEXT | no | "wishlist" | Must be valid status |
| notes | TEXT | no | "" | Free-form notes |
| applied_at | TEXT | no | "" | Date of application |
| created_at | TEXT | auto | NOW | RFC3339 timestamp |
| updated_at | TEXT | auto | NOW | RFC3339 timestamp |

### Valid Status Values (9)
`wishlist`, `applied`, `phone_screen`, `interview`, `offer`, `accepted`, `rejected`, `withdrawn`, `ghosted`

## Non-Goals
- Authentication/authorization (single-user local tool)
- Frontend/UI (API only)
- Pagination (v1 returns all results)
- Search/full-text search

## Success Criteria
1. `go build ./...` succeeds with no CGO
2. `go vet ./...` passes clean
3. All 5 CRUD endpoints work correctly
4. Status validation rejects invalid values
5. Empty list returns `[]` not `null`
6. Unit tests cover all endpoints and edge cases
7. README documents build, run, API usage, and test commands
