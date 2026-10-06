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

// Config is the runtime configuration.
type Config struct {
	Listen       string        // ARTHIK_LISTEN, default :8080
	DataDir      string        // ARTHIK_DATA, default /data/finance (owner's dataset)
	User         string        // ARTHIK_USER, default "admin"
	Password     string        // ARTHIK_PASSWORD (plain text or bcrypt hash) — required
	DemoEnabled  bool          // ARTHIK_DEMO, default true
	DemoUser     string        // ARTHIK_DEMO_USER, default "demouser"
	DemoPassword string        // ARTHIK_DEMO_PASSWORD, default "demo" (shown on the login screen)
	Secret       string        // ARTHIK_SECRET — signs sessions; random per start when empty
	WatchEvery   time.Duration // ARTHIK_WATCH_SECONDS, default 3
}

// DemoDir is where the demo dataset lives: <data>/demouser.
func (c Config) DemoDir() string { return filepath.Join(c.DataDir, "demouser") }

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

func env(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

// Load builds the Config from the environment.
func Load() (Config, error) {
	c := Config{
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
	if s := os.Getenv("ARTHIK_WATCH_SECONDS"); s != "" {
		var n int
		if _, err := fmt.Sscan(s, &n); err == nil && n > 0 {
			c.WatchEvery = time.Duration(n) * time.Second
		}
	}
	if c.Password == "" {
		return c, fmt.Errorf("ARTHIK_PASSWORD is not set (put it in .env — plain text or a hash from `arthik hash-password`)")
	}
	if strings.EqualFold(c.User, c.DemoUser) && c.DemoEnabled {
		return c, fmt.Errorf("ARTHIK_USER and ARTHIK_DEMO_USER must differ")
	}
	return c, nil
}
