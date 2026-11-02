package jira

import "github.com/lehtaku/alpr/internal/domain"

// mapUser leaves Username empty: Jira Cloud users have no public username.
func mapUser(u apiUser) domain.User {
	return domain.User{ID: u.AccountID, DisplayName: u.DisplayName}
}
