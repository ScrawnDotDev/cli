package setup

type Config struct {
	Kind           string
	TargetInput    string
	TargetPath     string
	PackageManager string
	UserIDType     string
	HMACSecret     string
	DatabaseURL    string
	RedisURL       string
	ClickhouseURL  string
	AppURL         string
	SentryDSN      string
}

type Result struct {
	TargetPath string
	APIKey     string
	APIKeyName string
	UsedPM     string
}
