package handlers

import (
	"fmt"
	"strings"

	"github.com/andreyst/tracker-messenger-bridge/bot"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"gopkg.in/go-playground/webhooks.v5/github"
)

// GithubIssueEventHandler - explains channel bumping policy
type GithubIssueEventHandler struct{}

// Handle - handle event
func (GithubIssueEventHandler) Handle(b *bot.Bot, event interface{}) bool {
	issue, ok := event.(github.IssuesPayload)
	if !ok {
		return false
	}

	fmt.Printf("NEW ISSUE====\n%+v\n", issue)

	msg := prepareMsg(issue, b.TelegramChatID, b.TelegramReplacer)
	m, err := b.TelegramClient.Send(msg)
	if err != nil {
		fmt.Printf("Error sending to Telegram: %v\n", err)
	}

	b.MessagesMap[int64(m.MessageID)] = bot.Issue{
		Owner:       issue.Repository.Owner.Login,
		Repo:        issue.Repository.Name,
		Number:      issue.Issue.Number,
		URL:         issue.Issue.URL,
		Author:      issue.Issue.User.Login,
		AuthorURL:   fmt.Sprintf("https://github.com/%s", issue.Issue.User.Login),
		Title:       issue.Issue.Title,
		Description: issue.Issue.Body,
	}

	return true
}

func prepareMsg(issue github.IssuesPayload, telegramChatID int64, replacer *strings.Replacer) tgbotapi.MessageConfig {
	msgText := fmt.Sprintf(
		"New issue: \\#%d [%s](%s) by [%s](https://github.com/%s)\nDescription:\n%s",
		issue.Issue.Number,
		issue.Issue.Title,
		issue.Issue.URL,
		issue.Issue.User.Login,
		issue.Issue.User.Login,
		replacer.Replace(issue.Issue.Body),
	)
	msg := tgbotapi.NewMessage(telegramChatID, msgText)
	msg.ParseMode = "MarkdownV2"
	msg.DisableWebPagePreview = true

	return msg
}
