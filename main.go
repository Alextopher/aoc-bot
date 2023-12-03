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
		log.Fatalln("Error opening config file: ", err)
	}

	// Parse the config file
	config, err := ParseConfig(file)
	if err != nil {
		log.Fatalln("Error parsing config file: ", err)
	}

	file.Close()

	// Create a new Discord session using the provided bot token.
	session, err := discordgo.New("Bot " + config.DiscordToken)
	if err != nil {
		log.Fatalln("Error creating Discord session: ", err)
	}

	// Create a new bot
	bot := NewBot(session, config.SessionCookie)

	// Start the bot
	err = bot.Start()
	if err != nil {
		log.Fatalln("Error starting bot: ", err)
	}

	// Add guilds
	for guildID, guildConfig := range config.Guilds {
		err = bot.AddGuild(guildID, guildConfig.Year, guildConfig.LeaderboardID)
		if err != nil {
			log.Fatalln("Error adding guild: ", err)
		}
	}

	// Register commands
	err = bot.RegisterCommands()
	if err != nil {
		log.Fatalln("Error registering commands: ", err)
	}

	// Register handlers
	bot.AddHandlers()

	// Add roles
	for _, guild := range bot.session.State.Guilds {
		err = bot.CreateRoles(guild)
		if err != nil {
			log.Fatalln("Error creating roles: ", err)
		}
	}

	log.Println("Press CTRL-C to exit.")

	// Every 15 minutes, sync the bot with the Advent of Code API
	bot.Sync()

	// Sleep until the next 15 minute mark
	now := time.Now()
	minutes := time.Duration(15 - (now.Minute() % 15))
	seconds := time.Duration(60 - now.Second())
	time.Sleep(minutes*time.Minute + seconds*time.Second)

	ticker := time.NewTicker(15 * time.Minute)

	// Start syncing every 15 minutes
	bot.Sync()
	for {
		<-ticker.C
		bot.Sync()
	}
}
