package main

import (
	"log"
	"os"

	bot "github.com/andreyst/tracker-messenger-bridge/bot"
	"github.com/andreyst/tracker-messenger-bridge/bot/updatehandlers"
	"github.com/andreyst/tracker-messenger-bridge/bot/webhookhandlers"
)

// Important:
// TODO: find user by telegram
// TODO: persist maps

// TODO: add queues for hooks
// TODO: make error handling in hooks/updates more robust
// TODO: redo env vars to configuration options + env vars
// TODO: check if env vars exist

func main() {
	bot, err := bot.NewBot()
	if err != nil {
		log.Fatalf("Unable to create bot: %v\n", err)
		os.Exit(1)
	}

	bot.AddUpdateHandler(updatehandlers.NoBumpingUpdateHandler{})
	bot.AddUpdateHandler(updatehandlers.ReplyToCommentUpdateHandler{})
	bot.AddWebhookHandler(webhookhandlers.GithubWebhookHandler{})
	bot.Start()
}
