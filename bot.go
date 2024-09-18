package main

import (
	"fmt"
	"log"
	"os"
	"time"

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
func (bot *Bot) AddGuild(guildID string, guildConfig GuildConfig) (err error) {
	// Create guild logFile file
	logFile, err := os.OpenFile(fmt.Sprintf("logs/%s.db", guildID), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}

	bot.states[guildID], err = NewGuildState(bot.sessionCookie, guildConfig, logFile)
	if err != nil {
		return err
	}

	return bot.states[guildID].adventOfCode.UpdateLeaderboard(guildConfig.Year)
}

// Start starts the bot (and waits for it to be ready)
func (bot *Bot) Start() error {
	ch := make(chan struct{})

	bot.session.AddHandlerOnce(func(s *discordgo.Session, event *discordgo.Ready) {
		log.Println("Bot is ready.")
		ch <- struct{}{}
	})

	err := bot.session.Open()
	if err != nil {
		return err
	}

	<-ch

	return nil
}

// Sync syncs the bot with the Advent of Code API
func (bot *Bot) Sync() {
	for _, guild := range bot.session.State.Guilds {
		err := bot.SyncAllRoles(guild)
		if err != nil {
			log.Println("Error (Sync) syncing roles: ", err)
		}
	}
}

// CreateRoles ensure that the server has the required roles
//
// 1 role for each day + 1 role for 10, 20, 30, 40, and 50 stars
func (bot *Bot) CreateRoles(guild *discordgo.Guild) error {
	True := true

	// Get the guild state
	guildState, ok := bot.states[guild.ID]
	if !ok {
		return ErrNotConfigured
	}

	// Spoiler role (allows users to access all channels)
	if !bot.CheckRole(guild, "Spoiler") {
		_, err := bot.session.GuildRoleCreate(guild.ID, &discordgo.RoleParams{
			Name:        "Spoiler",
			Mentionable: &True,
		})

		if err != nil {
			return err
		}
	}

	// Pair role name with color
	colors := map[string]int{
		"50 Stars":  0xF1C40F, // Yellow
		"40 Stars":  0xE91E63, // Red
		"30 Stars":  0x9B59B6, // Purple
		"20 Stars":  0x3498DB, // Blue
		"10 Stars":  0x2ECC71, // Green
		"Connected": 0x1ABC9C, // Less Green
	}

	for name, color := range colors {
		if !bot.CheckRole(guild, name) {
			_, err := bot.session.GuildRoleCreate(guild.ID, &discordgo.RoleParams{
				Name:        name,
				Color:       &color,
				Mentionable: &True,
				Hoist:       &True,
			})

			if err != nil {
				return err
			}
		}
	}

	if guildState.daily_roles {
		for i := 25; i > 0; i-- {
			name := fmt.Sprintf("Day %02d", i)

			if !bot.CheckRole(guild, name) {
				_, err := bot.session.GuildRoleCreate(guild.ID, &discordgo.RoleParams{
					Name: name,
				})

				if err != nil {
					return err
				}
			}
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

// SetupChannel sets up a channel for use by a given day (and spoiler)
func (bot *Bot) SetupChannel(guild *discordgo.Guild, channelID string, day int64) error {
	// Get the day role
	roleName := fmt.Sprintf("Day %02d", day)
	var roleID string
	for _, role := range guild.Roles {
		if role.Name == roleName {
			roleID = role.ID
			break
		}
	}

	// Get the spoiler role
	var spoilerID string
	for _, role := range guild.Roles {
		if role.Name == "Spoiler" {
			spoilerID = role.ID
			break
		}
	}

	// Get the everyone role
	var everyoneID string
	for _, role := range guild.Roles {
		if role.Name == "@everyone" {
			everyoneID = role.ID
			break
		}
	}

	_ = bot.session.ChannelPermissionSet(channelID, roleID, discordgo.PermissionOverwriteTypeRole, discordgo.PermissionViewChannel, 0)
	_ = bot.session.ChannelPermissionSet(channelID, spoilerID, discordgo.PermissionOverwriteTypeRole, discordgo.PermissionViewChannel, 0)
	_ = bot.session.ChannelPermissionSet(channelID, everyoneID, discordgo.PermissionOverwriteTypeRole, 0, discordgo.PermissionViewChannel)

	return nil
}

// SyncMemberRoles syncs a user's roles to reflect their current star count.
func (bot *Bot) SyncMemberRoles(guild *discordgo.Guild, guildMember *discordgo.Member) (err error) {
	guildState, ok := bot.states[guild.ID]
	if !ok {
		return ErrNotConfigured
	}

	adventID, ok := guildState.db.GetAdventID(guildMember.User.ID)
	if !ok {
		return ErrDoesNotExist
	}

	// Get the leaderboard
	leaderboard := guildState.GetLeaderboard()

	// Get the member
	member, ok := leaderboard.GetMemberByID(adventID)
	if !ok {
		return ErrDoesNotExist
	}

	return bot.syncRoles(guild, guildMember, member, guildState.daily_roles)
}

// SyncAllRoles updates each user's roles to reflect their current star count.
func (bot *Bot) SyncAllRoles(guild *discordgo.Guild) error {
	guildState, ok := bot.states[guild.ID]
	if !ok {
		return ErrNotConfigured
	}

	// Get the leaderboard
	leaderboard := guildState.GetLeaderboard()

	log.Printf("Syncing roles for %s %t\n", guild.Name, guildState.daily_roles)
	guildState.db.GoForEach(func(discord_id, advent_id string) {
		member, ok := leaderboard.GetMemberByID(advent_id)
		if !ok {
			log.Printf("Error (SyncAllRoles): Member %s not found\n", advent_id)
			return
		}

		// Get the member
		guildMember, err := bot.session.GuildMember(guild.ID, discord_id)
		if err != nil {
			log.Printf("Error (SyncAllRoles): Guild Member %s not found\n", discord_id)
			return
		}

		err = bot.syncRoles(guild, guildMember, member, guildState.daily_roles)
		if err != nil {
			return
		}
	})

	return nil
}

// syncRoles reduces code duplication between SyncRoles and SyncAllRoles
func (bot *Bot) syncRoles(guild *discordgo.Guild, guildMember *discordgo.Member, member *Member, daily_roles bool) error {
	stars := member.Stars

	// 10, 20, 30, 40, 50 stars
	for _, starCount := range []int{10, 20, 30, 40, 50} {
		role := fmt.Sprintf("%d Stars", starCount)
		err := bot.AddOrRemoveRole(guild, guildMember, role, stars >= starCount)
		if err != nil {
			log.Println("Error (syncRoles) adding/removing role: ", err)
			return err
		}
		time.Sleep(1 * time.Second)
	}

	// Connected
	err := bot.AddRole(guild, guildMember, "Connected")
	if err != nil {
		log.Println("Error (syncRoles) adding role: ", err)
		return err
	}

	// Day 1, 2, 3, ..., 25
	for day := 1; day <= 25; day++ {
		role := fmt.Sprintf("Day %02d", day)
		shouldAdd := member.CompletionDayLevel[day] != nil && len(member.CompletionDayLevel[day]) > 0
		err := bot.AddOrRemoveRole(guild, guildMember, role, shouldAdd && daily_roles)
		if err != nil {
			log.Println("Error (syncRoles) adding/removing role: ", err)
			return err
		}
		time.Sleep(1 * time.Second)
	}

	return nil
}

// AddOrRemoveRole adds or removes a role from a user
func (bot *Bot) AddOrRemoveRole(guild *discordgo.Guild, member *discordgo.Member, name string, add bool) error {
	if add {
		return bot.AddRole(guild, member, name)
	}

	return bot.RemoveRole(guild, member, name)
}

// ToggleRole toggles a role for a user
func (bot *Bot) ToggleRole(guild *discordgo.Guild, member *discordgo.Member, name string) (bool, error) {
	give := !bot.HasRole(guild, member, name)
	return give, bot.AddOrRemoveRole(guild, member, name, give)
}

// AddRole adds a role to a user
func (bot *Bot) AddRole(guild *discordgo.Guild, member *discordgo.Member, name string) error {
	// Check if the user already has the role
	if bot.HasRole(guild, member, name) {
		return nil
	}

	log.Printf("Adding role %s to %s\n", name, member.User.Username)
	for _, role := range guild.Roles {
		if role.Name == name {
			return bot.session.GuildMemberRoleAdd(guild.ID, member.User.ID, role.ID)
		}
	}

	return ErrDoesNotExist
}

// RemoveRole removes a role from a user
func (bot *Bot) RemoveRole(guild *discordgo.Guild, member *discordgo.Member, name string) error {
	// Check if the user already doesn't have the role
	if !bot.HasRole(guild, member, name) {
		return nil
	}

	log.Printf("Removing role %s from %s\n", name, member.User.Username)
	for _, role := range guild.Roles {
		if role.Name == name {
			return bot.session.GuildMemberRoleRemove(guild.ID, member.User.ID, role.ID)
		}
	}

	return ErrDoesNotExist
}

// HasRole checks if a user has a role
func (bot *Bot) HasRole(guild *discordgo.Guild, member *discordgo.Member, name string) bool {
	// Get the role ID
	var roleID string
	for _, role := range guild.Roles {
		if role.Name == name {
			roleID = role.ID
			break
		}
	}

	// Check if the user has the role
	for _, role := range member.Roles {
		if role == roleID {
			return true
		}
	}

	return false
}

// RemoveAllRoles removes all managed roles from a user
func (bot *Bot) RemoveAllRoles(guild *discordgo.Guild, member *discordgo.Member) error {
	managedRoles := []string{
		"10 Stars", "20 Stars", "30 Stars", "40 Stars", "50 Stars",
		"Day 1", "Day 2", "Day 3", "Day 4", "Day 5",
		"Day 6", "Day 7", "Day 8", "Day 9", "Day 10",
		"Day 11", "Day 12", "Day 13", "Day 14", "Day 15",
		"Day 16", "Day 17", "Day 18", "Day 19", "Day 20",
		"Day 21", "Day 22", "Day 23", "Day 24", "Day 25",
		"Spoiler", "Connected",
	}

	for _, roleID := range member.Roles {
		// Get the name of the role
		var roleName string
		for _, role := range guild.Roles {
			if role.ID == roleID {
				roleName = role.Name
				break
			}
		}

		// Check if the role is managed
		managed := false
		for _, managedRole := range managedRoles {
			if roleName == managedRole {
				managed = true
				break
			}
		}

		if managed {
			err := bot.session.GuildMemberRoleRemove(guild.ID, member.User.ID, roleID)
			if err != nil {
				return err
			}
		}

	}

	return nil
}

// Bit-mask to be considered an admin
const isAdmin = discordgo.PermissionAdministrator | discordgo.PermissionManageRoles

// IsAdmin checks if a user is an admin
func (bot *Bot) IsAdmin(member *discordgo.Member) bool {
	return member.Permissions&isAdmin != 0
}
