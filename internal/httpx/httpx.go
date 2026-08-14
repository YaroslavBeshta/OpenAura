package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/openaura/openaura/internal/store"
)

// ErrorResponse is the standard API error payload.
type ErrorResponse struct {
	Error string `json:"error" example:"resource not found"`
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, map[string]string{"error": message})
}

func DecodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func Pagination(r *http.Request) (limit, offset int) {
	limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
	return ClampPagination(limit, offset)
}

// ClampPagination clamps a requested limit/offset to the API's accepted range:
// limit defaults to 50 and caps at 100, offset floors at 0.
func ClampPagination(limit, offset int) (int, int) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// RepoErrorMessages carries the entity-specific messages a handler wants for
// the common store error cases. Leave a field blank to fall through to the
// default 500 for that case (e.g. an entity with no unique-name conflict).
type RepoErrorMessages struct {
	NotFound    string
	Conflict    string
	FKViolation string
}

// WriteRepoError maps a store-layer error to an HTTP response using
// entity-specific messages, covering the CRUD error handling that's
// identical across handlers apart from wording.
func WriteRepoError(w http.ResponseWriter, err error, msgs RepoErrorMessages) {
	switch {
	case msgs.NotFound != "" && errors.Is(err, store.ErrNotFound):
		WriteError(w, http.StatusNotFound, msgs.NotFound)
	case msgs.Conflict != "" && errors.Is(err, store.ErrConflict):
		WriteError(w, http.StatusConflict, msgs.Conflict)
	case msgs.FKViolation != "" && errors.Is(err, store.ErrFKViolation):
		WriteError(w, http.StatusBadRequest, msgs.FKViolation)
	case errors.Is(err, store.ErrAppMismatch), errors.Is(err, store.ErrInvalidInput):
		WriteError(w, http.StatusBadRequest, err.Error())
	default:
		WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}

func NormalizeMetadata(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return json.RawMessage(`{}`), nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, errors.New("metadata must be a JSON object")
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return out, nil
}
