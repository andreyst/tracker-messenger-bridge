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

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
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

	Storage *storage.Storage

	TelegramClient *tgbotapi.BotAPI
	GithubClient   *github.Client
	SqsClient      *sqs.Client

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

	b.Storage = storage.NewStorage(os.Getenv("DB_PATH"))
	// storage2 := storage.NewStorage(os.Getenv("DB_PATH"))

	// row := b.Storage.DB.QueryRow("SELECT is_processing FROM webhooks_data WHERE rowid=1")
	// var isProcessing int
	// err := row.Scan(&isProcessing)
	// if err != nil {
	// 	panic(err)
	// }
	// log.Printf("is processing: %d", isProcessing)

	// go func() {
	// 	log.Printf("Go tx 1")
	// 	tx, err := b.Storage.DB.Begin()
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	tx.Exec("UPDATE webhooks_data SET is_processing = 1 WHERE rowid=1")
	// 	time.Sleep(time.Hour * 1)
	// 	tx.Commit()
	// }()

	// time.Sleep(time.Second * 1)

	// go func() {
	// 	log.Printf("Go tx 2")
	// 	tx, err := b.Storage.DB.Begin()
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	row := tx.QueryRow("SELECT is_processing FROM webhooks_data WHERE rowid=1")
	// 	var isProcessing int
	// 	err = row.Scan(&isProcessing)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	log.Printf("is processing: %d", isProcessing)
	// 	time.Sleep(time.Second * 3)
	// 	log.Printf("Commit tx 2")
	// 	tx.Commit()
	// }()

	// time.Sleep(time.Hour * 1)

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

	err = b.initSqsClient()
	if err != nil {
		log.Fatalf("Unable to init SQS client: %v\n", err)
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

func (b *Bot) initSqsClient() error {
	config, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatal(err)
		return err
	}

	b.SqsClient = sqs.NewFromConfig(config)

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
	for path := range b.Webhooks {
		// TODO: send new messages via chan for quicker response
		// TODO: refactor to separate webhook serber
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

			// queueURL := os.Getenv("GITHUB_WEBHOOK_SQS_QUEUE_URL")
			// _, err = b.SqsClient.SendMessage(context.TODO(), &sqs.SendMessageInput{
			// 	QueueUrl:    &queueURL,
			// 	MessageBody: &strBody,
			// 	MessageAttributes: map[string]types.MessageAttributeValue{
			// 		"Path": {
			// 			DataType:    aws.String("String"),
			// 			StringValue: aws.String(path),
			// 		},
			// 		"Headers": {
			// 			DataType:    aws.String("String"),
			// 			StringValue: aws.String(string(headers)),
			// 		},
			// 	},
			// })
			b.Mutex.Lock()
			_, err = b.Storage.DB.Exec(`
			INSERT INTO webhooks_data(created_at, updated_at, path, headers, body, visible_at) VALUES(
				datetime("now"),
				datetime("now"),
				$1,
				$2,
				$3,
				datetime("now", "+30 seconds")
			)
			`, path, headers, body)
			b.Mutex.Unlock()

			if err != nil {
				log.Printf("Unable to send payload to queue: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Internal error"))
				return
			}
		})
	}

	go func() {
		// queueURL := os.Getenv("GITHUB_WEBHOOK_SQS_QUEUE_URL")
		// messageAttributeNames := []string{
		// 	"Path",
		// 	"Headers",
		// }

		for {
			time.Sleep(time.Second * 3)

			row := b.Storage.DB.QueryRow(`
			SELECT rowid, path, headers, body
			FROM webhooks_data
			WHERE is_processing = 0 AND visible_at < datetime("now")
			LIMIT 1
			`)
			if row == nil {
				continue
			}

			var rowid int
			var path string
			var strHeaders string
			var strBody string

			err := row.Scan(&rowid, &path, &strHeaders, &strBody)
			if err == sql.ErrNoRows {
				continue
			}
			if err != nil {
				panic(err)
			}

			res, err := b.Storage.DB.Exec(`
			UPDATE webhooks_data SET
				is_processing = 1,
				visible_at = datetime("now")
			WHERE rowid = $1 AND is_processing = 0
			`, rowid)
			if err != nil {
				panic(err)
			}

			rowsAffected, err := res.RowsAffected()
			if err != nil {
				panic(err)
			}

			if rowsAffected == 0 {
				continue
			}

			// TODO: wrap into transaction, check isolation level
			log.Printf("Received message: %v\n", strBody)

			var headers http.Header
			err = json.Unmarshal([]byte(strHeaders), &headers)
			if err != nil {
				log.Printf("Unable to parse JSON headers for message ID: %v, %s\n", rowid, strHeaders)
				continue
			}

			for webhookPath, webhook := range b.Webhooks {
				if webhookPath == path {
					body := ioutil.NopCloser(bytes.NewReader([]byte(strBody)))
					r := &http.Request{
						Method: "POST",
						Body:   body,
						Header: headers,
					}

					webhook.Handle(b, r)
				}
			}

			_, err = b.Storage.DB.Exec(`
			DELETE FROM webhooks_data
			WHERE rowid = $1
			`, rowid)
			if err != nil {
				panic(err)
			}

			// 	messages, err := b.SqsClient.ReceiveMessage(context.TODO(), &sqs.ReceiveMessageInput{
			// 		QueueUrl:              &queueURL,
			// 		WaitTimeSeconds:       20,
			// 		MaxNumberOfMessages:   10,
			// 		MessageAttributeNames: messageAttributeNames,
			// 	})
			// 	if err != nil {
			// 		log.Printf("Error receiving messages from queue: %v\n", err)
			// 		continue
			// 	}

			// 	log.Printf("Received %d messages\n", len(messages.Messages))
			// 	if len(messages.Messages) == 0 {
			// 		continue
			// 	}

			// 	del := []types.DeleteMessageBatchRequestEntry{}

			// 	for _, message := range messages.Messages {
			// 		del = append(del, types.DeleteMessageBatchRequestEntry{
			// 			Id:            message.MessageId,
			// 			ReceiptHandle: message.ReceiptHandle,
			// 		})

			// 		messagePathAttr, ok := message.MessageAttributes["Path"]
			// 		if !ok {
			// 			log.Printf("Skipping message, no Path attribute: %v\n", message)
			// 			continue
			// 		}

			// 		if messagePathAttr.StringValue == nil {
			// 			log.Printf("Nil StringValue for Path attribute for message %v\n", message.MessageId)
			// 		}

			// 		headersAttr, ok := message.MessageAttributes["Headers"]
			// 		if !ok {
			// 			log.Printf("Skipping message, no Headers attribute: %v\n", message)
			// 			continue
			// 		}

			// 		if headersAttr.StringValue == nil {
			// 			log.Printf("Nil StringValue for Headers attribute for message %v\n", message.MessageId)
			// 		}

			// 		var headers http.Header
			// 		err := json.Unmarshal([]byte(*headersAttr.StringValue), &headers)
			// 		if err != nil {
			// 			log.Printf("Unable to parse JSON headers for message ID: %v, %s\n", message.MessageId, *headersAttr.StringValue)
			// 		}

			// 		for path, webhook := range b.Webhooks {
			// 			if *messagePathAttr.StringValue == path {
			// 				log.Printf("Received message: %v\n", *message.Body)
			// 				body := ioutil.NopCloser(bytes.NewReader([]byte(*message.Body)))
			// 				r := &http.Request{
			// 					Method: "POST",
			// 					Body:   body,
			// 					Header: headers,
			// 				}

			// 				webhook.Handle(b, r)
			// 			}
			// 		}
			// 	}

			// 	res, err := b.SqsClient.DeleteMessageBatch(context.TODO(), &sqs.DeleteMessageBatchInput{
			// 		QueueUrl: &queueURL,
			// 		Entries:  del,
			// 	})
			// 	if err != nil {
			// 		log.Printf("Failed to delete messages: %v\n", err)
			// 	} else if len(res.Failed) > 0 {
			// 		log.Printf("Failed to delete %d messages from queue\n", len(res.Failed))
			// 	}
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
