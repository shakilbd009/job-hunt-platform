package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/shakilbd009/job-hunt-platform/internal/db"
	"github.com/shakilbd009/job-hunt-platform/internal/model"
)

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

	apps, err := h.store.List(status)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list applications")
		return
	}

	respondJSON(w, http.StatusOK, apps)
}

func (h *Handler) GetApplication(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	app, err := h.store.Get(id)
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

	app, err := h.store.Create(req)
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

	app, err := h.store.Update(id, fields)
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

	deleted, err := h.store.Delete(id)
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
