package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// User Agent used for requests
const userAgent = "github.com/Alextopher/aocbot"

// AdventOfCode is an Advent of Code API client
type AdventOfCode struct {
	sync.RWMutex

	id            string
	sessionCookie string

	leaderboards map[string]*Leaderboard
	lastUpdated  time.Time
}

// NewAdventOfCode creates a new Advent of Code API
func NewAdventOfCode(sessionCookie string, id string) *AdventOfCode {
	return &AdventOfCode{
		sessionCookie: sessionCookie,
		leaderboards:  make(map[string]*Leaderboard),
		id:            id,
	}
}

// GetLeaderboard gets the most recent leaderboard data from the API
func (aoc *AdventOfCode) GetLeaderboard(year string) *Leaderboard {
	if time.Since(aoc.lastUpdated) > 15*time.Minute {
		aoc.UpdateLeaderboard(year)
	}

	aoc.RLock()
	leaderboard, ok := aoc.leaderboards[year]
	if !ok {
		log.Println("Leaderboard not found for year: ", year)
	}
	aoc.RUnlock()

	return leaderboard
}

// UpdateLeaderboard updates a leaderboard by getting the latest data from the API
func (aoc *AdventOfCode) UpdateLeaderboard(year string) error {
	requestURL := "https://adventofcode.com/" + year + "/leaderboard/private/view/" + aoc.id + ".json"
	fmt.Println(requestURL)

	url, err := url.Parse(requestURL)
	if err != nil {
		log.Println("Error while parsing URL: ", err)
		return err
	}

	request := http.Request{
		Method: "GET",
		URL:    url,
		Header: http.Header{
			"Cookie":     []string{"session=" + aoc.sessionCookie},
			"User-Agent": []string{userAgent},
		},
	}

	response, err := http.DefaultClient.Do(&request)
	if err != nil {
		log.Println("Error while making request: ", err)
		return err
	}

	// Check the content type of the response, if it's not JSON then we can't parse it
	contentType := response.Header.Get("Content-Type")
	if contentType != "application/json" {
		return ErrInvalidSession
	}

	leaderboard, err := ParseLeaderboard(response.Body)
	if err != nil {
		log.Println("Error while parsing response: ", err)
		return err
	}

	aoc.Lock()
	aoc.leaderboards[year] = leaderboard
	aoc.lastUpdated = time.Now()
	aoc.Unlock()

	log.Printf("Updated leaderboard for (%s, %s)\n", aoc.id, year)
	return nil
}
