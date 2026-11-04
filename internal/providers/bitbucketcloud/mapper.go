package bitbucketcloud

import (
	"slices"
	"strconv"

	"github.com/lehtaku/alpr/internal/domain"
)

// host is the web host of Bitbucket Cloud repositories.
const host = "bitbucket.org"

func mapUser(u apiUser) domain.User {
	user := domain.User{ID: u.UUID, DisplayName: u.DisplayName}
	if u.Nickname != "" {
		user.Username = &u.Nickname
	}
	return user
}

func mapPullRequest(pr apiPullRequest, comments []apiComment) domain.PullRequest {
	return domain.PullRequest{
		PullRequestSummary: domain.PullRequestSummary{
			Repo:         host + "/" + pr.Destination.Repository.FullName,
			ID:           strconv.Itoa(pr.ID),
			URL:          pr.Links.HTML.Href,
			Title:        pr.Title,
			State:        mapState(pr.State),
			Draft:        pr.Draft,
			Author:       mapUser(pr.Author),
			SourceBranch: pr.Source.Branch.Name,
			TargetBranch: pr.Destination.Branch.Name,
			HeadCommit:   pr.Source.Commit.Hash,
			CreatedAt:    domain.FormatTime(pr.CreatedOn),
			UpdatedAt:    domain.FormatTime(pr.UpdatedOn),
		},
		Description: pr.Summary.Raw,
		Reviewers:   mapReviewers(pr.Reviewers, pr.Participants),
		Threads:     mapThreads(comments),
	}
}

// mapState maps OPEN and MERGED; DECLINED and SUPERSEDED are closed.
func mapState(state string) domain.PullRequestState {
	switch state {
	case "OPEN":
		return domain.PullRequestOpen
	case "MERGED":
		return domain.PullRequestMerged
	}
	return domain.PullRequestClosed
}

// mapReviewers lists the requested reviewers, then other participants who approved or requested changes.
func mapReviewers(reviewers []apiUser, participants []apiParticipant) []domain.Reviewer {
	result := []domain.Reviewer{}
	index := map[string]int{}
	add := func(u apiUser) int {
		if i, ok := index[u.UUID]; ok {
			return i
		}
		index[u.UUID] = len(result)
		result = append(result, domain.Reviewer{User: mapUser(u), Status: domain.ReviewNone})
		return len(result) - 1
	}

	for _, u := range reviewers {
		add(u)
	}
	for _, p := range participants {
		status := participantStatus(p)
		if p.Role == "REVIEWER" || status != domain.ReviewNone {
			result[add(p.User)].Status = status
		}
	}
	return result
}

func participantStatus(p apiParticipant) domain.ReviewStatus {
	switch {
	case p.State != nil && *p.State == "changes_requested":
		return domain.ReviewChangesRequested
	case p.Approved || p.State != nil && *p.State == "approved":
		return domain.ReviewApproved
	}
	return domain.ReviewNone
}

// mapThreads groups comments under their top-level comment. Deleted comments are left out but still hold their
// replies' thread; pending comments are unpublished drafts and are left out.
func mapThreads(comments []apiComment) []domain.Thread {
	byID := make(map[int64]apiComment, len(comments))
	for _, c := range comments {
		byID[c.ID] = c
	}
	root := func(c apiComment) apiComment {
		for range len(comments) {
			if c.Parent == nil {
				break
			}
			parent, ok := byID[c.Parent.ID]
			if !ok {
				break
			}
			c = parent
		}
		return c
	}

	threads := []domain.Thread{}
	index := map[int64]int{}
	for _, c := range comments {
		if c.Pending {
			continue
		}
		top := root(c)
		i, ok := index[top.ID]
		if !ok {
			i = len(threads)
			index[top.ID] = i
			threads = append(threads, newThread(top))
		}
		if !c.Deleted {
			threads[i].Comments = append(threads[i].Comments, mapComment(c))
		}
	}
	return slices.DeleteFunc(threads, func(t domain.Thread) bool { return len(t.Comments) == 0 })
}

// newThread describes the thread that top starts. Bitbucket doesn't report whether a thread is outdated.
func newThread(top apiComment) domain.Thread {
	thread := domain.Thread{ID: strconv.FormatInt(top.ID, 10), Kind: domain.ThreadGeneral, Comments: []domain.Comment{}}
	resolved := top.Resolution != nil
	if top.Inline != nil {
		thread.Kind = domain.ThreadInline
		thread.Resolved = &resolved
		thread.Location = mapLocation(*top.Inline)
	} else if resolved {
		// Whether general threads can be resolved is unconfirmed, so they report null unless they are.
		thread.Resolved = &resolved
	}
	return thread
}

// mapLocation prefers the new side; a comment on a removed line has only old-side numbers.
func mapLocation(in apiInline) *domain.Location {
	location := &domain.Location{Path: in.Path, Side: domain.SideNew, Line: in.To, StartLine: in.StartTo}
	if in.To == nil && in.From != nil {
		location.Side, location.Line, location.StartLine = domain.SideOld, in.From, in.StartFrom
	}
	return location
}

func mapComment(c apiComment) domain.Comment {
	comment := domain.Comment{
		ID:        strconv.FormatInt(c.ID, 10),
		Author:    mapUser(c.User),
		Body:      c.Content.Raw,
		CreatedAt: domain.FormatTime(c.CreatedOn),
		UpdatedAt: domain.FormatTime(c.UpdatedOn),
	}
	if href := c.Links.HTML.Href; href != "" {
		comment.URL = &href
	}
	return comment
}
