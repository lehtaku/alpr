package domain

// User is a person on a platform (spec §4.2).
type User struct {
	ID          string  `json:"id"`
	Username    *string `json:"username"`
	DisplayName string  `json:"display_name"`
}
