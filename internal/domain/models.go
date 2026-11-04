package domain

import "time"

// User is a person on a platform (spec §4.2).
type User struct {
	ID          string  `json:"id"`
	Username    *string `json:"username"`
	DisplayName string  `json:"display_name"`
}

// ReviewStatus is a reviewer's current verdict.
type ReviewStatus string

const (
	ReviewApproved         ReviewStatus = "approved"
	ReviewChangesRequested ReviewStatus = "changes_requested"
	ReviewNone             ReviewStatus = "none"
)

// Reviewer is a requested reviewer, or anyone who approved or requested changes.
type Reviewer struct {
	User   User         `json:"user"`
	Status ReviewStatus `json:"status"`
}

// Comment is one comment in a thread; Body is Markdown.
type Comment struct {
	ID        string  `json:"id"`
	Author    User    `json:"author"`
	Body      string  `json:"body"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
	URL       *string `json:"url"`
}

type ThreadKind string

const (
	ThreadInline  ThreadKind = "inline"
	ThreadGeneral ThreadKind = "general"
)

// Thread is a comment with its replies, oldest first. Resolved and Outdated are nil when the platform has no value.
type Thread struct {
	ID       string     `json:"id"`
	Kind     ThreadKind `json:"kind"`
	Resolved *bool      `json:"resolved"`
	Outdated *bool      `json:"outdated"`
	Location *Location  `json:"location"`
	Comments []Comment  `json:"comments"`
}

// Side is the version of a file a location refers to: new is the PR's source, old its target.
type Side string

const (
	SideNew Side = "new"
	SideOld Side = "old"
)

// Location is where an inline thread is anchored; Line is the last line of the range.
type Location struct {
	Path      string `json:"path"`
	Side      Side   `json:"side"`
	Line      *int   `json:"line"`
	StartLine *int   `json:"start_line"`
}

type PullRequestState string

const (
	PullRequestOpen   PullRequestState = "open"
	PullRequestMerged PullRequestState = "merged"
	PullRequestClosed PullRequestState = "closed"
)

// PullRequestSummary is a pull request without its description, reviewers and threads.
type PullRequestSummary struct {
	Repo         string           `json:"repo"` // host/owner/repo with the platform's own host
	ID           string           `json:"id"`
	URL          string           `json:"url"`
	Title        string           `json:"title"`
	State        PullRequestState `json:"state"`
	Draft        bool             `json:"draft"`
	Author       User             `json:"author"`
	SourceBranch string           `json:"source_branch"`
	TargetBranch string           `json:"target_branch"`
	HeadCommit   string           `json:"head_commit"`
	CreatedAt    string           `json:"created_at"`
	UpdatedAt    string           `json:"updated_at"`
}

// PullRequest is a pull request with its reviewers and comment threads.
type PullRequest struct {
	PullRequestSummary
	Description string     `json:"description"`
	Reviewers   []Reviewer `json:"reviewers"`
	Threads     []Thread   `json:"threads"`
}

// FormatTime formats t as a UTC RFC 3339 timestamp with second precision.
func FormatTime(t time.Time) string {
	return t.UTC().Truncate(time.Second).Format(time.RFC3339)
}
