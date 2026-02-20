package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/shakilbd009/job-hunt-platform/internal/db"
	"github.com/shakilbd009/job-hunt-platform/internal/model"
)

var validIDRegex = regexp.MustCompile(`^[0-9a-f]{8}$`)

func isValidID(id string) bool {
	return validIDRegex.MatchString(id)
}

const maxBodyBytes = 1 << 20 // 1 MB

type PaginatedResponse struct {
	Data       []model.Application `json:"data"`
	Pagination PaginationMeta      `json:"pagination"`
}

type PaginationMeta struct {
	Total   int  `json:"total"`
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	HasMore bool `json:"has_more"`
}

type Handler struct {
	store *db.Store
}

func New(store *db.Store) *Handler {
	return &Handler{store: store}
}

func requireJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "application/json") {
				respondError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func maxBodyMiddleware(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}

func (h *Handler) HealthRoutes(r chi.Router) {
	r.Get("/health", h.HealthCheck)
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	if err := h.store.Ping(r.Context()); err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "degraded",
			"db":     "error",
			"detail": err.Error(),
		})
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"db":     "ok",
	})
}

func (h *Handler) Routes(r chi.Router) {
	// Stats endpoint must be before {id} to avoid chi matching "stats" as an ID
	r.Get("/applications/stats", h.GetStats)
	r.Group(func(r chi.Router) {
		r.Use(maxBodyMiddleware(maxBodyBytes))
		r.Use(requireJSON)
		r.Get("/applications", h.ListApplications)
		r.Get("/applications/{id}", h.GetApplication)
		r.Post("/applications", h.CreateApplication)
		r.Put("/applications/{id}", h.UpdateApplication)
		r.Delete("/applications/{id}", h.DeleteApplication)
	})
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.store.Stats(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get application stats")
		return
	}
	respondJSON(w, http.StatusOK, stats)
}

func (h *Handler) ListApplications(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status != "" {
		if err := model.ValidateStatus(status); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid limit parameter")
			return
		}
		if n < 1 || n > 500 {
			respondError(w, http.StatusBadRequest, "limit must be between 1 and 500")
			return
		}
		limit = n
	}

	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid offset parameter")
			return
		}
		if n < 0 {
			respondError(w, http.StatusBadRequest, "offset must be non-negative")
			return
		}
		offset = n
	}

	// sort_by - default "updated_at", validate against ValidSortColumns
	sortBy := r.URL.Query().Get("sort_by")
	if sortBy == "" {
		sortBy = "updated_at"
	}
	if !model.ValidSortColumns[sortBy] {
		respondError(w, http.StatusBadRequest, "invalid sort_by: must be one of company, role, status, salary_min, salary_max, location, created_at, updated_at")
		return
	}

	// sort_order - default "desc", must be "asc" or "desc"
	sortOrder := r.URL.Query().Get("sort_order")
	if sortOrder == "" {
		sortOrder = "desc"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		respondError(w, http.StatusBadRequest, "sort_order must be asc or desc")
		return
	}

	// String filters (no validation needed, empty = no filter)
	company := r.URL.Query().Get("company")
	role := r.URL.Query().Get("role")
	location := r.URL.Query().Get("location")

	// Date filters - RFC3339 format
	appliedAfter := r.URL.Query().Get("applied_after")
	if appliedAfter != "" {
		if _, err := strconv.ParseInt(appliedAfter, 10, 64); err != nil {
			// Not a Unix timestamp, try RFC3339
			if _, err := time.Parse(time.RFC3339, appliedAfter); err != nil {
				respondError(w, http.StatusBadRequest, "invalid applied_after: must be RFC3339 format (e.g. 2026-01-01T00:00:00Z)")
				return
			}
		}
	}

	appliedBefore := r.URL.Query().Get("applied_before")
	if appliedBefore != "" {
		if _, err := strconv.ParseInt(appliedBefore, 10, 64); err != nil {
			// Not a Unix timestamp, try RFC3339
			if _, err := time.Parse(time.RFC3339, appliedBefore); err != nil {
				respondError(w, http.StatusBadRequest, "invalid applied_before: must be RFC3339 format (e.g. 2026-01-01T00:00:00Z)")
				return
			}
		}
	}

	// Salary filters - must be non-negative integers
	hasSalaryMinGTE := false
	salaryMinGTE := 0
	if v := r.URL.Query().Get("salary_min_gte"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			respondError(w, http.StatusBadRequest, "salary_min_gte must be a non-negative integer")
			return
		}
		if n < 0 {
			respondError(w, http.StatusBadRequest, "salary_min_gte must be a non-negative integer")
			return
		}
		salaryMinGTE = n
		hasSalaryMinGTE = true
	}

	hasSalaryMaxLTE := false
	salaryMaxLTE := 0
	if v := r.URL.Query().Get("salary_max_lte"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			respondError(w, http.StatusBadRequest, "salary_max_lte must be a non-negative integer")
			return
		}
		if n < 0 {
			respondError(w, http.StatusBadRequest, "salary_max_lte must be a non-negative integer")
			return
		}
		salaryMaxLTE = n
		hasSalaryMaxLTE = true
	}

	opts := model.ListOptions{
		Status:          status,
		Limit:           limit,
		Offset:          offset,
		SortBy:          sortBy,
		SortOrder:       sortOrder,
		Company:         company,
		Role:            role,
		Location:        location,
		AppliedAfter:    appliedAfter,
		AppliedBefore:   appliedBefore,
		SalaryMinGTE:    salaryMinGTE,
		SalaryMaxLTE:    salaryMaxLTE,
		HasSalaryMinGTE: hasSalaryMinGTE,
		HasSalaryMaxLTE: hasSalaryMaxLTE,
	}

	apps, err := h.store.List(r.Context(), opts)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list applications")
		return
	}

	total, err := h.store.Count(r.Context(), opts)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to count applications")
		return
	}

	respondJSON(w, http.StatusOK, PaginatedResponse{
		Data: apps,
		Pagination: PaginationMeta{
			Total:   total,
			Limit:   limit,
			Offset:  offset,
			HasMore: offset+len(apps) < total,
		},
	})
}

func (h *Handler) GetApplication(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !isValidID(id) {
		respondError(w, http.StatusBadRequest, "invalid application ID format")
		return
	}
	app, err := h.store.Get(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get application")
		return
	}
	if app == nil {
		respondError(w, http.StatusNotFound, "application not found")
		return
	}
	respondJSON(w, http.StatusOK, app)
}

func (h *Handler) CreateApplication(w http.ResponseWriter, r *http.Request) {
	var req model.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			respondError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := req.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	app, err := h.store.Create(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create application")
		return
	}

	respondJSON(w, http.StatusCreated, app)
}

func (h *Handler) UpdateApplication(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !isValidID(id) {
		respondError(w, http.StatusBadRequest, "invalid application ID format")
		return
	}

	var fields map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			respondError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if statusVal, ok := fields["status"]; ok {
		if s, ok := statusVal.(string); ok {
			if err := model.ValidateStatus(s); err != nil {
				respondError(w, http.StatusBadRequest, err.Error())
				return
			}
		}
	}

	if minVal, minOk := fields["salary_min"]; minOk {
		if maxVal, maxOk := fields["salary_max"]; maxOk {
			if salaryMin, ok := minVal.(float64); ok {
				if salaryMax, ok := maxVal.(float64); ok {
					if salaryMin > salaryMax {
						respondError(w, http.StatusBadRequest, "salary_min cannot be greater than salary_max")
						return
					}
				}
			}
		}
	}

	app, err := h.store.Update(r.Context(), id, fields)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update application")
		return
	}
	if app == nil {
		respondError(w, http.StatusNotFound, "application not found")
		return
	}

	respondJSON(w, http.StatusOK, app)
}

func (h *Handler) DeleteApplication(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !isValidID(id) {
		respondError(w, http.StatusBadRequest, "invalid application ID format")
		return
	}

	deleted, err := h.store.Delete(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete application")
		return
	}
	if !deleted {
		respondError(w, http.StatusNotFound, "application not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
