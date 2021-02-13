package main

import (
	"log"
	"os"

	bot "github.com/andreyst/tracker-messenger-bridge/bot"
	"github.com/andreyst/tracker-messenger-bridge/handlers"
	"github.com/andreyst/tracker-messenger-bridge/webhooks"
	"github.com/joho/godotenv"
)

// Important:
// TODO: find staff user by telegram
// TODO: persist maps
// TODO: find out about single connection

// TODO: make error handling in hooks/updates more robust
// TODO: redo env vars to configuration options + env vars
// TODO: check if env vars exist

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	bot, err := bot.NewBot()
	if err != nil {
		log.Fatalf("Unable to create bot: %v\n", err)
		os.Exit(1)
	}

	bot.AddWebhook("/github", webhooks.GithubWebhook{})

	bot.AddEventHandler(handlers.NoBumpingEventHandler{})
	bot.AddEventHandler(handlers.ReplyToCommentEventHandler{})

	bot.Start()
}
