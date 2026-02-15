package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
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

	var resp handler.PaginatedResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Data) != 0 {
		t.Fatalf("expected empty data, got %d items", len(resp.Data))
	}
	if resp.Pagination.Total != 0 {
		t.Fatalf("expected total 0, got %d", resp.Pagination.Total)
	}
	if resp.Pagination.Limit != 50 {
		t.Fatalf("expected default limit 50, got %d", resp.Pagination.Limit)
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

	req := httptest.NewRequest(http.MethodGet, "/applications/nonexistent", nil)
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
	req := httptest.NewRequest(http.MethodPut, "/applications/nonexistent", bytes.NewBufferString(body))
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

	req := httptest.NewRequest(http.MethodDelete, "/applications/nonexistent", nil)
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

	var resp handler.PaginatedResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 filtered result, got %d", len(resp.Data))
	}
	if resp.Data[0].Company != "Acme" {
		t.Errorf("expected 'Acme', got %q", resp.Data[0].Company)
	}
	if resp.Pagination.Total != 1 {
		t.Fatalf("expected total 1, got %d", resp.Pagination.Total)
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

func TestListApplications_PaginationParams(t *testing.T) {
	_, r := setupTest(t)

	// Create 3 apps
	for _, company := range []string{"A", "B", "C"} {
		body := `{"company":"` + company + `","role":"Eng"}`
		req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}

	// Page 1: limit=2, offset=0
	req := httptest.NewRequest(http.MethodGet, "/applications?limit=2&offset=0", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp handler.PaginatedResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Data))
	}
	if resp.Pagination.Total != 3 {
		t.Fatalf("expected total 3, got %d", resp.Pagination.Total)
	}
	if resp.Pagination.Limit != 2 {
		t.Fatalf("expected limit 2, got %d", resp.Pagination.Limit)
	}
	if !resp.Pagination.HasMore {
		t.Fatal("expected has_more=true")
	}

	// Page 2: limit=2, offset=2
	req = httptest.NewRequest(http.MethodGet, "/applications?limit=2&offset=2", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Data))
	}
	if resp.Pagination.HasMore {
		t.Fatal("expected has_more=false")
	}
}

func TestListApplications_InvalidLimit(t *testing.T) {
	_, r := setupTest(t)

	req := httptest.NewRequest(http.MethodGet, "/applications?limit=abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListApplications_LimitOutOfRange(t *testing.T) {
	_, r := setupTest(t)

	req := httptest.NewRequest(http.MethodGet, "/applications?limit=0", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for limit=0, got %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/applications?limit=501", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for limit=501, got %d", w.Code)
	}
}

func TestListApplications_NegativeOffset(t *testing.T) {
	_, r := setupTest(t)

	req := httptest.NewRequest(http.MethodGet, "/applications?offset=-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestMalformedIDs(t *testing.T) {
	h, _ := setupTest(t)

	tests := []struct {
		name string
		id   string
	}{
		{"too-long", strings.Repeat("a", 200)},
		{"sql-injection", "'; DROP TABLE applications;--"},
		{"unicode-emoji", "🎉emoji🎉"},
		{"has-spaces", "has spaces"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/applications/x", nil)
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tc.id)
			ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
			req = req.WithContext(ctx)
			w := httptest.NewRecorder()
			h.GetApplication(w, req)

			if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
				t.Fatalf("expected 400 or 404, got %d", w.Code)
			}
		})
	}
}

func TestOversizedPayload(t *testing.T) {
	_, r := setupTest(t)

	// Build a payload well over 1MB limit — use role field to push total over
	bigBody := `{"company":"Acme","role":"` + strings.Repeat("x", 2*1024*1024) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/applications", strings.NewReader(bigBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge && w.Code != http.StatusBadRequest {
		t.Fatalf("expected 413 or 400 for oversized body, got %d", w.Code)
	}
}

func TestInvalidContentTypePUT(t *testing.T) {
	_, r := setupTest(t)

	req := httptest.NewRequest(http.MethodPut, "/applications/testid", bytes.NewBufferString("this is not json"))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnsupportedMediaType && w.Code != http.StatusBadRequest {
		t.Fatalf("expected 415 or 400 for invalid content type, got %d", w.Code)
	}
}

func TestEmptyJSONBody(t *testing.T) {
	_, r := setupTest(t)

	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBufferString("{}"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty JSON body, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if errMsg, ok := resp["error"]; ok {
		if !strings.Contains(strings.ToLower(errMsg), "company") {
			t.Errorf("expected error to mention 'company', got %q", errMsg)
		}
	}
}
