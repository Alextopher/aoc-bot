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
			Name:        "stars",
			Description: "Returns how many stars you have collected (debugging)",
			Type:        discordgo.ChatApplicationCommand,
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

	for _, command := range commands {
		_, err := bot.session.ApplicationCommandCreate(bot.session.State.User.ID, "", command)
		if err != nil {
			return err
		}
	}

	return nil
}

// AddHandlers adds the bot's discordgo handlers
func (bot *Bot) AddHandlers() {
	bot.session.AddHandler(bot.onInteractionCreate)
}

func (bot *Bot) onInteractionCreate(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	if interaction.Type != discordgo.InteractionApplicationCommand {
		return
	}

	switch interaction.ApplicationCommandData().Name {
	case "claim":
		bot.onClaim(interaction)
	case "stars":
		bot.onStars(interaction)
	case "source":
		bot.respondToInteraction(interaction, "https://github.com/Alextopher/aocbot", false)
	case "help":
		bot.onHelp(interaction)
	}
}

func (bot *Bot) onClaim(interaction *discordgo.InteractionCreate) {
	username := interaction.ApplicationCommandData().Options[0].StringValue()

	log.Printf("Trying to claim username %s for @%s", username, interaction.Member.User.Username)

	// Get the guild state
	guildState, ok := bot.states[interaction.GuildID]
	if !ok {
		bot.respondToInteraction(interaction, "Error: This guild is not configured, yet.", true)
		return
	}

	// Try to claim the user by name
	err := guildState.ClaimName(interaction.Member.User.ID, username)
	if err == ErrDoesNotExist {
		// If the user doesn't exist, try to claim by ID
		err = guildState.ClaimID(interaction.Member.User.ID, username)
	}

	if err == ErrDoesNotExist {
		closeNames, err := guildState.CloseNames(username)
		if err != nil {
			// Report that their name is invalid
			bot.respondToInteraction(interaction, "Error: I couldn't find that user.", true)
			return
		}

		// Build the error message
		message := "Error: I couldn't find that user. Did you mean one of these?\n"
		for _, name := range closeNames {
			message += fmt.Sprintf("- '%s'\n", name)
		}

		bot.respondToInteraction(interaction, message, true)
	} else if err == ErrAlreadyClaimed {
		// Report that the user has already been claimed
		bot.respondToInteraction(interaction, "Error: This user has already been claimed, if you believe this is an error, please contact an administrator", true)
	} else if err != nil {
		// Report that something went wrong
		bot.respondToInteraction(interaction, "Error: Something went wrong, please try again later.", true)
	} else {
		// Report that the user has been claimed
		bot.respondToInteraction(interaction, "Success: You have claimed your Advent of Code user!", true)
	}

	guild, err := bot.session.State.Guild(interaction.GuildID)
	if err != nil {
		log.Println("Error getting guild: ", err)
		return
	}

	// Update roles
	err = bot.SyncRoles(guild)
	if err != nil {
		log.Println("Error syncing roles: ", err)
		return
	}
}

func (bot *Bot) onHelp(interaction *discordgo.InteractionCreate) {
	log.Printf("Help requested by @%s", interaction.Member.User.Username)

	help := "Help:\n- `/claim <username>`: Claims a username by Advent of Code name (or ID)\n- `/stars` returns how many stars I think you have collected (mostly for debugging)\n- `/source`: links my source code\n- `/help`: Shows this help message"
	bot.respondToInteraction(interaction, help, false)
}

func (bot *Bot) onStars(interaction *discordgo.InteractionCreate) {
	log.Printf("Star count requested by @%s", interaction.Member.User.Username)

	guildState, ok := bot.states[interaction.GuildID]
	if !ok {
		bot.respondToInteraction(interaction, "Error: This guild is not configured, yet.", true)
		return
	}

	id, ok := guildState.db.GetAdventID(interaction.Member.User.ID)
	if !ok {
		bot.respondToInteraction(interaction, "Error: You haven't ran `/claim` yet.", true)
		return
	}

	member, ok := guildState.GetLeaderboard().GetMemberByID(id)
	if !ok {
		bot.respondToInteraction(interaction, "Error: Something odd happened here, did you quit the leaderboard?", true)
		return
	}

	msg := fmt.Sprintf("You have collected **%d** stars!", member.Stars)
	bot.respondToInteraction(interaction, msg, true)
}

// respondToInteraction responds to a new interaction that hasn't been deferred
func (bot *Bot) respondToInteraction(i *discordgo.InteractionCreate, content string, isEphemeral bool) {
	flags := discordgo.MessageFlags(0)
	if isEphemeral {
		flags = discordgo.MessageFlagsEphemeral
	}

	err := bot.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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
