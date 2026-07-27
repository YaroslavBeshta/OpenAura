package tenant

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/openaura/openaura/internal/auth"
	"github.com/openaura/openaura/internal/httpx"
	"github.com/openaura/openaura/internal/store"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

// Create creates a tenant in the authenticated app.
//
//	@Summary		Create tenant
//	@Tags			tenants
//	@Accept			json
//	@Produce		json
//	@Param			X-API-Version	header		string		true	"API version"	default(1)
//	@Param			X-API-Key		header		string		true	"App API key"
//	@Param			body			body		CreateInput	true	"Tenant to create"
//	@Success		201				{object}	Tenant
//	@Router			/tenants [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	appID, ok := auth.RequireAppID(w, r)
	if !ok {
		return
	}
	var body CreateInput
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	t, err := h.repo.Create(r.Context(), appID, body)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, t)
}

// Get returns a tenant by id within the authenticated app.
//
//	@Summary		Get tenant
//	@Tags			tenants
//	@Produce		json
//	@Param			X-API-Version	header		string	true	"API version"	default(1)
//	@Param			X-API-Key		header		string	true	"App API key"
//	@Param			id				path		string	true	"Tenant ID"
//	@Success		200				{object}	Tenant
//	@Router			/tenants/{id} [get]
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	appID, ok := auth.RequireAppID(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid tenant id")
		return
	}
	t, err := h.repo.GetByID(r.Context(), appID, id)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, t)
}

// List returns tenants for the authenticated app.
//
//	@Summary		List tenants
//	@Tags			tenants
//	@Produce		json
//	@Param			X-API-Version	header		string	true	"API version"	default(1)
//	@Param			X-API-Key		header		string	true	"App API key"
//	@Success		200				{object}	ListResponse
//	@Router			/tenants [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	appID, ok := auth.RequireAppID(w, r)
	if !ok {
		return
	}
	limit, offset := httpx.Pagination(r)
	tenants, err := h.repo.List(r.Context(), ListFilter{AppID: appID, Limit: limit, Offset: offset})
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, ListResponse{Tenants: tenants})
}

// Update updates a tenant within the authenticated app.
//
//	@Summary		Update tenant
//	@Tags			tenants
//	@Accept			json
//	@Produce		json
//	@Param			X-API-Version	header		string		true	"API version"	default(1)
//	@Param			X-API-Key		header		string		true	"App API key"
//	@Param			id				path		string		true	"Tenant ID"
//	@Param			body			body		UpdateInput	true	"Fields to update"
//	@Success		200				{object}	Tenant
//	@Router			/tenants/{id} [patch]
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	appID, ok := auth.RequireAppID(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid tenant id")
		return
	}
	var body UpdateInput
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	t, err := h.repo.Update(r.Context(), appID, id, body)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, t)
}

// Delete soft-deletes a tenant within the authenticated app.
//
//	@Summary		Delete tenant
//	@Tags			tenants
//	@Param			X-API-Version	header	string	true	"API version"	default(1)
//	@Param			X-API-Key		header	string	true	"App API key"
//	@Param			id				path	string	true	"Tenant ID"
//	@Success		204
//	@Router			/tenants/{id} [delete]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	appID, ok := auth.RequireAppID(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid tenant id")
		return
	}
	if err := h.repo.SoftDelete(r.Context(), appID, id); err != nil {
		writeRepoError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "tenant not found")
	case errors.Is(err, store.ErrInvalidInput):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}
