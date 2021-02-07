package updatehandlers

import (
	"context"
	"fmt"
	"log"
	"regexp"

	bot "github.com/andreyst/tracker-messenger-bridge/bot"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/google/go-github/github"
)

// ReplyToCommentUpdateHandler - explains channel bumping policy
type ReplyToCommentUpdateHandler struct{}

// Handle - handles update
func (ReplyToCommentUpdateHandler) Handle(b *bot.Bot, update tgbotapi.Update) bool {
	fmt.Println("trying to handle")
	if update.Message == nil {
		return false
	}

	fmt.Println("trying to handle2")
	if update.Message.ReplyToMessage == nil {
		return false
	}

	b.Mutex.Lock()
	_, isOwn := b.MessagesMap[int64(update.Message.MessageID)]
	issueOrComment, isOwnReply := b.MessagesMap[int64(update.Message.ReplyToMessage.MessageID)]
	fmt.Printf("isOwn: %t, isOwnReply: %t\n", isOwn, isOwnReply)
	if isOwn || !isOwnReply {
		// Own comment or not a reply to own comment, skipping
		return false
	}
	b.Mutex.Unlock()

	var issueOwner string
	var issueRepo string
	var issueNumber int
	var commentBody string

	if issueOrComment.Issue != nil {
		issueOwner = issueOrComment.Issue.Owner
		issueRepo = issueOrComment.Issue.Repo
		issueNumber = int(issueOrComment.Issue.Number)

		commentBody = fmt.Sprintf("%s@ replies:\n%s",
			update.Message.From.UserName,
			update.Message.Text,
		)
	} else if issueOrComment.Comment != nil {
		issueOwner = issueOrComment.Comment.IssueOwner
		issueRepo = issueOrComment.Comment.IssueRepo
		issueNumber = int(issueOrComment.Comment.IssueNumber)

		re := regexp.MustCompile(`(?m)^(.*)$`)
		sub := `> $1`
		commentBody = fmt.Sprintf("%s\n\n%s@ replies:\n%s",
			re.ReplaceAllString(issueOrComment.Comment.Body, sub),
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
	b.Mutex.Lock()
	b.CommentsMap[*c.ID] = int64(update.Message.MessageID)
	b.Mutex.Unlock()

	return true
}
