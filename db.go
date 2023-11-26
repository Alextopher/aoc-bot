package main

import (
	"encoding/json"
	"io"
	"sync"
)

// DatabaseEvent is a single database event
type DatabaseEvent struct {
	Create *EventCreate `json:"create,omitempty"`
	Delete *EventDelete `json:"delete,omitempty"`
}

// EventCreate is a database event for creating a claim
type EventCreate struct {
	DiscordID string `json:"discord_id"`
	AdventID  string `json:"aoc_id"`
}

// NewEventCreate creates a new database event for creating a claim
func NewEventCreate(discordID, adventID string) *DatabaseEvent {
	return &DatabaseEvent{
		Create: &EventCreate{
			DiscordID: discordID,
			AdventID:  adventID,
		},
	}
}

// EventDelete is a database event for deleting a claim
type EventDelete struct {
	DiscordID string `json:"discord_id"`
}

// NewEventDelete creates a new database event for deleting a claim
func NewEventDelete(discordID string) *DatabaseEvent {
	return &DatabaseEvent{
		Delete: &EventDelete{
			DiscordID: discordID,
		},
	}
}

// Keeps an append-only log file of user claims
// (basically keeps a persistent map[string]string)

// Database keeps an append-only log file of user name claims and unclaims
type Database struct {
	sync.RWMutex

	// Where to write new events to
	writer *json.Encoder

	// In-memory, per guild, mapping of discord ids to Advent of Code ids
	mappings map[string]string
}

// NewDatabase creates a new database
func NewDatabase(reader io.Reader, writer io.Writer) (*Database, error) {
	// Create the database
	database := &Database{
		writer:   json.NewEncoder(writer),
		mappings: make(map[string]string),
	}

	decoder := json.NewDecoder(reader)

	// Decode in a loop to avoid EOF errors
	for {
		var event DatabaseEvent
		err := decoder.Decode(&event)

		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		switch {
		case event.Create != nil:
			database.mappings[event.Create.DiscordID] = event.Create.AdventID
		case event.Delete != nil:
			delete(database.mappings, event.Delete.DiscordID)
		}
	}

	return database, nil
}

// Claim adds a new claim to the database
func (database *Database) Claim(discordID, adventID string) error {
	database.Lock()

	// Add the claim to the database
	database.mappings[discordID] = adventID

	// Write the event to the database
	err := database.writer.Encode(NewEventCreate(discordID, adventID))

	database.Unlock()
	return err
}

// Unclaim removes a claim from the database
func (database *Database) Unclaim(discordID string) error {
	database.Lock()

	// Remove the claim from the database
	if _, ok := database.mappings[discordID]; !ok {
		database.Unlock()
		return ErrDoesNotExist
	}

	delete(database.mappings, discordID)

	// Write the event to the database
	err := database.writer.Encode(NewEventDelete(discordID))

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

// GetDiscordID gets the Discord ID for an Advent of Code user id
func (database *Database) GetDiscordID(adventID string) (string, bool) {
	database.RLock()

	// Get the Discord ID
	for discordID, id := range database.mappings {
		if id == adventID {
			database.RUnlock()
			return discordID, true
		}
	}

	database.RUnlock()
	return "", false
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

// GoForEach iterates over each claim and calls a function
func (database *Database) GoForEach(fn func(discord_id, advent_id string)) {
	database.RLock()

	// Iterate over each claim
	for discordID, adventID := range database.mappings {
		go fn(discordID, adventID)
	}

	database.RUnlock()
}
