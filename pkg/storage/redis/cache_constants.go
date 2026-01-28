package redis

import "time"

const (
	KeyAccountInfo   = "account_info"
	KeyAccountEmail  = "account_email"
)

const (
	KEY_ACCOUNT_INFO_EXPIRE  = 24 * time.Hour
	KEY_ACCOUNT_EMAIL_EXPIRE = 24 * time.Hour
)
