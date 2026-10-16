package main

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestStaleRoles(t *testing.T) {
	guild := &discordgo.Guild{
		Roles: []*discordgo.Role{
			// Current event shape, must be kept
			{ID: "1", Name: "Spoiler"},
			{ID: "2", Name: "Connected"},
			{ID: "3", Name: "Day 01"},
			{ID: "4", Name: "Day 12"},
			{ID: "5", Name: starRole(maxStars)},
			// Left over from the 25 day era, must be deleted
			{ID: "6", Name: "Day 13"},
			{ID: "7", Name: "Day 25"},
			{ID: "8", Name: "30 Stars"},
			{ID: "9", Name: "50 Stars"},
			// Roles the bot never created, must be kept
			{ID: "10", Name: "@everyone"},
			{ID: "11", Name: "Moderator"},
			{ID: "12", Name: "Day Trader"},
			{ID: "13", Name: "All The Stars"},
		},
	}

	want := map[string]bool{"6": true, "7": true, "8": true, "9": true}

	got := make(map[string]bool)
	for _, role := range staleRoles(guild) {
		got[role.ID] = true
	}

	for _, role := range guild.Roles {
		if want[role.ID] != got[role.ID] {
			t.Errorf("role %q (%s): stale = %t, want %t", role.Name, role.ID, got[role.ID], want[role.ID])
		}
	}
}
