package pollers

import (
	"encoding/json"
	"fmt"
	"log"

	bot "github.com/andreyst/tracker-messenger-bridge/bot"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// TelegramPoller - polls for Telegram updates
type TelegramPoller struct{}

// Start - starts poller
func (TelegramPoller) Start(b *bot.Bot) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updatesChannel, err := b.TelegramClient.GetUpdatesChan(u)
	if err != nil {
		log.Fatalf("Unable to get updates channel: %v\n", err)
		return err
	}

	for update := range updatesChannel {
		buf, _ := json.MarshalIndent(update, "", "  ")
		fmt.Printf("Telegram update: %v\n", string(buf))

		b.EventsChan <- update
	}

	return nil
}
