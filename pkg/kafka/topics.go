package kafka

import config "techinsights-auth-api/configs"

const TopicUserRegistered = "techinsight.user.registered"

func GetTopicWithEnv(cfg *config.Config, topic string) string {
	env := cfg.App.Environment
	if env == "" {
		env = "local"
	}
	return env + "." + topic
}
