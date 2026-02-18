# Spec: Transaction Safety and Missing Tests

## Overview
Harden the job-hunt-platform with transaction safety in the DB layer, salary validation in the model layer, and comprehensive test coverage for both packages. These were identified by QA spot-check and address real correctness gaps — not theoretical concerns.

## User Stories
- As a developer, I want Update() to be atomic so concurrent requests can't produce inconsistent state
- As a user, I want salary_min > salary_max rejected so I don't accidentally enter bad data
- As a developer, I want db and model packages tested so regressions are caught early

## Requirements

### Functional

1. **Transaction-safe Update** — `db.Store.Update()` must wrap its Get-UPDATE-Get sequence in a single `sql.Tx`. If any step fails, the transaction rolls back. No partial writes.

2. **Salary validation** — `model.CreateRequest.Validate()` must return an error when both `salary_min` and `salary_max` are non-nil and `*salary_min > *salary_max`. The error message should be clear: `"salary_min cannot be greater than salary_max"`. Update flow must also validate salary when both fields are present in the update payload.

3. **model_test.go** — Unit tests for `model` package covering:
   - CreateRequest.Validate() — happy path, missing company, missing role, invalid status, salary_min > salary_max
   - ValidateStatus() — valid status, invalid status, empty string

4. **db_test.go** — Integration tests for `db.Store` covering:
   - Create() — happy path, default status, missing required fields
   - Get() — existing record, non-existent ID
   - List() — multiple records, empty DB, status filter, invalid status filter
   - Update() — single field, multiple fields, non-existent ID, empty update body, status validation
   - Delete() — existing record, non-existent ID
   - Salary validation on create and update paths

### Non-Functional
- Tests use a temporary SQLite DB (`:memory:` or temp file), no test fixtures to maintain
- `go test ./...` must pass with 0 failures
- No new dependencies — use only stdlib `testing` package

## Edge Cases
- Update with salary_min > salary_max in partial update (only one of the two fields sent — should NOT validate cross-field unless both are present in the update payload)
- Update with empty JSON body `{}` — should return existing record unchanged
- Create with negative salary values — allowed (spec doesn't restrict, and salary fields are just integers)

## Non-Goals
- Concurrent update conflict detection (optimistic locking, ETags) — overkill for single-user tool
- Test coverage for handler layer — already at 82.4% with 17 tests
- Benchmarks or fuzz tests
- URL/date format validation

## Success Criteria
1. `db.Store.Update()` uses `sql.Tx` — visible in code review
2. Salary cross-validation rejects salary_min > salary_max on create
3. Salary cross-validation rejects salary_min > salary_max on update (when both fields present)
4. `internal/model/model_test.go` exists with >= 8 test cases
5. `internal/db/db_test.go` exists with >= 12 test cases
6. `go test ./...` passes with 0 failures
7. `go vet ./...` passes clean
