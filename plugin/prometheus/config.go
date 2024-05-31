package prometheus

import (
	"context"
	kaytuKubernetes "github.com/kaytu-io/plugin-kubernetes/plugin/kubernetes"
	"time"
)

type PromAuthType string

const (
	PromAuthTypeNone   PromAuthType = "none"
	PromAuthTypeBasic               = "basic"
	PromAuthTypeOAuth2              = "oauth2"
)

type Config struct {
	Address string `json:"address"`

	AuthType PromAuthType `json:"authType"`

	// BasicAuth
	BasicUsername string `json:"basicUsername"`
	BasicPassword string `json:"basicPassword"`

	// OAuth2
	OAuth2ClientID     string   `json:"oAuth2ClientID"`
	OAuth2ClientSecret string   `json:"oAuth2ClientSecret"`
	OAuth2TokenURL     string   `json:"oAuth2TokenURL"`
	OAuth2Scopes       []string `json:"oAuth2Scopes"`
}

func GetConfig(address, basicUsername, basicPassword, oAuth2ClientID, oAuth2ClientSecret, oAuth2TokenURL *string, oAuth2Scopes []string, client *kaytuKubernetes.Kubernetes) (Config, error) {
	cfg := Config{
		AuthType: PromAuthTypeNone,
	}

	if address != nil {
		cfg.Address = *address
	}

	if cfg.Address == "" {
		_, err := client.DiscoverPrometheus(context.Background())
		if err != nil {
			return cfg, err
		}
		time.Sleep(1 * time.Second)
		cfg.Address = "http://localhost:9090"
	}

	if basicUsername != nil && basicPassword != nil {
		cfg.AuthType = PromAuthTypeBasic
		cfg.BasicUsername = *basicUsername
		cfg.BasicPassword = *basicPassword
	} else if oAuth2ClientID != nil && oAuth2ClientSecret != nil && oAuth2TokenURL != nil {
		cfg.AuthType = PromAuthTypeOAuth2
		cfg.OAuth2ClientID = *oAuth2ClientID
		cfg.OAuth2ClientSecret = *oAuth2ClientSecret
		cfg.OAuth2TokenURL = *oAuth2TokenURL
		cfg.OAuth2Scopes = oAuth2Scopes
	}

	return cfg, nil
}
