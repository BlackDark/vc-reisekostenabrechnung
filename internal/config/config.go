// Package config loads process configuration from the environment.
package config

import (
	"fmt"
	"math"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the validated process configuration. Secrets are never logged.
type Config struct {
	AppBaseURL       string
	Origin           string
	ListenAddr       string
	DataDir          string
	DBPath           string
	LogLevel         string
	LogFormat        string
	DefaultLocale    string
	TZDefault        string
	TrustedProxies   []*net.IPNet
	CookieSecure     bool
	SessionIdle      time.Duration
	SessionLifetime  time.Duration
	InitialAdminUser string
	InitialAdminMail string
	InitialAdminPass string

	AuthPasswordEnabled bool
	Argon2MemoryKiB     uint32
	Argon2Time          uint32
	Argon2Threads       uint8

	OIDC   OIDC
	Header Header

	StorageBackend   string
	StorageLocalPath string
	S3               S3

	UploadMaxBytes   int64
	BelegFormat      string
	BelegAVIFQuality int
	BelegAVIFSpeed   int
	BelegWebPQuality int
	BelegJPEGQuality int
	BelegKarenz      time.Duration
	TypstPath        string
	ExportTimeout    time.Duration

	RateLogin  Rate
	RateAPI    Rate
	RateUpload Rate
	RateAI     Rate

	JobWorkers int
	AIEnabled  bool
}

// OIDC holds the OpenID Connect client settings.
type OIDC struct {
	Enabled           bool
	IssuerURL         string
	ClientID          string
	ClientSecret      string
	Scopes            []string
	GroupsClaim       string
	AdminGroup        string
	AllowedGroup      string
	AllowEmailLinking bool
	ButtonLabel       string
	AutoRedirect      bool
}

// Header holds forward-auth settings. Headers are honored only from TrustedProxies.
type Header struct {
	Enabled      bool
	UserHeader   string
	EmailHeader  string
	NameHeader   string
	GroupsHeader string
	AdminGroup   string
	AllowedGroup string
	LogoutURL    string
}

// S3 holds object-storage settings. DataLocation must be an EU/EEA country.
type S3 struct {
	Endpoint     string
	Region       string
	Bucket       string
	Prefix       string
	AccessKey    string
	SecretKey    string
	UsePathStyle bool
	DataLocation string
	AllowNonEU   bool
}

// Rate is a token-bucket limit expressed as count per unit.
type Rate struct {
	Count int
	Per   time.Duration
}

// Load reads and validates the environment for `serve`.
func Load() (Config, error) {
	var errs []string
	cfg := Config{
		AppBaseURL:          lookup("APP_BASE_URL"),
		ListenAddr:          lookupDefault("LISTEN_ADDR", ":8080"),
		DataDir:             lookupDefault("DATA_DIR", "/data"),
		LogLevel:            lookupDefault("LOG_LEVEL", "info"),
		LogFormat:           lookupDefault("LOG_FORMAT", "json"),
		DefaultLocale:       lookupDefault("DEFAULT_LOCALE", "de"),
		TZDefault:           lookupDefault("TZ_DEFAULT", "Europe/Berlin"),
		CookieSecure:        lookupBool("COOKIE_SECURE", true),
		SessionIdle:         lookupDuration("SESSION_IDLE_TIMEOUT", 168*time.Hour, &errs),
		SessionLifetime:     lookupDuration("SESSION_LIFETIME", 720*time.Hour, &errs),
		InitialAdminUser:    lookup("INITIAL_ADMIN_USERNAME"),
		InitialAdminMail:    lookup("INITIAL_ADMIN_EMAIL"),
		AuthPasswordEnabled: lookupBool("AUTH_PASSWORD_ENABLED", true),
		Argon2MemoryKiB:     lookupUint32("ARGON2_MEMORY_KIB", 65536, &errs),
		Argon2Time:          lookupUint32("ARGON2_TIME", 3, &errs),
		Argon2Threads:       lookupUint8("ARGON2_THREADS", 4, &errs),
		StorageBackend:      lookupDefault("STORAGE_BACKEND", "local"),
		UploadMaxBytes:      int64(lookupInt("UPLOAD_MAX_BYTES", 26214400, &errs)),
		BelegFormat:         lookupDefault("BELEG_FORMAT", "avif"),
		BelegAVIFQuality:    lookupInt("BELEG_AVIF_QUALITY", 40, &errs),
		BelegAVIFSpeed:      lookupInt("BELEG_AVIF_SPEED", 6, &errs),
		BelegWebPQuality:    lookupInt("BELEG_WEBP_QUALITY", 55, &errs),
		BelegJPEGQuality:    lookupInt("BELEG_JPEG_QUALITY", 70, &errs),
		BelegKarenz:         lookupDuration("BELEG_ERFASSUNG_KARENZ", 720*time.Hour, &errs),
		TypstPath:           lookupDefault("TYPST_PATH", "/usr/local/bin/typst"),
		ExportTimeout:       lookupDuration("EXPORT_TIMEOUT", 120*time.Second, &errs),
		JobWorkers:          lookupInt("JOB_WORKERS", 2, &errs),
		AIEnabled:           lookupBool("AI_ENABLED", false),
	}
	cfg.AppBaseURL = strings.TrimRight(cfg.AppBaseURL, "/")
	cfg.DBPath = lookupDefault("DB_PATH", cfg.DataDir+"/reisekosten.db")
	cfg.StorageLocalPath = lookupDefault("STORAGE_LOCAL_PATH", cfg.DataDir+"/files")

	pass, err := secret("INITIAL_ADMIN_PASSWORD")
	if err != nil {
		errs = append(errs, err.Error())
	}
	cfg.InitialAdminPass = pass

	oidcSecret, err := secret("OIDC_CLIENT_SECRET")
	if err != nil {
		errs = append(errs, err.Error())
	}
	s3secret, err := secret("S3_SECRET_ACCESS_KEY")
	if err != nil {
		errs = append(errs, err.Error())
	}

	cfg.OIDC = OIDC{
		Enabled:           lookupBool("OIDC_ENABLED", false),
		IssuerURL:         strings.TrimRight(lookup("OIDC_ISSUER_URL"), "/"),
		ClientID:          lookup("OIDC_CLIENT_ID"),
		ClientSecret:      oidcSecret,
		Scopes:            strings.Fields(lookupDefault("OIDC_SCOPES", "openid profile email groups")),
		GroupsClaim:       lookupDefault("OIDC_GROUPS_CLAIM", "groups"),
		AdminGroup:        lookup("OIDC_ADMIN_GROUP"),
		AllowedGroup:      lookup("OIDC_ALLOWED_GROUP"),
		AllowEmailLinking: lookupBool("OIDC_ALLOW_EMAIL_LINKING", false),
		ButtonLabel:       lookupDefault("OIDC_BUTTON_LABEL", "Mit SSO anmelden"),
		AutoRedirect:      lookupBool("OIDC_AUTO_REDIRECT", false),
	}
	cfg.Header = Header{
		Enabled:      lookupBool("HEADER_AUTH_ENABLED", false),
		UserHeader:   lookupDefault("HEADER_AUTH_USER_HEADER", "Remote-User"),
		EmailHeader:  lookupDefault("HEADER_AUTH_EMAIL_HEADER", "Remote-Email"),
		NameHeader:   lookupDefault("HEADER_AUTH_NAME_HEADER", "Remote-Name"),
		GroupsHeader: lookupDefault("HEADER_AUTH_GROUPS_HEADER", "Remote-Groups"),
		AdminGroup:   lookupDefault("HEADER_AUTH_ADMIN_GROUP", cfg.OIDC.AdminGroup),
		AllowedGroup: lookupDefault("HEADER_AUTH_ALLOWED_GROUP", cfg.OIDC.AllowedGroup),
		LogoutURL:    lookup("HEADER_AUTH_LOGOUT_URL"),
	}
	cfg.S3 = S3{
		Endpoint:     lookup("S3_ENDPOINT"),
		Region:       lookup("S3_REGION"),
		Bucket:       lookup("S3_BUCKET"),
		Prefix:       lookup("S3_PREFIX"),
		AccessKey:    lookup("S3_ACCESS_KEY_ID"),
		SecretKey:    s3secret,
		UsePathStyle: lookupBool("S3_USE_PATH_STYLE", false),
		DataLocation: strings.ToUpper(lookup("S3_DATA_LOCATION")),
		AllowNonEU:   lookupBool("S3_ALLOW_NON_EU", false),
	}

	if nets, err := parseCIDRs(lookup("TRUSTED_PROXIES")); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.TrustedProxies = nets
	}

	cfg.RateLogin = lookupRate("RATE_LIMIT_LOGIN", Rate{Count: 5, Per: time.Minute}, &errs)
	cfg.RateAPI = lookupRate("RATE_LIMIT_API", Rate{Count: 300, Per: time.Minute}, &errs)
	cfg.RateUpload = lookupRate("RATE_LIMIT_UPLOAD", Rate{Count: 30, Per: time.Minute}, &errs)
	cfg.RateAI = lookupRate("RATE_LIMIT_AI", Rate{Count: 20, Per: time.Minute}, &errs)

	errs = append(errs, validate(cfg)...)
	if len(errs) > 0 {
		return Config{}, &Error{Messages: errs}
	}
	return cfg, nil
}

func validate(cfg Config) []string {
	var errs []string
	if cfg.AppBaseURL == "" {
		errs = append(errs, "APP_BASE_URL is required")
	} else {
		u, err := url.Parse(cfg.AppBaseURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			errs = append(errs, "APP_BASE_URL must be an absolute http(s) URL")
		} else if u.Path != "" && u.Path != "/" {
			errs = append(errs, "APP_BASE_URL must not include a path")
		}
	}
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, "LOG_LEVEL must be debug, info, warn, or error")
	}
	switch cfg.LogFormat {
	case "json", "text":
	default:
		errs = append(errs, "LOG_FORMAT must be json or text")
	}
	if cfg.DefaultLocale != "de" && cfg.DefaultLocale != "en" {
		errs = append(errs, "DEFAULT_LOCALE must be de or en")
	}
	if _, err := time.LoadLocation(cfg.TZDefault); err != nil {
		errs = append(errs, "TZ_DEFAULT is not a valid IANA time zone")
	}
	if cfg.Argon2MemoryKiB < 8 || cfg.Argon2Time < 1 || cfg.Argon2Threads < 1 {
		errs = append(errs, "ARGON2_MEMORY_KIB, ARGON2_TIME and ARGON2_THREADS must be positive")
	}
	if (cfg.InitialAdminUser == "") != (cfg.InitialAdminPass == "") {
		errs = append(errs, "INITIAL_ADMIN_USERNAME and INITIAL_ADMIN_PASSWORD must be set together")
	}
	if cfg.InitialAdminPass != "" && (len([]rune(cfg.InitialAdminPass)) < 12 || len([]rune(cfg.InitialAdminPass)) > 256) {
		errs = append(errs, "INITIAL_ADMIN_PASSWORD must be 12 to 256 characters")
	}
	if !cfg.AuthPasswordEnabled && !cfg.OIDC.Enabled && !cfg.Header.Enabled && cfg.InitialAdminUser == "" {
		errs = append(errs, "at least one login method must be enabled (password, OIDC, or header auth)")
	}
	if cfg.OIDC.Enabled {
		if cfg.OIDC.IssuerURL == "" || cfg.OIDC.ClientID == "" {
			errs = append(errs, "OIDC_ISSUER_URL and OIDC_CLIENT_ID are required when OIDC_ENABLED=true")
		} else if u, err := url.Parse(cfg.OIDC.IssuerURL); err != nil || u.Scheme == "" || u.Host == "" {
			errs = append(errs, "OIDC_ISSUER_URL must be an absolute URL")
		}
	}
	if cfg.Header.Enabled && len(cfg.TrustedProxies) == 0 {
		errs = append(errs, "HEADER_AUTH_ENABLED=true requires TRUSTED_PROXIES")
	}
	switch cfg.StorageBackend {
	case "local":
	case "s3":
		if cfg.S3.Endpoint == "" || cfg.S3.Region == "" || cfg.S3.Bucket == "" || cfg.S3.AccessKey == "" || cfg.S3.SecretKey == "" {
			errs = append(errs, "S3_ENDPOINT, S3_REGION, S3_BUCKET, S3_ACCESS_KEY_ID and S3_SECRET_ACCESS_KEY are required when STORAGE_BACKEND=s3")
		}
		if cfg.S3.DataLocation == "" {
			errs = append(errs, "S3_DATA_LOCATION is required when STORAGE_BACKEND=s3")
		} else if !cfg.S3.AllowNonEU && !euEEA[cfg.S3.DataLocation] {
			errs = append(errs, "S3_DATA_LOCATION must be an EU/EEA country (or set S3_ALLOW_NON_EU=true)")
		}
	default:
		errs = append(errs, "STORAGE_BACKEND must be local or s3")
	}
	switch cfg.BelegFormat {
	case "avif", "webp":
	default:
		errs = append(errs, "BELEG_FORMAT must be avif or webp")
	}
	if cfg.BelegAVIFQuality < 1 || cfg.BelegAVIFQuality > 100 || cfg.BelegWebPQuality < 1 || cfg.BelegWebPQuality > 100 || cfg.BelegJPEGQuality < 1 || cfg.BelegJPEGQuality > 100 {
		errs = append(errs, "BELEG_AVIF_QUALITY, BELEG_WEBP_QUALITY and BELEG_JPEG_QUALITY must be 1..100")
	}
	if cfg.BelegAVIFSpeed < 0 || cfg.BelegAVIFSpeed > 10 {
		errs = append(errs, "BELEG_AVIF_SPEED must be 0..10")
	}
	if cfg.BelegKarenz <= 0 {
		errs = append(errs, "BELEG_ERFASSUNG_KARENZ must be positive")
	}
	if cfg.SessionIdle <= 0 || cfg.SessionLifetime <= 0 {
		errs = append(errs, "session timeouts must be positive")
	}
	if cfg.SessionIdle > cfg.SessionLifetime {
		errs = append(errs, "SESSION_IDLE_TIMEOUT cannot exceed SESSION_LIFETIME")
	}
	return errs
}

// OriginURL is scheme://host of APP_BASE_URL, used as the trusted CSRF origin.
func (c Config) OriginURL() string {
	u, err := url.Parse(c.AppBaseURL)
	if err != nil {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// CookieName is the session cookie. The __Host- prefix requires Secure.
func (c Config) CookieName(base string) string {
	if c.CookieSecure {
		return "__Host-" + base
	}
	return base
}

// Error lists configuration problems, one per line.
type Error struct {
	Messages []string
}

func (e *Error) Error() string {
	return "invalid configuration:\n- " + strings.Join(e.Messages, "\n- ")
}

func lookup(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok {
		return ""
	}
	return strings.TrimSpace(v)
}

func lookupDefault(key, def string) string {
	if v := lookup(key); v != "" {
		return v
	}
	return def
}

func lookupBool(key string, def bool) bool {
	v := lookup(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func lookupUint32(key string, def uint32, errs *[]string) uint32 {
	n := lookupInt(key, int(def), errs)
	if n < 0 || n > math.MaxUint32 {
		*errs = append(*errs, key+" is out of range")
		return def
	}
	return uint32(n)
}

func lookupUint8(key string, def uint8, errs *[]string) uint8 {
	n := lookupInt(key, int(def), errs)
	if n < 0 || n > math.MaxUint8 {
		*errs = append(*errs, key+" is out of range")
		return def
	}
	return uint8(n)
}

func lookupInt(key string, def int, errs *[]string) int {
	v := lookup(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		*errs = append(*errs, key+" must be an integer")
		return def
	}
	return n
}

func lookupDuration(key string, def time.Duration, errs *[]string) time.Duration {
	v := lookup(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		*errs = append(*errs, key+" must be a Go duration (for example 168h)")
		return def
	}
	return d
}

func lookupRate(key string, def Rate, errs *[]string) Rate {
	v := lookup(key)
	if v == "" {
		return def
	}
	r, err := ParseRate(v)
	if err != nil {
		*errs = append(*errs, key+": "+err.Error())
		return def
	}
	return r
}

// ParseRate parses values such as 5/m, 300/m, or 20/h.
func ParseRate(s string) (Rate, error) {
	count, unit, ok := strings.Cut(s, "/")
	if !ok {
		return Rate{}, fmt.Errorf("expected count/unit, got %q", s)
	}
	n, err := strconv.Atoi(count)
	if err != nil || n < 1 {
		return Rate{}, fmt.Errorf("invalid count in %q", s)
	}
	var per time.Duration
	switch unit {
	case "s":
		per = time.Second
	case "m":
		per = time.Minute
	case "h":
		per = time.Hour
	default:
		return Rate{}, fmt.Errorf("unit must be s, m, or h in %q", s)
	}
	return Rate{Count: n, Per: per}, nil
}

func secret(name string) (string, error) {
	direct := lookup(name)
	file := lookup(name + "_FILE")
	if direct != "" && file != "" {
		return "", fmt.Errorf("%s and %s_FILE are both set", name, name)
	}
	if file == "" {
		return direct, nil
	}
	b, err := os.ReadFile(file) // #nosec G304 -- path comes from the operator environment, not a request
	if err != nil {
		return "", fmt.Errorf("read %s: %w", name+"_FILE", err)
	}
	return strings.TrimRight(string(b), "\r\n"), nil
}

func parseCIDRs(raw string) ([]*net.IPNet, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var out []*net.IPNet
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !strings.Contains(part, "/") {
			if strings.Contains(part, ":") {
				part += "/128"
			} else {
				part += "/32"
			}
		}
		_, n, err := net.ParseCIDR(part)
		if err != nil {
			return nil, fmt.Errorf("TRUSTED_PROXIES: invalid CIDR %q", part)
		}
		out = append(out, n)
	}
	return out, nil
}

// IPTrusted reports whether ip belongs to a trusted proxy network.
func IPTrusted(nets []*net.IPNet, ip net.IP) bool {
	if ip == nil {
		return false
	}
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

var euEEA = map[string]bool{
	"AT": true, "BE": true, "BG": true, "HR": true, "CY": true, "CZ": true,
	"DK": true, "EE": true, "FI": true, "FR": true, "DE": true, "GR": true,
	"EL": true, "HU": true, "IE": true, "IT": true, "LV": true, "LT": true,
	"LU": true, "MT": true, "NL": true, "PL": true, "PT": true, "RO": true,
	"SK": true, "SI": true, "ES": true, "SE": true, "IS": true, "LI": true,
	"NO": true,
}
