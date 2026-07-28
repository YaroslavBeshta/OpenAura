package httpserver_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/openaura/openaura/internal/apitest"
	"github.com/openaura/openaura/internal/testutil"
	"github.com/openaura/openaura/internal/userauth"
)

func TestAuth_RegisterLogin(t *testing.T) {
	api := apitest.New(t)
	email := testutil.Email("auth")
	password := "correct-horse-battery"

	var registered userauth.TokenResponse
	status := api.JSON(http.MethodPost, "/auth/register", map[string]any{
		"email":    email,
		"password": password,
		"metadata": map[string]any{"name": "Ada"},
	}, &registered)
	if status != http.StatusCreated {
		t.Fatalf("register status=%d", status)
	}
	if registered.AccessToken == "" || registered.TokenType != "Bearer" || registered.ExpiresIn != 3600 {
		t.Fatalf("register token payload: %+v", registered)
	}
	if registered.User.Email != email || registered.User.ID.String() == "" {
		t.Fatalf("register user: %+v", registered.User)
	}
	if strings.Contains(string(mustJSON(t, registered)), "password") {
		t.Fatalf("response leaked password fields: %s", mustJSON(t, registered))
	}

	claims, err := userauth.ParseToken(api.TokenCfg, registered.AccessToken)
	if err != nil {
		t.Fatalf("parse register token: %v", err)
	}
	if claims.Subject != registered.User.ID.String() {
		t.Fatalf("sub=%q want %q", claims.Subject, registered.User.ID)
	}
	if claims.AppID != api.AppID.String() {
		t.Fatalf("app_id=%q want %q", claims.AppID, api.AppID)
	}
	if claims.Email != email {
		t.Fatalf("email=%q", claims.Email)
	}

	var loggedIn userauth.TokenResponse
	status = api.JSON(http.MethodPost, "/auth/login", map[string]any{
		"email":    email,
		"password": password,
	}, &loggedIn)
	if status != http.StatusOK {
		t.Fatalf("login status=%d", status)
	}
	if loggedIn.User.ID != registered.User.ID {
		t.Fatalf("login user id mismatch")
	}
	if _, err := userauth.ParseToken(api.TokenCfg, loggedIn.AccessToken); err != nil {
		t.Fatalf("parse login token: %v", err)
	}
}

func TestAuth_DuplicateRegister(t *testing.T) {
	api := apitest.New(t)
	email := testutil.Email("dup")
	body := map[string]any{"email": email, "password": "longpassword1"}

	if status := api.JSON(http.MethodPost, "/auth/register", body, nil); status != http.StatusCreated {
		t.Fatalf("first register status=%d", status)
	}
	if status := api.JSON(http.MethodPost, "/auth/register", body, nil); status != http.StatusConflict {
		t.Fatalf("second register status=%d want 409", status)
	}

	var page struct {
		Users []struct {
			Email string `json:"email"`
		} `json:"users"`
	}
	if status := api.JSON(http.MethodGet, "/users?limit=100", nil, &page); status != http.StatusOK {
		t.Fatalf("list users status=%d", status)
	}
	count := 0
	for _, u := range page.Users {
		if u.Email == email {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 user after failed duplicate register, got %d", count)
	}
}

func TestAuth_BadPasswordAndMissingUser(t *testing.T) {
	api := apitest.New(t)
	email := testutil.Email("badpw")

	if status := api.JSON(http.MethodPost, "/auth/register", map[string]any{
		"email": email, "password": "longpassword1",
	}, nil); status != http.StatusCreated {
		t.Fatalf("register status=%d", status)
	}

	var errBody map[string]string
	status := api.JSON(http.MethodPost, "/auth/login", map[string]any{
		"email": email, "password": "wrong-password",
	}, &errBody)
	if status != http.StatusUnauthorized {
		t.Fatalf("bad password status=%d", status)
	}
	if errBody["error"] != "invalid email or password" {
		t.Fatalf("error=%q", errBody["error"])
	}

	status = api.JSON(http.MethodPost, "/auth/login", map[string]any{
		"email": testutil.Email("missing"), "password": "longpassword1",
	}, &errBody)
	if status != http.StatusUnauthorized || errBody["error"] != "invalid email or password" {
		t.Fatalf("missing user status=%d error=%q", status, errBody["error"])
	}
}

func TestAuth_PasswordlessUserCannotLogin(t *testing.T) {
	api := apitest.New(t)
	email := testutil.Email("nopw")

	if status := api.JSON(http.MethodPost, "/users", map[string]any{"email": email}, nil); status != http.StatusCreated {
		t.Fatalf("create user status=%d", status)
	}

	var errBody map[string]string
	status := api.JSON(http.MethodPost, "/auth/login", map[string]any{
		"email": email, "password": "longpassword1",
	}, &errBody)
	if status != http.StatusUnauthorized {
		t.Fatalf("login status=%d want 401", status)
	}
}

func TestAuth_RequiresAppKeyAndValidPassword(t *testing.T) {
	api := apitest.New(t)

	resp := api.Do(http.MethodPost, "/auth/register", "1", "", map[string]any{
		"email": testutil.Email("nokey"), "password": "longpassword1",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("register without key status=%d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	resp = api.Do(http.MethodPost, "/auth/login", "1", "", map[string]any{
		"email": testutil.Email("nokey"), "password": "longpassword1",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("login without key status=%d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	var errBody map[string]string
	status := api.JSON(http.MethodPost, "/auth/register", map[string]any{
		"email": testutil.Email("short"), "password": "short",
	}, &errBody)
	if status != http.StatusBadRequest {
		t.Fatalf("short password status=%d", status)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}
