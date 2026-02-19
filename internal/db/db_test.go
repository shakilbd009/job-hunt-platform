package db

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/shakilbd009/job-hunt-platform/internal/model"
)

var ctx = context.Background()

func setupTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func createTestApp(t *testing.T, store *Store) *model.Application {
	t.Helper()
	app, err := store.Create(ctx, model.CreateRequest{
		Company: "TestCo",
		Role:    "Engineer",
	})
	if err != nil {
		t.Fatalf("failed to create test app: %v", err)
	}
	return app
}

func TestCreateHappyPath(t *testing.T) {
	store := setupTestStore(t)

	min, max := 100000, 200000
	app, err := store.Create(ctx, model.CreateRequest{
		Company:   "Acme",
		Role:      "SRE",
		URL:       "https://acme.com/jobs/1",
		SalaryMin: &min,
		SalaryMax: &max,
		Location:  "Remote",
		Status:    "applied",
		Notes:     "Great company",
		AppliedAt: "2026-02-10",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if app.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if app.Company != "Acme" {
		t.Fatalf("expected company Acme, got %s", app.Company)
	}
	if app.SalaryMin != 100000 {
		t.Fatalf("expected salary_min 100000, got %d", app.SalaryMin)
	}
	if app.Status != "applied" {
		t.Fatalf("expected status applied, got %s", app.Status)
	}
	if app.CreatedAt == "" || app.UpdatedAt == "" {
		t.Fatal("expected timestamps to be set")
	}
}

func TestCreateDefaultStatus(t *testing.T) {
	store := setupTestStore(t)

	app, err := store.Create(ctx, model.CreateRequest{
		Company: "DefaultCo",
		Role:    "Dev",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if app.Status != "wishlist" {
		t.Fatalf("expected default status wishlist, got %s", app.Status)
	}
}

func TestGetExisting(t *testing.T) {
	store := setupTestStore(t)
	created := createTestApp(t, store)

	got, err := store.Get(ctx,created.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected application, got nil")
	}
	if got.ID != created.ID {
		t.Fatalf("expected ID %s, got %s", created.ID, got.ID)
	}
}

func TestGetNonExistent(t *testing.T) {
	store := setupTestStore(t)

	got, err := store.Get(ctx,"nonexistent")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestListAll(t *testing.T) {
	store := setupTestStore(t)
	createTestApp(t, store)
	createTestApp(t, store)

	apps, err := store.List(ctx, model.ListOptions{Limit: 100, Offset: 0})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(apps))
	}
}

func TestListEmpty(t *testing.T) {
	store := setupTestStore(t)

	apps, err := store.List(ctx, model.ListOptions{Limit: 100, Offset: 0})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 0 {
		t.Fatalf("expected 0 apps, got %d", len(apps))
	}
}

func TestListWithStatusFilter(t *testing.T) {
	store := setupTestStore(t)

	store.Create(ctx, model.CreateRequest{Company: "A", Role: "R", Status: "applied"})
	store.Create(ctx, model.CreateRequest{Company: "B", Role: "R", Status: "interview"})
	store.Create(ctx, model.CreateRequest{Company: "C", Role: "R", Status: "applied"})

	apps, err := store.List(ctx, model.ListOptions{Status: "applied", Limit: 100, Offset: 0})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps with status applied, got %d", len(apps))
	}
}

func TestListInvalidStatusFilter(t *testing.T) {
	store := setupTestStore(t)
	createTestApp(t, store)

	apps, err := store.List(ctx, model.ListOptions{Status: "nonexistent_status", Limit: 100, Offset: 0})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 0 {
		t.Fatalf("expected 0 apps for invalid status, got %d", len(apps))
	}
}

func TestUpdateSingleField(t *testing.T) {
	store := setupTestStore(t)
	created := createTestApp(t, store)

	updated, err := store.Update(ctx,created.ID, map[string]interface{}{
		"company": "NewCo",
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Company != "NewCo" {
		t.Fatalf("expected company NewCo, got %s", updated.Company)
	}
	if updated.Role != created.Role {
		t.Fatalf("role should be unchanged, got %s", updated.Role)
	}
}

func TestUpdateMultipleFields(t *testing.T) {
	store := setupTestStore(t)
	created := createTestApp(t, store)

	updated, err := store.Update(ctx,created.ID, map[string]interface{}{
		"company":  "BigCo",
		"role":     "Senior Engineer",
		"location": "NYC",
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Company != "BigCo" {
		t.Fatalf("expected company BigCo, got %s", updated.Company)
	}
	if updated.Role != "Senior Engineer" {
		t.Fatalf("expected role Senior Engineer, got %s", updated.Role)
	}
	if updated.Location != "NYC" {
		t.Fatalf("expected location NYC, got %s", updated.Location)
	}
}

func TestUpdateNonExistent(t *testing.T) {
	store := setupTestStore(t)

	result, err := store.Update(ctx,"nonexistent", map[string]interface{}{
		"company": "X",
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil for non-existent, got %+v", result)
	}
}

func TestUpdateEmptyBody(t *testing.T) {
	store := setupTestStore(t)
	created := createTestApp(t, store)

	result, err := store.Update(ctx,created.ID, map[string]interface{}{})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected existing record, got nil")
	}
	if result.Company != created.Company {
		t.Fatalf("expected unchanged company %s, got %s", created.Company, result.Company)
	}
}

func TestDeleteExisting(t *testing.T) {
	store := setupTestStore(t)
	created := createTestApp(t, store)

	deleted, err := store.Delete(ctx,created.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if !deleted {
		t.Fatal("expected deleted=true")
	}

	got, err := store.Get(ctx,created.ID)
	if err != nil {
		t.Fatalf("Get after delete failed: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestDeleteNonExistent(t *testing.T) {
	store := setupTestStore(t)

	deleted, err := store.Delete(ctx,"nonexistent")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if deleted {
		t.Fatal("expected deleted=false for non-existent")
	}
}

func TestUpdateSalaryFields(t *testing.T) {
	store := setupTestStore(t)
	created := createTestApp(t, store)

	updated, err := store.Update(ctx,created.ID, map[string]interface{}{
		"salary_min": float64(80000),
		"salary_max": float64(120000),
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.SalaryMin != 80000 {
		t.Fatalf("expected salary_min 80000, got %d", updated.SalaryMin)
	}
	if updated.SalaryMax != 120000 {
		t.Fatalf("expected salary_max 120000, got %d", updated.SalaryMax)
	}
}

func TestCountAll(t *testing.T) {
	store := setupTestStore(t)
	createTestApp(t, store)
	createTestApp(t, store)
	createTestApp(t, store)

	count, err := store.Count(ctx, model.ListOptions{})
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected count 3, got %d", count)
	}
}

func TestCountWithStatus(t *testing.T) {
	store := setupTestStore(t)

	store.Create(ctx, model.CreateRequest{Company: "A", Role: "R", Status: "applied"})
	store.Create(ctx, model.CreateRequest{Company: "B", Role: "R", Status: "interview"})
	store.Create(ctx, model.CreateRequest{Company: "C", Role: "R", Status: "applied"})

	count, err := store.Count(ctx, model.ListOptions{Status: "applied"})
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected count 2, got %d", count)
	}
}

func TestCountEmpty(t *testing.T) {
	store := setupTestStore(t)

	count, err := store.Count(ctx, model.ListOptions{})
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected count 0, got %d", count)
	}
}

func TestListPagination(t *testing.T) {
	store := setupTestStore(t)
	for i := 0; i < 5; i++ {
		store.Create(ctx, model.CreateRequest{Company: "Co", Role: "R"})
	}

	// First page
	apps, err := store.List(ctx, model.ListOptions{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(apps))
	}

	// Second page
	apps, err = store.List(ctx, model.ListOptions{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(apps))
	}

	// Last page (partial)
	apps, err = store.List(ctx, model.ListOptions{Limit: 2, Offset: 4})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}

	// Beyond range
	apps, err = store.List(ctx, model.ListOptions{Limit: 2, Offset: 10})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 0 {
		t.Fatalf("expected 0 apps, got %d", len(apps))
	}
}

func TestStatsPopulated(t *testing.T) {
	store := setupTestStore(t)

	min1, max1 := 100000, 200000
	min2, max2 := 150000, 250000
	store.Create(ctx, model.CreateRequest{Company: "A", Role: "R", Status: "applied", SalaryMin: &min1, SalaryMax: &max1})
	store.Create(ctx, model.CreateRequest{Company: "B", Role: "R", Status: "applied", SalaryMin: &min2, SalaryMax: &max2})
	store.Create(ctx, model.CreateRequest{Company: "C", Role: "R", Status: "interview"})

	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats.Total != 3 {
		t.Fatalf("expected total 3, got %d", stats.Total)
	}
	if stats.ByStatus["applied"] != 2 {
		t.Fatalf("expected 2 applied, got %d", stats.ByStatus["applied"])
	}
	if stats.ByStatus["interview"] != 1 {
		t.Fatalf("expected 1 interview, got %d", stats.ByStatus["interview"])
	}
	if stats.ByStatus["wishlist"] != 0 {
		t.Fatalf("expected 0 wishlist, got %d", stats.ByStatus["wishlist"])
	}
	if stats.SalaryRange.Min != 100000 {
		t.Fatalf("expected salary min 100000, got %d", stats.SalaryRange.Min)
	}
	if stats.SalaryRange.Max != 250000 {
		t.Fatalf("expected salary max 250000, got %d", stats.SalaryRange.Max)
	}
	if stats.SalaryRange.Avg != 125000 {
		t.Fatalf("expected salary avg 125000, got %d", stats.SalaryRange.Avg)
	}
	// All 3 apps created "now" — should be in both 7-day and 30-day windows
	if stats.RecentActivity.Last7Days != 3 {
		t.Fatalf("expected 3 in last 7 days, got %d", stats.RecentActivity.Last7Days)
	}
	if stats.RecentActivity.Last30Days != 3 {
		t.Fatalf("expected 3 in last 30 days, got %d", stats.RecentActivity.Last30Days)
	}
}

func TestStatsEmpty(t *testing.T) {
	store := setupTestStore(t)

	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats.Total != 0 {
		t.Fatalf("expected total 0, got %d", stats.Total)
	}
	if stats.SalaryRange.Min != 0 || stats.SalaryRange.Max != 0 || stats.SalaryRange.Avg != 0 {
		t.Fatalf("expected all zeros for salary range, got min=%d max=%d avg=%d",
			stats.SalaryRange.Min, stats.SalaryRange.Max, stats.SalaryRange.Avg)
	}
	if stats.RecentActivity.Last7Days != 0 || stats.RecentActivity.Last30Days != 0 {
		t.Fatalf("expected 0 recent activity, got 7d=%d 30d=%d",
			stats.RecentActivity.Last7Days, stats.RecentActivity.Last30Days)
	}
	// All 9 statuses should be present with 0
	if len(stats.ByStatus) != len(model.ValidStatuses) {
		t.Fatalf("expected %d statuses, got %d", len(model.ValidStatuses), len(stats.ByStatus))
	}
}

func TestStatsZeroSalaries(t *testing.T) {
	store := setupTestStore(t)

	// Create apps with no salary data (defaults to 0)
	store.Create(ctx, model.CreateRequest{Company: "A", Role: "R", Status: "applied"})
	store.Create(ctx, model.CreateRequest{Company: "B", Role: "R", Status: "interview"})

	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats.SalaryRange.Min != 0 || stats.SalaryRange.Max != 0 || stats.SalaryRange.Avg != 0 {
		t.Fatalf("expected all zeros for salary range when all salaries are 0, got min=%d max=%d avg=%d",
			stats.SalaryRange.Min, stats.SalaryRange.Max, stats.SalaryRange.Avg)
	}
}

func TestStatsEdgeOfWindow(t *testing.T) {
	store := setupTestStore(t)

	// Insert directly with a created_at exactly 7 days ago
	now := time.Now().UTC()
	sevenDaysAgo := now.AddDate(0, 0, -7).Format(time.RFC3339)
	eightDaysAgo := now.AddDate(0, 0, -8).Format(time.RFC3339)

	// Manually insert rows with specific created_at timestamps
	store.db.ExecContext(ctx,
		"INSERT INTO applications (id, company, role, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		"edge7", "EdgeCo7", "Eng", "applied", sevenDaysAgo, sevenDaysAgo)
	store.db.ExecContext(ctx,
		"INSERT INTO applications (id, company, role, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		"edge8", "EdgeCo8", "Eng", "applied", eightDaysAgo, eightDaysAgo)

	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	// edge7 is exactly at the boundary (>= 7 days ago) — included in 7-day window
	// edge8 is 8 days ago — excluded from 7-day window but included in 30-day
	if stats.RecentActivity.Last7Days != 1 {
		t.Fatalf("expected 1 in last 7 days (edge case), got %d", stats.RecentActivity.Last7Days)
	}
	if stats.RecentActivity.Last30Days != 2 {
		t.Fatalf("expected 2 in last 30 days, got %d", stats.RecentActivity.Last30Days)
	}

}

// Test sorting by company ascending
func TestListSortByCompanyAsc(t *testing.T) {
	store := setupTestStore(t)

	store.Create(ctx, model.CreateRequest{Company: "Zebra", Role: "R"})
	store.Create(ctx, model.CreateRequest{Company: "Alpha", Role: "R"})
	store.Create(ctx, model.CreateRequest{Company: "Beta", Role: "R"})

	apps, err := store.List(ctx, model.ListOptions{SortBy: "company", SortOrder: "ASC", Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 3 {
		t.Fatalf("expected 3 apps, got %d", len(apps))
	}
	if apps[0].Company != "Alpha" || apps[1].Company != "Beta" || apps[2].Company != "Zebra" {
		t.Fatalf("expected sorted order Alpha, Beta, Zebra, got %s, %s, %s", apps[0].Company, apps[1].Company, apps[2].Company)
	}
}

// Test sorting by salary_min descending
func TestListSortBySalaryMinDesc(t *testing.T) {
	store := setupTestStore(t)

	min1, max1 := 100000, 150000
	min2, max2 := 200000, 250000
	min3, max3 := 50000, 100000
	store.Create(ctx, model.CreateRequest{Company: "A", Role: "R", SalaryMin: &min1, SalaryMax: &max1})
	store.Create(ctx, model.CreateRequest{Company: "B", Role: "R", SalaryMin: &min2, SalaryMax: &max2})
	store.Create(ctx, model.CreateRequest{Company: "C", Role: "R", SalaryMin: &min3, SalaryMax: &max3})

	apps, err := store.List(ctx, model.ListOptions{SortBy: "salary_min", SortOrder: "DESC", Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 3 {
		t.Fatalf("expected 3 apps, got %d", len(apps))
	}
	if apps[0].SalaryMin != 200000 || apps[1].SalaryMin != 100000 || apps[2].SalaryMin != 50000 {
		t.Fatalf("expected sorted order 200000, 100000, 50000, got %d, %d, %d", apps[0].SalaryMin, apps[1].SalaryMin, apps[2].SalaryMin)
	}
}

// Test filter by company substring (case-insensitive)
func TestListFilterByCompany(t *testing.T) {
	store := setupTestStore(t)

	store.Create(ctx, model.CreateRequest{Company: "Acme Corp", Role: "R"})
	store.Create(ctx, model.CreateRequest{Company: "Acme Industries", Role: "R"})
	store.Create(ctx, model.CreateRequest{Company: "Beta Corp", Role: "R"})

	apps, err := store.List(ctx, model.ListOptions{Company: "acme", Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps matching 'acme', got %d", len(apps))
	}

	// Test case insensitivity
	apps, err = store.List(ctx, model.ListOptions{Company: "ACME", Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps matching 'ACME' (case insensitive), got %d", len(apps))
	}
}

// Test filter by location substring (case-insensitive)
func TestListFilterByLocation(t *testing.T) {
	store := setupTestStore(t)

	store.Create(ctx, model.CreateRequest{Company: "A", Role: "R", Location: "New York"})
	store.Create(ctx, model.CreateRequest{Company: "B", Role: "R", Location: "New Jersey"})
	store.Create(ctx, model.CreateRequest{Company: "C", Role: "R", Location: "San Francisco"})

	apps, err := store.List(ctx, model.ListOptions{Location: "new", Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps matching location 'new', got %d", len(apps))
	}
}

// Test filter by role substring
func TestListFilterByRole(t *testing.T) {
	store := setupTestStore(t)

	store.Create(ctx, model.CreateRequest{Company: "A", Role: "Senior Engineer"})
	store.Create(ctx, model.CreateRequest{Company: "B", Role: "Junior Engineer"})
	store.Create(ctx, model.CreateRequest{Company: "C", Role: "Product Manager"})

	apps, err := store.List(ctx, model.ListOptions{Role: "engineer", Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps matching role 'engineer', got %d", len(apps))
	}
}

// Test date range filter (applied_after, applied_before using created_at)
func TestListFilterByDateRange(t *testing.T) {
	store := setupTestStore(t)

	// Insert with specific timestamps
	now := time.Now().UTC()
	day1 := now.AddDate(0, 0, -5).Format(time.RFC3339)
	day2 := now.AddDate(0, 0, -3).Format(time.RFC3339)
	day3 := now.AddDate(0, 0, -1).Format(time.RFC3339)

	store.db.ExecContext(ctx,
		"INSERT INTO applications (id, company, role, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		"d1", "Day1", "R", "applied", day1, day1)
	store.db.ExecContext(ctx,
		"INSERT INTO applications (id, company, role, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		"d2", "Day2", "R", "applied", day2, day2)
	store.db.ExecContext(ctx,
		"INSERT INTO applications (id, company, role, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		"d3", "Day3", "R", "applied", day3, day3)

	// Filter after day 4 ago (should get day1, day2, day3)
	after := now.AddDate(0, 0, -6).Format(time.RFC3339)
	apps, err := store.List(ctx, model.ListOptions{AppliedAfter: after, Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 3 {
		t.Fatalf("expected 3 apps after %s, got %d", after, len(apps))
	}

	// Filter before day 2 ago (should get day2, day3)
	before := now.AddDate(0, 0, -2).Format(time.RFC3339)
	apps, err = store.List(ctx, model.ListOptions{AppliedBefore: before, Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps before %s, got %d", before, len(apps))
	}

	// Filter between day 4 ago and day 2 ago (should get day2 only)
	after = now.AddDate(0, 0, -4).Format(time.RFC3339)
	before = now.AddDate(0, 0, -2).Format(time.RFC3339)
	apps, err = store.List(ctx, model.ListOptions{AppliedAfter: after, AppliedBefore: before, Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 app between %s and %s, got %d", after, before, len(apps))
	}
	if apps[0].Company != "Day2" {
		t.Fatalf("expected Day2, got %s", apps[0].Company)
	}
}

// Test salary_min_gte filter
func TestListFilterBySalaryMinGTE(t *testing.T) {
	store := setupTestStore(t)

	min1, max1 := 50000, 100000
	min2, max2 := 100000, 150000
	min3, max3 := 150000, 200000
	store.Create(ctx, model.CreateRequest{Company: "A", Role: "R", SalaryMin: &min1, SalaryMax: &max1})
	store.Create(ctx, model.CreateRequest{Company: "B", Role: "R", SalaryMin: &min2, SalaryMax: &max2})
	store.Create(ctx, model.CreateRequest{Company: "C", Role: "R", SalaryMin: &min3, SalaryMax: &max3})

	apps, err := store.List(ctx, model.ListOptions{HasSalaryMinGTE: true, SalaryMinGTE: 100000, Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps with salary_min >= 100000, got %d", len(apps))
	}
}

// Test salary_max_lte filter
func TestListFilterBySalaryMaxLTE(t *testing.T) {
	store := setupTestStore(t)

	min1, max1 := 50000, 100000
	min2, max2 := 100000, 150000
	min3, max3 := 150000, 200000
	store.Create(ctx, model.CreateRequest{Company: "A", Role: "R", SalaryMin: &min1, SalaryMax: &max1})
	store.Create(ctx, model.CreateRequest{Company: "B", Role: "R", SalaryMin: &min2, SalaryMax: &max2})
	store.Create(ctx, model.CreateRequest{Company: "C", Role: "R", SalaryMin: &min3, SalaryMax: &max3})

	apps, err := store.List(ctx, model.ListOptions{HasSalaryMaxLTE: true, SalaryMaxLTE: 150000, Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps with salary_max <= 150000, got %d", len(apps))
	}
}

// Test salary_max_lte=0 returns nothing (edge case: salary_max > 0 required)
func TestListFilterBySalaryMaxLTEZero(t *testing.T) {
	store := setupTestStore(t)

	min1, max1 := 50000, 100000
	min2, max2 := 100000, 0 // salary_max is 0
	store.Create(ctx, model.CreateRequest{Company: "A", Role: "R", SalaryMin: &min1, SalaryMax: &max1})
	store.Create(ctx, model.CreateRequest{Company: "B", Role: "R", SalaryMin: &min2, SalaryMax: &max2})

	// Even though we set salary_max_lte=0, the condition requires salary_max > 0
	// So nothing should match
	apps, err := store.List(ctx, model.ListOptions{HasSalaryMaxLTE: true, SalaryMaxLTE: 0, Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 0 {
		t.Fatalf("expected 0 apps with salary_max <= 0 (edge case), got %d", len(apps))
	}
}

// Test all filters combined
func TestListAllFiltersCombined(t *testing.T) {
	store := setupTestStore(t)

	min1, max1 := 100000, 150000
	min2, max2 := 200000, 250000
	// This one matches all criteria
	store.Create(ctx, model.CreateRequest{Company: "Acme Corp", Role: "Senior Engineer", Location: "New York", Status: "applied", SalaryMin: &min1, SalaryMax: &max1})
	// Different status
	store.Create(ctx, model.CreateRequest{Company: "Acme Inc", Role: "Senior Engineer", Location: "New York", Status: "interview", SalaryMin: &min1, SalaryMax: &max1})
	// Different company
	store.Create(ctx, model.CreateRequest{Company: "Beta Corp", Role: "Senior Engineer", Location: "New York", Status: "applied", SalaryMin: &min1, SalaryMax: &max1})
	// Different role
	store.Create(ctx, model.CreateRequest{Company: "Acme Corp", Role: "Junior Engineer", Location: "New York", Status: "applied", SalaryMin: &min1, SalaryMax: &max1})
	// Different location
	store.Create(ctx, model.CreateRequest{Company: "Acme Corp", Role: "Senior Engineer", Location: "San Francisco", Status: "applied", SalaryMin: &min1, SalaryMax: &max1})
	// Different salary
	store.Create(ctx, model.CreateRequest{Company: "Acme Corp", Role: "Senior Engineer", Location: "New York", Status: "applied", SalaryMin: &min2, SalaryMax: &max2})

	opts := model.ListOptions{
		Status:          "applied",
		Company:         "acme",
		Role:            "senior",
		Location:        "new york",
		HasSalaryMinGTE: true,
		SalaryMinGTE:    50000,
		HasSalaryMaxLTE: true,
		SalaryMaxLTE:    200000,
		Limit:           10,
		Offset:          0,
	}

	apps, err := store.List(ctx, opts)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 app matching all filters, got %d", len(apps))
	}
	if apps[0].Company != "Acme Corp" {
		t.Fatalf("expected Acme Corp, got %s", apps[0].Company)
	}

	// Also test Count with same filters
	count, err := store.Count(ctx, opts)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count 1, got %d", count)
	}
}

func setupFileStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create file-based store: %v", err)
	}
	// Serialize writes through single connection — SQLite is single-writer.
	// This tests concurrent Go goroutines safely accessing the store.
	store.db.SetMaxOpenConns(1)
	t.Cleanup(func() { store.Close() })
	return store
}

func TestConcurrentCreate(t *testing.T) {
	store := setupFileStore(t)

	const n = 10
	errs := make(chan error, n)
	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			_, err := store.Create(ctx, model.CreateRequest{
				Company: fmt.Sprintf("Co-%d", i),
				Role:    "Eng",
			})
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Create failed: %v", err)
		}
	}

	apps, err := store.List(ctx, model.ListOptions{Limit: 100})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != n {
		t.Fatalf("expected %d apps, got %d", n, len(apps))
	}
}

func TestConcurrentUpdateSameRecord(t *testing.T) {
	store := setupFileStore(t)

	app, err := store.Create(ctx, model.CreateRequest{Company: "ConcCo", Role: "Eng"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	updates := []map[string]interface{}{
		{"status": "applied"},
		{"notes": "updated"},
		{"location": "NYC"},
		{"url": "https://example.com"},
		{"salary_min": float64(100000)},
	}

	errs := make(chan error, len(updates))
	var wg sync.WaitGroup
	wg.Add(len(updates))

	for _, u := range updates {
		go func(fields map[string]interface{}) {
			defer wg.Done()
			_, err := store.Update(ctx, app.ID, fields)
			errs <- err
		}(u)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Update failed: %v", err)
		}
	}

	got, err := store.Get(ctx, app.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Status != "applied" {
		t.Errorf("expected status applied, got %s", got.Status)
	}
	if got.Notes != "updated" {
		t.Errorf("expected notes updated, got %s", got.Notes)
	}
	if got.Location != "NYC" {
		t.Errorf("expected location NYC, got %s", got.Location)
	}
	if got.URL != "https://example.com" {
		t.Errorf("expected url https://example.com, got %s", got.URL)
	}
	if got.SalaryMin != 100000 {
		t.Errorf("expected salary_min 100000, got %d", got.SalaryMin)
	}
}

func TestNewStoreInvalidPath(t *testing.T) {
	_, err := NewStore("/nonexistent/deeply/nested/dir/db.sqlite")
	if err == nil {
		t.Fatal("expected error for invalid path, got nil")
	}
}
