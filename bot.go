package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// numDays is the number of puzzle days in the event
const numDays = 12

// starTiers are the star counts that earn a milestone role
var starTiers = []int{4, 8, 12, 16, 20, 24}

// starColors are the colors of each milestone role, indexed alongside starTiers
var starColors = []int{
	0x2ECC71, // Green
	0x1ABC9C, // Teal
	0x3498DB, // Blue
	0x9B59B6, // Purple
	0xE91E63, // Red/Pink
	0xF1C40F, // Gold
}

// maxStars is the number of stars available in the event (2 per day)
const maxStars = numDays * 2

// Guard the milestone tables against drift if numDays ever changes again
func init() {
	if len(starTiers) != len(starColors) {
		panic(fmt.Sprintf("starTiers (%d) and starColors (%d) must be the same length", len(starTiers), len(starColors)))
	}

	if len(starTiers) == 0 || starTiers[len(starTiers)-1] != maxStars {
		panic(fmt.Sprintf("the final star tier must be %d (%d days x 2 stars)", maxStars, numDays))
	}
}

// starRole is the name of the role earned by collecting a given number of stars
func starRole(stars int) string {
	return fmt.Sprintf("%d Stars", stars)
}

// dayRole is the name of the role earned by making progress on a given day
func dayRole(day int) string {
	return fmt.Sprintf("Day %02d", day)
}

// managedRoles is every role the bot hands out
func managedRoles() []string {
	roles := []string{"Spoiler", "Connected"}
	for _, stars := range starTiers {
		roles = append(roles, starRole(stars))
	}
	for day := 1; day <= numDays; day++ {
		roles = append(roles, dayRole(day))
	}
	return roles
}

// roleID looks up a guild role by name
func roleID(guild *discordgo.Guild, name string) (string, bool) {
	for _, role := range guild.Roles {
		if role.Name == name {
			return role.ID, true
		}
	}

	return "", false
}

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
// 1 role for each day + 1 role for each star milestone
func (bot *Bot) CreateRoles(guild *discordgo.Guild) error {
	yes := true

	// Get the guild state
	guildState, ok := bot.states[guild.ID]
	if !ok {
		return ErrNotConfigured
	}

	// Spoiler role (allows users to access all channels)
	err := bot.createRole(guild, &discordgo.RoleParams{
		Name:        "Spoiler",
		Mentionable: &yes,
	})
	if err != nil {
		return err
	}

	// Connected marks users who have claimed an Advent of Code account
	connected := 0x1ABC9C // Less Green
	err = bot.createRole(guild, &discordgo.RoleParams{
		Name:        "Connected",
		Color:       &connected,
		Mentionable: &yes,
		Hoist:       &yes,
	})
	if err != nil {
		return err
	}

	// One hoisted role per star milestone
	for i, stars := range starTiers {
		color := starColors[i]
		err = bot.createRole(guild, &discordgo.RoleParams{
			Name:        starRole(stars),
			Color:       &color,
			Mentionable: &yes,
			Hoist:       &yes,
		})
		if err != nil {
			return err
		}
	}

	if !guildState.daily_roles {
		return nil
	}

	for day := 1; day <= numDays; day++ {
		err = bot.createRole(guild, &discordgo.RoleParams{Name: dayRole(day)})
		if err != nil {
			return err
		}
	}

	return nil
}

// createRole creates a role, unless the guild already has one with that name
func (bot *Bot) createRole(guild *discordgo.Guild, params *discordgo.RoleParams) error {
	if _, ok := roleID(guild, params.Name); ok {
		return nil
	}

	_, err := bot.session.GuildRoleCreate(guild.ID, params)
	return err
}

// dayRolePattern matches the day roles the bot has ever created, e.g. "Day 07" or "Day 25"
var dayRolePattern = regexp.MustCompile(`^Day (\d{1,2})$`)

// starRolePattern matches the milestone roles the bot has ever created, e.g. "20 Stars"
var starRolePattern = regexp.MustCompile(`^(\d{1,3}) Stars$`)

// staleRoles are roles the bot created for a previous event shape (e.g. "Day 25" or
// "50 Stars" from the 25 day era) and no longer hands out
func staleRoles(guild *discordgo.Guild) []*discordgo.Role {
	var stale []*discordgo.Role

	for _, role := range guild.Roles {
		if match := dayRolePattern.FindStringSubmatch(role.Name); match != nil {
			day, err := strconv.Atoi(match[1])
			if err == nil && (day < 1 || day > numDays) {
				stale = append(stale, role)
			}
			continue
		}

		if match := starRolePattern.FindStringSubmatch(role.Name); match != nil {
			stars, err := strconv.Atoi(match[1])
			if err == nil && !slices.Contains(starTiers, stars) {
				stale = append(stale, role)
			}
		}
	}

	return stale
}

// CleanupRoles deletes roles left over from a previous event shape, such as the
// "Day 13"-"Day 25" and "30/40/50 Stars" roles from when the event ran 25 days.
//
// Deleting the role removes it from every member, so this also cleans up members.
func (bot *Bot) CleanupRoles(guild *discordgo.Guild) error {
	for _, role := range staleRoles(guild) {
		log.Printf("Deleting stale role %s from %s\n", role.Name, guild.Name)

		err := bot.session.GuildRoleDelete(guild.ID, role.ID)
		if err != nil {
			return fmt.Errorf("deleting role %q: %w", role.Name, err)
		}

		time.Sleep(1 * time.Second)
	}

	return nil
}

// ErrInvalidDay is returned when a day falls outside of the event
var ErrInvalidDay = fmt.Errorf("day must be between 1 and %d", numDays)

// SetupChannel sets up a channel for use by a given day (and spoiler)
func (bot *Bot) SetupChannel(guild *discordgo.Guild, channelID string, day int) error {
	if day < 1 || day > numDays {
		return ErrInvalidDay
	}

	overwrites := []struct {
		role  string
		allow int64
		deny  int64
	}{
		{dayRole(day), discordgo.PermissionViewChannel, 0},
		{"Spoiler", discordgo.PermissionViewChannel, 0},
		{"@everyone", 0, discordgo.PermissionViewChannel},
	}

	for _, overwrite := range overwrites {
		id, ok := roleID(guild, overwrite.role)
		if !ok {
			return fmt.Errorf("role %q: %w", overwrite.role, ErrDoesNotExist)
		}

		err := bot.session.ChannelPermissionSet(channelID, id, discordgo.PermissionOverwriteTypeRole, overwrite.allow, overwrite.deny)
		if err != nil {
			return err
		}
	}

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
	var wg sync.WaitGroup
	guildState.db.RLock()
	for discordID, adventID := range guildState.db.mappings {
		wg.Go(func() {
			member, ok := leaderboard.GetMemberByID(adventID)
			if !ok {
				log.Printf("Error (SyncAllRoles): Member %s not found\n", adventID)
				return
			}

			// Get the member
			guildMember, err := bot.session.GuildMember(guild.ID, discordID)
			if err != nil {
				log.Printf("Error (SyncAllRoles): Guild Member %s not found\n", discordID)
				return
			}

			bot.syncRoles(guild, guildMember, member, guildState.daily_roles)
		})
	}
	guildState.db.RUnlock()

	wg.Wait()

	return nil
}

// syncRoles reduces code duplication between SyncRoles and SyncAllRoles
func (bot *Bot) syncRoles(guild *discordgo.Guild, guildMember *discordgo.Member, member *Member, daily_roles bool) error {
	// Star milestones
	for _, stars := range starTiers {
		err := bot.syncRole(guild, guildMember, starRole(stars), member.Stars >= stars)
		if err != nil {
			return err
		}
	}

	// Connected
	err := bot.syncRole(guild, guildMember, "Connected", true)
	if err != nil {
		return err
	}

	// Day 1, 2, 3, ..., numDays
	for day := 1; day <= numDays; day++ {
		solved := daily_roles && len(member.CompletionDayLevel[day]) > 0
		err := bot.syncRole(guild, guildMember, dayRole(day), solved)
		if err != nil {
			return err
		}
	}

	return nil
}

// syncRole applies a single role change, pausing only when Discord was actually called
func (bot *Bot) syncRole(guild *discordgo.Guild, member *discordgo.Member, name string, add bool) error {
	changed, err := bot.SetRole(guild, member, name, add)
	if err != nil {
		log.Printf("Error (syncRoles) setting role %s on %s: %v\n", name, member.User.Username, err)
		return err
	}

	if changed {
		time.Sleep(1 * time.Second)
	}

	return nil
}

// SetRole adds or removes a role from a user, reporting whether a change was made
func (bot *Bot) SetRole(guild *discordgo.Guild, member *discordgo.Member, name string, add bool) (bool, error) {
	id, ok := roleID(guild, name)
	if !ok {
		return false, ErrDoesNotExist
	}

	// Nothing to do if the user is already in the desired state
	if add == slices.Contains(member.Roles, id) {
		return false, nil
	}

	if add {
		log.Printf("Adding role %s to %s\n", name, member.User.Username)
		return true, bot.session.GuildMemberRoleAdd(guild.ID, member.User.ID, id)
	}

	log.Printf("Removing role %s from %s\n", name, member.User.Username)
	return true, bot.session.GuildMemberRoleRemove(guild.ID, member.User.ID, id)
}

// ToggleRole toggles a role for a user
func (bot *Bot) ToggleRole(guild *discordgo.Guild, member *discordgo.Member, name string) (bool, error) {
	give := !bot.HasRole(guild, member, name)
	_, err := bot.SetRole(guild, member, name, give)
	return give, err
}

// HasRole checks if a user has a role
func (bot *Bot) HasRole(guild *discordgo.Guild, member *discordgo.Member, name string) bool {
	id, ok := roleID(guild, name)
	return ok && slices.Contains(member.Roles, id)
}

// RemoveAllRoles removes all managed roles from a user
func (bot *Bot) RemoveAllRoles(guild *discordgo.Guild, member *discordgo.Member) error {
	for _, name := range managedRoles() {
		// Guilds aren't required to have every managed role (e.g. daily roles)
		_, err := bot.SetRole(guild, member, name, false)
		if err != nil && err != ErrDoesNotExist {
			return err
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
