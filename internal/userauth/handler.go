package userauth

import (
	"errors"
	"net/http"
	"strings"

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
	apple  AppleVerifier
}

func NewHandler(pool *pgxpool.Pool, users *user.Repository, idents *useridentity.Repository, token TokenConfig) *Handler {
	return &Handler{pool: pool, users: users, idents: idents, token: token}
}

// WithApple enables POST /auth/apple. Without a verifier the route returns 501.
func (h *Handler) WithApple(v AppleVerifier) *Handler {
	h.apple = v
	return h
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

// Apple authenticates a Sign in with Apple identity token and returns a JWT.
//
//	@Summary		Sign in with Apple
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			X-API-Version	header		string		true	"API version"	default(1)
//	@Param			X-API-Key		header		string		true	"App API key"
//	@Param			body			body		AppleInput	true	"Apple identity token"
//	@Success		200				{object}	TokenResponse
//	@Success		201				{object}	TokenResponse
//	@Failure		400				{object}	httpx.ErrorResponse
//	@Failure		401				{object}	httpx.ErrorResponse
//	@Failure		501				{object}	httpx.ErrorResponse
//	@Router			/auth/apple [post]
func (h *Handler) Apple(w http.ResponseWriter, r *http.Request) {
	appID, ok := auth.RequireAppID(w, r)
	if !ok {
		return
	}
	if h.apple == nil {
		httpx.WriteError(w, http.StatusNotImplemented, ErrAppleNotConfigured.Error())
		return
	}

	var body AppleInput
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if strings.TrimSpace(body.IdentityToken) == "" {
		httpx.WriteError(w, http.StatusBadRequest, "identity_token is required")
		return
	}

	claims, err := h.apple.Verify(r.Context(), body.IdentityToken)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	subject, err := appleSubject(claims.Subject)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	if ident, err := h.idents.GetByProviderSubject(r.Context(), appID, useridentity.ProviderApple, subject); err == nil {
		u, err := h.users.GetByID(r.Context(), appID, ident.UserID)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		h.writeToken(w, http.StatusOK, u)
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		writeAuthError(w, err)
		return
	}

	email, err := normalizeAppleEmail(claims.Email, body.Email)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	email, err = user.NormalizeEmail(email)
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

	status := http.StatusOK
	u, err := h.users.GetByEmail(r.Context(), appID, email)
	if errors.Is(err, store.ErrNotFound) {
		u, err = h.users.CreateTx(r.Context(), tx, appID, user.CreateInput{
			Email:    email,
			Metadata: body.Metadata,
		})
		if err != nil {
			writeAuthError(w, err)
			return
		}
		status = http.StatusCreated
	} else if err != nil {
		writeAuthError(w, err)
		return
	}

	if _, err := h.idents.CreateTx(r.Context(), tx, appID, useridentity.CreateInput{
		UserID:          u.ID,
		Provider:        useridentity.ProviderApple,
		ProviderSubject: subject,
		Metadata:        body.Metadata,
	}); err != nil {
		writeAuthError(w, err)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	h.writeToken(w, status, u)
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
	case errors.Is(err, ErrPasswordTooShort), errors.Is(err, ErrAppleEmailRequired):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrInvalidAppleToken), errors.Is(err, ErrAppleAudience), errors.Is(err, ErrInvalidPassword):
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, ErrAppleNotConfigured):
		httpx.WriteError(w, http.StatusNotImplemented, err.Error())
	case errors.Is(err, store.ErrConflict):
		httpx.WriteError(w, http.StatusConflict, "user with this email already exists in app")
	case errors.Is(err, user.ErrInvalidEmail), errors.Is(err, store.ErrInvalidInput):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}
