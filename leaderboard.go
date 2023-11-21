package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/hbollon/go-edlib"
)

// Leaderboard is what is returned by the Advent of Code API
type Leaderboard struct {
	// Event is the year of the event
	Event string `json:"event"`
	// OwnerID is the user ID of the owner of the leaderboard
	OwnerID int `json:"owner_id"`
	// Members is a map of member IDs to members
	Members map[string]*Member `json:"members"`
}

// Member is a member of the leaderboard
type Member struct {
	// Name is the chosen human-readable name of the member
	// This is not guaranteed to be unique, and can be changed by the user
	Name string `json:"name"`
	// LocalScore is the number of points the member has earned in the event, relative to other members
	LocalScore int `json:"local_score"`
	// GlobalScore is the number of points the member has earned in the event, relative to all users
	GlobalScore int `json:"global_score"`
	// Stars is the number of stars the member has earned in the event
	Stars int `json:"stars"`
	// LastStarTS is the timestamp of the last star the member earned
	LastStarTS int `json:"last_star_ts"`
	// ID is the user ID of the member
	ID int `json:"id"`
	// CompletionDayLevel is a map of days to their completion status
	CompletionDayLevel map[int]map[int]*CompletionDayLevel `json:"completion_day_level"`
}

func (member Member) String() string {
	return fmt.Sprintf("AOC Name: %s\nTotal Stars: %d", member.Name, member.Stars)
}

// CompletionDayLevel is the completion status of a day
type CompletionDayLevel struct {
	// GetStarTS is the timestamp of when the star was earned
	GetStarTS int `json:"get_star_ts"`
	// StarIndex I'm not sure what this is
	StarIndex int `json:"star_index"`
}

// ParseLeaderboard parses a leaderboard from a reader
func ParseLeaderboard(r io.Reader) (*Leaderboard, error) {
	var leaderboard Leaderboard
	err := json.NewDecoder(r).Decode(&leaderboard)
	if err != nil {
		return nil, err
	}
	return &leaderboard, nil
}

// GetMemberByName gets a member by Name
func (leaderboard *Leaderboard) GetMemberByName(name string) (*Member, bool) {
	for _, member := range leaderboard.Members {
		if member.Name == name {
			return member, true
		}
	}
	return nil, false
}

// GetMemberByID gets a member by id
func (leaderboard *Leaderboard) GetMemberByID(id string) (*Member, bool) {
	member, ok := leaderboard.Members[id]
	return member, ok
}

// CloseNames returns the list of member names that are closest to the given name
func (leaderboard *Leaderboard) CloseNames(name string) ([]string, error) {
	var names []string
	for _, member := range leaderboard.Members {
		names = append(names, member.Name)
	}

	return edlib.FuzzySearchSet(name, names, 3, edlib.Levenshtein)
}
