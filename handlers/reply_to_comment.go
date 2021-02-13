package handlers

import (
	"context"
	"fmt"
	"log"
	"regexp"

	bot "github.com/andreyst/tracker-messenger-bridge/bot"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/google/go-github/github"
)

// ReplyToCommentEventHandler - explains channel bumping policy
type ReplyToCommentEventHandler struct{}

// Handle - handles update
func (ReplyToCommentEventHandler) Handle(b *bot.Bot, event interface{}) bool {
	update, ok := event.(tgbotapi.Update)
	if !ok {
		return false
	}

	if update.Message == nil {
		return false
	}

	if update.Message.ReplyToMessage == nil {
		return false
	}

	_, isOwn := b.MessagesMap[int64(update.Message.MessageID)]
	source, isOwnReply := b.MessagesMap[int64(update.Message.ReplyToMessage.MessageID)]
	fmt.Printf("isOwn: %t, isOwnReply: %t\n", isOwn, isOwnReply)
	if isOwn || !isOwnReply {
		// Own comment or not a reply to own comment, skipping
		return false
	}

	var issueOwner string
	var issueRepo string
	var issueNumber int
	var commentBody string

	switch source.(type) {
	case bot.Issue:
		issue := source.(bot.Issue)

		issueOwner = issue.Owner
		issueRepo = issue.Repo
		issueNumber = int(issue.Number)

		commentBody = fmt.Sprintf("%s@ replies:\n%s",
			update.Message.From.UserName,
			update.Message.Text,
		)
	case bot.Comment:
		comment := source.(bot.Comment)

		issueOwner = comment.IssueOwner
		issueRepo = comment.IssueRepo
		issueNumber = int(comment.IssueNumber)

		re := regexp.MustCompile(`(?m)^(.*)$`)
		sub := `> $1`
		commentBody = fmt.Sprintf("%s\n\n%s@ replies:\n%s",
			re.ReplaceAllString(comment.Body, sub),
			update.Message.From.UserName,
			update.Message.Text,
		)
	}

	comment := &github.IssueComment{
		Body: &commentBody,
	}

	c, _, err := b.GithubClient.Issues.CreateComment(context.Background(), issueOwner, issueRepo, issueNumber, comment)
	if err != nil {
		log.Fatalf("Error posting comment to issue: %v\n", err)
	}
	b.CommentsMap[*c.ID] = int64(update.Message.MessageID)

	return true
}
