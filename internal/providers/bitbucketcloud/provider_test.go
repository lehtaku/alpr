package bitbucketcloud

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCurrentUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/user" || r.Header.Get("Authorization") != "Bearer secret" {
			http.Error(w, "unexpected request", http.StatusTeapot)
			return
		}
		_, _ = w.Write([]byte(`{"uuid": "{1234}", "account_id": "5f00", "nickname": "nick", "display_name": "Some One"}`))
	}))
	defer server.Close()

	user, err := New(server.URL+"/2.0", "secret").CurrentUser(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if user.ID != "{1234}" || user.Username == nil || *user.Username != "nick" || user.DisplayName != "Some One" {
		t.Errorf("user %+v", user)
	}
}
