package setup

import "time"

const (
	GitHubZipURL        = "https://codeload.github.com/ScrawnDotDev/Scrawn/zip/refs/heads/main"
	DockerComposeURL    = "https://raw.githubusercontent.com/ScrawnDotDev/Scrawn/refs/heads/main/docker-compose.yml"
	DockerComposeFileName = "scrawn.docker-compose.yml"
	DefaultHTTPURL      = "http://127.0.0.1:8070/"
	GRPCPort            = "8069"
	ServerTimeout       = 25 * time.Second
)
