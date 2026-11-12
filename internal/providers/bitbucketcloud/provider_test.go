package bitbucketcloud

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/lehtaku/alpr/internal/domain"
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

const pullRequestJSON = `{
  "id": 42, "title": "Add parser", "summary": {"raw": "Adds the **parser**."},
  "state": "DECLINED", "draft": true,
  "author": {"uuid": "{a}", "nickname": "author", "display_name": "Author Person"},
  "source": {"branch": {"name": "feature/parser"}, "commit": {"hash": "abc123def456"}, "repository": {"full_name": "fork/repo"}},
  "destination": {"branch": {"name": "main"}, "commit": {"hash": "fff000fff000"}, "repository": {"full_name": "ws/repo"}},
  "created_on": "2026-10-30T08:00:00.123456+00:00", "updated_on": "2026-10-31T09:15:00.999999+02:00",
  "links": {"html": {"href": "https://bitbucket.org/ws/repo/pull-requests/42"}},
  "reviewers": [{"uuid": "{ann}", "nickname": "ann", "display_name": "Ann"}, {"uuid": "{bob}", "nickname": "bob", "display_name": "Bob"}],
  "participants": [
    {"user": {"uuid": "{ann}", "nickname": "ann", "display_name": "Ann"}, "role": "REVIEWER", "approved": true, "state": "approved"},
    {"user": {"uuid": "{bob}", "nickname": "bob", "display_name": "Bob"}, "role": "REVIEWER", "approved": false, "state": "changes_requested"},
    {"user": {"uuid": "{cid}", "display_name": "Cid"}, "role": "PARTICIPANT", "approved": false, "state": null},
    {"user": {"uuid": "{dee}", "display_name": "Dee"}, "role": "PARTICIPANT", "approved": true, "state": "approved"}
  ]
}`

const commentsPage2JSON = `{"values": [
  {"id": 4, "parent": {"id": 3}, "content": {"raw": "Thanks."}, "user": {"uuid": "{ann}", "display_name": "Ann"},
   "created_on": "2026-10-31T10:03:00+00:00", "updated_on": "2026-10-31T10:03:00+00:00"},
  {"id": 5, "deleted": true, "inline": {"path": "src/b.go", "from": 7, "to": null}, "content": {"raw": ""},
   "user": {"uuid": "{bob}", "display_name": "Bob"}, "created_on": "2026-10-31T10:04:00+00:00", "updated_on": "2026-10-31T10:04:00+00:00"},
  {"id": 6, "parent": {"id": 5}, "content": {"raw": "Why removed?"}, "user": {"uuid": "{ann}", "display_name": "Ann"},
   "created_on": "2026-10-31T10:05:00+00:00", "updated_on": "2026-10-31T10:05:00+00:00"},
  {"id": 7, "pending": true, "content": {"raw": "draft"}, "user": {"uuid": "{ann}", "display_name": "Ann"},
   "created_on": "2026-10-31T10:06:00+00:00", "updated_on": "2026-10-31T10:06:00+00:00"},
  {"id": 8, "deleted": true, "content": {"raw": ""}, "user": {"uuid": "{ann}", "display_name": "Ann"},
   "created_on": "2026-10-31T10:07:00+00:00", "updated_on": "2026-10-31T10:07:00+00:00"}
]}`

func commentsPage1JSON(next string) string {
	return `{"next": "` + next + `", "values": [
  {"id": 1, "inline": {"path": "src/a.go", "from": null, "to": 12, "start_from": null, "start_to": 10},
   "resolution": {"type": "comment_resolution"}, "content": {"raw": "Rename this."},
   "user": {"uuid": "{ann}", "nickname": "ann", "display_name": "Ann"},
   "links": {"html": {"href": "https://bitbucket.org/ws/repo/pull-requests/42/_/diff#comment-1"}},
   "created_on": "2026-10-31T10:00:00+00:00", "updated_on": "2026-10-31T10:30:00+00:00"},
  {"id": 2, "content": {"raw": "Looks off."}, "user": {"uuid": "{bob}", "display_name": "Bob"},
   "created_on": "2026-10-31T10:01:00+00:00", "updated_on": "2026-10-31T10:01:00+00:00"},
  {"id": 3, "parent": {"id": 1}, "content": {"raw": "Done."}, "user": {"uuid": "{a}", "display_name": "Author Person"},
   "created_on": "2026-10-31T10:02:00+00:00", "updated_on": "2026-10-31T10:02:00+00:00"}
]}`
}

func TestPullRequest(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			http.Error(w, "unexpected request", http.StatusTeapot)
			return
		}
		const prPath = "/2.0/repositories/ws/repo/pullrequests/42"
		switch {
		case r.URL.Path == prPath:
			_, _ = w.Write([]byte(pullRequestJSON))
		case r.URL.Path == prPath+"/comments" && r.URL.Query().Get("page") == "2":
			_, _ = w.Write([]byte(commentsPage2JSON))
		case r.URL.Path == prPath+"/comments" && r.URL.Query().Get("pagelen") == "100":
			_, _ = w.Write([]byte(commentsPage1JSON(server.URL + prPath + "/comments?pagelen=100&page=2")))
		default:
			http.Error(w, "unexpected request", http.StatusTeapot)
		}
	}))
	defer server.Close()

	pr, err := New(server.URL+"/2.0", "secret").PullRequest(context.Background(), "ws", "repo", "42")
	if err != nil {
		t.Fatal(err)
	}

	summary := pr.PullRequestSummary
	if summary.Repo != "bitbucket.org/ws/repo" || summary.ID != "42" || summary.State != domain.PullRequestClosed || !summary.Draft ||
		summary.SourceBranch != "feature/parser" || summary.TargetBranch != "main" || summary.HeadCommit != "abc123def456" ||
		summary.CreatedAt != "2026-10-30T08:00:00Z" || summary.UpdatedAt != "2026-10-31T07:15:00Z" ||
		summary.URL != "https://bitbucket.org/ws/repo/pull-requests/42" || *summary.Author.Username != "author" {
		t.Errorf("summary %+v", summary)
	}
	if pr.Description != "Adds the **parser**." {
		t.Errorf("description %q", pr.Description)
	}

	var reviewers []string
	for _, r := range pr.Reviewers {
		reviewers = append(reviewers, r.User.DisplayName+":"+string(r.Status))
	}
	if got := strings.Join(reviewers, " "); got != "Ann:approved Bob:changes_requested Dee:approved" {
		t.Errorf("reviewers %s", got)
	}

	if len(pr.Threads) != 3 {
		t.Fatalf("threads %+v", pr.Threads)
	}
	first, general, removed := pr.Threads[0], pr.Threads[1], pr.Threads[2]
	if first.ID != "1" || first.Kind != domain.ThreadInline || !*first.Resolved || first.Outdated != nil ||
		first.Location.Path != "src/a.go" || first.Location.Side != domain.SideNew || *first.Location.Line != 12 || *first.Location.StartLine != 10 || commentIDs(first) != "1 3 4" {
		t.Errorf("inline thread %+v, location %+v", first, first.Location)
	}
	if c := first.Comments[0]; c.Body != "Rename this." || c.URL == nil || c.CreatedAt != "2026-10-31T10:00:00Z" || c.UpdatedAt != "2026-10-31T10:30:00Z" {
		t.Errorf("first comment %+v", c)
	}
	if general.ID != "2" || general.Kind != domain.ThreadGeneral || general.Resolved != nil || general.Location != nil ||
		commentIDs(general) != "2" || general.Comments[0].URL != nil {
		t.Errorf("general thread %+v", general)
	}
	if removed.ID != "5" || *removed.Resolved || removed.Location.Side != domain.SideOld || *removed.Location.Line != 7 ||
		removed.Location.StartLine != nil || commentIDs(removed) != "6" {
		t.Errorf("thread of a deleted comment %+v, location %+v", removed, removed.Location)
	}
}

func commentIDs(thread domain.Thread) string {
	ids := make([]string, 0, len(thread.Comments))
	for _, c := range thread.Comments {
		ids = append(ids, c.ID)
	}
	return strings.Join(ids, " ")
}

func TestMapState(t *testing.T) {
	for state, want := range map[string]domain.PullRequestState{
		"OPEN": domain.PullRequestOpen, "MERGED": domain.PullRequestMerged,
		"DECLINED": domain.PullRequestClosed, "SUPERSEDED": domain.PullRequestClosed,
	} {
		if got := mapState(state); got != want {
			t.Errorf("%s: %s, want %s", state, got, want)
		}
	}
}

func listedPullRequestJSON(id int, updated string) string {
	return fmt.Sprintf(`{"id": %d, "title": "PR %d", "state": "OPEN", "draft": false,
  "author": {"uuid": "{a}", "nickname": "author", "display_name": "Author Person"},
  "source": {"branch": {"name": "feature/%d"}, "commit": {"hash": "abc%d"}},
  "destination": {"branch": {"name": "main"}, "repository": {"full_name": "ws/repo"}},
  "created_on": "2026-10-30T08:00:00+00:00", "updated_on": "%s",
  "links": {"html": {"href": "https://bitbucket.org/ws/repo/pull-requests/%d"}}}`, id, id, id, id, updated, id)
}

func TestPullRequests(t *testing.T) {
	tests := []struct {
		name, reviewer, wantQuery string
	}{
		{"all", "", `state = "OPEN"`},
		{"@me", "@me", `state = "OPEN" AND reviewers.uuid = "{me}"`},
		{"account UUID", "{0a1b-2c3d}", `state = "OPEN" AND reviewers.uuid = "{0a1b-2c3d}"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				const listPath = "/2.0/repositories/ws/repo/pullrequests"
				query := r.URL.Query()
				switch {
				case r.URL.Path == "/2.0/user":
					_, _ = w.Write([]byte(`{"uuid": "{me}", "nickname": "me", "display_name": "Me"}`))
				case r.URL.Path != listPath || query.Get("q") != tt.wantQuery || query.Get("sort") != "-updated_on" ||
					query.Get("pagelen") != "3" || query.Has("state"):
					http.Error(w, "unexpected request", http.StatusTeapot)
				case query.Get("page") == "2":
					_, _ = w.Write([]byte(`{"values": [` + listedPullRequestJSON(3, "2026-10-31T07:00:00+00:00") + `,` +
						listedPullRequestJSON(4, "2026-10-31T06:00:00+00:00") + `]}`))
				default:
					next := server.URL + listPath + "?" + query.Encode() + "&page=2"
					_, _ = w.Write([]byte(`{"next": "` + next + `", "values": [` + listedPullRequestJSON(1, "2026-10-31T09:00:00+00:00") + `,` +
						listedPullRequestJSON(2, "2026-10-31T08:00:00+00:00") + `]}`))
				}
			}))
			defer server.Close()

			prs, err := New(server.URL+"/2.0", "secret").PullRequests(context.Background(), "ws", "repo", tt.reviewer, 3)
			if err != nil {
				t.Fatal(err)
			}
			var ids []string
			for _, pr := range prs {
				ids = append(ids, pr.ID)
			}
			if got := strings.Join(ids, " "); got != "1 2 3" {
				t.Fatalf("ids %s", got)
			}
			if pr := prs[0]; pr.Repo != "bitbucket.org/ws/repo" || pr.State != domain.PullRequestOpen || pr.SourceBranch != "feature/1" ||
				pr.HeadCommit != "abc1" || pr.UpdatedAt != "2026-10-31T09:00:00Z" || pr.Author.DisplayName != "Author Person" {
				t.Errorf("first pull request %+v", pr)
			}
		})
	}
}

func TestPullRequestsRejectsReviewer(t *testing.T) {
	_, err := New("http://127.0.0.1:1", "secret").PullRequests(context.Background(), "ws", "repo", `x" OR state = "MERGED`, 50)
	var domainErr *domain.Error
	if !errors.As(err, &domainErr) || domainErr.Code != domain.CodeUsage || !strings.Contains(domainErr.Message, "account UUID") {
		t.Errorf("error %v", err)
	}
}

func TestPullRequestsIn(t *testing.T) {
	var listed []string
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case path == "/2.0/repositories/ws" && r.URL.Query().Get("fields") == "next,values.slug":
			_, _ = w.Write([]byte(`{"values": [{"slug": "api"}, {"slug": "Team-A"}, {"slug": "team-b"}, {"slug": "team-c"}]}`))
		case strings.HasSuffix(path, "/pullrequests"):
			mu.Lock()
			listed = append(listed, path)
			mu.Unlock()
			switch path {
			case "/2.0/repositories/ws/Team-A/pullrequests":
				_, _ = w.Write([]byte(`{"values": [` + listedPullRequestJSON(1, "2026-10-31T09:00:00+00:00") + `]}`))
			case "/2.0/repositories/ws/team-b/pullrequests":
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"type": "error", "error": {"message": "Access denied"}}`))
			default:
				_, _ = w.Write([]byte(`{"values": []}`))
			}
		default:
			http.Error(w, "unexpected request", http.StatusTeapot)
		}
	}))
	defer server.Close()

	prs, items, err := New(server.URL+"/2.0", "secret").PullRequestsIn(context.Background(), "ws", "team-*", "", 50)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(listed)
	if got := strings.Join(listed, " "); got != "/2.0/repositories/ws/Team-A/pullrequests /2.0/repositories/ws/team-b/pullrequests /2.0/repositories/ws/team-c/pullrequests" {
		t.Errorf("listed %s", got)
	}
	if len(prs) != 1 || prs[0].ID != "1" {
		t.Errorf("pull requests %+v", prs)
	}
	if len(items) != 1 || items[0].Scope != "bitbucket.org/ws/team-b" || items[0].Code != domain.CodePermissionDenied {
		t.Errorf("error items %+v", items)
	}
}

func TestPullRequestsInSingleRepository(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/repositories/ws/api/pullrequests" {
			http.Error(w, "unexpected request", http.StatusTeapot)
			return
		}
		_, _ = w.Write([]byte(`{"values": [` + listedPullRequestJSON(7, "2026-10-31T09:00:00+00:00") + `]}`))
	}))
	defer server.Close()

	prs, items, err := New(server.URL+"/2.0", "secret").PullRequestsIn(context.Background(), "ws", "api", "", 50)
	if err != nil || len(items) != 0 || len(prs) != 1 || prs[0].ID != "7" {
		t.Errorf("pull requests %+v, items %+v, error %v", prs, items, err)
	}
}

func TestPullRequestsInListingFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"type": "error", "error": {"message": "Workspace not found"}}`))
	}))
	defer server.Close()

	_, _, err := New(server.URL+"/2.0", "secret").PullRequestsIn(context.Background(), "nope", "*", "", 50)
	var domainErr *domain.Error
	if !errors.As(err, &domainErr) || domainErr.Code != domain.CodeNotFound {
		t.Errorf("error %v", err)
	}
}
