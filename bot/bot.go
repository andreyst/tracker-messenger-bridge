package bot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

// TODO: move path to configuration options
const (
	path   = "/github"
	chatID = -277738237
)

// Bot - Bot for bridging issue tracker and messaging system
// This implementation bridges Github issues and Telegram
type Bot struct {
	TelegramChatID int64
	UserName       string

	UpdateHandlers  []UpdateHandler
	WebhookHandlers []WebhookHandler

	TelegramClient *tgbotapi.BotAPI
	GithubClient   *github.Client

	CommentsMap map[int64]int64
	MessagesMap map[int64]IssueOrComment

	Mutex            sync.Mutex
	TelegramReplacer *strings.Replacer
}

// Issue - issue description
type Issue struct {
	Owner       string
	Repo        string
	Number      int64
	URL         string
	Author      string
	AuthorURL   string
	Title       string
	Description string
}

// Comment — comment description
type Comment struct {
	ID          int64
	URL         string
	IssueOwner  string
	IssueRepo   string
	IssueNumber int64
	IssueURL    string
	Author      string
	AuthorURL   string
	Body        string
}

// IssueOrComment - issue or comment
type IssueOrComment struct {
	Issue   *Issue
	Comment *Comment
}

// UpdateHandler - Telegram Bot command handler
type UpdateHandler interface {
	Handle(bot *Bot, update tgbotapi.Update) bool
}

// WebhookHandler - Webhook handler
type WebhookHandler interface {
	Handle(bot *Bot, w http.ResponseWriter, r *http.Request)
}

// NewBot - Creates and initializes new Bot instance
func NewBot() (*Bot, error) {
	// TODO: move chatID to configuration options
	b := &Bot{
		TelegramChatID: chatID,

		CommentsMap: make(map[int64]int64),
		MessagesMap: make(map[int64]IssueOrComment),
	}

	b.TelegramReplacer = strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		"`", "\\`",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
	)

	err := b.initTelegramClient()
	if err != nil {
		log.Fatalf("Unable to init telegram client: %v\n", err)
		return nil, err
	}

	err = b.initTelegramUsername()
	if err != nil {
		log.Fatalf("Unable to init telegram username: %v\n", err)
		return nil, err
	}

	err = b.initGithubClient()
	if err != nil {
		log.Fatalf("Unable to init github client: %v\n", err)
		return nil, err
	}

	return b, nil
}

func (b *Bot) initTelegramClient() error {
	telegramClient, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_TOKEN"))
	if err != nil {
		log.Fatalf("Unable to create telegram client: %v\n", err)
		return err
	}
	b.TelegramClient = telegramClient

	return nil
}

func (b *Bot) initTelegramUsername() error {
	me, err := b.TelegramClient.GetMe()
	if err != nil {
		log.Fatalf("Unable to call getMe: %v\n", err)
		return err
	}
	b.UserName = me.UserName

	return nil
}

func (b *Bot) initGithubClient() error {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	tc := oauth2.NewClient(ctx, ts)
	b.GithubClient = github.NewClient(tc)

	return nil
}

// Start - start processing updates
func (b *Bot) Start() (bool, error) {

	http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		for _, handler := range b.WebhookHandlers {
			handler.Handle(b, w, r)
		}
	})

	portStr := os.Getenv("PORT")
	if portStr == "" {
		log.Fatalf("Missing PORT env variable")
		return false, errors.New("Missing PORT env variable")
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Fatalf("Incorrect PORT value (expected int): %v\n", err)
		return false, err
	}

	go http.ListenAndServe(fmt.Sprintf(":%d", port), nil)

	updatesChannel, err := b.startUpdates()
	if err != nil {
		log.Fatalf("Unable to get updates channel: %v\n", err)
		return false, err
	}

	b.processUpdates(updatesChannel)
	return true, nil
}

// AddUpdateHandler - add an update handler
func (b *Bot) AddUpdateHandler(handler UpdateHandler) {
	b.UpdateHandlers = append(b.UpdateHandlers, handler)
}

// AddWebhookHandler - add a webhook handler
func (b *Bot) AddWebhookHandler(handler WebhookHandler) {
	b.WebhookHandlers = append(b.WebhookHandlers, handler)
}

func (b *Bot) startUpdates() (tgbotapi.UpdatesChannel, error) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updatesChannel, err := b.TelegramClient.GetUpdatesChan(u)
	return updatesChannel, err
}

func (b *Bot) processUpdates(updatesChannel tgbotapi.UpdatesChannel) {
	for update := range updatesChannel {
		buf, _ := json.MarshalIndent(update, "", "  ")
		fmt.Printf("Update: %v\n", string(buf))

		handled := false
		for _, updateHandler := range b.UpdateHandlers {
			handled = handled || updateHandler.Handle(b, update)
		}
	}
}
