package updatehandlers

import (
	"encoding/json"
	"fmt"

	bot "github.com/andreyst/tracker-messenger-bridge/bot"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// NoBumpingUpdateHandler - explains channel bumping policy
type NoBumpingUpdateHandler struct{}

// Handle - handles update
func (NoBumpingUpdateHandler) Handle(bot *bot.Bot, update tgbotapi.Update) bool {
	if update.Message == nil {
		return false
	}

	if update.Message.Text != fmt.Sprintf("/noup@%s", bot.UserName) {
		return false
	}

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Please do not bump!")
	if update.Message.ReplyToMessage != nil {
		msg.ReplyToMessageID = update.Message.ReplyToMessage.MessageID
		msg.Text = fmt.Sprintf("@%s Please do not bump!", update.Message.From.UserName)
	}

	if m, err2 := bot.TelegramClient.Send(msg); err2 != nil {
		fmt.Printf("Error replying: %v\n", err2)
	} else {
		b, _ := json.MarshalIndent(m, "", "  ")
		fmt.Printf("Sent message: %v\n", string(b))
	}

	return true
}
