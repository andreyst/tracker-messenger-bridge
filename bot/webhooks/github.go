package webhooks

import (
	"fmt"
	"net/http"
	"os"

	bot "github.com/andreyst/tracker-messenger-bridge/bot"
	"gopkg.in/go-playground/webhooks.v5/github"
)

// GithubWebhook - handle for github webhook
type GithubWebhook struct{}

// Handle - handle github webhook
func (GithubWebhook) Handle(b *bot.Bot, r *http.Request) {
	hook, _ := github.New(github.Options.Secret(os.Getenv("GITHUB_WEBHOOK_SECRET")))
	payload, err := hook.Parse(r, github.IssuesEvent, github.IssueCommentEvent)
	fmt.Printf("===NEW PAYLOAD:\n%v\n", payload)
	if err != nil {
		if err == github.ErrEventNotFound {
			fmt.Printf("Skipping github event\n")
			// ok event wasn't one of the ones asked to be parsed
		} else {
			fmt.Printf("Github hook parse: %v\n", err)
		}
	}

	b.EventsChan <- payload
}
