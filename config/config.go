package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	viper *viper.Viper
}

func (c *Config) GetSlackToken() string {
	return c.viper.GetString("slack.token")
}

func (c *Config) GetSlackSigningSecret() string {
	return c.viper.GetString("slack.signing_secret")
}

func (c *Config) GetDBUsername() string {
	return c.viper.GetString("db.username")
}

func (c *Config) GetDBPassword() string {
	return c.viper.GetString("db.password")
}

func (c *Config) GetDBName() string {
	return c.viper.GetString("db.name")
}

func (c *Config) GetDBInstance() string {
	return c.viper.GetString("db.host")
}

func LoadConfig() (*Config, error) {
	v := viper.New()

	config := Config{
		viper: v,
	}

	v.SetEnvPrefix("BBP_")

	_ = v.BindEnv("slack.token", "BBP_SLACK_TOKEN")
	_ = v.BindEnv("slack.signing_secret", "BBP_SLACK_SIGNING_SECRET")

	_ = v.BindEnv("db.username", "BBP_DB_USERNAME")
	_ = v.BindEnv("db.password", "BBP_DB_PASSWORD")
	_ = v.BindEnv("db.name", "BBP_DB_NAME")
	_ = v.BindEnv("db.host", "BBP_HOST")

	return &config, nil
}
