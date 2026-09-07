// Package config owns the typed configuration struct and Load(), which runs at
// the very top of boot — before the database is opened and before Discord is
// contacted. Load validates every required key, aggregates all problems into a
// single error, and (on success) logs the effective configuration once with the
// Discord token redacted.
package config

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Defaults for the optional keys. Applied silently — no warning when a key is
// absent and its default is used.
const (
	defaultDBPath   = "/data/fpldiscord.db"
	defaultPort     = "8080"
	defaultLogLevel = "info"
)

// Config is the fully-resolved configuration for one process. Every field is
// populated by Load; optional keys carry their default when unset.
type Config struct {
	// fly secrets — identifying values, never committed (the repo is public).
	DiscordToken          string   // DISCORD_TOKEN   (required)
	NotificationChannelID string   // NOTIFICATION_CHANNEL_ID (required)
	AdminIDs              []string // ADMIN_IDS       (required, non-empty)
	DevGuildID            string   // DEV_GUILD_ID    (optional; unset => global command registration)

	// fly.toml [env] — operational, non-identifying values, committed.
	LeagueID string // LEAGUE_ID  (required)
	Season   string // SEASON     (required, e.g. "2026/27")
	DBPath   string // DB_PATH    (default /data/fpldiscord.db)
	Port     string // PORT       (default 8080)
	LogLevel string // LOG_LEVEL  (default info)
}

// LogSlogLevel maps the LOG_LEVEL string onto a slog.Level. An unrecognised
// value is rejected by Load, so this only ever sees the four valid strings.
func (c Config) LogSlogLevel() slog.Level {
	switch c.LogLevel {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// IsAdmin reports whether the given Discord user id is on the admin allowlist.
func (c Config) IsAdmin(discordUserID string) bool {
	for _, id := range c.AdminIDs {
		if id == discordUserID {
			return true
		}
	}
	return false
}

// Load reads configuration from the process environment. In local development a
// repo-root .env file (if present) is loaded first, never overriding a value
// already set in the real environment; on fly there is no .env file so this is a
// no-op. Every missing or invalid required key is collected and returned as one
// error — Load never fails on the first problem.
func Load() (Config, error) {
	loadDotEnv(".env")

	var problems []string
	req := func(key string) string {
		v := strings.TrimSpace(os.Getenv(key))
		if v == "" {
			problems = append(problems, fmt.Sprintf("%s is required but not set", key))
		}
		return v
	}

	cfg := Config{
		DiscordToken:          req("DISCORD_TOKEN"),
		NotificationChannelID: req("NOTIFICATION_CHANNEL_ID"),
		DevGuildID:            strings.TrimSpace(os.Getenv("DEV_GUILD_ID")),
		LeagueID:              req("LEAGUE_ID"),
		Season:                req("SEASON"),
		DBPath:                envOr("DB_PATH", defaultDBPath),
		Port:                  envOr("PORT", defaultPort),
		LogLevel:              envOr("LOG_LEVEL", defaultLogLevel),
	}

	adminRaw := strings.TrimSpace(os.Getenv("ADMIN_IDS"))
	cfg.AdminIDs = splitAndTrim(adminRaw)
	if len(cfg.AdminIDs) == 0 {
		// Empty is a fail, not "no admins": a silently un-administrable bet
		// feature is worse than a boot error.
		problems = append(problems, "ADMIN_IDS is required and must contain at least one Discord user id")
	}

	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		problems = append(problems, fmt.Sprintf("LOG_LEVEL must be one of debug|info|warn|error, got %q", cfg.LogLevel))
	}

	if len(problems) > 0 {
		return Config{}, fmt.Errorf("configuration invalid:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return cfg, nil
}

// LogEffective logs the resolved configuration once at info level with
// DISCORD_TOKEN redacted. Snowflake ids are printed in full — they are not
// credentials and seeing them saves a fly-deploy debugging round-trip.
func (c Config) LogEffective(log *slog.Logger) {
	log.Info("effective configuration",
		"DISCORD_TOKEN", redact(c.DiscordToken),
		"NOTIFICATION_CHANNEL_ID", c.NotificationChannelID,
		"ADMIN_IDS", strings.Join(c.AdminIDs, ","),
		"DEV_GUILD_ID", orNone(c.DevGuildID),
		"LEAGUE_ID", c.LeagueID,
		"SEASON", c.Season,
		"DB_PATH", c.DBPath,
		"PORT", c.Port,
		"LOG_LEVEL", c.LogLevel,
	)
}

func redact(s string) string {
	if s == "" {
		return ""
	}
	return "***"
}

func orNone(s string) string {
	if s == "" {
		return "(unset — global command registration)"
	}
	return s
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func splitAndTrim(csv string) []string {
	if csv == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// loadDotEnv parses a minimal KEY=VALUE .env file and sets any key not already
// present in the environment. Missing file is not an error. Supports optional
// "export " prefixes, # comments, and single/double quoted values.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		if key == "" {
			continue
		}
		if _, present := os.LookupEnv(key); !present {
			_ = os.Setenv(key, val)
		}
	}
}
