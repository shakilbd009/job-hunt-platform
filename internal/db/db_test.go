package db

import (
	"fmt"
	"path/filepath"
	"sync"
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

	apps, err := store.List("", 100, 0)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(apps))
	}
}

func TestListEmpty(t *testing.T) {
	store := setupTestStore(t)

	apps, err := store.List("", 100, 0)
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

	apps, err := store.List("applied", 100, 0)
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

	apps, err := store.List("nonexistent_status", 100, 0)
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

func TestCountAll(t *testing.T) {
	store := setupTestStore(t)
	createTestApp(t, store)
	createTestApp(t, store)
	createTestApp(t, store)

	count, err := store.Count("")
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected count 3, got %d", count)
	}
}

func TestCountWithStatus(t *testing.T) {
	store := setupTestStore(t)

	store.Create(model.CreateRequest{Company: "A", Role: "R", Status: "applied"})
	store.Create(model.CreateRequest{Company: "B", Role: "R", Status: "interview"})
	store.Create(model.CreateRequest{Company: "C", Role: "R", Status: "applied"})

	count, err := store.Count("applied")
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected count 2, got %d", count)
	}
}

func TestCountEmpty(t *testing.T) {
	store := setupTestStore(t)

	count, err := store.Count("")
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
		store.Create(model.CreateRequest{Company: "Co", Role: "R"})
	}

	// First page
	apps, err := store.List("", 2, 0)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(apps))
	}

	// Second page
	apps, err = store.List("", 2, 2)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(apps))
	}

	// Last page (partial)
	apps, err = store.List("", 2, 4)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}

	// Beyond range
	apps, err = store.List("", 2, 10)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 0 {
		t.Fatalf("expected 0 apps, got %d", len(apps))
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
			_, err := store.Create(model.CreateRequest{
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

	apps, err := store.List("", 100, 0)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != n {
		t.Fatalf("expected %d apps, got %d", n, len(apps))
	}
}

func TestConcurrentUpdateSameRecord(t *testing.T) {
	store := setupFileStore(t)

	app, err := store.Create(model.CreateRequest{Company: "ConcCo", Role: "Eng"})
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
			_, err := store.Update(app.ID, fields)
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

	got, err := store.Get(app.ID)
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
