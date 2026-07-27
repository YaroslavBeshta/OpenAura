package user

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

// Create creates a user in the authenticated app.
//
//	@Summary		Create user
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			X-API-Version	header		string		true	"API version"	default(1)
//	@Param			X-API-Key		header		string		true	"App API key"
//	@Param			body			body		CreateInput	true	"User to create"
//	@Success		201				{object}	User
//	@Failure		400				{object}	httpx.ErrorResponse
//	@Failure		401				{object}	httpx.ErrorResponse
//	@Failure		409				{object}	httpx.ErrorResponse
//	@Router			/users [post]
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
	u, err := h.repo.Create(r.Context(), appID, body)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, u)
}

// Get returns a user by id within the authenticated app.
//
//	@Summary		Get user
//	@Tags			users
//	@Produce		json
//	@Param			X-API-Version	header		string	true	"API version"	default(1)
//	@Param			X-API-Key		header		string	true	"App API key"
//	@Param			id				path		string	true	"User ID"
//	@Success		200				{object}	User
//	@Router			/users/{id} [get]
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	appID, ok := auth.RequireAppID(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	u, err := h.repo.GetByID(r.Context(), appID, id)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, u)
}

// List returns users for the authenticated app.
//
//	@Summary		List users
//	@Tags			users
//	@Produce		json
//	@Param			X-API-Version	header		string	true	"API version"	default(1)
//	@Param			X-API-Key		header		string	true	"App API key"
//	@Param			limit			query		int		false	"Page size"	default(50)
//	@Param			offset			query		int		false	"Offset"	default(0)
//	@Success		200				{object}	ListResponse
//	@Router			/users [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	appID, ok := auth.RequireAppID(w, r)
	if !ok {
		return
	}
	limit, offset := httpx.Pagination(r)
	users, err := h.repo.List(r.Context(), ListFilter{AppID: appID, Limit: limit, Offset: offset})
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, ListResponse{Users: users})
}

// Update updates a user within the authenticated app.
//
//	@Summary		Update user
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			X-API-Version	header		string		true	"API version"	default(1)
//	@Param			X-API-Key		header		string		true	"App API key"
//	@Param			id				path		string		true	"User ID"
//	@Param			body			body		UpdateInput	true	"Fields to update"
//	@Success		200				{object}	User
//	@Router			/users/{id} [patch]
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	appID, ok := auth.RequireAppID(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	var body UpdateInput
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	u, err := h.repo.Update(r.Context(), appID, id, body)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, u)
}

// Delete soft-deletes a user within the authenticated app.
//
//	@Summary		Delete user
//	@Tags			users
//	@Param			X-API-Version	header	string	true	"API version"	default(1)
//	@Param			X-API-Key		header	string	true	"App API key"
//	@Param			id				path	string	true	"User ID"
//	@Success		204
//	@Router			/users/{id} [delete]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	appID, ok := auth.RequireAppID(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid user id")
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
		httpx.WriteError(w, http.StatusNotFound, "user not found")
	case errors.Is(err, store.ErrConflict):
		httpx.WriteError(w, http.StatusConflict, "user with this email already exists in app")
	case errors.Is(err, ErrInvalidEmail), errors.Is(err, store.ErrInvalidInput):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}
