package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/shakilbd009/job-hunt-platform/internal/db"
	"github.com/shakilbd009/job-hunt-platform/internal/handler"
	"github.com/shakilbd009/job-hunt-platform/internal/model"
)

func setupTest(t *testing.T) (*handler.Handler, chi.Router) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	h := handler.New(store)
	r := chi.NewRouter()
	h.Routes(r)
	return h, r
}

func TestListApplications_Empty(t *testing.T) {
	_, r := setupTest(t)

	req := httptest.NewRequest(http.MethodGet, "/applications", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var apps []model.Application
	json.NewDecoder(w.Body).Decode(&apps)
	if len(apps) != 0 {
		t.Fatalf("expected empty array, got %d items", len(apps))
	}
}

func TestCreateApplication_Success(t *testing.T) {
	_, r := setupTest(t)

	body := `{"company":"Acme Corp","role":"Backend Engineer","status":"applied"}`
	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var app model.Application
	json.NewDecoder(w.Body).Decode(&app)
	if app.Company != "Acme Corp" {
		t.Errorf("expected company 'Acme Corp', got %q", app.Company)
	}
	if app.Status != "applied" {
		t.Errorf("expected status 'applied', got %q", app.Status)
	}
	if app.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestCreateApplication_DefaultStatus(t *testing.T) {
	_, r := setupTest(t)

	body := `{"company":"Acme Corp","role":"Backend Engineer"}`
	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	var app model.Application
	json.NewDecoder(w.Body).Decode(&app)
	if app.Status != "wishlist" {
		t.Errorf("expected default status 'wishlist', got %q", app.Status)
	}
}

func TestCreateApplication_MissingCompany(t *testing.T) {
	_, r := setupTest(t)

	body := `{"role":"Backend Engineer"}`
	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateApplication_MissingRole(t *testing.T) {
	_, r := setupTest(t)

	body := `{"company":"Acme Corp"}`
	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateApplication_InvalidStatus(t *testing.T) {
	_, r := setupTest(t)

	body := `{"company":"Acme Corp","role":"Engineer","status":"invalid"}`
	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetApplication_Success(t *testing.T) {
	_, r := setupTest(t)

	// Create first
	body := `{"company":"Acme Corp","role":"Engineer"}`
	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var created model.Application
	json.NewDecoder(w.Body).Decode(&created)

	// Get by ID
	req = httptest.NewRequest(http.MethodGet, "/applications/"+created.ID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var app model.Application
	json.NewDecoder(w.Body).Decode(&app)
	if app.ID != created.ID {
		t.Errorf("expected ID %q, got %q", created.ID, app.ID)
	}
}

func TestGetApplication_NotFound(t *testing.T) {
	_, r := setupTest(t)

	req := httptest.NewRequest(http.MethodGet, "/applications/deadbeef", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestUpdateApplication_Success(t *testing.T) {
	_, r := setupTest(t)

	// Create
	body := `{"company":"Acme Corp","role":"Engineer"}`
	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var created model.Application
	json.NewDecoder(w.Body).Decode(&created)

	// Update
	update := `{"status":"applied","notes":"Submitted resume"}`
	req = httptest.NewRequest(http.MethodPut, "/applications/"+created.ID, bytes.NewBufferString(update))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated model.Application
	json.NewDecoder(w.Body).Decode(&updated)
	if updated.Status != "applied" {
		t.Errorf("expected status 'applied', got %q", updated.Status)
	}
	if updated.Notes != "Submitted resume" {
		t.Errorf("expected notes 'Submitted resume', got %q", updated.Notes)
	}
}

func TestUpdateApplication_NotFound(t *testing.T) {
	_, r := setupTest(t)

	body := `{"status":"applied"}`
	req := httptest.NewRequest(http.MethodPut, "/applications/deadbeef", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestUpdateApplication_InvalidStatus(t *testing.T) {
	_, r := setupTest(t)

	// Create
	body := `{"company":"Acme Corp","role":"Engineer"}`
	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var created model.Application
	json.NewDecoder(w.Body).Decode(&created)

	// Update with invalid status
	update := `{"status":"invalid"}`
	req = httptest.NewRequest(http.MethodPut, "/applications/"+created.ID, bytes.NewBufferString(update))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDeleteApplication_Success(t *testing.T) {
	_, r := setupTest(t)

	// Create
	body := `{"company":"Acme Corp","role":"Engineer"}`
	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var created model.Application
	json.NewDecoder(w.Body).Decode(&created)

	// Delete
	req = httptest.NewRequest(http.MethodDelete, "/applications/"+created.ID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	// Verify deleted
	req = httptest.NewRequest(http.MethodGet, "/applications/"+created.ID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", w.Code)
	}
}

func TestDeleteApplication_NotFound(t *testing.T) {
	_, r := setupTest(t)

	req := httptest.NewRequest(http.MethodDelete, "/applications/deadbeef", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestListApplications_FilterByStatus(t *testing.T) {
	_, r := setupTest(t)

	// Create two apps with different statuses
	body1 := `{"company":"Acme","role":"Eng","status":"applied"}`
	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBufferString(body1))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body2 := `{"company":"Beta","role":"Eng","status":"interview"}`
	req = httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBufferString(body2))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Filter by applied
	req = httptest.NewRequest(http.MethodGet, "/applications?status=applied", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var apps []model.Application
	json.NewDecoder(w.Body).Decode(&apps)
	if len(apps) != 1 {
		t.Fatalf("expected 1 filtered result, got %d", len(apps))
	}
	if apps[0].Company != "Acme" {
		t.Errorf("expected 'Acme', got %q", apps[0].Company)
	}
}

func TestListApplications_InvalidStatusFilter(t *testing.T) {
	_, r := setupTest(t)

	req := httptest.NewRequest(http.MethodGet, "/applications?status=invalid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateApplication_WithSalary(t *testing.T) {
	_, r := setupTest(t)

	body := `{"company":"Acme Corp","role":"Engineer","salary_min":150000,"salary_max":200000}`
	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	var app model.Application
	json.NewDecoder(w.Body).Decode(&app)
	if app.SalaryMin != 150000 {
		t.Errorf("expected salary_min 150000, got %d", app.SalaryMin)
	}
	if app.SalaryMax != 200000 {
		t.Errorf("expected salary_max 200000, got %d", app.SalaryMax)
	}
}

func TestCreateApplication_InvalidJSON(t *testing.T) {
	_, r := setupTest(t)

	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBufferString("not json"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestIDValidation(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want int
	}{
		{"valid 8-char hex", "a1b2c3d4", http.StatusNotFound},
		{"single char", "a", http.StatusBadRequest},
		{"too short", "abc", http.StatusBadRequest},
		{"too long", "a1b2c3d4e5", http.StatusBadRequest},
		{"uppercase rejected", "ABCDEFGH", http.StatusBadRequest},
		{"non-hex chars", "ghijklmn", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run("Get_"+tt.name, func(t *testing.T) {
			_, r := setupTest(t)
			req := httptest.NewRequest(http.MethodGet, "/applications/"+tt.id, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tt.want {
				t.Errorf("GET /applications/%s: expected %d, got %d", tt.id, tt.want, w.Code)
			}
		})

		t.Run("Update_"+tt.name, func(t *testing.T) {
			_, r := setupTest(t)
			body := `{"status":"applied"}`
			req := httptest.NewRequest(http.MethodPut, "/applications/"+tt.id, bytes.NewBufferString(body))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tt.want {
				t.Errorf("PUT /applications/%s: expected %d, got %d", tt.id, tt.want, w.Code)
			}
		})

		t.Run("Delete_"+tt.name, func(t *testing.T) {
			_, r := setupTest(t)
			req := httptest.NewRequest(http.MethodDelete, "/applications/"+tt.id, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tt.want {
				t.Errorf("DELETE /applications/%s: expected %d, got %d", tt.id, tt.want, w.Code)
			}
		})
	}
}
