// Package transport sends JSON requests to platform APIs and turns failures into domain errors.
package transport

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lehtaku/alpr/internal/domain"
)

const (
	defaultTimeout = 30 * time.Second
	maxErrorBody   = 1 << 20
)

// Client sends requests to one API host.
type Client struct {
	BaseURL       string            // API root, e.g. https://api.github.com
	Authorization string            // Authorization header value, from BearerAuth or BasicAuth
	Headers       map[string]string // extra headers sent with every request
	HTTP          *http.Client      // nil means a client with a 30-second timeout
}

// BearerAuth returns an Authorization header value for a bearer token.
func BearerAuth(token string) string {
	return "Bearer " + token
}

// BasicAuth returns an Authorization header value for Basic auth.
func BasicAuth(username, token string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+token))
}

// Do sends a request and returns the response headers. A non-nil body is sent as JSON, and a JSON response is
// decoded into a non-nil out. path is relative to BaseURL, or an absolute URL on the same host (a next-page link).
func (c *Client) Do(ctx context.Context, method, path string, body, out any) (http.Header, error) {
	target, err := c.resolve(path)
	if err != nil {
		return nil, err
	}

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, domain.Errorf(domain.CodeInternal, "encoding the request body: %v", err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, domain.Errorf(domain.CodeInternal, "building the request: %v", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "alpr")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range c.Headers {
		req.Header.Set(key, value)
	}
	if c.Authorization != "" {
		req.Header.Set("Authorization", c.Authorization)
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return nil, domain.Errorf(domain.CodeNetworkError, "%s %s: %v", method, target, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		return nil, statusError(method, target, resp)
	}
	if out != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return nil, domain.Errorf(domain.CodeAPIError, "%s %s: unexpected response: %v", method, target, err)
		}
	}
	return resp.Header, nil
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: defaultTimeout}
}

// resolve builds the request URL. Absolute URLs must point at the API host so the token never goes elsewhere.
func (c *Client) resolve(path string) (string, error) {
	if !strings.HasPrefix(path, "https://") && !strings.HasPrefix(path, "http://") {
		return strings.TrimSuffix(c.BaseURL, "/") + "/" + strings.TrimPrefix(path, "/"), nil
	}
	target, err := url.Parse(path)
	base, baseErr := url.Parse(c.BaseURL)
	if err != nil || baseErr != nil || target.Scheme != base.Scheme || target.Host != base.Host {
		return "", domain.Errorf(domain.CodeAPIError, "refusing to send credentials to %q: it isn't on the API host", path)
	}
	return path, nil
}

func statusError(method, target string, resp *http.Response) *domain.Error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	message := fmt.Sprintf("%s %s: HTTP %s", method, target, resp.Status)
	if apiMsg := apiMessage(data); apiMsg != "" {
		message += ": " + apiMsg
	}
	details := map[string]any{"status": resp.StatusCode, "url": target}

	code := domain.CodeAPIError
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		code = domain.CodeAuthInvalid
	case http.StatusForbidden:
		code = domain.CodePermissionDenied
	case http.StatusNotFound:
		code = domain.CodeNotFound
	}
	if wait, limited := rateLimit(resp); limited {
		code = domain.CodeRateLimited
		if wait >= 0 {
			details["retry_after_seconds"] = wait
		}
	}
	return &domain.Error{Code: code, Message: message, Details: details}
}

// rateLimit reports whether a 403 or 429 response is a rate limit, and the seconds to wait (-1 if unknown).
// GitHub signals its limits with 403 plus Retry-After or X-RateLimit-Remaining: 0.
func rateLimit(resp *http.Response) (int, bool) {
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusTooManyRequests {
		return 0, false
	}
	if seconds, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil {
		return seconds, true
	}
	if resp.Header.Get("X-RateLimit-Remaining") == "0" {
		if reset, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
			return max(0, int(time.Until(time.Unix(reset, 0)).Seconds())), true
		}
		return -1, true
	}
	return -1, resp.StatusCode == http.StatusTooManyRequests
}

// apiMessage extracts the error text from GitHub, Bitbucket or Jira error bodies.
func apiMessage(body []byte) string {
	var fields map[string]any
	if json.Unmarshal(body, &fields) != nil {
		return ""
	}
	if msg, ok := fields["message"].(string); ok { // GitHub
		return msg
	}
	switch errField := fields["error"].(type) {
	case map[string]any: // Bitbucket
		if msg, ok := errField["message"].(string); ok {
			return msg
		}
	case string:
		return errField
	}
	if msgs, ok := fields["errorMessages"].([]any); ok && len(msgs) > 0 { // Jira
		if msg, ok := msgs[0].(string); ok {
			return msg
		}
	}
	return ""
}
