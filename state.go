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

// GuildState keeps track of the state of a single guild
type GuildState struct {
	adventOfCode *AdventOfCode
	db           *Database
}

// NewGuildState creates a new guild state
func NewGuildState(sessionCookie, year, id string, log *os.File) (*GuildState, error) {
	database, err := NewDatabase(log, log)
	if err != nil {
		return nil, err
	}

	return &GuildState{
		adventOfCode: NewAdventOfCode(sessionCookie, year, id),
		db:           database,
	}, nil
}

// ClaimName claims a user by Advent of Code name
func (guildState *GuildState) ClaimName(discordUserID string, username string) error {
	member, ok := guildState.GetLeaderboard().GetMemberByName(username)
	if !ok {
		// Update the leaderboard and try again
		member, ok = guildState.UpdateLeaderboard().GetMemberByName(username)
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

// ClaimID claims a user by Advent of Code ID
func (guildState *GuildState) ClaimID(discordUserID string, id string) error {
	member, ok := guildState.GetLeaderboard().GetMemberByID(id)
	if !ok {
		// Update the leaderboard and try again
		member, ok = guildState.UpdateLeaderboard().GetMemberByID(id)
	}

	if !ok {
		return ErrDoesNotExist
	}

	id = fmt.Sprint(member.ID)

	// Check if the user is already claimed
	if guildState.db.CheckClaim(id) {
		return ErrAlreadyClaimed
	}

	return guildState.db.Claim(discordUserID, id)
}

// Unclaim removes a claim from a user by Discord ID
func (guildState *GuildState) Unclaim(discordUserID string) error {
	return guildState.db.Unclaim(discordUserID)
}

// CloseNames gets a list of 3 close names to the given name
func (guildState *GuildState) CloseNames(username string) ([]string, error) {
	return guildState.GetLeaderboard().CloseNames(username)
}

// GetLeaderboard is wrapper for guildState.adventOfCode.GetLeaderboard()
func (guildState *GuildState) GetLeaderboard() *Leaderboard {
	return guildState.adventOfCode.GetLeaderboard()
}

// UpdateLeaderboard updates the leaderboard before returning it
func (guildState *GuildState) UpdateLeaderboard() *Leaderboard {
	guildState.adventOfCode.UpdateLeaderboard()
	return guildState.adventOfCode.GetLeaderboard()
}
