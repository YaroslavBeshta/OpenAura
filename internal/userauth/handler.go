package userauth

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openaura/openaura/internal/auth"
	"github.com/openaura/openaura/internal/httpx"
	"github.com/openaura/openaura/internal/store"
	"github.com/openaura/openaura/internal/user"
	"github.com/openaura/openaura/internal/useridentity"
)

type Handler struct {
	pool   *pgxpool.Pool
	users  *user.Repository
	idents *useridentity.Repository
	token  TokenConfig
}

func NewHandler(pool *pgxpool.Pool, users *user.Repository, idents *useridentity.Repository, token TokenConfig) *Handler {
	return &Handler{pool: pool, users: users, idents: idents, token: token}
}

// Register creates a user with a password identity and returns a JWT access token.
//
//	@Summary		Register user
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			X-API-Version	header		string			true	"API version"	default(1)
//	@Param			X-API-Key		header		string			true	"App API key"
//	@Param			body			body		RegisterInput	true	"Registration"
//	@Success		201				{object}	TokenResponse
//	@Failure		400				{object}	httpx.ErrorResponse
//	@Failure		401				{object}	httpx.ErrorResponse
//	@Failure		409				{object}	httpx.ErrorResponse
//	@Router			/auth/register [post]
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	appID, ok := auth.RequireAppID(w, r)
	if !ok {
		return
	}

	var body RegisterInput
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	hash, err := hashPassword(body.Password)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	email, err := user.NormalizeEmail(body.Email)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	u, err := h.users.CreateTx(r.Context(), tx, appID, user.CreateInput{
		Email:    email,
		Metadata: body.Metadata,
	})
	if err != nil {
		writeAuthError(w, err)
		return
	}

	if _, err := h.idents.CreateTx(r.Context(), tx, appID, useridentity.CreateInput{
		UserID:          u.ID,
		Provider:        useridentity.ProviderPassword,
		ProviderSubject: email,
		SecretHash:      hash,
	}); err != nil {
		writeAuthError(w, err)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	h.writeToken(w, http.StatusCreated, u)
}

// Login authenticates email/password and returns a JWT access token.
//
//	@Summary		Login
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			X-API-Version	header		string		true	"API version"	default(1)
//	@Param			X-API-Key		header		string		true	"App API key"
//	@Param			body			body		LoginInput	true	"Credentials"
//	@Success		200				{object}	TokenResponse
//	@Failure		400				{object}	httpx.ErrorResponse
//	@Failure		401				{object}	httpx.ErrorResponse
//	@Router			/auth/login [post]
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	appID, ok := auth.RequireAppID(w, r)
	if !ok {
		return
	}

	var body LoginInput
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if body.Email == "" || body.Password == "" {
		httpx.WriteError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	cred, err := h.idents.GetPasswordByEmail(r.Context(), appID, body.Email)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, user.ErrInvalidEmail) {
			httpx.WriteError(w, http.StatusUnauthorized, ErrInvalidPassword.Error())
			return
		}
		writeAuthError(w, err)
		return
	}

	if err := checkPassword(cred.SecretHash, body.Password); err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, ErrInvalidPassword.Error())
		return
	}

	h.writeToken(w, http.StatusOK, cred.User)
}

func (h *Handler) writeToken(w http.ResponseWriter, status int, u user.User) {
	accessToken, expiresIn, err := issueToken(h.token, u)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httpx.WriteJSON(w, status, TokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   expiresIn,
		User:        u,
	})
}

func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrPasswordTooShort):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, store.ErrConflict):
		httpx.WriteError(w, http.StatusConflict, "user with this email already exists in app")
	case errors.Is(err, user.ErrInvalidEmail), errors.Is(err, store.ErrInvalidInput):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}
