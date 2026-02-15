package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/shakilbd009/job-hunt-platform/internal/db"
	"github.com/shakilbd009/job-hunt-platform/internal/model"
)

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

func (h *Handler) Routes(r chi.Router) {
	r.Get("/applications", h.ListApplications)
	r.Get("/applications/{id}", h.GetApplication)
	r.Post("/applications", h.CreateApplication)
	r.Put("/applications/{id}", h.UpdateApplication)
	r.Delete("/applications/{id}", h.DeleteApplication)
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

	apps, err := h.store.List(r.Context(), status, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list applications")
		return
	}

	total, err := h.store.Count(r.Context(), status)
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

	var fields map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
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
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
