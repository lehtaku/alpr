package github

import (
	"strconv"

	"github.com/lehtaku/alpr/internal/domain"
)

func mapUser(u apiUser) domain.User {
	user := domain.User{ID: strconv.FormatInt(u.ID, 10), Username: &u.Login, DisplayName: u.Login}
	if u.Name != nil && *u.Name != "" {
		user.DisplayName = *u.Name
	}
	return user
}
