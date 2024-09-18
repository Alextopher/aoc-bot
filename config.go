package main

import (
	"encoding/json"
	"io"
)

// GuildConfig is the config per guild
type GuildConfig struct {
	Year          string `json:"year"`
	Mode          string `json:"mode"`
	LeaderboardID string `json:"leaderboard_id"`
	DailyRoles    bool   `json:"daily_roles"`
}

// Config is the bot config
type Config struct {
	// The Discord bot token
	DiscordToken string `json:"discord_token"`

	// The Advent of Code session cookie
	SessionCookie string `json:"session_cookie"`

	// Map guild ids to (year, leaderboard id) pairs
	Guilds map[string]GuildConfig `json:"guilds"`
}

// ParseConfig parses a config file
func ParseConfig(reader io.Reader) (*Config, error) {
	var config Config
	err := json.NewDecoder(reader).Decode(&config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
