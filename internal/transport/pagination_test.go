package transport

import (
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/lehtaku/alpr/internal/domain"
)

func TestPages(t *testing.T) {
	next := map[string]string{"first": "second", "second": "third", "third": ""}
	var visited []string
	err := Pages("first", func(path string) (string, error) {
		visited = append(visited, path)
		return next[path], nil
	})
	if err != nil || !slices.Equal(visited, []string{"first", "second", "third"}) {
		t.Errorf("visited %v, err %v", visited, err)
	}
}

func TestPagesStopsOnError(t *testing.T) {
	failure := errors.New("boom")
	calls := 0
	err := Pages("first", func(string) (string, error) {
		calls++
		return "next", failure
	})
	if !errors.Is(err, failure) || calls != 1 {
		t.Errorf("err %v after %d calls", err, calls)
	}
}

func TestPagesStopsEndlessListing(t *testing.T) {
	calls := 0
	err := Pages("loop", func(string) (string, error) {
		calls++
		return "loop", nil
	})
	if code := domainError(t, err).Code; code != domain.CodeAPIError || calls != maxPages {
		t.Errorf("code %s after %d calls", code, calls)
	}
}

func TestLinkNext(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{"none", nil, ""},
		{"next and last", []string{`<https://api.github.com/x?page=2>; rel="next", <https://api.github.com/x?page=5>; rel="last"`}, "https://api.github.com/x?page=2"},
		{"next listed second", []string{`<https://api.github.com/x?page=1>; rel="prev", <https://api.github.com/x?page=3>; rel="next"`}, "https://api.github.com/x?page=3"},
		{"split over two headers", []string{`<https://api.github.com/x?page=1>; rel="prev"`, `<https://api.github.com/x?page=3>; rel="next"`}, "https://api.github.com/x?page=3"},
		{"last page", []string{`<https://api.github.com/x?page=1>; rel="first", <https://api.github.com/x?page=4>; rel="prev"`}, ""},
	}
	for _, tt := range tests {
		header := http.Header{"Link": tt.values}
		if got := LinkNext(header); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}
