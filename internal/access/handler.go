package access

import (
	"errors"
	"net/http"

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

// Check evaluates whether a user may perform an action on a resource within a tenant.
//
//	@Summary		Check access
//	@Tags			access
//	@Accept			json
//	@Produce		json
//	@Param			X-API-Version	header		string		true	"API version"	default(1)
//	@Param			X-API-Key		header		string		true	"App API key"
//	@Param			body			body		CheckInput	true	"Access check request"
//	@Success		200				{object}	CheckResponse
//	@Router			/access/check [post]
func (h *Handler) Check(w http.ResponseWriter, r *http.Request) {
	appID, ok := auth.RequireAppID(w, r)
	if !ok {
		return
	}
	var body CheckInput
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	allowed, err := h.repo.Check(r.Context(), appID, body)
	if err != nil {
		if errors.Is(err, store.ErrInvalidInput) {
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, CheckResponse{Allowed: allowed})
}
