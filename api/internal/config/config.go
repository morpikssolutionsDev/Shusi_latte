// Package config reads runtime settings from the environment.
package config

import (
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	DatabaseURL    string   // POSTGRE_URL: the PostgreSQL database (ADR-0007), set in the environment
	Addr           string   // SL_ADDR, default 127.0.0.1:8080 (Caddy fronts it on the LAN)
	DataDir        string   // SL_DATA_DIR, default ../data (repo-root data/ when run from api/)
	AssetsDir      string   // SL_ASSETS_DIR, default ../docs/images (demo photos copied by the seed)
	DemoMode       bool     // SL_DEMO_MODE
	SecureCookie   bool     // SL_SECURE_COOKIE, default true (browsers allow Secure cookies on localhost)
	OriginPatterns []string // SL_WS_ORIGINS, comma-separated host patterns accepted for WebSockets
	LogLevel       string   // SL_LOG_LEVEL: debug|info|warn|error
	MasterKey      string   // SL_MASTER_KEY: base64 32-byte key wrapping the daily data keys (TR-RET-04)
}

func Load() Config {
	return Config{
		DatabaseURL:    os.Getenv("POSTGRE_URL"),
		Addr:           env("SL_ADDR", "127.0.0.1:8080"),
		DataDir:        env("SL_DATA_DIR", filepath.Join("..", "data")),
		AssetsDir:      env("SL_ASSETS_DIR", filepath.Join("..", "docs", "images")),
		DemoMode:       envBool("SL_DEMO_MODE", false),
		SecureCookie:   envBool("SL_SECURE_COOKIE", true),
		OriginPatterns: strings.Split(env("SL_WS_ORIGINS", "localhost:*,127.0.0.1:*"), ","),
		LogLevel:       env("SL_LOG_LEVEL", "info"),
		MasterKey:      os.Getenv("SL_MASTER_KEY"),
	}
}

// ImagesDir holds served images: originals and generated variants (TR-DM-09).
func (c Config) ImagesDir() string { return filepath.Join(c.DataDir, "images") }

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	switch strings.ToLower(env(key, "")) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}

// KeysPath is the crypto-shredding key file, kept outside the database and its backups (TR-RET-04).
func (c Config) KeysPath() string { return filepath.Join(c.DataDir, "keys.json") }

// MasterKeyPath is the prototype fallback when SL_MASTER_KEY is not set.
func (c Config) MasterKeyPath() string { return filepath.Join(c.DataDir, "master.key") }
