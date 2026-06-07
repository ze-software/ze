// Package ci provides CI/CD integration functionality.
package ci

import (
	"os"
	"strconv"
	"strings"
)

// Config represents CI-specific configuration.
type Config struct {
	Mode       string
	PRNumber   int
	BaseRef    string
	HeadRef    string
	Repository string
	Actor      string
	EventName  string
	Workspace  string
}

// QualityGate represents quality gate configuration.
type QualityGate struct {
	Enabled           bool    `json:"enabled"`
	MinMutationScore  float64 `json:"minMutationScore"`
	MaxSurvivors      int     `json:"maxSurvivors"`
	FailOnQualityGate bool    `json:"failOnQualityGate"`
}

// NotificationConfig represents notification configuration.
type NotificationConfig struct {
	Enabled bool         `json:"enabled"`
	Slack   SlackConfig  `json:"slack"`
	GitHub  GitHubConfig `json:"github"`
}

// SlackConfig represents Slack notification configuration.
type SlackConfig struct {
	Enabled    bool   `json:"enabled"`
	WebhookURL string `json:"webhookUrl"`
}

// GitHubConfig represents GitHub notification configuration.
type GitHubConfig struct {
	Enabled    bool `json:"enabled"`
	PRComments bool `json:"prComments"`
}

// LoadConfigFromEnv creates a new CI config from environment variables.
func LoadConfigFromEnv() *Config {
	return &Config{
		Mode:       getEnv("CI_MODE", "pr"),
		PRNumber:   getEnvInt("GITHUB_PR_NUMBER", 0),
		BaseRef:    getEnv("GITHUB_BASE_REF", "main"),
		HeadRef:    getEnv("GITHUB_HEAD_REF", ""),
		Repository: getEnv("GITHUB_REPOSITORY", ""),
		Actor:      getEnv("GITHUB_ACTOR", ""),
		EventName:  getEnv("GITHUB_EVENT_NAME", ""),
		Workspace:  getEnv("GITHUB_WORKSPACE", "."),
	}
}

// NewConfigFromEnv creates a new CI config from environment variables.
func NewConfigFromEnv() *Config {
	return LoadConfigFromEnv()
}

// IsCIMode returns true if running in CI mode.
func (c *Config) IsCIMode() bool {
	mode := strings.ToLower(c.Mode)

	return mode == "true" || mode == "1" || mode == "on" || mode == "yes"
}

// IsPullRequest returns true if this is a pull request event.
func (c *Config) IsPullRequest() bool {
	return c.EventName == "pull_request" && c.PRNumber > 0
}

// GetBaseBranch returns the base branch for comparison.
func (c *Config) GetBaseBranch() string {
	if c.BaseRef != "" {
		return c.BaseRef
	}

	return "main"
}

// getEnv gets environment variable with default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return defaultValue
}

// getEnvInt gets environment variable as integer with default value.
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}

	return defaultValue
}
