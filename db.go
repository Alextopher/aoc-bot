package main

import (
	"encoding/json"
	"io"
	"sync"
)

// Keeps an append-only log file of user claims
// (basically keeps a persistent map[string]string)

// Database keeps an append-only log file of user name claims
type Database struct {
	sync.RWMutex

	// Where to write new events to
	writer *json.Encoder

	// In-memory, per guild, mapping of discord ids to Advent of Code ids
	mappings map[string]string
}

// DatabaseEvent is a single database event
type DatabaseEvent struct {
	DiscordID string `json:"discord_id"`
	AdventID  string `json:"aoc_id"`
}

// NewDatabase creates a new database
func NewDatabase(reader io.Reader, writer io.Writer) (*Database, error) {
	// Create the database
	database := &Database{
		writer:   json.NewEncoder(writer),
		mappings: make(map[string]string),
	}

	// Decode in a loop to avoid EOF errors
	for {
		var event DatabaseEvent
		err := json.NewDecoder(reader).Decode(&event)

		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		database.mappings[event.DiscordID] = event.AdventID
	}

	return database, nil
}

// Claim adds a new claim to the database
func (database *Database) Claim(discordID, adventID string) error {
	database.Lock()

	// Add the claim to the database
	database.mappings[discordID] = adventID

	// Write the event to the database
	err := database.writer.Encode(DatabaseEvent{
		DiscordID: discordID,
		AdventID:  adventID,
	})

	database.Unlock()
	return err
}

// GetAdventID gets the Advent of Code ID for a discord user
func (database *Database) GetAdventID(discordID string) (string, bool) {
	database.RLock()

	// Get the Advent of Code ID
	aocID, ok := database.mappings[discordID]

	database.RUnlock()
	return aocID, ok
}

// CheckClaim checks if an Advent of Code user has been claimed
func (database *Database) CheckClaim(adventID string) bool {
	database.RLock()

	// Check if the user has been claimed
	for _, id := range database.mappings {
		if id == adventID {
			database.RUnlock()
			return true
		}
	}

	database.RUnlock()
	return false
}

// ForEach iterates over each claim and calls a function
func (database *Database) ForEach(fn func(discord_id, advent_id string)) {
	database.RLock()

	// Iterate over each claim
	for discordID, adventID := range database.mappings {
		fn(discordID, adventID)
	}

	database.RUnlock()
}
