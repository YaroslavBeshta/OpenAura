package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openaura/openaura/internal/store"
)

func TestNormalizeMetadata(t *testing.T) {
	got, err := NormalizeMetadata(nil)
	if err != nil {
		t.Fatalf("nil: %v", err)
	}
	if string(got) != "{}" {
		t.Fatalf("nil => %s", got)
	}

	got, err = NormalizeMetadata(json.RawMessage(`{"b":1,"a":2}`))
	if err != nil {
		t.Fatalf("object: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if obj["a"] != float64(2) || obj["b"] != float64(1) {
		t.Fatalf("unexpected object: %v", obj)
	}

	if _, err := NormalizeMetadata(json.RawMessage(`["not","object"]`)); err == nil {
		t.Fatal("expected error for array metadata")
	}
	if _, err := NormalizeMetadata(json.RawMessage(`null`)); err == nil {
		t.Fatal("expected error for null metadata")
	}
}

func TestPagination(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x?limit=10&offset=5", nil)
	limit, offset := Pagination(req)
	if limit != 10 || offset != 5 {
		t.Fatalf("got (%d,%d)", limit, offset)
	}

	req = httptest.NewRequest(http.MethodGet, "/x?limit=1000&offset=-3", nil)
	limit, offset = Pagination(req)
	if limit != 50 || offset != 0 {
		t.Fatalf("clamped got (%d,%d)", limit, offset)
	}
}

func TestClampPagination(t *testing.T) {
	tests := []struct {
		limit, offset      int
		wantLimit, wantOff int
	}{
		{0, 0, 50, 0},
		{-1, -5, 50, 0},
		{10, 3, 10, 3},
		{101, 0, 50, 0},
	}
	for _, tt := range tests {
		gotLimit, gotOff := ClampPagination(tt.limit, tt.offset)
		if gotLimit != tt.wantLimit || gotOff != tt.wantOff {
			t.Fatalf("ClampPagination(%d,%d)=(%d,%d), want (%d,%d)",
				tt.limit, tt.offset, gotLimit, gotOff, tt.wantLimit, tt.wantOff)
		}
	}
}

func TestWriteRepoError(t *testing.T) {
	msgs := RepoErrorMessages{
		NotFound:    "widget not found",
		Conflict:    "widget already exists",
		FKViolation: "owner_id does not exist",
	}
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{"not found", store.ErrNotFound, http.StatusNotFound, "widget not found"},
		{"conflict", store.ErrConflict, http.StatusConflict, "widget already exists"},
		{"fk violation", store.ErrFKViolation, http.StatusBadRequest, "owner_id does not exist"},
		{"app mismatch", store.ErrAppMismatch, http.StatusBadRequest, store.ErrAppMismatch.Error()},
		{"invalid input", fmt.Errorf("%w: name is required", store.ErrInvalidInput), http.StatusBadRequest, "invalid input: name is required"},
		{"unknown", errors.New("boom"), http.StatusInternalServerError, "internal server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			WriteRepoError(rec, tt.err, msgs)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			var body ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body.Error != tt.wantBody {
				t.Fatalf("body = %q, want %q", body.Error, tt.wantBody)
			}
		})
	}
}

func TestWriteRepoErrorBlankMessageFallsThrough(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteRepoError(rec, store.ErrConflict, RepoErrorMessages{NotFound: "widget not found"})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d (blank Conflict message should not match)", rec.Code, http.StatusInternalServerError)
	}
}
