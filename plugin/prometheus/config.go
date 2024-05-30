package prometheus

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

func GetConfig(address *string, basicUsername, basicPassword, oAuth2ClientID, oAuth2ClientSecret, oAuth2TokenURL *string, oAuth2Scopes []string) Config {
	cfg := Config{
		Address:  "http://localhost:9090",
		AuthType: PromAuthTypeNone,
	}

	if address != nil {
		cfg.Address = *address
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

	return cfg
}
