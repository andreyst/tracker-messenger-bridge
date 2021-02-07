package webhookhandlers

import (
	"fmt"
	"net/http"
	"os"

	bot "github.com/andreyst/tracker-messenger-bridge/bot"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"gopkg.in/go-playground/webhooks.v5/github"
)

// GithubWebhookHandler - handle for github webhook
type GithubWebhookHandler struct{}

// Handle - handle github webhook
func (GithubWebhookHandler) Handle(b *bot.Bot, w http.ResponseWriter, r *http.Request) {
	hook, _ := github.New(github.Options.Secret(os.Getenv("GITHUB_WEBHOOK_SECRET")))

	payload, err := hook.Parse(r, github.IssuesEvent, github.IssueCommentEvent)
	fmt.Printf("===NEW PAYLOAD:\n%v\n", payload)
	if err != nil {
		if err == github.ErrEventNotFound {
			// ok event wasn't one of the ones asked to be parsed
		} else {
			fmt.Printf("Error: %v\n", err)
		}
	}

	switch payload.(type) {

	case github.IssuesPayload:
		// TODO:
		// else send to telegram and store as pair message_id:issue
		// take telegram lock while sending
		issue := payload.(github.IssuesPayload)
		// Do whatever you want from here...
		fmt.Printf("NEW ISSUE====\n%+v\n", issue)

		b.Mutex.Lock()
		msgText := fmt.Sprintf(
			"New issue: \\#%d [%s](%s) by [%s](https://github.com/%s)\nDescription:\n%s",
			issue.Issue.Number,
			issue.Issue.Title,
			issue.Issue.URL,
			issue.Issue.User.Login,
			issue.Issue.User.Login,
			b.TelegramReplacer.Replace(issue.Issue.Body),
		)
		msg := tgbotapi.NewMessage(b.TelegramChatID, msgText)
		msg.ParseMode = "MarkdownV2"
		msg.DisableWebPagePreview = true
		m, err := b.TelegramClient.Send(msg)
		if err != nil {
			fmt.Printf("Error sending to Telegram: %v\n", err)
		}
		b.MessagesMap[int64(m.MessageID)] = bot.IssueOrComment{
			Issue: &bot.Issue{
				Owner:       issue.Repository.Owner.Login,
				Repo:        issue.Repository.Name,
				Number:      issue.Issue.Number,
				URL:         issue.Issue.URL,
				Author:      issue.Issue.User.Login,
				AuthorURL:   fmt.Sprintf("https://github.com/%s", issue.Issue.User.Login),
				Title:       issue.Issue.Title,
				Description: issue.Issue.Body,
			},
		}
		b.Mutex.Unlock()

	case github.IssueCommentPayload:
		// TODO: check if comment is own, ignore
		// else send to telegram and store as pair message_id:issue comment
		// take telegram lock while sending
		comment := payload.(github.IssueCommentPayload)
		// Do whatever you want from here...
		fmt.Printf("NEW COMMENT====\n%+v\n", comment)

		b.Mutex.Lock()
		_, ok := b.CommentsMap[comment.Comment.ID]
		b.Mutex.Unlock()
		fmt.Printf("Own comment: %t\n", ok)
		if ok {
			// Own comment, skipping
			return
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
		b.Mutex.Lock()
		b.MessagesMap[int64(m.MessageID)] = bot.IssueOrComment{
			Comment: &bot.Comment{
				ID:          comment.Comment.ID,
				URL:         comment.Comment.URL,
				IssueOwner:  comment.Repository.Owner.Login,
				IssueRepo:   comment.Repository.Name,
				IssueNumber: comment.Issue.Number,
				IssueURL:    comment.Issue.URL,
				Author:      comment.Comment.User.Login,
				AuthorURL:   fmt.Sprintf("https://github.com/%s", comment.Comment.User.Login),
				Body:        comment.Comment.Body,
			},
		}
		b.Mutex.Unlock()
	}
}
