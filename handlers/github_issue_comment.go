package handlers

import (
	"fmt"

	"github.com/andreyst/tracker-messenger-bridge/bot"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"gopkg.in/go-playground/webhooks.v5/github"
)

// GithubIssueCommentEventHandler - explains channel bumping policy
type GithubIssueCommentEventHandler struct{}

// Handle - handle event
func (GithubIssueCommentEventHandler) Handle(b *bot.Bot, event interface{}) bool {
	comment, ok := event.(github.IssueCommentPayload)
	if !ok {
		return false
	}

	fmt.Printf("NEW COMMENT====\n%+v\n", comment)

	_, ok = b.CommentsMap[comment.Comment.ID]
	fmt.Printf("Own comment: %t\n", ok)
	if ok {
		// Own comment, skipping
		return true
	}

	msgText := fmt.Sprintf(
		"Comment on \\#%d [%s](%s) by [%s](https://github.com/%s):\n%s",
		comment.Issue.Number,
		comment.Issue.Title,
		comment.Issue.URL,
		comment.Sender.Login,
		comment.Sender.Login,
		b.TelegramReplacer.Replace(comment.Comment.Body),
	)

	msg := tgbotapi.NewMessage(b.TelegramChatID, msgText)
	msg.ParseMode = "MarkdownV2"
	msg.DisableWebPagePreview = true
	m, err := b.TelegramClient.Send(msg)
	if err != nil {
		fmt.Printf("Error sending to Telegram: %v\n", err)
	}

	b.MessagesMap[int64(m.MessageID)] = bot.Comment{
		ID:          comment.Comment.ID,
		URL:         comment.Comment.URL,
		IssueOwner:  comment.Repository.Owner.Login,
		IssueRepo:   comment.Repository.Name,
		IssueNumber: comment.Issue.Number,
		IssueURL:    comment.Issue.URL,
		Author:      comment.Comment.User.Login,
		AuthorURL:   fmt.Sprintf("https://github.com/%s", comment.Comment.User.Login),
		Body:        comment.Comment.Body,
	}

	return true
}
