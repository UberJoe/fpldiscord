package config

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// setEnv sets the full 9-key environment for a test and clears it afterwards, so
// each case starts from a known-empty base.
func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	keys := []string{
		"DISCORD_TOKEN", "NOTIFICATION_CHANNEL_ID", "ADMIN_IDS", "DEV_GUILD_ID",
		"LEAGUE_ID", "SEASON", "DB_PATH", "PORT", "LOG_LEVEL",
	}
	for _, k := range keys {
		t.Setenv(k, "") // register for cleanup + clear
	}
	for k, v := range kv {
		t.Setenv(k, v)
	}
}

func validEnv() map[string]string {
	return map[string]string{
		"DISCORD_TOKEN":           "tok-abc",
		"NOTIFICATION_CHANNEL_ID": "111222333",
		"ADMIN_IDS":               "444,555",
		"LEAGUE_ID":               "64",
		"SEASON":                  "2026/27",
	}
}

func TestLoad_Valid_AppliesDefaults(t *testing.T) {
	setEnv(t, validEnv())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.DBPath != defaultDBPath {
		t.Errorf("DBPath = %q, want default %q", cfg.DBPath, defaultDBPath)
	}
	if cfg.Port != defaultPort {
		t.Errorf("Port = %q, want default %q", cfg.Port, defaultPort)
	}
	if cfg.LogLevel != defaultLogLevel {
		t.Errorf("LogLevel = %q, want default %q", cfg.LogLevel, defaultLogLevel)
	}
	if got := cfg.AdminIDs; len(got) != 2 || got[0] != "444" || got[1] != "555" {
		t.Errorf("AdminIDs = %v, want [444 555]", got)
	}
	if cfg.DevGuildID != "" {
		t.Errorf("DevGuildID = %q, want empty", cfg.DevGuildID)
	}
}

func TestLoad_AggregatesEveryMissingKey(t *testing.T) {
	setEnv(t, map[string]string{}) // everything missing

	_, err := Load()
	if err == nil {
		t.Fatal("Load() succeeded with no env set; want aggregated error")
	}
	msg := err.Error()
	for _, want := range []string{
		"DISCORD_TOKEN", "NOTIFICATION_CHANNEL_ID", "ADMIN_IDS", "LEAGUE_ID", "SEASON",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message missing %q; full message:\n%s", want, msg)
		}
	}
}

func TestLoad_EmptyAdminIDsIsFailure(t *testing.T) {
	env := validEnv()
	env["ADMIN_IDS"] = "  ,  , "
	setEnv(t, env)

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "ADMIN_IDS") {
		t.Fatalf("want ADMIN_IDS failure, got %v", err)
	}
}

func TestLoad_InvalidLogLevelRejected(t *testing.T) {
	env := validEnv()
	env["LOG_LEVEL"] = "verbose"
	setEnv(t, env)

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "LOG_LEVEL") {
		t.Fatalf("want LOG_LEVEL failure, got %v", err)
	}
}

func TestLoad_DevGuildIDOptional(t *testing.T) {
	env := validEnv()
	env["DEV_GUILD_ID"] = "999888"
	setEnv(t, env)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.DevGuildID != "999888" {
		t.Errorf("DevGuildID = %q, want 999888", cfg.DevGuildID)
	}
}

func TestLogEffective_RedactsTokenButPrintsSnowflakes(t *testing.T) {
	cfg := Config{
		DiscordToken:          "super-secret-token",
		NotificationChannelID: "111222333",
		AdminIDs:              []string{"444", "555"},
		LeagueID:              "64",
		Season:                "2026/27",
		DBPath:                "/data/fpldiscord.db",
		Port:                  "8080",
		LogLevel:              "info",
	}

	var buf bytes.Buffer
	cfg.LogEffective(slog.New(slog.NewTextHandler(&buf, nil)))
	out := buf.String()

	if strings.Contains(out, "super-secret-token") {
		t.Errorf("effective-config log leaked the raw token:\n%s", out)
	}
	if !strings.Contains(out, "***") {
		t.Errorf("effective-config log missing the *** redaction:\n%s", out)
	}
	for _, snowflake := range []string{"111222333", "444", "555"} {
		if !strings.Contains(out, snowflake) {
			t.Errorf("effective-config log missing snowflake %q (should print in full):\n%s", snowflake, out)
		}
	}
}

func TestConfig_IsAdmin(t *testing.T) {
	cfg := Config{AdminIDs: []string{"1", "2"}}
	if !cfg.IsAdmin("2") {
		t.Error("IsAdmin(2) = false, want true")
	}
	if cfg.IsAdmin("3") {
		t.Error("IsAdmin(3) = true, want false")
	}
}
