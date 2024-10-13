package main

import (
	"database/sql"
	"github.com/dx314/movie_beacon_bot/db"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"log"
	_ "modernc.org/sqlite"
	"os"
	"regexp"
	"sync"
)

const (
	nzbInfoPrefix        = "nzbinfo:"
	userCategoryPrefix   = "user_category:"
	resultsMessagePrefix = "results_message:"
)

type Enclosure struct {
	URL    string `xml:"url,attr"`
	Length string `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}

type customBotAPI struct {
	*tgbotapi.BotAPI
}

func (c *customBotAPI) SendMessage(msg tgbotapi.MessageConfig) (tgbotapi.Message, error) {
	return c.Send(msg)
}

func (c *customBotAPI) SendMessageWithButtons(chatID int64, text string, buttons [][]tgbotapi.InlineKeyboardButton) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
	return c.Send(msg)
}

var (
	bot                 *customBotAPI
	yearRegex           = regexp.MustCompile(`\b(19|20)\d{2}\b`)
	userStates          = make(map[int64]string)
	sabnzbdAPI          string
	sabnzbdAPIKey       string
	activeMonitors      = make(map[string]bool)
	activeMonitorsMutex sync.Mutex
	queries             *db.Queries
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Initialize SQLite database
	dbConn, err := sql.Open("sqlite", "./nzbot.db")
	if err != nil {
		log.Fatal(err)
	}
	defer dbConn.Close()

	// Initialize queries
	queries = db.New(dbConn)

	botAPI, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		log.Panic(err)
	}

	bot = &customBotAPI{botAPI}

	sabnzbdAPI = os.Getenv("SABNZBD_API_URL")
	sabnzbdAPIKey = os.Getenv("SABNZBD_API_KEY")

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	go resumeDownloadMonitoring()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			if update.Message.IsCommand() {
				handleCommand(update.Message)
			} else {
				handleInput(update.Message)
			}
		} else if update.CallbackQuery != nil {
			handleCallbackQuery(update.CallbackQuery)
		}
	}
}

func manageMonitorState(nzbUUID string) bool {
	activeMonitorsMutex.Lock()
	defer activeMonitorsMutex.Unlock()

	if activeMonitors[nzbUUID] {
		// A monitor is already running for this item
		return false
	}

	// No monitor is running, so we can start one
	activeMonitors[nzbUUID] = true
	return true
}

func releaseMonitorState(nzbUUID string) {
	activeMonitorsMutex.Lock()
	defer activeMonitorsMutex.Unlock()

	delete(activeMonitors, nzbUUID)
}
