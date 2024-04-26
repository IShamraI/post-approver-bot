package env

import (
	"log"
	"os"
	"strconv"
	"strings"
)

type Env struct {
	TelegramToken     string
	TelegramWhiteList []int64
	AirtableApiKey    string
	AirtableBaseId    string
	AirtableTableName string
}

func New() *Env {
	env := &Env{}

	env.TelegramToken = os.Getenv("TELEGRAM_TOKEN")
	if env.TelegramToken == "" {
		log.Fatalf("TELEGRAM_TOKEN is not set")
	}
	sWhiteListIDs := strings.Split(os.Getenv("TELEGRAM_WHITELIST"), ",")
	whiteListIDs := make([]int64, len(sWhiteListIDs))
	for _, id := range sWhiteListIDs {
		i, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			log.Fatal(err)
		}
		whiteListIDs = append(whiteListIDs, i)
	}
	env.TelegramWhiteList = whiteListIDs
	env.AirtableApiKey = os.Getenv("AIRTABLE_API_KEY")
	if env.AirtableApiKey == "" {
		log.Fatalf("AIRTABLE_API_KEY is not set")
	}
	env.AirtableBaseId = os.Getenv("AIRTABLE_BASE_ID")
	if env.AirtableBaseId == "" {
		log.Fatalf("AIRTABLE_BASE_ID is not set")
	}
	env.AirtableTableName = os.Getenv("AIRTABLE_TABLE_NAME")
	if env.AirtableTableName == "" {
		log.Fatalf("AIRTABLE_TABLE_NAME is not set")
	}
	return env
}
