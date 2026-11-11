// Package github is the GitHub adapter.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lehtaku/alpr/internal/domain"
	"github.com/lehtaku/alpr/internal/transport"
)

// DefaultBaseURL is the GitHub REST API root.
const DefaultBaseURL = "https://api.github.com"

type client struct {
	http       *transport.Client
	graphqlURL string
}

// apiUser is a user object, as returned by GET /user.
type apiUser struct {
	ID    int64   `json:"id"`
	Login string  `json:"login"`
	Name  *string `json:"name"`
}

// apiSimpleUser is a user embedded in other objects; it carries no name.
type apiSimpleUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

type apiRef struct {
	Ref  string `json:"ref"`
	SHA  string `json:"sha"`
	Repo struct {
		FullName string `json:"full_name"`
	} `json:"repo"`
}

type apiPullRequest struct {
	Number             int             `json:"number"`
	HTMLURL            string          `json:"html_url"`
	Title              string          `json:"title"`
	Body               *string         `json:"body"`
	State              string          `json:"state"` // open or closed
	Merged             bool            `json:"merged"`
	Draft              bool            `json:"draft"`
	User               *apiSimpleUser  `json:"user"`
	Head               apiRef          `json:"head"`
	Base               apiRef          `json:"base"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	RequestedReviewers []apiSimpleUser `json:"requested_reviewers"`
}

type apiReview struct {
	ID          int64          `json:"id"`
	User        *apiSimpleUser `json:"user"`
	Body        string         `json:"body"`
	State       string         `json:"state"` // APPROVED, CHANGES_REQUESTED, COMMENTED, DISMISSED or PENDING
	HTMLURL     string         `json:"html_url"`
	SubmittedAt *time.Time     `json:"submitted_at"`
}

// apiIssueComment is a comment in the pull request's conversation.
type apiIssueComment struct {
	ID        int64          `json:"id"`
	User      *apiSimpleUser `json:"user"`
	Body      string         `json:"body"`
	HTMLURL   string         `json:"html_url"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type apiPageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

// apiActor is a GraphQL Actor; DatabaseID is set for users and bots.
type apiActor struct {
	Login      string `json:"login"`
	DatabaseID *int64 `json:"databaseId"`
}

type apiReviewComment struct {
	FullDatabaseID string    `json:"fullDatabaseId"` // BigInt, encoded as a string
	State          string    `json:"state"`          // PENDING or SUBMITTED
	Author         *apiActor `json:"author"`
	Body           string    `json:"body"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	URL            string    `json:"url"`
}

type apiReviewComments struct {
	PageInfo apiPageInfo        `json:"pageInfo"`
	Nodes    []apiReviewComment `json:"nodes"`
}

type apiReviewThread struct {
	ID         string            `json:"id"`
	IsResolved bool              `json:"isResolved"`
	IsOutdated bool              `json:"isOutdated"`
	Path       string            `json:"path"`
	Line       *int              `json:"line"`
	StartLine  *int              `json:"startLine"`
	DiffSide   string            `json:"diffSide"` // LEFT (old) or RIGHT (new)
	Comments   apiReviewComments `json:"comments"`
}

const reviewCommentFields = `pageInfo { hasNextPage endCursor }
nodes {
  fullDatabaseId state body createdAt updatedAt url
  author { login ... on User { databaseId } ... on Bot { databaseId } }
}`

const reviewThreadsQuery = `query($owner: String!, $repo: String!, $number: Int!, $cursor: String) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      reviewThreads(first: 100, after: $cursor) {
        pageInfo { hasNextPage endCursor }
        nodes {
          id isResolved isOutdated path line startLine diffSide
          comments(first: 100) { ` + reviewCommentFields + ` }
        }
      }
    }
  }
}`

const threadCommentsQuery = `query($id: ID!, $cursor: String) {
  node(id: $id) {
    ... on PullRequestReviewThread {
      comments(first: 100, after: $cursor) { ` + reviewCommentFields + ` }
    }
  }
}`

func newClient(baseURL, token string) *client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &client{
		http: &transport.Client{
			BaseURL:       baseURL,
			Authorization: transport.BearerAuth(token),
			Headers:       map[string]string{"Accept": "application/vnd.github+json"},
		},
		graphqlURL: graphqlURL(baseURL),
	}
}

// graphqlURL maps the REST root to the GraphQL endpoint: https://<host>/api/v3 → https://<host>/api/graphql on
// GitHub Enterprise Server, otherwise <root>/graphql.
func graphqlURL(baseURL string) string {
	root := strings.TrimSuffix(baseURL, "/")
	if prefix, ok := strings.CutSuffix(root, "/api/v3"); ok {
		return prefix + "/api/graphql"
	}
	return root + "/graphql"
}

func (c *client) currentUser(ctx context.Context) (apiUser, error) {
	var user apiUser
	_, err := c.http.Do(ctx, http.MethodGet, "user", nil, &user)
	return user, err
}

func repoPath(owner, repo string) string {
	return fmt.Sprintf("repos/%s/%s", url.PathEscape(owner), url.PathEscape(repo))
}

func (c *client) pullRequest(ctx context.Context, owner, repo string, number int) (apiPullRequest, error) {
	var pr apiPullRequest
	_, err := c.http.Do(ctx, http.MethodGet, fmt.Sprintf("%s/pulls/%d", repoPath(owner, repo), number), nil, &pr)
	return pr, err
}

// reviews returns every review of a pull request, oldest first.
func (c *client) reviews(ctx context.Context, owner, repo string, number int) ([]apiReview, error) {
	var reviews []apiReview
	err := transport.Pages(fmt.Sprintf("%s/pulls/%d/reviews?per_page=100", repoPath(owner, repo), number), func(path string) (string, error) {
		var page []apiReview
		header, err := c.http.Do(ctx, http.MethodGet, path, nil, &page)
		if err != nil {
			return "", err
		}
		reviews = append(reviews, page...)
		return transport.LinkNext(header), nil
	})
	return reviews, err
}

// issueComments returns the pull request's conversation comments, oldest first.
func (c *client) issueComments(ctx context.Context, owner, repo string, number int) ([]apiIssueComment, error) {
	var comments []apiIssueComment
	err := transport.Pages(fmt.Sprintf("%s/issues/%d/comments?per_page=100", repoPath(owner, repo), number), func(path string) (string, error) {
		var page []apiIssueComment
		header, err := c.http.Do(ctx, http.MethodGet, path, nil, &page)
		if err != nil {
			return "", err
		}
		comments = append(comments, page...)
		return transport.LinkNext(header), nil
	})
	return comments, err
}

// reviewThreads returns every review thread of a pull request with all of its comments.
func (c *client) reviewThreads(ctx context.Context, owner, repo string, number int) ([]apiReviewThread, error) {
	var threads []apiReviewThread
	// The page "path" is the GraphQL cursor; the first page has none.
	err := transport.Pages("", func(cursor string) (string, error) {
		var data struct {
			Repository *struct {
				PullRequest *struct {
					ReviewThreads struct {
						PageInfo apiPageInfo       `json:"pageInfo"`
						Nodes    []apiReviewThread `json:"nodes"`
					} `json:"reviewThreads"`
				} `json:"pullRequest"`
			} `json:"repository"`
		}
		variables := map[string]any{"owner": owner, "repo": repo, "number": number, "cursor": nullable(cursor)}
		if err := c.graphql(ctx, reviewThreadsQuery, variables, &data); err != nil {
			return "", err
		}
		if data.Repository == nil || data.Repository.PullRequest == nil {
			return "", domain.Errorf(domain.CodeNotFound, "pull request %d not found in %s/%s", number, owner, repo)
		}
		page := data.Repository.PullRequest.ReviewThreads
		threads = append(threads, page.Nodes...)
		return nextCursor(page.PageInfo), nil
	})
	if err != nil {
		return nil, err
	}

	for i := range threads {
		if err := c.remainingComments(ctx, &threads[i]); err != nil {
			return nil, err
		}
	}
	return threads, nil
}

// remainingComments appends the comments of a thread that didn't fit in the first page.
func (c *client) remainingComments(ctx context.Context, thread *apiReviewThread) error {
	start := nextCursor(thread.Comments.PageInfo)
	if start == "" {
		return nil
	}
	return transport.Pages(start, func(cursor string) (string, error) {
		var data struct {
			Node struct {
				Comments apiReviewComments `json:"comments"`
			} `json:"node"`
		}
		if err := c.graphql(ctx, threadCommentsQuery, map[string]any{"id": thread.ID, "cursor": cursor}, &data); err != nil {
			return "", err
		}
		thread.Comments.Nodes = append(thread.Comments.Nodes, data.Node.Comments.Nodes...)
		return nextCursor(data.Node.Comments.PageInfo), nil
	})
}

// graphql sends a query and decodes its data into out. GraphQL reports failures in an errors list with HTTP 200.
func (c *client) graphql(ctx context.Context, query string, variables map[string]any, out any) error {
	var resp struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	body := map[string]any{"query": query, "variables": variables}
	if _, err := c.http.Do(ctx, http.MethodPost, c.graphqlURL, body, &resp); err != nil {
		return err
	}
	if len(resp.Errors) > 0 {
		return domain.Errorf(domain.CodeAPIError, "POST %s: %s", c.graphqlURL, resp.Errors[0].Message)
	}
	if err := json.Unmarshal(resp.Data, out); err != nil {
		return domain.Errorf(domain.CodeAPIError, "POST %s: unexpected response: %v", c.graphqlURL, err)
	}
	return nil
}

func nextCursor(page apiPageInfo) string {
	if !page.HasNextPage {
		return ""
	}
	return page.EndCursor
}

// nullable turns an empty cursor into a GraphQL null.
func nullable(cursor string) any {
	if cursor == "" {
		return nil
	}
	return cursor
}
