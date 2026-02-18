# Design: Transaction Safety and Missing Tests

## Overview
Harden job-hunt-platform with three targeted changes: transaction-safe `Update()`, salary cross-validation, and unit/integration tests for the `model` and `db` packages. No architectural changes — all modifications are within existing file boundaries.

## Architecture
No new components. Changes are localized to three existing files plus two new test files:

| File | Change |
|------|--------|
| `internal/model/model.go` | Add salary cross-validation to `CreateRequest.Validate()` |
| `internal/db/db.go` | Wrap `Update()` in `sql.Tx`; add salary validation to update path |
| `internal/handler/handler.go` | Add salary cross-validation to `UpdateApplication` handler |
| `internal/model/model_test.go` | **New** — 8+ unit tests |
| `internal/db/db_test.go` | **New** — 12+ integration tests |

## Technical Decisions

**1. Transaction scope for Update()**
Wrap the entire Get→UPDATE→Get sequence in `sql.Tx`. Use `tx.QueryRow` for reads and `tx.Exec` for the update, with `defer tx.Rollback()` + `tx.Commit()` pattern. This is the standard Go approach.

**2. Salary validation placement**
- **Create path:** Add to `model.CreateRequest.Validate()` — check when both `SalaryMin` and `SalaryMax` are non-nil and `*SalaryMin > *SalaryMax`. This keeps validation in the model layer where it belongs.
- **Update path:** Add salary cross-validation in the handler's `UpdateApplication` before calling `store.Update()`. Extract both `salary_min` and `salary_max` from the fields map. Only validate when BOTH are present in the update payload (partial updates with one field skip this check). The handler already validates status here, so salary validation follows the same pattern.

**3. Test approach**
- `model_test.go`: Pure unit tests, no DB. Table-driven tests for `Validate()` and `ValidateStatus()`.
- `db_test.go`: Integration tests using in-memory SQLite (`:memory:`). Each test gets a fresh `NewStore` with the `:memory:` path. Use `testing` stdlib only — no testify or external frameworks.

## Implementation Plan

### Task 1 (P1): Add salary validation to model and update handler
- In `model.go:Validate()`, after the status check, add: if both `SalaryMin` and `SalaryMax` are non-nil and `*SalaryMin > *SalaryMax`, return `"salary_min cannot be greater than salary_max"`
- In `handler.go:UpdateApplication`, after the status validation block (~line 97), add salary cross-validation: extract both salary fields from `fields` map, if both present as `float64` (JSON numbers decode as float64 in `map[string]interface{}`), check `salaryMin > salaryMax`, return 400 if so
- In `db.go:Update()`, wrap the Get→UPDATE→Get in `sql.Tx`: call `s.db.Begin()`, use `tx.QueryRow` for Get, `tx.Exec` for UPDATE, `tx.QueryRow` for final Get, `defer tx.Rollback()`, `tx.Commit()` at end

### Task 2 (P2): Add model_test.go
- File: `internal/model/model_test.go`
- Table-driven tests covering: happy path create request, missing company, missing role, invalid status, salary_min > salary_max, salary_min == salary_max (allowed), ValidateStatus valid, ValidateStatus invalid, ValidateStatus empty string
- Minimum 8 test cases

### Task 3 (P3): Add db_test.go
- File: `internal/db/db_test.go`
- Helper: `setupTestStore(t)` → `NewStore(":memory:")` — note: the existing `NewStore` uses `os.MkdirAll` on the path dir, so for `:memory:` this will need a small guard (skip mkdir when path is `:memory:`)
- Test cases: Create happy path, Create default status, Create missing fields (via direct store call — store doesn't validate, DB will reject NOT NULL), Get existing, Get non-existent, List all, List empty, List with status filter, List invalid status (store doesn't filter invalid — returns empty), Update single field, Update multiple fields, Update non-existent ID, Update empty body, Delete existing, Delete non-existent, Create with salary_min > salary_max (store doesn't validate — model does), Update with both salary fields
- Minimum 12 test cases

## Dependencies
None new. All work uses stdlib `testing`, `database/sql`, and existing `modernc.org/sqlite`.

## Risks
- **`:memory:` with NewStore:** The existing `NewStore` calls `os.MkdirAll(filepath.Dir(dbPath))` which will try to mkdir for `:memory:`. Need to guard this with a `dbPath != ":memory:"` check or use a temp file instead. Low risk — simple fix.
- **JSON float64 casting in handler:** `map[string]interface{}` decodes JSON numbers as `float64`, not `int`. The salary validation in the handler must cast to `float64` first, then compare. The existing `db.Update()` already handles this correctly via SQLite's type affinity.
