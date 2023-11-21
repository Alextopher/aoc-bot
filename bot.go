package main

import (
	"fmt"
	"log"
	"os"

	"github.com/bwmarrin/discordgo"
)

// Bot is the main bot struct
type Bot struct {
	// The Discord session
	session *discordgo.Session

	// Each discord server has its own state
	states map[string]*GuildState

	// The Advent of Code API session cookie
	sessionCookie string
}

// NewBot creates a new bot
func NewBot(session *discordgo.Session, sessionCookie string) *Bot {
	return &Bot{
		session:       session,
		states:        make(map[string]*GuildState),
		sessionCookie: sessionCookie,
	}
}

// AddGuild adds a guild to the bot
func (bot *Bot) AddGuild(guildID string, year string, leaderboardID string) (err error) {
	// Create guild log file
	log, err := os.OpenFile(fmt.Sprintf("logs/%s.db", guildID), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}

	bot.states[guildID], err = NewGuildState(bot.sessionCookie, year, leaderboardID, log)
	if err != nil {
		return err
	}

	return bot.states[guildID].adventOfCode.UpdateLeaderboard()
}

// Start starts the bot
func (bot *Bot) Start() error {
	return bot.session.Open()
}

// Sync syncs the bot with the Advent of Code API
func (bot *Bot) Sync() {
	for _, guild := range bot.session.State.Guilds {
		err := bot.SyncRoles(guild)
		if err != nil {
			log.Println("Error syncing roles: ", err)
		}
	}
}

// CreateRoles ensure that the server has the required roles
//
// 1 role for each day + 1 role for 10, 20, 30, 40, and 50 stars
func (bot *Bot) CreateRoles(guild *discordgo.Guild) error {
	True := true

	// 50 stars
	Yellow := 0xF1C40F
	if !bot.CheckRole(guild, "50 Stars") {
		_, err := bot.session.GuildRoleCreate(guild.ID, &discordgo.RoleParams{
			Name:        "50 Stars",
			Color:       &Yellow,
			Mentionable: &True,
			Hoist:       &True,
		})

		if err != nil {
			return err
		}
	}

	// 40 stars
	Orange := 0xE91E63
	if !bot.CheckRole(guild, "40 Stars") {
		_, err := bot.session.GuildRoleCreate(guild.ID, &discordgo.RoleParams{
			Name:        "40 Stars",
			Color:       &Orange,
			Mentionable: &True,
			Hoist:       &True,
		})

		if err != nil {
			return err
		}
	}

	// 30 stars
	Red := 0x9B59B6
	if !bot.CheckRole(guild, "30 Stars") {
		_, err := bot.session.GuildRoleCreate(guild.ID, &discordgo.RoleParams{
			Name:        "30 Stars",
			Color:       &Red,
			Mentionable: &True,
			Hoist:       &True,
		})

		if err != nil {
			return err
		}
	}

	// 20 stars
	Purple := 0x3498DB
	if !bot.CheckRole(guild, "20 Stars") {
		_, err := bot.session.GuildRoleCreate(guild.ID, &discordgo.RoleParams{
			Name:        "20 Stars",
			Color:       &Purple,
			Hoist:       &True,
			Mentionable: &True,
		})

		if err != nil {
			return err
		}
	}

	// 10 stars
	Blue := 0x2ECC71
	if !bot.CheckRole(guild, "10 Stars") {
		_, err := bot.session.GuildRoleCreate(guild.ID, &discordgo.RoleParams{
			Name:        "10 Stars",
			Color:       &Blue,
			Hoist:       &True,
			Mentionable: &True,
		})

		if err != nil {
			return err
		}
	}

	names := make([]string, 0)
	for i := 25; i > 0; i-- {
		names = append(names, fmt.Sprintf("Day %d", i))
	}

	for _, name := range names {
		if bot.CheckRole(guild, name) {
			continue
		}

		// Create the role
		_, err := bot.session.GuildRoleCreate(guild.ID, &discordgo.RoleParams{
			Name: name,
		})

		if err != nil {
			return err
		}
	}

	return nil
}

// CheckRole checks if a role exists
func (bot *Bot) CheckRole(guild *discordgo.Guild, name string) bool {
	for _, role := range guild.Roles {
		if role.Name == name {
			return true
		}
	}

	return false
}

// SyncRoles updates each user's roles to reflect their current star count
// and the days they have completed
func (bot *Bot) SyncRoles(guild *discordgo.Guild) error {
	guildState, ok := bot.states[guild.ID]
	if !ok {
		return fmt.Errorf("guild not found")
	}

	// Get the leaderboard
	leaderboard := guildState.GetLeaderboard()

	guildState.db.ForEach(func(discord_id, advent_id string) {
		member, ok := leaderboard.GetMemberByID(advent_id)
		if !ok {
			log.Printf("Error: Member %s not found\n", advent_id)
			return
		}

		// Get the member
		guildMember, err := bot.session.GuildMember(guild.ID, discord_id)
		if err != nil {
			log.Printf("Error: Guild Member %s not found\n", discord_id)
			return
		}

		stars := member.Stars

		if stars >= 50 {
			err = bot.AddRole(guild, guildMember, "50 Stars")
			if err != nil {
				log.Println("Error adding role: ", err)
				return
			}
		}

		if stars >= 40 {
			err = bot.AddRole(guild, guildMember, "40 Stars")
			if err != nil {
				log.Println("Error adding role: ", err)
				return
			}
		}

		if stars >= 30 {
			err = bot.AddRole(guild, guildMember, "30 Stars")
			if err != nil {
				log.Println("Error adding role: ", err)
				return
			}
		}

		if stars >= 20 {
			err = bot.AddRole(guild, guildMember, "20 Stars")
			if err != nil {
				log.Println("Error adding role: ", err)
				return
			}
		}

		if stars >= 10 {
			err = bot.AddRole(guild, guildMember, "10 Stars")
			if err != nil {
				log.Println("Error adding role: ", err)
				return
			}
		}

		for day := range member.CompletionDayLevel {
			err = bot.AddRole(guild, guildMember, fmt.Sprintf("Day %d", day))
			if err != nil {
				log.Println("Error adding role: ", err)
				return
			}
		}
	})

	return nil
}

// AddRole adds a role to a user
func (bot *Bot) AddRole(guild *discordgo.Guild, member *discordgo.Member, name string) error {
	for _, role := range guild.Roles {
		if role.Name == name {
			return bot.session.GuildMemberRoleAdd(guild.ID, member.User.ID, role.ID)
		}
	}

	return fmt.Errorf("role not found")
}
