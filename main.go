package main

import (
	"log"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
)

func main() {
	file, err := os.Open("config.json")
	if err != nil {
		log.Println("Error opening config file: ", err)
		return
	}

	// Parse the config file
	config, err := ParseConfig(file)
	if err != nil {
		log.Println("Error parsing config file: ", err)
		return
	}

	file.Close()

	// Create a new Discord session using the provided bot token.
	session, err := discordgo.New("Bot " + config.DiscordToken)
	if err != nil {
		log.Println("Error creating Discord session: ", err)
		return
	}

	// Create a new bot
	bot := NewBot(session, config.SessionCookie)

	// Start the bot
	err = bot.Start()
	if err != nil {
		log.Println("Error starting bot: ", err)
		return
	}

	// Add guilds
	for guildID, guildConfig := range config.Guilds {
		err = bot.AddGuild(guildID, guildConfig.Year, guildConfig.LeaderboardID)
		if err != nil {
			log.Println("Error adding guild: ", err)
			return
		}
	}

	// Register commands
	err = bot.RegisterCommands()
	if err != nil {
		log.Println("Error registering commands: ", err)
		return
	}

	// Register handlers
	bot.AddHandlers()

	// Add roles
	for _, guild := range bot.session.State.Guilds {
		err = bot.CreateRoles(guild)
		if err != nil {
			log.Println("Error creating roles: ", err)
			return
		}
	}

	log.Println("Press CTRL-C to exit.")

	// Every 15 minutes, sync the bot with the Advent of Code API
	bot.Sync()

	ticker := time.NewTicker(15 * time.Minute)
	for {
		<-ticker.C
		bot.Sync()
	}
}
