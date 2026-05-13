package setup

type Config struct {
	Kind              string
	TargetInput       string
	TargetPath        string
	PackageManager    string
	UserIDType        string
	HMACSecret        string
	DatabaseURL       string
	RedisURL          string
	ClickhouseURL     string
	AppURL            string
	SentryDSN         string
	DodoLiveAPIKey    string
	DodoTestAPIKey    string
	DodoProductID     string
	DodoWebhookSecret string
}

type Result struct {
	TargetPath string
	APIKey     string
	APIKeyName string
	UsedPM     string
}
