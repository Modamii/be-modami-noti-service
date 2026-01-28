package redis

import (
	"strings"

	config "techinsights-auth-api/configs"
)

type CacheKeys struct {
	prefix string
}

func NewCacheKeys(cfg *config.Config) *CacheKeys {
	return &CacheKeys{
		prefix: cfg.App.Environment,
	}
}

func (k *CacheKeys) buildKey(components ...string) string {
	allComponents := append([]string{k.prefix}, components...)
	return strings.Join(allComponents, ":")
}

func (k *CacheKeys) GetAccountByID(accountID string) string {
	return k.buildKey(KeyAccountInfo, accountID)
}

func (k *CacheKeys) GetAccountByEmail(email string) string {
	return k.buildKey(KeyAccountEmail, email)
}
