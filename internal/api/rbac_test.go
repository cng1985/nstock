package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"nstock/internal/auth"
)

func loginAs(t *testing.T, h http.Handler, username, password string) string {
	t.Helper()
	rec := call(h, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": username, "password": password,
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s %d %s", username, rec.Code, rec.Body.String())
	}
	var body struct {
		Data struct {
			Token string `json:"token"`
			User  struct {
				Username    string   `json:"username"`
				Permissions []string `json:"permissions"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Data.Token == "" {
		t.Fatalf("login body %s", rec.Body.String())
	}
	return body.Data.Token
}

func TestRBAC(t *testing.T) {
	h := testEngine(t)
	rec := call(h, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": auth.UserAdmin, "password": "nope",
	}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad login %d %s", rec.Code, rec.Body.String())
	}

	admin := loginAs(t, h, auth.UserAdmin, auth.PassAdmin)
	analyst := loginAs(t, h, auth.UserAnalyst, auth.PassAnalyst)
	trader := loginAs(t, h, auth.UserTrader, auth.PassTrader)

	rec = call(h, http.MethodPost, "/api/v1/order/create", map[string]any{"code": "600000", "side": "buy", "qty": 100}, analyst)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("analyst order %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPost, "/api/v1/account/get", map[string]any{}, analyst)
	if rec.Code != http.StatusOK {
		t.Fatalf("analyst account %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPost, "/api/v1/user/list", map[string]any{}, trader)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("trader users %d %s", rec.Code, rec.Body.String())
	}

	rec = call(h, http.MethodPost, "/api/v1/auth/me", map[string]any{}, admin)
	if rec.Code != http.StatusOK || !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("me %d %s", rec.Code, rec.Body.String())
	}

	rec = call(h, http.MethodPost, "/api/v1/user/create", map[string]any{
		"username": "liming", "displayName": "李明", "password": "liming1", "roleIds": []uint{},
	}, admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("create user %d %s", rec.Code, rec.Body.String())
	}
	guest := loginAs(t, h, "liming", "liming1")
	rec = call(h, http.MethodPost, "/api/v1/system/health", map[string]any{}, guest)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("guest health %d %s", rec.Code, rec.Body.String())
	}

	rec = call(h, http.MethodPost, "/api/v1/auth/logout", map[string]any{}, guest)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPost, "/api/v1/auth/me", map[string]any{}, guest)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout %d %s", rec.Code, rec.Body.String())
	}

	rec = call(h, http.MethodPost, "/api/v1/user/list", map[string]any{}, "not-the-token")
	if rec.Code == http.StatusOK {
		t.Fatal("unknown token must not list users")
	}
}

func TestStaticTokenBypassesRBAC(t *testing.T) {
	h := testEngineToken(t, "super-token")
	rec := call(h, http.MethodPost, "/api/v1/user/list", map[string]any{}, "super-token")
	if rec.Code != http.StatusOK {
		t.Fatalf("static token %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data []struct {
			Username string `json:"username"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || len(body.Data) < 3 {
		t.Fatalf("users %s", rec.Body.String())
	}
	session := loginAs(t, h, auth.UserAnalyst, auth.PassAnalyst)
	rec = call(h, http.MethodPost, "/api/v1/order/create", map[string]any{"code": "600000", "side": "buy", "qty": 100}, session)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("session still follows rbac %d %s", rec.Code, rec.Body.String())
	}
}
