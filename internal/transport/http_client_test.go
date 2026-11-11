package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lehtaku/alpr/internal/domain"
)

func domainError(t *testing.T, err error) *domain.Error {
	t.Helper()
	var domainErr *domain.Error
	if !errors.As(err, &domainErr) {
		t.Fatalf("want *domain.Error, got %T: %v", err, err)
	}
	return domainErr
}

func TestDoGet(t *testing.T) {
	var got *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		w.Header().Set("X-Test", "yes")
		_, _ = w.Write([]byte(`{"login": "someone"}`))
	}))
	defer server.Close()

	client := &Client{
		BaseURL:       server.URL + "/api/",
		Authorization: BearerAuth("secret"),
		Headers:       map[string]string{"Accept": "application/vnd.github+json"},
	}
	var user struct {
		Login string `json:"login"`
	}
	header, err := client.Do(context.Background(), http.MethodGet, "/user?x=1", nil, &user)
	if err != nil {
		t.Fatal(err)
	}

	if user.Login != "someone" || header.Get("X-Test") != "yes" {
		t.Errorf("user %+v, header %v", user, header)
	}
	if got.URL.Path != "/api/user" || got.URL.RawQuery != "x=1" {
		t.Errorf("request URL %s", got.URL)
	}
	if got.Header.Get("Authorization") != "Bearer secret" || got.Header.Get("User-Agent") != "alpr" ||
		got.Header.Get("Accept") != "application/vnd.github+json" || got.Header.Get("Content-Type") != "" {
		t.Errorf("request headers %v", got.Header)
	}
}

func TestDoPostBody(t *testing.T) {
	var contentType string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL}
	var out map[string]any
	if _, err := client.Do(context.Background(), http.MethodPost, "comments", map[string]string{"body": "hi"}, &out); err != nil {
		t.Fatal(err)
	}
	if contentType != "application/json" || body["body"] != "hi" {
		t.Errorf("content type %q, body %v", contentType, body)
	}
}

func TestDoStatusErrors(t *testing.T) {
	reset := strconv.FormatInt(time.Now().Add(90*time.Second).Unix(), 10)
	tests := []struct {
		name      string
		status    int
		headers   map[string]string
		body      string
		wantCode  domain.Code
		wantMsg   string
		wantRetry any
	}{
		{"unauthorized", 401, nil, `{"message": "Bad credentials"}`, domain.CodeAuthInvalid, "HTTP 401 Unauthorized: Bad credentials", nil},
		{"forbidden", 403, nil, `{"type": "error", "error": {"message": "Your credentials lack scopes"}}`, domain.CodePermissionDenied, "lack scopes", nil},
		{"not found", 404, nil, `{"type": "error", "error": {"message": "Resource not found"}}`, domain.CodeNotFound, "Resource not found", nil},
		{"unprocessable", 422, nil, `{"message": "Validation Failed", "errors": [{"code": "invalid"}]}`, domain.CodeAPIError, "Validation Failed", nil},
		{"server error with HTML", 502, nil, `<html>Bad Gateway</html>`, domain.CodeAPIError, "HTTP 502 Bad Gateway", nil},
		{"429 with Retry-After", 429, map[string]string{"Retry-After": "30"}, ``, domain.CodeRateLimited, "HTTP 429", 30},
		{"429 without hint", 429, nil, ``, domain.CodeRateLimited, "HTTP 429", nil},
		{"GitHub secondary limit", 403, map[string]string{"Retry-After": "60"}, `{"message": "secondary rate limit"}`, domain.CodeRateLimited, "secondary rate limit", 60},
		{"GitHub primary limit", 403, map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": reset}, `{"message": "API rate limit exceeded"}`, domain.CodeRateLimited, "rate limit exceeded", "around 90"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for key, value := range tt.headers {
					w.Header().Set(key, value)
				}
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()

			_, err := (&Client{BaseURL: server.URL}).Do(context.Background(), http.MethodGet, "thing", nil, nil)
			got := domainError(t, err)
			if got.Code != tt.wantCode || !strings.Contains(got.Message, tt.wantMsg) {
				t.Errorf("code %s, message %q", got.Code, got.Message)
			}
			if got.Details["status"] != tt.status || got.Details["url"] != server.URL+"/thing" {
				t.Errorf("details %v", got.Details)
			}
			retry, hasRetry := got.Details["retry_after_seconds"]
			switch want := tt.wantRetry.(type) {
			case nil:
				if hasRetry {
					t.Errorf("unexpected retry_after_seconds %v", retry)
				}
			case int:
				if retry != want {
					t.Errorf("retry_after_seconds %v, want %d", retry, want)
				}
			case string: // a reset time: allow for clock movement during the test
				if seconds, ok := retry.(int); !ok || seconds < 85 || seconds > 90 {
					t.Errorf("retry_after_seconds %v, want %s", retry, want)
				}
			}
		})
	}
}

func TestDoUnexpectedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html>maintenance</html>`))
	}))
	defer server.Close()

	var out map[string]any
	_, err := (&Client{BaseURL: server.URL}).Do(context.Background(), http.MethodGet, "x", nil, &out)
	if code := domainError(t, err).Code; code != domain.CodeAPIError {
		t.Errorf("code %s", code)
	}
}

func TestDoNetworkErrors(t *testing.T) {
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	_, err := (&Client{BaseURL: closed.URL}).Do(context.Background(), http.MethodGet, "x", nil, nil)
	if code := domainError(t, err).Code; code != domain.CodeNetworkError {
		t.Errorf("closed server: code %s", code)
	}

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
	}))
	defer slow.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = (&Client{BaseURL: slow.URL}).Do(ctx, http.MethodGet, "x", nil, nil)
	if got := domainError(t, err); got.Code != domain.CodeNetworkError || !strings.Contains(got.Message, "deadline") {
		t.Errorf("timeout: %s %q", got.Code, got.Message)
	}
}

func TestDoAbsoluteURLs(t *testing.T) {
	otherHit := false
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { otherHit = true }))
	defer other.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer api.Close()

	client := &Client{BaseURL: api.URL + "/2.0", Authorization: BearerAuth("secret")}
	if _, err := client.Do(context.Background(), http.MethodGet, api.URL+"/2.0/page?page=2", nil, nil); err != nil {
		t.Errorf("same host: %v", err)
	}
	_, err := client.Do(context.Background(), http.MethodGet, other.URL+"/steal", nil, nil)
	if code := domainError(t, err).Code; code != domain.CodeAPIError || otherHit {
		t.Errorf("other host: code %s, request sent: %v", code, otherHit)
	}
}
