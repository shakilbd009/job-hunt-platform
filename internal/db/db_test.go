package db

import (
	"testing"

	"github.com/shakilbd009/job-hunt-platform/internal/model"
)

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
	app, err := store.Create(model.CreateRequest{
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
	app, err := store.Create(model.CreateRequest{
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

	app, err := store.Create(model.CreateRequest{
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

	got, err := store.Get(created.ID)
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

	got, err := store.Get("nonexistent")
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

	apps, err := store.List("")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(apps))
	}
}

func TestListEmpty(t *testing.T) {
	store := setupTestStore(t)

	apps, err := store.List("")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 0 {
		t.Fatalf("expected 0 apps, got %d", len(apps))
	}
}

func TestListWithStatusFilter(t *testing.T) {
	store := setupTestStore(t)

	store.Create(model.CreateRequest{Company: "A", Role: "R", Status: "applied"})
	store.Create(model.CreateRequest{Company: "B", Role: "R", Status: "interview"})
	store.Create(model.CreateRequest{Company: "C", Role: "R", Status: "applied"})

	apps, err := store.List("applied")
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

	apps, err := store.List("nonexistent_status")
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

	updated, err := store.Update(created.ID, map[string]interface{}{
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

	updated, err := store.Update(created.ID, map[string]interface{}{
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

	result, err := store.Update("nonexistent", map[string]interface{}{
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

	result, err := store.Update(created.ID, map[string]interface{}{})
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

	deleted, err := store.Delete(created.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if !deleted {
		t.Fatal("expected deleted=true")
	}

	got, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("Get after delete failed: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestDeleteNonExistent(t *testing.T) {
	store := setupTestStore(t)

	deleted, err := store.Delete("nonexistent")
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

	updated, err := store.Update(created.ID, map[string]interface{}{
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
