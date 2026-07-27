package adminapikey

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

// Create creates an admin API key.
//
//	@Summary		Create admin API key
//	@Tags			admin-api_keys
//	@Accept			json
//	@Produce		json
//	@Param			X-API-Version	header		string		true	"API version"	default(1)
//	@Param			X-API-Key		header		string		true	"Admin API key"
//	@Param			body			body		CreateInput	true	"Admin API key to create"
//	@Success		201				{object}	AdminAPIKey
//	@Router			/admin/api_keys [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var body CreateInput
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	k, err := h.repo.Create(r.Context(), body)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, k)
}

// Get returns an admin API key.
//
//	@Summary		Get admin API key
//	@Tags			admin-api_keys
//	@Produce		json
//	@Param			X-API-Version	header		string	true	"API version"	default(1)
//	@Param			X-API-Key		header		string	true	"Admin API key"
//	@Param			id				path		string	true	"Admin API key ID"
//	@Success		200				{object}	AdminAPIKey
//	@Router			/admin/api_keys/{id} [get]
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid admin api key id")
		return
	}
	k, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, k)
}

// List lists admin API keys.
//
//	@Summary		List admin API keys
//	@Tags			admin-api_keys
//	@Produce		json
//	@Param			X-API-Version	header		string	true	"API version"	default(1)
//	@Param			X-API-Key		header		string	true	"Admin API key"
//	@Success		200				{object}	ListResponse
//	@Router			/admin/api_keys [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	limit, offset := httpx.Pagination(r)
	keys, err := h.repo.List(r.Context(), ListFilter{Limit: limit, Offset: offset})
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, ListResponse{AdminAPIKeys: keys})
}

// Delete revokes an admin API key.
//
//	@Summary		Revoke admin API key
//	@Tags			admin-api_keys
//	@Param			X-API-Version	header	string	true	"API version"	default(1)
//	@Param			X-API-Key		header	string	true	"Admin API key"
//	@Param			id				path	string	true	"Admin API key ID"
//	@Success		204
//	@Router			/admin/api_keys/{id} [delete]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid admin api key id")
		return
	}
	if err := h.repo.Revoke(r.Context(), id); err != nil {
		writeRepoError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "admin api key not found")
	case errors.Is(err, store.ErrConflict):
		httpx.WriteError(w, http.StatusConflict, "admin api key already exists")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}
