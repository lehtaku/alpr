package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCurrentUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, token, ok := r.BasicAuth()
		if r.URL.Path != "/rest/api/3/myself" || !ok || username != "you@example.com" || token != "secret" {
			http.Error(w, "unexpected request", http.StatusTeapot)
			return
		}
		_, _ = w.Write([]byte(`{"accountId": "5b10", "displayName": "Some One", "emailAddress": "you@example.com"}`))
	}))
	defer server.Close()

	user, err := New(server.URL, "you@example.com", "secret").CurrentUser(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if user.ID != "5b10" || user.Username != nil || user.DisplayName != "Some One" {
		t.Errorf("user %+v", user)
	}
}
