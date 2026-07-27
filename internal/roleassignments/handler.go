package roleassignments

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

// Create creates a role assignment within the authenticated app.
//
//	@Summary		Create role assignment
//	@Tags			roleassignments
//	@Accept			json
//	@Produce		json
//	@Param			X-API-Version	header		string		true	"API version"	default(1)
//	@Param			X-API-Key		header		string		true	"App API key"
//	@Param			body			body		CreateInput	true	"Assignment to create"
//	@Success		201				{object}	RoleAssignment
//	@Router			/roleassignments [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.RequireAppID(w, r); !ok {
		return
	}
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

// Get returns a role assignment by id within the authenticated app.
//
//	@Summary		Get role assignment
//	@Tags			roleassignments
//	@Produce		json
//	@Param			X-API-Version	header		string	true	"API version"	default(1)
//	@Param			X-API-Key		header		string	true	"App API key"
//	@Param			id				path		string	true	"Role assignment ID"
//	@Success		200				{object}	RoleAssignment
//	@Router			/roleassignments/{id} [get]
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	appID, ok := auth.RequireAppID(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid role assignment id")
		return
	}
	a, err := h.repo.GetByID(r.Context(), appID, id)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a)
}

// List returns role assignments for the authenticated app.
//
//	@Summary		List role assignments
//	@Tags			roleassignments
//	@Produce		json
//	@Param			X-API-Version	header		string	true	"API version"	default(1)
//	@Param			X-API-Key		header		string	true	"App API key"
//	@Param			user_id			query		string	false	"Filter by user ID"
//	@Param			role_id			query		string	false	"Filter by role ID"
//	@Param			tenant_id		query		string	false	"Filter by tenant ID"
//	@Success		200				{object}	ListResponse
//	@Router			/roleassignments [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	appID, ok := auth.RequireAppID(w, r)
	if !ok {
		return
	}
	limit, offset := httpx.Pagination(r)
	filter := ListFilter{AppID: appID, Limit: limit, Offset: offset}

	if v := r.URL.Query().Get("user_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid user_id")
			return
		}
		filter.UserID = &id
	}
	if v := r.URL.Query().Get("role_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid role_id")
			return
		}
		filter.RoleID = &id
	}
	if v := r.URL.Query().Get("tenant_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid tenant_id")
			return
		}
		filter.TenantID = &id
	}

	assignments, err := h.repo.List(r.Context(), filter)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, ListResponse{RoleAssignments: assignments})
}

// Update updates a role assignment within the authenticated app.
//
//	@Summary		Update role assignment
//	@Tags			roleassignments
//	@Accept			json
//	@Produce		json
//	@Param			X-API-Version	header		string		true	"API version"	default(1)
//	@Param			X-API-Key		header		string		true	"App API key"
//	@Param			id				path		string		true	"Role assignment ID"
//	@Param			body			body		UpdateInput	true	"Fields to update"
//	@Success		200				{object}	RoleAssignment
//	@Router			/roleassignments/{id} [patch]
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	appID, ok := auth.RequireAppID(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid role assignment id")
		return
	}
	var body UpdateInput
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	a, err := h.repo.Update(r.Context(), appID, id, body)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a)
}

// Delete soft-deletes a role assignment within the authenticated app.
//
//	@Summary		Delete role assignment
//	@Tags			roleassignments
//	@Param			X-API-Version	header	string	true	"API version"	default(1)
//	@Param			X-API-Key		header	string	true	"App API key"
//	@Param			id				path	string	true	"Role assignment ID"
//	@Success		204
//	@Router			/roleassignments/{id} [delete]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	appID, ok := auth.RequireAppID(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid role assignment id")
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
		httpx.WriteError(w, http.StatusNotFound, "role assignment not found")
	case errors.Is(err, store.ErrConflict):
		httpx.WriteError(w, http.StatusConflict, "role assignment already exists")
	case errors.Is(err, store.ErrFKViolation):
		httpx.WriteError(w, http.StatusBadRequest, "user_id, role_id, or tenant_id does not exist")
	case errors.Is(err, store.ErrAppMismatch):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, store.ErrInvalidInput):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}
