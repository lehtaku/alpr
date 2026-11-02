package transport

import (
	"net/http"
	"strings"

	"github.com/lehtaku/alpr/internal/domain"
)

// maxPages stops a listing whose next-page links never end.
const maxPages = 100

// Pages calls fetch with path, then with each next-page path that fetch returns, until it returns "".
func Pages(path string, fetch func(path string) (next string, err error)) error {
	for range maxPages {
		next, err := fetch(path)
		if err != nil || next == "" {
			return err
		}
		path = next
	}
	return domain.Errorf(domain.CodeAPIError, "stopped after %d pages; the listing didn't end", maxPages)
}

// LinkNext returns the rel="next" URL from an RFC 8288 Link header, as GitHub sends it, or "".
func LinkNext(header http.Header) string {
	for _, link := range strings.Split(strings.Join(header.Values("Link"), ","), ",") {
		target, params, ok := strings.Cut(link, ";")
		if !ok {
			continue
		}
		for _, param := range strings.Split(params, ";") {
			if strings.TrimSpace(param) == `rel="next"` {
				return strings.Trim(strings.TrimSpace(target), "<>")
			}
		}
	}
	return ""
}
