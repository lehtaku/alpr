package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lehtaku/alpr/internal/domain"
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

const pullRequestJSON = `{
  "number": 7, "html_url": "https://github.com/o/r/pull/7", "title": "Fix race", "body": null,
  "state": "closed", "merged": true, "draft": false,
  "user": {"id": 1, "login": "author"},
  "head": {"ref": "fix/race", "sha": "0123456789abcdef0123456789abcdef01234567", "repo": null},
  "base": {"ref": "main", "sha": "fff", "repo": {"full_name": "o/r"}},
  "created_at": "2026-10-30T08:00:00Z", "updated_at": "2026-10-31T09:15:00Z",
  "requested_reviewers": [{"id": 2, "login": "ann"}, {"id": 5, "login": "eve"}]
}`

const reviewsPage2JSON = `[
  {"id": 102, "user": {"id": 3, "login": "bob"}, "body": "", "state": "APPROVED", "submitted_at": "2026-10-31T11:00:00Z"},
  {"id": 103, "user": {"id": 4, "login": "cid"}, "body": "", "state": "DISMISSED", "submitted_at": "2026-10-31T11:01:00Z"},
  {"id": 104, "user": null, "body": "", "state": "APPROVED", "submitted_at": "2026-10-31T11:02:00Z"},
  {"id": 105, "user": {"id": 2, "login": "ann"}, "body": "draft summary", "state": "PENDING"}
]`

const reviewsPage1JSON = `[
  {"id": 100, "user": {"id": 2, "login": "ann"}, "body": "", "state": "COMMENTED", "submitted_at": "2026-10-31T10:00:00Z"},
  {"id": 101, "user": {"id": 3, "login": "bob"}, "body": "Please fix the lock.", "state": "CHANGES_REQUESTED",
   "html_url": "https://github.com/o/r/pull/7#pullrequestreview-101", "submitted_at": "2026-10-31T10:05:00Z"}
]`

const issueCommentsJSON = `[
  {"id": 200, "user": {"id": 1, "login": "author"}, "body": "Ready for review.", "html_url": "https://github.com/o/r/pull/7#issuecomment-200",
   "created_at": "2026-10-31T09:00:00Z", "updated_at": "2026-10-31T09:30:00Z"},
  {"id": 201, "user": null, "body": "Old note.", "created_at": "2026-10-31T12:00:00Z", "updated_at": "2026-10-31T12:00:00Z"}
]`

const threadsPage1JSON = `{"data": {"repository": {"pullRequest": {"reviewThreads": {
  "pageInfo": {"hasNextPage": true, "endCursor": "T1"},
  "nodes": [
    {"id": "PRRT_a", "isResolved": true, "isOutdated": false, "path": "src/lock.go", "line": 20, "startLine": 18, "diffSide": "RIGHT",
     "comments": {"pageInfo": {"hasNextPage": false, "endCursor": "x"}, "nodes": [
       {"fullDatabaseId": "300", "state": "SUBMITTED", "author": {"login": "ann", "databaseId": 2}, "body": "Hold the lock.",
        "createdAt": "2026-10-31T10:01:00Z", "updatedAt": "2026-10-31T10:02:00Z", "url": "https://github.com/o/r/pull/7#discussion_r300"},
       {"fullDatabaseId": "301", "state": "SUBMITTED", "author": {"login": "author", "databaseId": 1}, "body": "Done.",
        "createdAt": "2026-10-31T10:20:00Z", "updatedAt": "2026-10-31T10:20:00Z", "url": "https://github.com/o/r/pull/7#discussion_r301"}
     ]}},
    {"id": "PRRT_b", "isResolved": false, "isOutdated": true, "path": "src/old.go", "line": null, "startLine": null, "diffSide": "LEFT",
     "comments": {"pageInfo": {"hasNextPage": true, "endCursor": "C1"}, "nodes": [
       {"fullDatabaseId": "310", "state": "SUBMITTED", "author": {"login": "bob", "databaseId": 3}, "body": "Why removed?",
        "createdAt": "2026-10-31T10:04:00Z", "updatedAt": "2026-10-31T10:04:00Z", "url": "https://github.com/o/r/pull/7#discussion_r310"}
     ]}}
  ]
}}}}}`

const threadsPage2JSON = `{"data": {"repository": {"pullRequest": {"reviewThreads": {
  "pageInfo": {"hasNextPage": false, "endCursor": "T2"},
  "nodes": [
    {"id": "PRRT_c", "isResolved": false, "isOutdated": false, "path": "src/x.go", "line": 3, "startLine": null, "diffSide": "RIGHT",
     "comments": {"pageInfo": {"hasNextPage": false, "endCursor": "y"}, "nodes": [
       {"fullDatabaseId": "320", "state": "PENDING", "author": {"login": "ann", "databaseId": 2}, "body": "draft",
        "createdAt": "2026-10-31T10:40:00Z", "updatedAt": "2026-10-31T10:40:00Z", "url": "https://github.com/o/r/pull/7#discussion_r320"}
     ]}}
  ]
}}}}}`

const threadCommentsJSON = `{"data": {"node": {"comments": {
  "pageInfo": {"hasNextPage": false, "endCursor": "C2"},
  "nodes": [
    {"fullDatabaseId": "311", "state": "SUBMITTED", "author": null, "body": "Because.",
     "createdAt": "2026-10-31T10:30:00Z", "updatedAt": "2026-10-31T10:30:00Z", "url": "https://github.com/o/r/pull/7#discussion_r311"}
  ]
}}}}`

// githubServer serves pull request 7 of o/r over REST and GraphQL, with two pages of reviews and review threads.
func githubServer(t *testing.T) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			http.Error(w, "unexpected request", http.StatusTeapot)
			return
		}
		const prPath = "/repos/o/r/pulls/7"
		switch {
		case r.Method == http.MethodGet && r.URL.Path == prPath:
			_, _ = w.Write([]byte(pullRequestJSON))
		case r.URL.Path == prPath+"/reviews" && r.URL.Query().Get("page") == "2":
			_, _ = w.Write([]byte(reviewsPage2JSON))
		case r.URL.Path == prPath+"/reviews" && r.URL.Query().Get("per_page") == "100":
			w.Header().Set("Link", `<`+server.URL+prPath+`/reviews?per_page=100&page=2>; rel="next", <`+server.URL+prPath+`/reviews?per_page=100&page=2>; rel="last"`)
			_, _ = w.Write([]byte(reviewsPage1JSON))
		case r.URL.Path == "/repos/o/r/issues/7/comments" && r.URL.Query().Get("per_page") == "100":
			_, _ = w.Write([]byte(issueCommentsJSON))
		case r.Method == http.MethodPost && r.URL.Path == "/graphql":
			var req struct {
				Query     string         `json:"query"`
				Variables map[string]any `json:"variables"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			vars := req.Variables
			switch {
			case strings.Contains(req.Query, "node(id: $id)") && vars["id"] == "PRRT_b" && vars["cursor"] == "C1":
				_, _ = w.Write([]byte(threadCommentsJSON))
			case strings.Contains(req.Query, "reviewThreads") && vars["owner"] == "o" && vars["repo"] == "r" && vars["number"] == float64(7):
				switch vars["cursor"] {
				case nil:
					_, _ = w.Write([]byte(threadsPage1JSON))
				case "T1":
					_, _ = w.Write([]byte(threadsPage2JSON))
				default:
					http.Error(w, "unexpected cursor", http.StatusTeapot)
				}
			default:
				http.Error(w, "unexpected query", http.StatusTeapot)
			}
		default:
			http.Error(w, "unexpected request", http.StatusTeapot)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestPullRequest(t *testing.T) {
	server := githubServer(t)
	pr, err := New(server.URL, "secret").PullRequest(context.Background(), "o", "r", "7")
	if err != nil {
		t.Fatal(err)
	}

	summary := pr.PullRequestSummary
	if summary.Repo != "github.com/o/r" || summary.ID != "7" || summary.State != domain.PullRequestMerged || summary.Draft ||
		summary.SourceBranch != "fix/race" || summary.TargetBranch != "main" || summary.HeadCommit != "0123456789abcdef0123456789abcdef01234567" ||
		summary.CreatedAt != "2026-10-30T08:00:00Z" || summary.UpdatedAt != "2026-10-31T09:15:00Z" ||
		summary.URL != "https://github.com/o/r/pull/7" || summary.Author.ID != "1" || summary.Author.DisplayName != "author" {
		t.Errorf("summary %+v", summary)
	}
	if pr.Description != "" {
		t.Errorf("description %q", pr.Description)
	}

	var reviewers []string
	for _, r := range pr.Reviewers {
		reviewers = append(reviewers, r.User.DisplayName+":"+string(r.Status))
	}
	if got := strings.Join(reviewers, " "); got != "ann:none eve:none bob:approved cid:none" {
		t.Errorf("reviewers %s", got)
	}

	var threadIDs []string
	for _, thread := range pr.Threads {
		threadIDs = append(threadIDs, thread.ID)
	}
	if got := strings.Join(threadIDs, " "); got != "200 300 310 101 201" {
		t.Fatalf("thread IDs %s", got)
	}
	conversation, resolved, outdated, summaryThread, ghostComment := pr.Threads[0], pr.Threads[1], pr.Threads[2], pr.Threads[3], pr.Threads[4]
	if conversation.Kind != domain.ThreadGeneral || conversation.Resolved != nil || conversation.Outdated != nil || conversation.Location != nil ||
		conversation.Comments[0].UpdatedAt != "2026-10-31T09:30:00Z" || *conversation.Comments[0].URL != "https://github.com/o/r/pull/7#issuecomment-200" {
		t.Errorf("conversation thread %+v", conversation)
	}
	if resolved.Kind != domain.ThreadInline || !*resolved.Resolved || *resolved.Outdated || resolved.Location.Path != "src/lock.go" ||
		resolved.Location.Side != domain.SideNew || *resolved.Location.Line != 20 || *resolved.Location.StartLine != 18 || commentIDs(resolved) != "300 301" {
		t.Errorf("resolved thread %+v, location %+v", resolved, resolved.Location)
	}
	if c := resolved.Comments[0]; c.Author.ID != "2" || *c.Author.Username != "ann" || c.Body != "Hold the lock." || c.UpdatedAt != "2026-10-31T10:02:00Z" ||
		*c.URL != "https://github.com/o/r/pull/7#discussion_r300" {
		t.Errorf("inline comment %+v", c)
	}
	if *outdated.Resolved || !*outdated.Outdated || outdated.Location.Side != domain.SideOld || outdated.Location.Line != nil ||
		outdated.Location.StartLine != nil || commentIDs(outdated) != "310 311" || outdated.Comments[1].Author.DisplayName != "ghost" {
		t.Errorf("outdated thread %+v, location %+v", outdated, outdated.Location)
	}
	if c := summaryThread.Comments[0]; summaryThread.Kind != domain.ThreadGeneral || summaryThread.Resolved != nil || c.Body != "Please fix the lock." ||
		c.CreatedAt != "2026-10-31T10:05:00Z" || c.UpdatedAt != c.CreatedAt || c.Author.DisplayName != "bob" {
		t.Errorf("review summary thread %+v", summaryThread)
	}
	if c := ghostComment.Comments[0]; c.Author.ID != "" || c.Author.Username != nil || c.Author.DisplayName != "ghost" || c.URL != nil {
		t.Errorf("comment of a deleted user %+v", c)
	}
}

func commentIDs(thread domain.Thread) string {
	ids := make([]string, 0, len(thread.Comments))
	for _, c := range thread.Comments {
		ids = append(ids, c.ID)
	}
	return strings.Join(ids, " ")
}

func TestPullRequestErrors(t *testing.T) {
	tests := []struct {
		name     string
		graphql  string
		wantCode domain.Code
		wantMsg  string
	}{
		{"GraphQL error", `{"data": null, "errors": [{"type": "FORBIDDEN", "message": "Resource not accessible by integration"}]}`,
			domain.CodeAPIError, "Resource not accessible by integration"},
		{"no pull request", `{"data": {"repository": {"pullRequest": null}}}`, domain.CodeNotFound, "pull request 7 not found in o/r"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/repos/o/r/pulls/7":
					_, _ = w.Write([]byte(pullRequestJSON))
				case "/repos/o/r/pulls/7/reviews", "/repos/o/r/issues/7/comments":
					_, _ = w.Write([]byte(`[]`))
				case "/graphql":
					_, _ = w.Write([]byte(tt.graphql))
				default:
					http.Error(w, "unexpected request", http.StatusTeapot)
				}
			}))
			defer server.Close()

			_, err := New(server.URL, "secret").PullRequest(context.Background(), "o", "r", "7")
			var domainErr *domain.Error
			if !errors.As(err, &domainErr) || domainErr.Code != tt.wantCode || !strings.Contains(domainErr.Message, tt.wantMsg) {
				t.Errorf("error %v", err)
			}
		})
	}
}

func TestGraphqlURL(t *testing.T) {
	for base, want := range map[string]string{
		"https://api.github.com":              "https://api.github.com/graphql",
		"https://github.example.com/api/v3":   "https://github.example.com/api/graphql",
		"https://github.example.com/api/v3/":  "https://github.example.com/api/graphql",
		"https://proxy.example.com/github/v3": "https://proxy.example.com/github/v3/graphql",
	} {
		if got := graphqlURL(base); got != want {
			t.Errorf("%s: %s, want %s", base, got, want)
		}
	}
}

func TestMapState(t *testing.T) {
	tests := []struct {
		state  string
		merged bool
		want   domain.PullRequestState
	}{
		{"open", false, domain.PullRequestOpen},
		{"closed", false, domain.PullRequestClosed},
		{"closed", true, domain.PullRequestMerged},
	}
	for _, tt := range tests {
		if got := mapState(tt.state, tt.merged); got != tt.want {
			t.Errorf("%s, merged %v: %s, want %s", tt.state, tt.merged, got, tt.want)
		}
	}
}

func searchNode(number int, state string, draft bool, updated string) string {
	return fmt.Sprintf(`{"number": %d, "url": "https://github.com/o/r/pull/%d", "title": "PR %d", "state": %q, "isDraft": %v,
  "headRefName": "feature/%d", "baseRefName": "main", "headRefOid": "sha%d", "createdAt": "2026-10-30T08:00:00Z", "updatedAt": %q,
  "author": {"login": "author", "databaseId": 1}, "repository": {"nameWithOwner": "o/r"}}`, number, number, number, state, draft, number, number, updated)
}

func TestPullRequests(t *testing.T) {
	tests := []struct {
		name, reviewer, wantQuery string
	}{
		{"all", "", "repo:o/r is:pr is:open sort:updated-desc"},
		{"@me", "@me", "repo:o/r is:pr is:open sort:updated-desc review-involves:@me"},
		{"login", "some-one", "repo:o/r is:pr is:open sort:updated-desc review-involves:some-one"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Query     string         `json:"query"`
					Variables map[string]any `json:"variables"`
				}
				if r.URL.Path != "/graphql" || json.NewDecoder(r.Body).Decode(&req) != nil ||
					!strings.Contains(req.Query, "search(query: $query, type: ISSUE") || req.Variables["query"] != tt.wantQuery {
					http.Error(w, "unexpected request", http.StatusTeapot)
					return
				}
				switch vars := req.Variables; {
				case vars["cursor"] == nil && vars["first"] == float64(3):
					_, _ = w.Write([]byte(`{"data": {"search": {"pageInfo": {"hasNextPage": true, "endCursor": "S1"}, "nodes": [` +
						searchNode(1, "OPEN", true, "2026-10-31T09:00:00Z") + `,` + searchNode(2, "OPEN", false, "2026-10-31T08:00:00Z") + `]}}}`))
				case vars["cursor"] == "S1" && vars["first"] == float64(1):
					_, _ = w.Write([]byte(`{"data": {"search": {"pageInfo": {"hasNextPage": true, "endCursor": "S2"}, "nodes": [` +
						searchNode(3, "OPEN", false, "2026-10-31T07:00:00Z") + `]}}}`))
				default:
					http.Error(w, "unexpected page", http.StatusTeapot)
				}
			}))
			defer server.Close()

			prs, err := New(server.URL, "secret").PullRequests(context.Background(), "o", "r", tt.reviewer, 3)
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
			if pr := prs[0]; pr.Repo != "github.com/o/r" || pr.State != domain.PullRequestOpen || !pr.Draft || pr.SourceBranch != "feature/1" ||
				pr.TargetBranch != "main" || pr.HeadCommit != "sha1" || pr.UpdatedAt != "2026-10-31T09:00:00Z" ||
				pr.Author.ID != "1" || pr.Author.DisplayName != "author" {
				t.Errorf("first pull request %+v", pr)
			}
		})
	}
}

func TestPullRequestsRejectsReviewer(t *testing.T) {
	_, err := New("http://127.0.0.1:1", "secret").PullRequests(context.Background(), "o", "r", "x repo:other/repo", 50)
	var domainErr *domain.Error
	if !errors.As(err, &domainErr) || domainErr.Code != domain.CodeUsage || !strings.Contains(domainErr.Message, "@me or a login") {
		t.Errorf("error %v", err)
	}
}

func TestMapSearchPullRequestState(t *testing.T) {
	for state, want := range map[string]domain.PullRequestState{
		"OPEN": domain.PullRequestOpen, "CLOSED": domain.PullRequestClosed, "MERGED": domain.PullRequestMerged,
	} {
		if got := mapSearchPullRequest(apiSearchPullRequest{State: state}).State; got != want {
			t.Errorf("%s: %s, want %s", state, got, want)
		}
	}
}
