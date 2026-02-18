package db

import (
	"context"
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

	apps, err := store.List(ctx,"", 100, 0)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(apps))
	}
}

func TestListEmpty(t *testing.T) {
	store := setupTestStore(t)

	apps, err := store.List(ctx,"", 100, 0)
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

	apps, err := store.List(ctx,"applied", 100, 0)
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

	apps, err := store.List(ctx,"nonexistent_status", 100, 0)
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

	count, err := store.Count(ctx,"")
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

	count, err := store.Count(ctx,"applied")
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected count 2, got %d", count)
	}
}

func TestCountEmpty(t *testing.T) {
	store := setupTestStore(t)

	count, err := store.Count(ctx,"")
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
	apps, err := store.List(ctx,"", 2, 0)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(apps))
	}

	// Second page
	apps, err = store.List(ctx,"", 2, 2)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(apps))
	}

	// Last page (partial)
	apps, err = store.List(ctx,"", 2, 4)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}

	// Beyond range
	apps, err = store.List(ctx,"", 2, 10)
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
