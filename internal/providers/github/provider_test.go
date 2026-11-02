package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCurrentUser(t *testing.T) {
	tests := []struct {
		name, body, wantDisplay string
	}{
		{"with name", `{"id": 42, "login": "someone", "name": "Some One"}`, "Some One"},
		{"without name", `{"id": 42, "login": "someone", "name": null}`, "someone"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/user" || r.Header.Get("Authorization") != "Bearer secret" ||
					r.Header.Get("Accept") != "application/vnd.github+json" {
					http.Error(w, "unexpected request", http.StatusTeapot)
					return
				}
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			user, err := New(server.URL, "secret").CurrentUser(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if user.ID != "42" || *user.Username != "someone" || user.DisplayName != tt.wantDisplay {
				t.Errorf("user %+v", user)
			}
		})
	}
}
