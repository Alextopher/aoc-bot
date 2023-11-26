package main

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

// RegisterCommands registers the bot's commands with Discord
func (bot *Bot) RegisterCommands() error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "claim",
			Description: "Claims a username or user ID",
			Type:        discordgo.ChatApplicationCommand,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "username",
					Description: "The username to claim",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
			},
		},
		{
			Name:        "unclaim",
			Description: "Removes your claim to an advent of code account",
			Type:        discordgo.ChatApplicationCommand,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "member",
					Description: "The _discord_ user to remove the claim from (Admin only)",
					Type:        discordgo.ApplicationCommandOptionUser,
					Required:    false,
				},
			},
		},
		{
			Name:        "stars",
			Description: "Returns how many stars you have collected (debugging)",
			Type:        discordgo.ChatApplicationCommand,
		},
		{
			Name:        "spoilers",
			Description: "Gives you access to the spoiler channels (toggle)\n",
			Type:        discordgo.ChatApplicationCommand,
		},
		{
			Name:        "setup",
			Description: "Sets up this channel for use as a spoiler channel for a given day (Admin only)",
			Type:        discordgo.ChatApplicationCommand,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "day",
					Description: "The day to set up this channel for",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    true,
				},
			},
		},
		{
			Name:        "source",
			Description: "Returns the source code for the bot",
			Type:        discordgo.ChatApplicationCommand,
		},
		{
			Name:        "help",
			Description: "Shows help",
			Type:        discordgo.ChatApplicationCommand,
		},
	}

	commands, err := bot.session.ApplicationCommandBulkOverwrite(bot.session.State.User.ID, "", commands)
	if err != nil {
		return err
	}

	for _, command := range commands {
		log.Printf("Registered command: %s", command.Name)
	}

	return nil
}

// AddHandlers adds the bot's discordgo handlers
func (bot *Bot) AddHandlers() {
	bot.session.AddHandler(bot.onInteractionCreate)
}

func (bot *Bot) onInteractionCreate(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	i := interaction.Interaction

	if interaction.Type != discordgo.InteractionApplicationCommand {
		return
	}

	switch interaction.ApplicationCommandData().Name {
	case "claim":
		bot.onClaim(i)
	case "unclaim":
		bot.onUnclaim(i)
	case "stars":
		bot.onStars(i)
	case "spoilers":
		bot.onSpoil(i)
	case "setup":
		bot.onSetup(i)
	case "source":
		log.Printf("Source code requested by @%s", interaction.Member.User.Username)
		bot.respondToInteraction(i, "https://github.com/Alextopher/aocbot", false)
	case "help":
		log.Printf("Help requested by @%s", interaction.Member.User.Username)
		msg := "Help:\n"
		msg += "- `/claim <username>`: Claims a username by Advent of Code name (or ID)\n"
		msg += "- `/unclaim`: Removes your claim to an advent of code account\n"
		msg += "- `/unclaim <member>`: Removes another user's claim to an advent of code account (Admin only)\n"
		msg += "- `/stars`: Returns how many stars you have collected (debugging)\n"
		msg += "- `/spoilers`: Gives you access to the spoiler channels (toggle)\n"
		msg += "- `/source`: links my source code\n"
		msg += "- `/help`: Shows this help message"
		bot.respondToInteraction(i, msg, false)
	}
}

func (bot *Bot) onClaim(interaction *discordgo.Interaction) {
	defered := bot.deferInteraction(interaction, true)

	username := interaction.ApplicationCommandData().Options[0].StringValue()

	log.Printf("Trying to claim username %s for @%s", username, interaction.Member.User.Username)

	// Get the guild state
	guildState, ok := bot.states[interaction.GuildID]
	if !ok {
		defered.finalize("Error: This guild is not configured, yet.")
		return
	}

	// Try to claim the user by name
	err := guildState.ClaimName(interaction.Member.User.ID, username)
	if err == ErrDoesNotExist {
		// If the user doesn't exist, try to claim by ID
		err = guildState.ClaimID(interaction.Member.User.ID, username)
	}

	if err == ErrDoesNotExist {
		// If the user still doesn't exist, try to find close names to help the user out
		closeNames, err := guildState.CloseNames(username)
		if err != nil {
			// Report that their name is invalid
			defered.finalize("Error: I couldn't find that user.")
			return
		}

		// Build the error message
		message := "Error: I couldn't find that user. Did you mean one of these?\n"
		for _, name := range closeNames {
			message += fmt.Sprintf("- '%s'\n", name)
		}

		defered.finalize(message)
	} else if err == ErrAlreadyClaimed {
		// Check if this user just tried to re-claim themselves
		aocID, ok := guildState.db.GetAdventID(interaction.Member.User.ID)
		if ok {
			member, ok := guildState.GetLeaderboard().GetMemberByID(aocID)
			if aocID == username || (ok && member.Name == username) {
				// Report that this user just tried to re-claim themselves
				defered.finalize("You have already claimed this user :smile:")
				return
			}
		}

		// Report that the user has already been claimed
		defered.finalize("Error: This user has already been claimed, if you believe this is an error, please contact an administrator")
	} else if err != nil {
		// Report that something went wrong
		defered.finalize("Error: Something went wrong, please try again later.")
	} else {
		// Report that the user has been claimed
		defered.finalize("Success: You have claimed your Advent of Code user!")
	}

	guild, err := bot.session.State.Guild(interaction.GuildID)
	if err != nil {
		log.Println("Error getting guild: ", err)
		return
	}

	// Update roles
	err = bot.SyncMemberRoles(guild, interaction.Member)
	if err != nil {
		log.Println("Error syncing roles: ", err)
		return
	}
}

func (bot *Bot) onUnclaim(interaction *discordgo.Interaction) {
	// Defer the interaction response
	defered := bot.deferInteraction(interaction, true)

	// Get the optional user
	user := interaction.Member.User
	// `self` is true if the user is unclaiming themselves, false if they are unclaiming another user
	self := true
	if len(interaction.ApplicationCommandData().Options) > 0 {
		// Verify that the caller is an admin
		if !bot.IsAdmin(interaction.Member) {
			defered.finalize("Error: You must be an admin to remove another user's claim.")
			return
		}

		user = interaction.ApplicationCommandData().Options[0].UserValue(bot.session)
		self = false
	}

	log.Printf("Unclaim requested by @%s for @%s", interaction.Member.User.Username, user.Username)

	// Get the guild state
	guildState, ok := bot.states[interaction.GuildID]
	if !ok {
		defered.finalize("Error: This guild is not configured, yet.")
		return
	}

	// Try to unclaim the user
	log.Println("Unclaiming user: ", user.ID)
	err := guildState.Unclaim(user.ID)
	if err == ErrDoesNotExist {
		// Report that the discord user never claimed an Advent of Code user
		if self {
			defered.finalize("Success?: You ever claimed an Advent of Code user.")
		} else {
			defered.finalize("Success?: That user never claimed an Advent of Code user.")
		}
	} else if err != nil {
		// Report that something went wrong
		defered.finalize("Error: Something went wrong, please try again later.")
	} else {
		// Report that the user has been unclaimed
		if self {
			defered.finalize("Success: You have unclaimed your Advent of Code user!")
		} else {
			defered.finalize("Success: That user has been unclaimed!")
		}
	}

	guild, err := bot.session.State.Guild(interaction.GuildID)
	if err != nil {
		log.Println("Error getting guild: ", err)
		return
	}

	// Convert User to Member
	member, err := bot.session.GuildMember(guild.ID, user.ID)
	if err != nil {
		log.Println("Error getting guild member: ", err)
		return
	}

	bot.RemoveAllRoles(guild, member)
}

func (bot *Bot) onStars(interaction *discordgo.Interaction) {
	// Defer the interaction response
	defered := bot.deferInteraction(interaction, true)

	log.Printf("Star count requested by @%s", interaction.Member.User.Username)

	guildState, ok := bot.states[interaction.GuildID]
	if !ok {
		defered.finalize("Error: This guild is not configured, yet.")
		return
	}

	id, ok := guildState.db.GetAdventID(interaction.Member.User.ID)
	if !ok {
		defered.finalize("Error: You haven't ran `/claim` yet.")
		return
	}

	member, ok := guildState.GetLeaderboard().GetMemberByID(id)
	if !ok {
		defered.finalize("Error: Something odd happened here, did you quit the leaderboard?")
		return
	}

	msg := fmt.Sprintf("You have collected **%d** stars!", member.Stars)
	defered.finalize(msg)
}

func (bot *Bot) onSpoil(interaction *discordgo.Interaction) {
	defered := bot.deferInteraction(interaction, true)

	log.Printf("Toggling spoilers has been requested by @%s", interaction.Member.User.Username)

	guild, err := bot.session.State.Guild(interaction.GuildID)
	if err != nil {
		log.Println("Error getting guild: ", err)
		defered.finalize("Error: Something went wrong, please try again later.")
		return
	}

	added, err := bot.ToggleRole(guild, interaction.Member, "Spoiler")
	if err != nil {
		log.Println("Error toggling role: ", err)
		defered.finalize("Error: Something went wrong, please try again later.")
	} else if added {
		defered.finalize("Success: You have been given access to the spoiler channels!")
	} else {
		defered.finalize("Success: You have been removed from the spoiler channels!")
	}
}

func (bot *Bot) onSetup(interaction *discordgo.Interaction) {
	// Defer the interaction response
	defered := bot.deferInteraction(interaction, true)

	log.Printf("Setup requested by @%s", interaction.Member.User.Username)

	// Verify that the caller is an admin
	if !bot.IsAdmin(interaction.Member) {
		defered.finalize("Error: You must be an admin to set up a channel.")
		return
	}

	day := interaction.ApplicationCommandData().Options[0].IntValue()

	guild, err := bot.session.Guild(interaction.GuildID)
	if err != nil {
		log.Println("Error getting guild: ", err)
		defered.finalize("Error: Something went wrong, please try again later.")
		return
	}

	// Set up this channel for this day
	err = bot.SetupChannel(guild, interaction.ChannelID, day)
	if err != nil {
		defered.finalize("Error: Something went wrong, please try again later.")
	} else {
		defered.finalize("Success: This channel has been set up for spoilers!")
	}
}

// There are 2 methods for responding to interactions:
// 1. Defer the interaction, then respond to it later
// 2. Respond to the interaction immediately (must never block)
// Either way can be ephemeral, but the choice whether to defer or not must be made immediately
type DeferredInteraction struct {
	interaction *discordgo.Interaction
	bot         *Bot
}

func (bot *Bot) deferInteraction(i *discordgo.Interaction, isEphemeral bool) DeferredInteraction {
	flags := discordgo.MessageFlags(0)
	if isEphemeral {
		flags = discordgo.MessageFlagsEphemeral
	}

	err := bot.session.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: flags,
		},
	})

	if err != nil {
		log.Println("deferInteraction failed while responding to interaction: ", err)
	}

	return DeferredInteraction{
		interaction: i,
		bot:         bot,
	}
}

func (di *DeferredInteraction) finalize(content string) {
	_, err := di.bot.session.InteractionResponseEdit(di.interaction, &discordgo.WebhookEdit{
		Content: &content,
	})

	if err != nil {
		log.Println("finalize failed while responding to interaction: ", err)
	}
}

// respondToInteraction responds to a new interaction that hasn't been deferred
func (bot *Bot) respondToInteraction(i *discordgo.Interaction, content string, isEphemeral bool) {
	flags := discordgo.MessageFlags(0)
	if isEphemeral {
		flags = discordgo.MessageFlagsEphemeral
	}

	err := bot.session.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   flags,
		},
	})

	if err != nil {
		log.Println("respondToInteraction failed while responding to interaction: ", err)
	}
}
