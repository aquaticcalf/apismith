package cognito

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// AuthFlow selects how tokens are obtained from Cognito.
type AuthFlow string

const (
	AuthFlowSRP      AuthFlow = "srp"
	AuthFlowPassword AuthFlow = "password"
)

// Config is the Cognito client configuration used by both the JWT CLI and the
// API Test Console. The JSON field names match jwt-token-printer's config.json
// so existing printer configs keep working.
type Config struct {
	UserPoolID   string   `json:"userPoolID" yaml:"userPoolID"`
	ClientID     string   `json:"clientID" yaml:"clientID"`
	ClientSecret string   `json:"clientSecret" yaml:"clientSecret"`
	Username     string   `json:"username" yaml:"username"`
	Password     string   `json:"password" yaml:"password"`
	Region       string   `json:"region" yaml:"region"`
	Endpoint     string   `json:"endpoint,omitempty" yaml:"endpoint"`
	AuthFlow     AuthFlow `json:"authFlow,omitempty" yaml:"authFlow"`
}

// LoadFile reads a JSON or YAML credentials file. JSON is the jwt-token-printer
// format; YAML is the console's credentials.yaml format.
func LoadFile(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read credentials %s: %w", path, err)
	}
	cfg := &Config{}
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, "{") {
		if err := json.Unmarshal(raw, cfg); err != nil {
			return nil, fmt.Errorf("parse credentials JSON: %w", err)
		}
	} else {
		if err := yaml.Unmarshal(raw, cfg); err != nil {
			return nil, fmt.Errorf("parse credentials YAML: %w", err)
		}
	}
	cfg.Normalize()
	return cfg, nil
}

// Normalize trims fields and applies defaults.
func (c *Config) Normalize() {
	if c == nil {
		return
	}
	c.UserPoolID = strings.TrimSpace(c.UserPoolID)
	c.ClientID = strings.TrimSpace(c.ClientID)
	c.ClientSecret = strings.TrimSpace(c.ClientSecret)
	c.Username = strings.TrimSpace(c.Username)
	// Password is deliberately not trimmed: passwords may contain
	// significant leading or trailing spaces.
	c.Region = strings.TrimSpace(c.Region)
	c.Endpoint = strings.TrimSpace(c.Endpoint)
	c.AuthFlow = AuthFlow(strings.ToLower(strings.TrimSpace(string(c.AuthFlow))))
	if c.AuthFlow == "" {
		c.AuthFlow = AuthFlowSRP
	}
	if c.Region == "" {
		c.Region = "ap-south-1"
	}
}

// Validate checks the fields required to talk to Cognito.
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("cognito config is nil")
	}
	c.Normalize()
	switch {
	case c.UserPoolID == "":
		return fmt.Errorf("user pool id is required")
	case c.ClientID == "":
		return fmt.Errorf("client id is required")
	case c.Username == "":
		return fmt.Errorf("username is required")
	case c.Password == "":
		return fmt.Errorf("password is required")
	case c.AuthFlow != AuthFlowSRP && c.AuthFlow != AuthFlowPassword:
		return fmt.Errorf("authFlow must be %q or %q", AuthFlowSRP, AuthFlowPassword)
	}
	return nil
}

// PublicClientID returns the app client id, suitable for showing in the UI.
func (c *Config) PublicClientID() string {
	if c == nil {
		return ""
	}
	return c.ClientID
}
