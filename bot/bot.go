package bot

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andreyst/tracker-messenger-bridge/storage"
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

	DB *sql.DB

	TelegramClient *tgbotapi.BotAPI
	GithubClient   *github.Client

	EventsChan chan interface{}

	Pollers  map[string]Poller
	Webhooks map[string]Webhook

	EventHandlers []EventHandler

	CommentsMap map[int64]int64
	MessagesMap map[int64]interface{}

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

// Poller - Poller handler
type Poller interface {
	Start(bot *Bot)
}

// Webhook - Webhook handler
type Webhook interface {
	Handle(bot *Bot, r *http.Request)
}

// EventHandler - event handler
type EventHandler interface {
	Handle(bot *Bot, event interface{}) bool
}

// NewBot - Creates and initializes new Bot instance
func NewBot() (*Bot, error) {
	// TODO: move chatID to configuration options
	b := &Bot{
		TelegramChatID: chatID,

		Webhooks: make(map[string]Webhook),

		EventsChan: make(chan interface{}),

		CommentsMap: make(map[int64]int64),
		MessagesMap: make(map[int64]interface{}),
	}

	b.DB = storage.NewDB(os.Getenv("DB_PATH"))

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
func (b *Bot) Start() {

	b.startPollers()
	b.startWebhooks()

	for event := range b.EventsChan {
		handled := false
		for _, updateHandler := range b.EventHandlers {
			handled = handled || updateHandler.Handle(b, event)
		}
	}

}

func (b *Bot) startPollers() error {
	for _, poller := range b.Pollers {
		go poller.Start(b)
	}

	return nil
}

func (b *Bot) startWebhooks() error {
	newWebhooksChan := make(chan interface{})

	for path := range b.Webhooks {
		// TODO: refactor to separate webhook server
		// TODO: allow for responses from handler, timeout requests
		// TODO: error handle on write errors, return 500
		http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				log.Printf("Unable to read webhook body: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Internal error"))
				return
			}
			if len(body) == 0 {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Bad request"))
				return
			}

			headers, err := json.Marshal(r.Header)
			if err != nil {
				log.Printf("Unable to marshal headers to json: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Internal error"))
				return
			}

			whd := storage.WebhookData{
				Path:    path,
				Headers: string(headers),
				Body:    string(body),
			}

			storage.SaveWebhookData(b.DB, whd)

			// Try to notify webhook data handlers, skip if nobody is listening
			var notification interface{}
			select {
			case newWebhooksChan <- notification:
			default:
			}
		})
	}

	go func() {
		for {
			whd := storage.LoadWebhookData(b.DB)
			if whd == nil {
				// Wait for new webhook to process them immediately
				// or timeout in case some stored webhook data became visible
				// after previous try
				select {
				case <-newWebhooksChan:
				case <-time.After(5 * time.Second):
				}
				continue
			}

			log.Printf("Received message: %v\n", whd.Body)

			var headers http.Header
			err := json.Unmarshal([]byte(whd.Headers), &headers)
			if err != nil {
				log.Printf("Unable to parse JSON headers for message ID: %v, %s\n", whd.RowID, whd.Headers)
				continue
			}

			for webhookPath, webhook := range b.Webhooks {
				if webhookPath == path {
					body := ioutil.NopCloser(bytes.NewReader([]byte(whd.Body)))
					r := &http.Request{
						Method: "POST",
						Body:   body,
						Header: headers,
					}

					webhook.Handle(b, r)
				}
			}

			storage.DeleteWebhookData(b.DB, whd.RowID)
		}
	}()

	port, err := initPort()
	if err != nil {
		log.Fatalf("Unable to init webhook port: %v\n", err)
		return err
	}

	addr := fmt.Sprintf(":%d", port)
	go http.ListenAndServe(addr, nil)
	log.Printf("Listening %s for webhooks", addr)

	return nil
}

func initPort() (int, error) {
	portStr := os.Getenv("PORT")
	if portStr == "" {
		log.Fatalf("Missing PORT env variable")
		return 0, errors.New("Missing PORT env variable")
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Fatalf("Incorrect PORT value (expected int): %v\n", err)
		return 0, err
	}

	return port, nil
}

// AddPoller - add a poller
func (b *Bot) AddPoller(path string, poller Poller) {
	b.Pollers[path] = poller
}

// AddWebhook - add a webhook
func (b *Bot) AddWebhook(path string, webhook Webhook) {
	b.Webhooks[path] = webhook
}

// AddEventHandler - add an event handler
func (b *Bot) AddEventHandler(eventHandler EventHandler) {
	b.EventHandlers = append(b.EventHandlers, eventHandler)
}
