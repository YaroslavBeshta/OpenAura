package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
