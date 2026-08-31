package globals

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Config is the fully resolved runtime configuration. Every field is populated
// from the environment (a .env file is loaded first, if present).
type Config struct {
	AuthenticationServerPort int
	SecureServerHost         string
	SecureServerPort         int

	PostgresURI string

	// AccessKey seeds every PRUDP packet signature. Unlike the other Protarium
	// Wii U servers, Lost Reavers' access key is NOT publicly documented (it is
	// absent from kinnay's NEX server list and Pretendo's nex-viewer access.txt,
	// and it does not appear as a bare string in the game's executable -- see
	// docs/ACCESS_KEY.md). There is deliberately no default: starting with a
	// wrong guess makes every console packet get silently discarded with no
	// error at all, which is a much worse failure than refusing to start.
	AccessKey string

	NEXVersionMajor int
	NEXVersionMinor int
	NEXVersionPatch int

	// S3 storage for DataStore object bodies (treasure/inventory blobs, replay
	// data, etc.). DataStore is confirmed used by Lost Reavers (extensive
	// nn::nex::DataStore* symbols in the executable), unlike Ultra Smash.
	S3Endpoint  string
	S3Region    string
	S3Bucket    string
	S3AccessKey string
	S3SecretKey string
	S3UseSSL    bool

	LocalAuth     bool
	SettingsFile  string
	AccountSecret string

	AccountGRPCHost   string
	AccountGRPCPort   int
	AccountGRPCAPIKey string

	FriendsGRPCHost   string
	FriendsGRPCPort   int
	FriendsGRPCAPIKey string

	VerboseRMC bool
	LogRMC     bool

	// LogMatchmaking dumps the GameMode, attributes and search criteria the
	// title sends. Nobody has published a capture of Lost Reavers' matchmaking
	// traffic, so this is the only way to find out what its four-player lobby
	// search actually asks for.
	LogMatchmaking bool

	HealthCheckPort int

	AllowPublicMatchmaking bool

	// TokenServerPort, when non-zero, runs a small local-only HTTP endpoint
	// answering GET /v1/api/provider/nex_token/@me?game_server_id=1018D900
	// with this server's own address. It exists to sit behind a reverse
	// proxy's path/query matcher (e.g. Caddy's `@lostreavers_token` block) so
	// that a real account server can keep handling everything else (OAuth
	// login, service tokens) for a title Pretendo's real account database has
	// no entry for -- see docs/ACCESS_KEY.md. Bind it to localhost only; it
	// answers unauthenticated and is meant to be reached exclusively through
	// the reverse proxy's path match, not directly from the internet.
	TokenServerPort int
	TokenServerHost string

	// NEXTokenPID / NEXTokenPassword are the standalone-mode NEX credentials
	// the token endpoint hands back. Must match an entry in SettingsFile.
	NEXTokenPID      uint64
	NEXTokenPassword string
}

var Settings Config

func envString(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}

	return fallback
}

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}

	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func envInt(key string, fallback int) (int, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback, nil
	}

	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s is not a valid integer, got %q", key, v)
	}

	return n, nil
}

func envPort(key string, fallback int) (int, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback, nil
	}

	port, err := strconv.Atoi(v)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("%s is not a valid port. Expected 1-65535, got %q", key, v)
	}

	return port, nil
}

func postgresDatabaseURI(rawURI, database string) string {
	if rawURI == "" {
		return ""
	}

	parsed, err := url.Parse(rawURI)
	if err != nil {
		return ""
	}

	parsed.Path = "/" + database
	return parsed.String()
}

// LoadConfig reads the environment into Settings and reports every problem it
// finds at once, rather than one error per run.
func LoadConfig() error {
	var problems []string

	authPort, err := envPort("PN_LR_AUTHENTICATION_SERVER_PORT", 60000)
	if err != nil {
		problems = append(problems, err.Error())
	}

	securePort, err := envPort("PN_LR_SECURE_SERVER_PORT", 60001)
	if err != nil {
		problems = append(problems, err.Error())
	}

	accountPort, err := envPort("PN_LR_ACCOUNT_GRPC_PORT", 0)
	if err != nil {
		problems = append(problems, err.Error())
	}

	friendsPort, err := envPort("PN_LR_FRIENDS_GRPC_PORT", 0)
	if err != nil {
		problems = append(problems, err.Error())
	}

	healthCheckPort, err := envPort("PN_LR_HEALTH_CHECK_PORT", 0)
	if err != nil {
		problems = append(problems, err.Error())
	}

	tokenServerPort, err := envPort("PN_LR_TOKEN_SERVER_PORT", 0)
	if err != nil {
		problems = append(problems, err.Error())
	}

	nexTokenPID, err := envInt("PN_LR_NEX_TOKEN_PID", 0)
	if err != nil {
		problems = append(problems, err.Error())
	}

	// * 3.10.0 is confirmed via kinnay's wiiu.json library fingerprint database
	// * -- see the comment on globals.go's game constants. Still overridable
	// * without a rebuild in case a packet capture ever shows otherwise.
	nexMajor, err := envInt("PN_LR_NEX_VERSION_MAJOR", 3)
	if err != nil {
		problems = append(problems, err.Error())
	}

	nexMinor, err := envInt("PN_LR_NEX_VERSION_MINOR", 10)
	if err != nil {
		problems = append(problems, err.Error())
	}

	nexPatch, err := envInt("PN_LR_NEX_VERSION_PATCH", 0)
	if err != nil {
		problems = append(problems, err.Error())
	}

	Settings = Config{
		AuthenticationServerPort: authPort,
		SecureServerHost:         envString("PN_LR_SECURE_SERVER_HOST", ""),
		SecureServerPort:         securePort,
		PostgresURI:              envString("PN_LR_POSTGRES_URI", postgresDatabaseURI(envString("PN_POSTGRES_URI", ""), "lostreavers")),
		AccessKey:                envString("PN_LR_ACCESS_KEY", ""),
		NEXVersionMajor:          nexMajor,
		NEXVersionMinor:          nexMinor,
		NEXVersionPatch:          nexPatch,
		S3Endpoint:               envString("PN_LR_S3_ENDPOINT", ""),
		S3Region:                 envString("PN_LR_S3_REGION", "us-east-1"),
		S3Bucket:                 envString("PN_LR_S3_BUCKET", "lost-reavers-datastore"),
		S3AccessKey:              envString("PN_LR_S3_ACCESS_KEY", ""),
		S3SecretKey:              envString("PN_LR_S3_SECRET_KEY", ""),
		S3UseSSL:                 envBool("PN_LR_S3_USE_SSL", false),
		LocalAuth:                envBool("PN_LR_LOCAL_AUTH", false),
		SettingsFile:             envString("PN_LR_SETTINGS_FILE", "settings.json"),
		AccountSecret:            envString("PN_LR_ACCOUNT_SECRET", envString("PN_NEX_PASSWORD_SECRET", "")),
		AccountGRPCHost:          envString("PN_LR_ACCOUNT_GRPC_HOST", ""),
		AccountGRPCPort:          accountPort,
		AccountGRPCAPIKey:        envString("PN_LR_ACCOUNT_GRPC_API_KEY", ""),
		FriendsGRPCHost:          envString("PN_LR_FRIENDS_GRPC_HOST", ""),
		FriendsGRPCPort:          friendsPort,
		FriendsGRPCAPIKey:        envString("PN_LR_FRIENDS_GRPC_API_KEY", ""),
		VerboseRMC:               envBool("PN_LR_VERBOSE_RMC", false),
		HealthCheckPort:          healthCheckPort,
		LogRMC:                   envBool("PN_LR_LOG_RMC", true),
		LogMatchmaking:           envBool("PN_LR_LOG_MATCHMAKING", true),
		AllowPublicMatchmaking:   envBool("PN_LR_ALLOW_PUBLIC_MATCHMAKING", true),
		TokenServerPort:          tokenServerPort,
		TokenServerHost:          envString("PN_LR_TOKEN_SERVER_HOST", "127.0.0.1"),
		NEXTokenPID:              uint64(nexTokenPID),
		NEXTokenPassword:         envString("PN_LR_NEX_TOKEN_PASSWORD", ""),
	}

	if Settings.TokenServerPort != 0 && (Settings.NEXTokenPID == 0 || Settings.NEXTokenPassword == "") {
		problems = append(problems, "PN_LR_TOKEN_SERVER_PORT is set but PN_LR_NEX_TOKEN_PID / PN_LR_NEX_TOKEN_PASSWORD are not -- "+
			"the token endpoint needs to know which standalone account to vouch for, and it must match an entry in settings.json")
	}

	if Settings.SecureServerHost == "" {
		problems = append(problems, "PN_LR_SECURE_SERVER_HOST environment variable not set. It must be the address the console can reach this machine on")
	}

	if Settings.PostgresURI == "" {
		problems = append(problems, "PN_LR_POSTGRES_URI environment variable not set. Example: postgres://lostreavers:lostreavers@localhost/lostreavers?sslmode=disable")
	}

	if Settings.AccessKey == "" {
		problems = append(problems, "PN_LR_ACCESS_KEY environment variable not set. Lost Reavers' access key is not publicly documented and there is no safe default -- "+
			"see docs/ACCESS_KEY.md for how to capture it. Do not guess: a wrong key makes every console packet fail silently")
	}

	if Settings.AuthenticationServerPort == Settings.SecureServerPort {
		problems = append(problems, "PN_LR_AUTHENTICATION_SERVER_PORT and PN_LR_SECURE_SERVER_PORT must differ")
	}

	if Settings.SecureServerHost == "0.0.0.0" || Settings.SecureServerHost == "::" {
		problems = append(problems, "PN_LR_SECURE_SERVER_HOST is a wildcard address. It must be an address the console can dial, such as your LAN IP")
	}

	if Settings.HealthCheckPort != 0 &&
		(Settings.HealthCheckPort == Settings.AuthenticationServerPort || Settings.HealthCheckPort == Settings.SecureServerPort) {
		problems = append(problems, "PN_LR_HEALTH_CHECK_PORT must not be the same as either server port")
	}

	if Settings.S3Endpoint == "" {
		problems = append(problems, "PN_LR_S3_ENDPOINT environment variable not set. DataStore needs an S3-compatible bucket (MinIO works well self-hosted) -- see compose.yaml")
	} else if Settings.S3AccessKey == "" || Settings.S3SecretKey == "" {
		problems = append(problems, "PN_LR_S3_ENDPOINT is set but PN_LR_S3_ACCESS_KEY / PN_LR_S3_SECRET_KEY are not")
	}

	problems = append(problems, accountProblems()...)

	if len(problems) != 0 {
		return fmt.Errorf("invalid configuration:\n  - %s", strings.Join(problems, "\n  - "))
	}

	return nil
}

// accountProblems checks the account configuration. The three sources are
// mutually exclusive: an account server switches login token validation on,
// so a console whose password only exists in settings.json or is derived from
// a shared secret could never satisfy it.
func accountProblems() []string {
	var problems []string

	if Settings.AccountGRPCHost != "" && Settings.AccountGRPCPort == 0 {
		problems = append(problems, "PN_LR_ACCOUNT_GRPC_HOST is set but PN_LR_ACCOUNT_GRPC_PORT is not. Set both or neither")
	}

	if Settings.AccountGRPCHost == "" && Settings.AccountGRPCPort != 0 {
		problems = append(problems, "PN_LR_ACCOUNT_GRPC_PORT is set but PN_LR_ACCOUNT_GRPC_HOST is not. Set both or neither")
	}

	if Settings.FriendsGRPCHost != "" && Settings.FriendsGRPCPort == 0 {
		problems = append(problems, "PN_LR_FRIENDS_GRPC_HOST is set but PN_LR_FRIENDS_GRPC_PORT is not. Set both or neither")
	}

	if Settings.FriendsGRPCHost == "" && Settings.FriendsGRPCPort != 0 {
		problems = append(problems, "PN_LR_FRIENDS_GRPC_PORT is set but PN_LR_FRIENDS_GRPC_HOST is not. Set both or neither")
	}

	accountSources := 0
	if Settings.LocalAuth {
		accountSources++
	}
	if Settings.UsesAccountGRPC() {
		accountSources++
	}
	if Settings.AccountSecret != "" {
		accountSources++
		secret, err := hex.DecodeString(Settings.AccountSecret)
		if err != nil || len(secret) == 0 {
			problems = append(problems, "PN_LR_ACCOUNT_SECRET must be non-empty hexadecimal")
		}
	}

	switch {
	case accountSources > 1:
		problems = append(problems, "local, gRPC and derived-secret account modes are mutually exclusive; configure exactly one")
	case Settings.LocalAuth:
		problems = append(problems, settingsFileProblems()...)
	case accountSources == 0:
		problems = append(problems, "no account source configured. Set local auth, account gRPC, or PN_LR_ACCOUNT_SECRET")
	}

	return problems
}

func settingsFileProblems() []string {
	info, err := os.Stat(Settings.SettingsFile)
	if err != nil {
		return []string{fmt.Sprintf("PN_LR_LOCAL_AUTH=1 but %s cannot be read (%v) -- copy settings.example.json to it and add your console", Settings.SettingsFile, err)}
	}

	if info.IsDir() {
		return []string{fmt.Sprintf("%s is a directory, not a file. Docker creates one when a bind mount points at a file that does not exist yet -- delete it and create the file first", Settings.SettingsFile)}
	}

	return nil
}

// UsesAccountGRPC reports whether an account server is configured.
func (c Config) UsesAccountGRPC() bool {
	return c.AccountGRPCHost != "" && c.AccountGRPCPort != 0
}

// UsesFriendsGRPC reports whether a friends server is configured.
func (c Config) UsesFriendsGRPC() bool {
	return c.FriendsGRPCHost != "" && c.FriendsGRPCPort != 0
}
