package main

import (
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// User Agent used for requests
const userAgent = "github.com/Alextopher/aocbot"

var ticker *time.Ticker

// AdventOfCode is an Advent of Code API client
type AdventOfCode struct {
	sync.RWMutex

	year          string
	id            string
	sessionCookie string

	leaderboard *Leaderboard
	lastUpdated time.Time
}

// NewAdventOfCode creates a new Advent of Code API
func NewAdventOfCode(sessionCookie string, year string, id string) *AdventOfCode {
	return &AdventOfCode{
		sessionCookie: sessionCookie,
		year:          year,
		id:            id,
	}
}

// GetLeaderboard gets the most recent leaderboard data from the API
func (aoc *AdventOfCode) GetLeaderboard() *Leaderboard {
	if time.Since(aoc.lastUpdated) > 15*time.Minute {
		aoc.UpdateLeaderboard()
		ticker.Reset(15 * time.Minute)
	}

	aoc.RLock()
	leaderboard := aoc.leaderboard
	aoc.RUnlock()

	return leaderboard
}

// UpdateLeaderboard updates a leaderboard by getting the latest data from the API
func (aoc *AdventOfCode) UpdateLeaderboard() error {
	requestURL := "https://adventofcode.com/" + aoc.year + "/leaderboard/private/view/" + aoc.id + ".json"

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

	leaderboard, err := ParseLeaderboard(response.Body)
	if err != nil {
		log.Println("Error while parsing response: ", err)
		return err
	}

	aoc.Lock()
	aoc.leaderboard = leaderboard
	aoc.lastUpdated = time.Now()
	aoc.Unlock()

	log.Println("Updated leaderboard for (" + aoc.year + ", " + aoc.id + ")")
	return nil
}
