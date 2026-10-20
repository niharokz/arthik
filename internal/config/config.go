// Package config reads Arthik's settings from the environment. In Docker these come
// from the gitlab-ignored .env via env_file; when run by hand, a .env file in the
// working directory is read too (real environment variables win).
//
// Only container-side values live here. Host paths, UID/GID and the published port
// are used by docker-compose.yml alone.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Modes: "server" is the web app on the home server; "local" is the engine inside the phone app.
const (
	ModeServer = "server"
	ModeLocal  = "local"
)

// Config is the runtime configuration.
type Config struct {
	Mode         string        // ARTHIK_MODE: server (default) | local
	LocalToken   string        // ARTHIK_LOCAL_TOKEN: secret the phone app uses to open a session (local mode)
	Primary      bool          // ARTHIK_PRIMARY: main device writes computed files, merges, imports the inbox (default true)
	ExitOnEOF    bool          // ARTHIK_EXIT_ON_STDIN_EOF: stop when the parent app closes our stdin
	DemoDirEnv   string        // ARTHIK_DEMO_DIR: demo folder (default <data>/demouser)
	Listen       string        // ARTHIK_LISTEN, default :8080
	DataDir      string        // ARTHIK_DATA, default /data/finance (owner's dataset; may be empty in local mode)
	User         string        // ARTHIK_USER, default "admin"
	Password     string        // ARTHIK_PASSWORD (plain text or bcrypt hash) — required
	DemoEnabled  bool          // ARTHIK_DEMO, default true
	DemoUser     string        // ARTHIK_DEMO_USER, default "demouser"
	DemoPassword string        // ARTHIK_DEMO_PASSWORD, default "demo" (shown on the login screen)
	Secret       string        // ARTHIK_SECRET — signs sessions; random per start when empty
	WatchEvery   time.Duration // ARTHIK_WATCH_SECONDS, default 3

	Security Security
	Inbox    Inbox
}

// Security configures sessions and banning (server mode).
type Security struct {
	SessionTTL     time.Duration // ARTHIK_SESSION_HOURS: a sign-in without the device key lasts this long (default 12)
	DeviceKey      string        // ARTHIK_DEVICE_KEY: typed into Settings, keeps that device signed in until sign-out ("" = off)
	MaxFails       int           // ARTHIK_LOGIN_MAX_FAILS: failed sign-ins (within a day) before an address is banned (default 5)
	BanTerm        time.Duration // ARTHIK_BAN_DAYS: how long a ban lasts (default 30)
	StateDir       string        // ARTHIK_STATE_DIR: server-only files such as bans.json (default /state)
	TrustedProxies string        // ARTHIK_TRUSTED_PROXIES: CIDRs whose forwarding headers are believed (default: private ranges)
	BanExempt      string        // ARTHIK_BAN_EXEMPT: CIDRs that are never banned (default none)
}

// DefaultTrustedProxies are the private ranges a reverse proxy in Docker uses.
const DefaultTrustedProxies = "127.0.0.0/8,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,::1/128,fc00::/7"

// Inbox configures the review inbox (SMS and Kronos drafts) and the optional AI.
type Inbox struct {
	MaxAgeDays int           // ARTHIK_INBOX_MAX_AGE_DAYS: ignore source items older than this (default 30)
	KeepDays   int           // ARTHIK_INBOX_KEEP_DAYS: forget reviewed drafts after this (default 120)
	PairDays   int           // ARTHIK_INBOX_PAIR_DAYS: two SMS this close become one transfer (default 2)
	Rescan     time.Duration // ARTHIK_INBOX_RESCAN_MINUTES: re-read the inbox files this often even when no change was seen (default 10)
	AIURL      string        // ARTHIK_AI_URL: Ollama base URL, e.g. http://ollama:11434 (empty = no AI)
	AIModel    string        // ARTHIK_AI_MODEL: model name, e.g. qwen2.5:7b-instruct-q4_K_M
	AITimeout  time.Duration // ARTHIK_AI_TIMEOUT_SECONDS: per request (default 120)
}

// AIEnabled reports whether drafts may be categorised by the local AI.
func (i Inbox) AIEnabled() bool { return i.AIURL != "" && i.AIModel != "" }

// DemoDir is where the demo dataset lives: ARTHIK_DEMO_DIR or <data>/demouser.
func (c Config) DemoDir() string {
	if c.DemoDirEnv != "" {
		return c.DemoDirEnv
	}
	return filepath.Join(c.DataDir, "demouser")
}

// Local reports whether arthik runs inside the phone app.
func (c Config) Local() bool { return c.Mode == ModeLocal }

// LoadDotEnv sets variables from a KEY=VALUE file without overriding the real
// environment. A missing file is not an error.
func LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(strings.TrimPrefix(line, "export "), "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"' || v[0] == '\'' && v[len(v)-1] == '\'') {
			v = v[1 : len(v)-1]
		}
		if _, set := os.LookupEnv(k); !set {
			os.Setenv(k, v)
		}
	}
	return sc.Err()
}

// envInt reads a positive integer (def when missing or invalid).
func envInt(k string, def int) int {
	var n int
	if _, err := fmt.Sscan(os.Getenv(k), &n); err == nil && n > 0 {
		return n
	}
	return def
}

func env(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

// Load builds the Config from the environment.
func Load() (Config, error) {
	c := Config{
		Mode:         strings.ToLower(env("ARTHIK_MODE", ModeServer)),
		LocalToken:   os.Getenv("ARTHIK_LOCAL_TOKEN"),
		Primary:      !strings.EqualFold(env("ARTHIK_PRIMARY", "true"), "false"),
		ExitOnEOF:    env("ARTHIK_EXIT_ON_STDIN_EOF", "") == "1",
		DemoDirEnv:   os.Getenv("ARTHIK_DEMO_DIR"),
		Listen:       env("ARTHIK_LISTEN", ":8080"),
		DataDir:      env("ARTHIK_DATA", "/data/finance"),
		User:         env("ARTHIK_USER", "admin"),
		Password:     os.Getenv("ARTHIK_PASSWORD"),
		DemoEnabled:  !strings.EqualFold(env("ARTHIK_DEMO", "true"), "false"),
		DemoUser:     env("ARTHIK_DEMO_USER", "demouser"),
		DemoPassword: env("ARTHIK_DEMO_PASSWORD", "demo"),
		Secret:       os.Getenv("ARTHIK_SECRET"),
		WatchEvery:   3 * time.Second,
	}
	if n := envInt("ARTHIK_WATCH_SECONDS", 3); n > 0 {
		c.WatchEvery = time.Duration(n) * time.Second
	}
	c.Security = Security{
		SessionTTL:     time.Duration(envInt("ARTHIK_SESSION_HOURS", 12)) * time.Hour,
		DeviceKey:      strings.TrimSpace(os.Getenv("ARTHIK_DEVICE_KEY")),
		MaxFails:       envInt("ARTHIK_LOGIN_MAX_FAILS", 5),
		BanTerm:        time.Duration(envInt("ARTHIK_BAN_DAYS", 30)) * 24 * time.Hour,
		StateDir:       env("ARTHIK_STATE_DIR", "/state"),
		TrustedProxies: env("ARTHIK_TRUSTED_PROXIES", DefaultTrustedProxies),
		BanExempt:      env("ARTHIK_BAN_EXEMPT", ""),
	}
	c.Inbox = Inbox{
		MaxAgeDays: envInt("ARTHIK_INBOX_MAX_AGE_DAYS", 30),
		KeepDays:   envInt("ARTHIK_INBOX_KEEP_DAYS", 120),
		PairDays:   envInt("ARTHIK_INBOX_PAIR_DAYS", 2),
		Rescan:     time.Duration(max(1, envInt("ARTHIK_INBOX_RESCAN_MINUTES", 10))) * time.Minute,
		AIURL:      strings.TrimRight(env("ARTHIK_AI_URL", ""), "/"),
		AIModel:    env("ARTHIK_AI_MODEL", ""),
		AITimeout:  time.Duration(envInt("ARTHIK_AI_TIMEOUT_SECONDS", 120)) * time.Second,
	}
	if c.Inbox.KeepDays <= c.Inbox.MaxAgeDays {
		return c, fmt.Errorf("ARTHIK_INBOX_KEEP_DAYS (%d) must be larger than ARTHIK_INBOX_MAX_AGE_DAYS (%d), or old items would be imported again", c.Inbox.KeepDays, c.Inbox.MaxAgeDays)
	}
	switch c.Mode {
	case ModeLocal:
		if len(c.LocalToken) < 16 {
			return c, fmt.Errorf("ARTHIK_LOCAL_TOKEN must be at least 16 characters in local mode")
		}
		if os.Getenv("ARTHIK_DATA") == "" {
			c.DataDir = "" // demo only until a folder is chosen
		}
		if c.DataDir == "" && c.DemoDirEnv == "" {
			return c, fmt.Errorf("local mode needs ARTHIK_DATA or ARTHIK_DEMO_DIR")
		}
		if os.Getenv("ARTHIK_USER") == "" {
			c.User = "me"
		}
		return c, nil
	case ModeServer:
	default:
		return c, fmt.Errorf("ARTHIK_MODE must be server or local, not %q", c.Mode)
	}
	if c.Password == "" {
		return c, fmt.Errorf("ARTHIK_PASSWORD is not set (put it in .env — plain text or a hash from `arthik hash-password`)")
	}
	if k := c.Security.DeviceKey; k != "" && len(k) < 24 {
		return c, fmt.Errorf("ARTHIK_DEVICE_KEY must be at least 24 characters (make one with: openssl rand -hex 32)")
	}
	if strings.EqualFold(c.User, c.DemoUser) && c.DemoEnabled {
		return c, fmt.Errorf("ARTHIK_USER and ARTHIK_DEMO_USER must differ")
	}
	return c, nil
}
