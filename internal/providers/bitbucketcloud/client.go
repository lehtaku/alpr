// Package bitbucketcloud is the Bitbucket Cloud adapter.
package bitbucketcloud

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/lehtaku/alpr/internal/transport"
)

// DefaultBaseURL is the Bitbucket Cloud REST API root.
const DefaultBaseURL = "https://api.bitbucket.org/2.0"

type client struct {
	http *transport.Client
}

// apiUser is an account object, as returned by GET /user.
type apiUser struct {
	UUID        string `json:"uuid"`
	Nickname    string `json:"nickname"`
	DisplayName string `json:"display_name"`
}

type apiLinks struct {
	HTML struct {
		Href string `json:"href"`
	} `json:"html"`
}

type apiEndpoint struct {
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Branch struct {
		Name string `json:"name"`
	} `json:"branch"`
	Commit struct {
		Hash string `json:"hash"`
	} `json:"commit"`
}

type apiParticipant struct {
	User     apiUser `json:"user"`
	Role     string  `json:"role"` // PARTICIPANT or REVIEWER
	Approved bool    `json:"approved"`
	State    *string `json:"state"` // approved, changes_requested or null
}

// apiPullRequest is a pull request as GET {pr} returns it; only that endpoint includes reviewers and participants.
type apiPullRequest struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Summary struct {
		Raw string `json:"raw"`
	} `json:"summary"`
	State        string           `json:"state"`
	Draft        bool             `json:"draft"`
	Author       apiUser          `json:"author"`
	Source       apiEndpoint      `json:"source"`
	Destination  apiEndpoint      `json:"destination"`
	CreatedOn    time.Time        `json:"created_on"`
	UpdatedOn    time.Time        `json:"updated_on"`
	Links        apiLinks         `json:"links"`
	Reviewers    []apiUser        `json:"reviewers"`
	Participants []apiParticipant `json:"participants"`
}

type apiInline struct {
	Path      string `json:"path"`
	From      *int   `json:"from"` // old side
	To        *int   `json:"to"`   // new side
	StartFrom *int   `json:"start_from"`
	StartTo   *int   `json:"start_to"`
}

type apiComment struct {
	ID        int64     `json:"id"`
	CreatedOn time.Time `json:"created_on"`
	UpdatedOn time.Time `json:"updated_on"`
	Content   struct {
		Raw string `json:"raw"`
	} `json:"content"`
	User    apiUser `json:"user"`
	Deleted bool    `json:"deleted"`
	Pending bool    `json:"pending"`
	Parent  *struct {
		ID int64 `json:"id"`
	} `json:"parent"`
	Inline     *apiInline `json:"inline"`
	Resolution *struct {
		Type string `json:"type"`
	} `json:"resolution"`
	Links apiLinks `json:"links"`
}

func newClient(baseURL, token string) *client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	// API tokens work as bearer tokens, so no account email is needed.
	return &client{http: &transport.Client{BaseURL: baseURL, Authorization: transport.BearerAuth(token)}}
}

func (c *client) currentUser(ctx context.Context) (apiUser, error) {
	var user apiUser
	_, err := c.http.Do(ctx, http.MethodGet, "user", nil, &user)
	return user, err
}

func pullRequestPath(workspace, slug, id string) string {
	return fmt.Sprintf("repositories/%s/%s/pullrequests/%s", url.PathEscape(workspace), url.PathEscape(slug), url.PathEscape(id))
}

func (c *client) pullRequest(ctx context.Context, workspace, slug, id string) (apiPullRequest, error) {
	var pr apiPullRequest
	_, err := c.http.Do(ctx, http.MethodGet, pullRequestPath(workspace, slug, id), nil, &pr)
	return pr, err
}

// comments returns every comment of a pull request, oldest first, including deleted ones and replies.
func (c *client) comments(ctx context.Context, workspace, slug, id string) ([]apiComment, error) {
	var comments []apiComment
	err := transport.Pages(pullRequestPath(workspace, slug, id)+"/comments?pagelen=100", func(path string) (string, error) {
		var page struct {
			Values []apiComment `json:"values"`
			Next   string       `json:"next"`
		}
		if _, err := c.http.Do(ctx, http.MethodGet, path, nil, &page); err != nil {
			return "", err
		}
		comments = append(comments, page.Values...)
		return page.Next, nil
	})
	return comments, err
}
