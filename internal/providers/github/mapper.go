package github

import (
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lehtaku/alpr/internal/domain"
)

// ghost is shown for users whose account was deleted; the API returns them as null.
const ghost = "ghost"

func mapUser(u apiUser) domain.User {
	user := domain.User{ID: strconv.FormatInt(u.ID, 10), Username: &u.Login, DisplayName: u.Login}
	if u.Name != nil && *u.Name != "" {
		user.DisplayName = *u.Name
	}
	return user
}

// mapSimpleUser shows the login as the display name, since embedded user objects carry no name.
func mapSimpleUser(u *apiSimpleUser) domain.User {
	if u == nil {
		return domain.User{DisplayName: ghost}
	}
	return domain.User{ID: strconv.FormatInt(u.ID, 10), Username: &u.Login, DisplayName: u.Login}
}

func mapActor(a *apiActor) domain.User {
	if a == nil {
		return domain.User{DisplayName: ghost}
	}
	user := domain.User{Username: &a.Login, DisplayName: a.Login}
	if a.DatabaseID != nil {
		user.ID = strconv.FormatInt(*a.DatabaseID, 10)
	}
	return user
}

func mapPullRequest(pr apiPullRequest, reviews []apiReview, comments []apiIssueComment, threads []apiReviewThread) domain.PullRequest {
	description := ""
	if pr.Body != nil {
		description = *pr.Body
	}
	return domain.PullRequest{
		PullRequestSummary: domain.PullRequestSummary{
			Repo:         webHost(pr.HTMLURL) + "/" + pr.Base.Repo.FullName,
			ID:           strconv.Itoa(pr.Number),
			URL:          pr.HTMLURL,
			Title:        pr.Title,
			State:        mapState(pr.State, pr.Merged),
			Draft:        pr.Draft,
			Author:       mapSimpleUser(pr.User),
			SourceBranch: pr.Head.Ref,
			TargetBranch: pr.Base.Ref,
			HeadCommit:   pr.Head.SHA,
			CreatedAt:    domain.FormatTime(pr.CreatedAt),
			UpdatedAt:    domain.FormatTime(pr.UpdatedAt),
		},
		Description: description,
		Reviewers:   mapReviewers(pr.RequestedReviewers, reviews),
		Threads:     mapThreads(reviews, comments, threads),
	}
}

func mapSearchPullRequest(pr apiSearchPullRequest) domain.PullRequestSummary {
	return domain.PullRequestSummary{
		Repo:         webHost(pr.URL) + "/" + pr.Repository.NameWithOwner,
		ID:           strconv.Itoa(pr.Number),
		URL:          pr.URL,
		Title:        pr.Title,
		State:        mapState(strings.ToLower(pr.State), pr.State == "MERGED"),
		Draft:        pr.IsDraft,
		Author:       mapActor(pr.Author),
		SourceBranch: pr.HeadRefName,
		TargetBranch: pr.BaseRefName,
		HeadCommit:   pr.HeadRefOid,
		CreatedAt:    domain.FormatTime(pr.CreatedAt),
		UpdatedAt:    domain.FormatTime(pr.UpdatedAt),
	}
}

// webHost is github.com, or the GitHub Enterprise Server host, taken from the pull request's web URL.
func webHost(htmlURL string) string {
	u, err := url.Parse(htmlURL)
	if err != nil {
		return ""
	}
	return u.Host
}

func mapState(state string, merged bool) domain.PullRequestState {
	switch {
	case merged:
		return domain.PullRequestMerged
	case state == "open":
		return domain.PullRequestOpen
	}
	return domain.PullRequestClosed
}

// mapReviewers lists the requested reviewers, then other users who approved or requested changes. A user's latest
// approving, changes-requested or dismissed review sets the status; dismissed is none.
func mapReviewers(requested []apiSimpleUser, reviews []apiReview) []domain.Reviewer {
	result := []domain.Reviewer{}
	index := map[int64]int{}
	add := func(u apiSimpleUser) int {
		if i, ok := index[u.ID]; ok {
			return i
		}
		index[u.ID] = len(result)
		result = append(result, domain.Reviewer{User: mapSimpleUser(&u), Status: domain.ReviewNone})
		return len(result) - 1
	}

	for _, u := range requested {
		add(u)
	}
	for _, r := range reviews {
		if r.User == nil {
			continue
		}
		switch r.State {
		case "APPROVED":
			result[add(*r.User)].Status = domain.ReviewApproved
		case "CHANGES_REQUESTED":
			result[add(*r.User)].Status = domain.ReviewChangesRequested
		case "DISMISSED":
			result[add(*r.User)].Status = domain.ReviewNone
		}
	}
	return result
}

// mapThreads returns review threads, review summaries and conversation comments, ordered by their first comment.
// Review summaries and conversation comments are one-comment general threads that can't be resolved.
func mapThreads(reviews []apiReview, comments []apiIssueComment, threads []apiReviewThread) []domain.Thread {
	result := []domain.Thread{}
	for _, t := range threads {
		if thread, ok := mapReviewThread(t); ok {
			result = append(result, thread)
		}
	}
	for _, r := range reviews {
		if r.Body == "" || r.State == "PENDING" || r.SubmittedAt == nil {
			continue
		}
		result = append(result, generalThread(r.ID, r.User, r.Body, r.HTMLURL, *r.SubmittedAt, *r.SubmittedAt))
	}
	for _, c := range comments {
		result = append(result, generalThread(c.ID, c.User, c.Body, c.HTMLURL, c.CreatedAt, c.UpdatedAt))
	}
	slices.SortStableFunc(result, func(a, b domain.Thread) int {
		return strings.Compare(a.Comments[0].CreatedAt, b.Comments[0].CreatedAt)
	})
	return result
}

// mapReviewThread leaves out comments of pending reviews, which are unpublished drafts; ok is false if none remain.
// The thread's ID is its first comment's ID.
func mapReviewThread(t apiReviewThread) (domain.Thread, bool) {
	comments := []domain.Comment{}
	for _, c := range t.Comments.Nodes {
		if c.State == "PENDING" {
			continue
		}
		comment := domain.Comment{
			ID:        c.FullDatabaseID,
			Author:    mapActor(c.Author),
			Body:      c.Body,
			CreatedAt: domain.FormatTime(c.CreatedAt),
			UpdatedAt: domain.FormatTime(c.UpdatedAt),
		}
		if c.URL != "" {
			comment.URL = &c.URL
		}
		comments = append(comments, comment)
	}
	if len(comments) == 0 {
		return domain.Thread{}, false
	}

	side := domain.SideNew
	if t.DiffSide == "LEFT" {
		side = domain.SideOld
	}
	return domain.Thread{
		ID:       comments[0].ID,
		Kind:     domain.ThreadInline,
		Resolved: &t.IsResolved,
		Outdated: &t.IsOutdated,
		Location: &domain.Location{Path: t.Path, Side: side, Line: t.Line, StartLine: t.StartLine},
		Comments: comments,
	}, true
}

func generalThread(id int64, author *apiSimpleUser, body, htmlURL string, createdAt, updatedAt time.Time) domain.Thread {
	comment := domain.Comment{
		ID:        strconv.FormatInt(id, 10),
		Author:    mapSimpleUser(author),
		Body:      body,
		CreatedAt: domain.FormatTime(createdAt),
		UpdatedAt: domain.FormatTime(updatedAt),
	}
	if htmlURL != "" {
		comment.URL = &htmlURL
	}
	return domain.Thread{ID: comment.ID, Kind: domain.ThreadGeneral, Comments: []domain.Comment{comment}}
}
