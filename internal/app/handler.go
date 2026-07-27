package app

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/openaura/openaura/internal/httpx"
	"github.com/openaura/openaura/internal/store"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

// Create creates an app.
//
//	@Summary		Create app
//	@Tags			admin-apps
//	@Accept			json
//	@Produce		json
//	@Param			X-API-Version	header		string		true	"API version"	default(1)
//	@Param			X-API-Key		header		string		true	"Admin API key"
//	@Param			body			body		CreateInput	true	"App to create"
//	@Success		201				{object}	App
//	@Failure		400				{object}	httpx.ErrorResponse
//	@Failure		401				{object}	httpx.ErrorResponse
//	@Router			/admin/apps [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var body CreateInput
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	a, err := h.repo.Create(r.Context(), body)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, a)
}

// Get returns an app by id.
//
//	@Summary		Get app
//	@Tags			admin-apps
//	@Produce		json
//	@Param			X-API-Version	header		string	true	"API version"	default(1)
//	@Param			X-API-Key		header		string	true	"Admin API key"
//	@Param			id				path		string	true	"App ID"
//	@Success		200				{object}	App
//	@Router			/admin/apps/{id} [get]
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid app id")
		return
	}
	a, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a)
}

// List returns apps.
//
//	@Summary		List apps
//	@Tags			admin-apps
//	@Produce		json
//	@Param			X-API-Version	header		string	true	"API version"	default(1)
//	@Param			X-API-Key		header		string	true	"Admin API key"
//	@Param			limit			query		int		false	"Page size"	default(50)
//	@Param			offset			query		int		false	"Offset"	default(0)
//	@Success		200				{object}	ListResponse
//	@Router			/admin/apps [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	limit, offset := httpx.Pagination(r)
	apps, err := h.repo.List(r.Context(), ListFilter{Limit: limit, Offset: offset})
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, ListResponse{Apps: apps})
}

// Update updates an app.
//
//	@Summary		Update app
//	@Tags			admin-apps
//	@Accept			json
//	@Produce		json
//	@Param			X-API-Version	header		string		true	"API version"	default(1)
//	@Param			X-API-Key		header		string		true	"Admin API key"
//	@Param			id				path		string		true	"App ID"
//	@Param			body			body		UpdateInput	true	"Fields to update"
//	@Success		200				{object}	App
//	@Router			/admin/apps/{id} [patch]
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid app id")
		return
	}
	var body UpdateInput
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	a, err := h.repo.Update(r.Context(), id, body)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a)
}

// Delete soft-deletes an app.
//
//	@Summary		Delete app
//	@Tags			admin-apps
//	@Param			X-API-Version	header	string	true	"API version"	default(1)
//	@Param			X-API-Key		header	string	true	"Admin API key"
//	@Param			id				path	string	true	"App ID"
//	@Success		204
//	@Router			/admin/apps/{id} [delete]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid app id")
		return
	}
	if err := h.repo.SoftDelete(r.Context(), id); err != nil {
		writeRepoError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "app not found")
	case errors.Is(err, store.ErrInvalidInput):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}
