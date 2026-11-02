package bitbucketcloud

import "github.com/lehtaku/alpr/internal/domain"

func mapUser(u apiUser) domain.User {
	user := domain.User{ID: u.UUID, DisplayName: u.DisplayName}
	if u.Nickname != "" {
		user.Username = &u.Nickname
	}
	return user
}
