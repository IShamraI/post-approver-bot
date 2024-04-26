package main

import (
	"fmt"
	"log"
	"time"

	ttlcache "github.com/jellydator/ttlcache/v3"

	"github.com/IShamraI/post-approver-bot/internal/buttons"
	"github.com/IShamraI/post-approver-bot/internal/env"
	"github.com/IShamraI/post-approver-bot/internal/helpers"
	"github.com/mehanizm/airtable"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var oneTimePostKB = tgbotapi.NewOneTimeReplyKeyboard(
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton(buttons.ApproveButton.Text()),
		tgbotapi.NewKeyboardButton(buttons.RejectButton.Text()),
	),
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton(buttons.SkipButton.Text()),
	),
)

func main() {
	// Initialize Telegram bot
	envVars := env.New()
	bot, err := tgbotapi.NewBotAPI(envVars.TelegramToken)
	if err != nil {
		log.Panic(err)
	}
	bot.Debug = true

	// Initialize Airtable client
	client := airtable.NewClient(envVars.AirtableApiKey)
	if err != nil {
		log.Fatal(err)
	}
	table := client.GetTable(envVars.AirtableBaseId, envVars.AirtableTableName)

	// Set up bot commands
	commands := []tgbotapi.BotCommand{
		{Command: "getpost", Description: "Get post"},
		// {Command: "getstats", Description: "Get statistics"},
	}
	setCommands := tgbotapi.NewSetMyCommands(commands...)
	if _, err := bot.Request(setCommands); err != nil {
		log.Panic("Unable to set commands")
	}
	// bot.SetMyCommands(commands)

	log.Printf("Authorized on account %s", bot.Self.UserName)

	cache := ttlcache.New[string, bool](
		ttlcache.WithTTL[string, bool](24 * time.Hour),
	)

	go cache.Start() // starts automatic expired item deletion

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	var currentPost *airtable.Record

	for update := range updates {
		if !helpers.IDContains(envVars.TelegramWhiteList, update.FromChat().ID) {
			log.Printf("got update from unknown user: %+v", update)
			continue
		}
		// Create a new MessageConfig. We don't have text yet,
		// so we leave it empty.
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")

		if update.Message.IsCommand() {
			log.Printf("got command: %s", update.Message.Command())

			// Extract the command from the Message.
			switch update.Message.Command() {
			case "start":
				msg.Text = "Hi!"
			case "getpost":
				records, err := table.GetRecords().
					FromView("view_1").
					WithFilterFormula("AND({ToInvistigate} = 0, {IsApproved} = 0, {IsRejected} = 0)").
					ReturnFields("Title", "guid").
					InStringFormat("Europe/Moscow", "ru").
					Do()
				if err != nil {
					log.Panic(err)
				}
				for i, record := range records.Records {
					if cache.Has(record.Fields["guid"].(string)) {
						continue
					}
					currentPost = records.Records[i]
					break
				}
				msg.Text = fmt.Sprintf("Пост: %s\n%s", currentPost.Fields["Title"], currentPost.Fields["guid"])
				msg.ReplyMarkup = oneTimePostKB
			case "help":
				msg.Text = "I understand /sayhi and /status."
			case "status":
				msg.Text = "I'm ok."
			default:
				msg.Text = "I don't know that command"
			}
		} else {
			log.Printf("got text: %s", update.Message.Text)
			switch update.Message.Text {
			case buttons.ApproveButton.Text():
				msg.Text = "Пост принят"
				_, err := currentPost.UpdateRecordPartial(map[string]any{"IsApproved": true, "IsRejected": false, "ToInvistigate": false})
				if err != nil {
					log.Printf("error while approving: %s", err)
					msg.Text = fmt.Sprintf("Произошла ошибка: %s", err)
					currentPost = nil
				}
			case buttons.RejectButton.Text():
				msg.Text = "Пост отклонен"
				_, err := currentPost.UpdateRecordPartial(map[string]any{"IsRejected": true, "IsApproved": false, "ToInvistigate": false})
				if err != nil {
					log.Printf("error while rejecting: %s", err)
					msg.Text = fmt.Sprintf("Произошла ошибка: %s", err)
					currentPost = nil
				}
			case buttons.SkipButton.Text():
				msg.Text = "Пост пропущен"
				cache.Set(currentPost.Fields["guid"].(string), true, ttlcache.DefaultTTL)
				currentPost = nil
			default:
				msg.Text = "Кнопка не поддерживается"
				currentPost = nil
			}

		}

		if _, err := bot.Send(msg); err != nil {
			log.Panic(err)
		}
	}
}
