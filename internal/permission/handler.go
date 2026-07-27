package permission

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

// Create grants a permission to a role.
//
//	@Summary		Create role permission
//	@Tags			permissions
//	@Accept			json
//	@Produce		json
//	@Param			X-API-Version	header		string		true	"API version"	default(1)
//	@Param			X-API-Key		header		string		true	"App API key"
//	@Param			id				path		string		true	"Role ID"
//	@Param			body			body		CreateInput	true	"Permission to create"
//	@Success		201				{object}	Permission
//	@Router			/roles/{id}/permissions [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	appID, ok := auth.RequireAppID(w, r)
	if !ok {
		return
	}
	roleID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid role id")
		return
	}
	var body CreateInput
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	p, err := h.repo.Create(r.Context(), appID, roleID, body)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, p)
}

// Get returns a permission for a role.
//
//	@Summary		Get role permission
//	@Tags			permissions
//	@Produce		json
//	@Param			X-API-Version	header		string	true	"API version"	default(1)
//	@Param			X-API-Key		header		string	true	"App API key"
//	@Param			id				path		string	true	"Role ID"
//	@Param			permission_id	path		string	true	"Permission ID"
//	@Success		200				{object}	Permission
//	@Router			/roles/{id}/permissions/{permission_id} [get]
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	appID, ok := auth.RequireAppID(w, r)
	if !ok {
		return
	}
	roleID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid role id")
		return
	}
	permissionID, err := uuid.Parse(r.PathValue("permission_id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid permission id")
		return
	}
	p, err := h.repo.GetByID(r.Context(), appID, roleID, permissionID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}

// List returns permissions for a role.
//
//	@Summary		List role permissions
//	@Tags			permissions
//	@Produce		json
//	@Param			X-API-Version	header		string	true	"API version"	default(1)
//	@Param			X-API-Key		header		string	true	"App API key"
//	@Param			id				path		string	true	"Role ID"
//	@Param			resource_id		query		string	false	"Filter by resource ID"
//	@Param			action_id		query		string	false	"Filter by action ID"
//	@Success		200				{object}	ListResponse
//	@Router			/roles/{id}/permissions [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	appID, ok := auth.RequireAppID(w, r)
	if !ok {
		return
	}
	roleID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid role id")
		return
	}
	limit, offset := httpx.Pagination(r)
	filter := ListFilter{AppID: appID, RoleID: roleID, Limit: limit, Offset: offset}

	if v := r.URL.Query().Get("resource_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid resource_id")
			return
		}
		filter.ResourceID = &id
	}
	if v := r.URL.Query().Get("action_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid action_id")
			return
		}
		filter.ActionID = &id
	}

	items, err := h.repo.List(r.Context(), filter)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, ListResponse{Permissions: items})
}

// Delete revokes a permission from a role.
//
//	@Summary		Delete role permission
//	@Tags			permissions
//	@Param			X-API-Version	header	string	true	"API version"	default(1)
//	@Param			X-API-Key		header	string	true	"App API key"
//	@Param			id				path	string	true	"Role ID"
//	@Param			permission_id	path	string	true	"Permission ID"
//	@Success		204
//	@Router			/roles/{id}/permissions/{permission_id} [delete]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	appID, ok := auth.RequireAppID(w, r)
	if !ok {
		return
	}
	roleID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid role id")
		return
	}
	permissionID, err := uuid.Parse(r.PathValue("permission_id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid permission id")
		return
	}
	if err := h.repo.SoftDelete(r.Context(), appID, roleID, permissionID); err != nil {
		writeRepoError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "permission not found")
	case errors.Is(err, store.ErrConflict):
		httpx.WriteError(w, http.StatusConflict, "permission already exists")
	case errors.Is(err, store.ErrFKViolation):
		httpx.WriteError(w, http.StatusBadRequest, "role_id, resource_id, or action_id does not exist")
	case errors.Is(err, store.ErrAppMismatch):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, store.ErrInvalidInput):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}
