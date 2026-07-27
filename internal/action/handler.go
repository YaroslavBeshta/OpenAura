package action

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

// Create creates an action in the authenticated app.
//
//	@Summary		Create action
//	@Tags			actions
//	@Accept			json
//	@Produce		json
//	@Param			X-API-Version	header		string		true	"API version"	default(1)
//	@Param			X-API-Key		header		string		true	"App API key"
//	@Param			body			body		CreateInput	true	"Action to create"
//	@Success		201				{object}	Action
//	@Router			/actions [post]
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
	a, err := h.repo.Create(r.Context(), appID, body)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, a)
}

// Get returns an action by id within the authenticated app.
//
//	@Summary		Get action
//	@Tags			actions
//	@Produce		json
//	@Param			X-API-Version	header		string	true	"API version"	default(1)
//	@Param			X-API-Key		header		string	true	"App API key"
//	@Param			id				path		string	true	"Action ID"
//	@Success		200				{object}	Action
//	@Router			/actions/{id} [get]
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	appID, ok := auth.RequireAppID(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid action id")
		return
	}
	a, err := h.repo.GetByID(r.Context(), appID, id)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a)
}

// List returns actions for the authenticated app.
//
//	@Summary		List actions
//	@Tags			actions
//	@Produce		json
//	@Param			X-API-Version	header		string	true	"API version"	default(1)
//	@Param			X-API-Key		header		string	true	"App API key"
//	@Success		200				{object}	ListResponse
//	@Router			/actions [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	appID, ok := auth.RequireAppID(w, r)
	if !ok {
		return
	}
	limit, offset := httpx.Pagination(r)
	items, err := h.repo.List(r.Context(), ListFilter{AppID: appID, Limit: limit, Offset: offset})
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, ListResponse{Actions: items})
}

// Update updates an action within the authenticated app.
//
//	@Summary		Update action
//	@Tags			actions
//	@Accept			json
//	@Produce		json
//	@Param			X-API-Version	header		string		true	"API version"	default(1)
//	@Param			X-API-Key		header		string		true	"App API key"
//	@Param			id				path		string		true	"Action ID"
//	@Param			body			body		UpdateInput	true	"Fields to update"
//	@Success		200				{object}	Action
//	@Router			/actions/{id} [patch]
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	appID, ok := auth.RequireAppID(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid action id")
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

// Delete soft-deletes an action within the authenticated app.
//
//	@Summary		Delete action
//	@Tags			actions
//	@Param			X-API-Version	header	string	true	"API version"	default(1)
//	@Param			X-API-Key		header	string	true	"App API key"
//	@Param			id				path	string	true	"Action ID"
//	@Success		204
//	@Router			/actions/{id} [delete]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	appID, ok := auth.RequireAppID(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid action id")
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
		httpx.WriteError(w, http.StatusNotFound, "action not found")
	case errors.Is(err, store.ErrConflict):
		httpx.WriteError(w, http.StatusConflict, "action name already exists in app")
	case errors.Is(err, store.ErrInvalidInput):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}
