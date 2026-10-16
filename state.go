package main

import (
	"errors"
	"fmt"
	"os"
)

// ErrNotConfigured is returned when a guild is not configured
var ErrNotConfigured = errors.New("guild is not configured")

// ErrAlreadyClaimed is returned when a user is already claimed
var ErrAlreadyClaimed = errors.New("user is already claimed")

// ErrDoesNotExist is returned when a user does not exist
var ErrDoesNotExist = errors.New("user does not exist")

// ErrInvalidSession is returned when the advent of code session is invalid
var ErrInvalidSession = errors.New("advent of code session has expired, please update the session cookie")

// GuildState keeps track of the state of a single guild
type GuildState struct {
	adventOfCode *AdventOfCode
	db           *Database
	year         string
	daily_roles  bool
}

// NewGuildState creates a new guild state
func NewGuildState(sessionCookie string, config GuildConfig, log *os.File) (*GuildState, error) {
	database, err := NewDatabase(log, log)
	if err != nil {
		return nil, err
	}

	return &GuildState{
		adventOfCode: NewAdventOfCode(sessionCookie, config.LeaderboardID),
		db:           database,
		year:         config.Year,
		daily_roles:  config.DailyRoles,
	}, nil
}

// Claim claims an Advent of Code account by name, falling back to its ID
func (guildState *GuildState) Claim(discordUserID string, nameOrID string) error {
	lookup := func(leaderboard *Leaderboard) (*Member, bool) {
		if member, ok := leaderboard.GetMemberByName(nameOrID); ok {
			return member, true
		}

		return leaderboard.GetMemberByID(nameOrID)
	}

	member, ok := lookup(guildState.GetLeaderboard())
	if !ok {
		// Update the leaderboard and try again
		member, ok = lookup(guildState.UpdateLeaderboard())
	}

	if !ok {
		return ErrDoesNotExist
	}

	id := fmt.Sprint(member.ID)

	// Check if the user is already claimed
	if guildState.db.CheckClaim(id) {
		return ErrAlreadyClaimed
	}

	return guildState.db.Claim(discordUserID, id)
}

// GetLeaderboard is wrapper for guildState.adventOfCode.GetLeaderboard()
func (guildState *GuildState) GetLeaderboard() *Leaderboard {
	return guildState.adventOfCode.GetLeaderboard(guildState.year)
}

// UpdateLeaderboard updates the leaderboard before returning it
func (guildState *GuildState) UpdateLeaderboard() *Leaderboard {
	guildState.adventOfCode.UpdateLeaderboard(guildState.year)
	return guildState.adventOfCode.GetLeaderboard(guildState.year)
}
