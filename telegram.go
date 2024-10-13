package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/dx314/movie_beacon_bot/db"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"html"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var messageCache = make(map[int]string)

// editMessage edits a message with the given text
func editMessage(chatID int64, messageID int, text string) error {
	s, _ := messageCache[messageID]
	if s == text {
		return nil
	}

	messageCache[messageID] = text

	fmt.Printf("editMessage: %v, %v, %v\n", chatID, messageID, text)
	msg := tgbotapi.NewEditMessageText(chatID, messageID, text)
	_, err := bot.Send(msg)
	if err != nil {
		log.Printf("Error editing message: %v", err)
	}
	return err
}

// sendErrorMessage sends an error message to the user
func sendResultsAsButtons(chatID int64, msgData *db.MsgDatum, items []Item) {
	if len(items) == 0 {
		msg := tgbotapi.NewMessage(chatID, "No results found.")
		bot.Send(msg)
		return
	}

	var buttons [][]tgbotapi.InlineKeyboardButton
	var messageText strings.Builder
	messageText.WriteString("Search Results:\n\n")

	distinctEmojis := []string{"🍎", "🍌", "🍒", "🍊", "🍋", "🥝", "🍍", "🥭", "🍉"}

	// Determine the number of results to display (up to 10)
	numResults := len(items)
	if numResults > 9 {
		numResults = 9
	}

	var currentRow []tgbotapi.InlineKeyboardButton

	for i := 0; i < numResults; i++ {
		item := items[i]
		size, _ := strconv.ParseInt(item.Enclosure.Length, 10, 64)
		age := calculateAge(item.PubDate)

		nzbUUID := uuid.New().String()
		titleInfo := parseMovieTitle(item.Title)

		itemText := fmt.Sprintf("%s <b>%s</b>\n   <b>Year:</b> %s   <b>Size:</b> %s\n   <b>Resolution:</b> %s<b>   Release:</b> %s\n   <b>Age:</b> %s\n\n",
			distinctEmojis[i],
			html.EscapeString(titleInfo.Title),
			html.EscapeString(titleInfo.Year),
			html.EscapeString(formatSize(size)),
			html.EscapeString(titleInfo.Resolution),
			html.EscapeString(titleInfo.LastTag),
			html.EscapeString(age))

		messageText.WriteString(itemText)

		nzbInfo := db.NzbInfo{
			Url:         item.Enclosure.URL,
			Name:        fmt.Sprintf("%s (%s)", titleInfo.Title, titleInfo.Year),
			ChatID:      chatID,
			Status:      "Pending",
			LastUpdated: time.Now().Unix(),
			Selected:    0, // Initialize as not selected
			Category:    msgData.Category,
		}

		if err := storeNZBInfo(nzbUUID, nzbInfo); err != nil {
			log.Printf("Error storing NZB info: %v", err)
			continue
		}

		button := tgbotapi.NewInlineKeyboardButtonData(distinctEmojis[i], nzbUUID)
		currentRow = append(currentRow, button)

		// Create a new row after every 3 buttons, or for the last button
		if len(currentRow) == 3 || i == numResults-1 {
			buttons = append(buttons, currentRow)
			currentRow = []tgbotapi.InlineKeyboardButton{}
		}
	}

	// Add the cancel button as the 10th button
	cancelButton := tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "cancel")
	if len(currentRow) > 0 {
		currentRow = append(currentRow, cancelButton)
		buttons = append(buttons, currentRow)
	} else {
		buttons = append(buttons, []tgbotapi.InlineKeyboardButton{cancelButton})
	}

	msg := tgbotapi.NewMessage(chatID, messageText.String())
	msg.ParseMode = "HTML"

	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
	sentMsg, err := bot.Send(msg)

	if err != nil {
		log.Printf("Error sending message with buttons: %v", err)
	}

	if _, err := queries.InsertMessageData(context.Background(), db.InsertMessageDataParams{
		MessageID: sentMsg.MessageID,
		UserID:    sentMsg.From.ID,
		Category:  msgData.Category,
		Year:      msgData.Year,
		Search:    msgData.Search,
	}); err != nil {
		log.Printf("Error inserting message data: %v", err)
	}
}

// handleCallbackQuery handles the callback query when a user selects an option
func handleCallbackQuery(query *tgbotapi.CallbackQuery) {
	// Defer the deletion of the message data
	defer func(msgID int) {
		err := queries.DeleteMessageData(context.Background(), msgID)
		if err != nil {
			log.Printf("Error deleting message data for msg %d: %v", query.Message.MessageID, err)
		}

		// Delete the OMDB results message
		deleteMsg := tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID)
		if _, err := bot.Request(deleteMsg); err != nil {
			log.Printf("Error deleting OMDB results message: %v", err)
		}
	}(query.Message.MessageID)

	if strings.HasPrefix(query.Data, "tvimdb:") {
		parts := strings.Split(strings.TrimPrefix(query.Data, "tvimdb:"), ":")
		imdbID := parts[0]
		season := "S" + strAddLeadingZero(parts[1])

		msgData, err := queries.GetMessageData(context.Background(), query.Message.MessageID)
		if err != nil {
			fmt.Printf("no row found for message id %d\n", query.Message.MessageID)
			panic(err)
		}
		if msgData.Category == "" {
			panic("Category not set in message data")
		}

		callback := tgbotapi.NewCallback(query.ID, "Searching for NZBs...")
		if _, err := bot.Request(callback); err != nil {
			log.Printf("Error answering callback query: %v", err)
		}

		searchResult, err := searchNZBGeek(msgData.Search+"."+season+".", "", msgData.Category)
		if err != nil {
			errorMsg := fmt.Sprintf("Error searching NZBGeek: %v", err)
			log.Println(errorMsg)
			msg := tgbotapi.NewMessage(query.Message.Chat.ID, errorMsg)
			bot.Send(msg)
			return
		}

		if searchResult.RemainingCount == 0 {
			if searchResult.TotalFound == 0 {
				msg := tgbotapi.NewMessage(query.Message.Chat.ID, fmt.Sprintf("No results found for: %s (%s)", msgData.Search, imdbID))
				bot.Send(msg)
			} else {
				sendResultsAsButtons(query.Message.Chat.ID, &msgData, searchResult.Items)
				if searchResult.FilteredCount > 0 {
					infoMsg := fmt.Sprintf("Found %d results. %d were filtered out, showing %d relevant results.",
						searchResult.TotalFound, searchResult.FilteredCount, len(searchResult.Items))
					bot.Send(tgbotapi.NewMessage(query.Message.Chat.ID, infoMsg))
				}
			}
		} else {
			sendResultsAsButtons(query.Message.Chat.ID, &msgData, searchResult.Items)
			if searchResult.FilteredCount > 0 {
				infoMsg := fmt.Sprintf("Found %d results. %d were filtered out, showing %d relevant results.",
					searchResult.TotalFound, searchResult.FilteredCount, len(searchResult.Items))
				bot.Send(tgbotapi.NewMessage(query.Message.Chat.ID, infoMsg))
			}
		}
		return
	}

	if strings.HasPrefix(query.Data, "imdb:") {
		imdbID := strings.TrimPrefix(query.Data, "imdb:")
		msgData, err := queries.GetMessageData(context.Background(), query.Message.MessageID)
		if err != nil {
			fmt.Printf("no row found for message id %d\n", query.Message.MessageID)
			panic(err)
		}
		if msgData.Category == "" {
			panic("Category not set in message data")
		}

		callback := tgbotapi.NewCallback(query.ID, "Searching for NZBs...")
		if _, err := bot.Request(callback); err != nil {
			log.Printf("Error answering callback query: %v", err)
		}

		searchResult, err := lookupNZBGeek(imdbID, msgData.Category)
		if err != nil {
			errorMsg := fmt.Sprintf("Error looking up on NZBGeek: %v", err)
			log.Println(errorMsg)
			msg := tgbotapi.NewMessage(query.Message.Chat.ID, errorMsg)
			bot.Send(msg)
			return
		}

		if searchResult.TotalFound == 0 {
			log.Println("Searching NZBGeek as fallback...")
			searchResult, err = searchNZBGeek(msgData.Search, msgData.Year, msgData.Category)
			if err != nil {
				errorMsg := fmt.Sprintf("Error searching NZBGeek: %v", err)
				log.Println(errorMsg)
				msg := tgbotapi.NewMessage(query.Message.Chat.ID, errorMsg)
				bot.Send(msg)
				return
			}
		}

		if searchResult.RemainingCount == 0 {
			msg := tgbotapi.NewMessage(query.Message.Chat.ID, fmt.Sprintf("No results found for IMDb ID: %s", imdbID))
			bot.Send(msg)
		} else {
			sendResultsAsButtons(query.Message.Chat.ID, &msgData, searchResult.Items)
			if searchResult.FilteredCount > 0 {
				infoMsg := fmt.Sprintf("Found %d results. %d were filtered out, showing %d relevant results.",
					searchResult.TotalFound, searchResult.FilteredCount, len(searchResult.Items))
				bot.Send(tgbotapi.NewMessage(query.Message.Chat.ID, infoMsg))
			}
		}

		return
	}

	if query.Data != "cancel" {
		// Rest of the existing handleCallbackQuery function for handling NZB selection
		nzbUUID := query.Data
		nzbInfo, err := getNZBInfo(nzbUUID)
		if err != nil {
			log.Printf("Error retrieving NZB info: %v", err)
			sendErrorMessage(query.Message.Chat.ID, "Failed to retrieve the download information.")
			return
		}

		sabnzbdID, err := addNZBToSABnzbd(nzbInfo.Url, nzbInfo.Category)
		if err != nil {
			log.Printf("Error adding NZB to SABnzbd: %v", err)
			sendErrorMessage(query.Message.Chat.ID, "Failed to add the NZB to SABnzbd.")
			return
		}

		nzbInfo.SabnzbdID = sabnzbdID
		nzbInfo.Status = "Queued"
		nzbInfo.LastUpdated = time.Now().Unix()
		nzbInfo.Selected = 1 // Mark as selected

		msg := tgbotapi.NewMessage(query.Message.Chat.ID, fmt.Sprintf("NZB '%s' added to SABnzbd. Initializing...", nzbInfo.Name))
		sentMsg, err := bot.Send(msg)
		if err != nil {
			log.Printf("Error sending initial status message: %v", err)
			return
		}

		nzbInfo.MessageID = sentMsg.MessageID
		if err := storeNZBInfo(nzbUUID, nzbInfo); err != nil {
			log.Printf("Error updating NZB info with SABnzbd ID: %v", err)
		}

		go monitorDownloadProgress(nzbUUID)
	}

	// Delete the results message
	deleteMsg := tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID)
	if _, err := bot.Request(deleteMsg); err != nil {
		log.Printf("Error deleting NZBGeek results message: %v", err)
	}

	// Remove unselected options from the database
	if err := queries.DeleteUnselectedOptions(context.Background(), query.Message.Chat.ID); err != nil {
		log.Printf("Error removing unselected options: %v", err)
	}
}

func checkSABnzbdHistory(nzbID string) (string, string, error) {
	apiURL := fmt.Sprintf("%s/api?output=json&apikey=%s&mode=history&nzo_ids=%s", sabnzbdAPI, sabnzbdAPIKey, nzbID)

	resp, err := http.Get(apiURL)
	if err != nil {
		return "", "", fmt.Errorf("failed to get SABnzbd history: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("bad status from SABnzbd API: %s", resp.Status)
	}

	var result SabNZBResponse

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read SABnzbd response: %v", err)
	}

	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println(string(body))
		return "", "", fmt.Errorf("failed to unmarshal SABnzbd response: %v", err)
	}

	for _, slot := range result.History.Slots {
		if slot.NzoID == nzbID {
			if slot.Status == "Completed" {
				totalTime, err := calculateTotalTime(slot.DownloadTime, slot.PostprocTime)
				if err != nil {
					log.Printf("Error calculating total time: %v", err)
					return "Completed", "100% (Total time: Unknown)", nil
				}
				sizeInMB := float64(int64(slot.Completed)) / 1024 / 1024
				progress := fmt.Sprintf("Progress: %.2f MB / %.2f MB (100%%)\nTotal time: %d seconds\nStorage: %s",
					sizeInMB, sizeInMB, totalTime, slot.Storage)
				return "Completed", progress, nil
			}
			return slot.Status, "100%", nil
		}
	}

	return "Unknown", "Unknown", nil
}

var commandCategories = map[string]string{
	"movie": "movies",
	"km":    "kids_movies",
	"ktv":   "kids_tv",
	"tv":    "tv",
}

type UserState struct {
	ChatID    int64
	State     string
	Category  string
	CreatedAt time.Time
}

type UserStateStore struct {
	sync.Mutex
	m map[int64]UserState
}

func NewUserStateStore() *UserStateStore {
	return &UserStateStore{m: make(map[int64]UserState)}
}

func (s *UserStateStore) Set(userID int64, state UserState) {
	s.Lock()
	defer s.Unlock()
	s.m[userID] = state
}

func (s *UserStateStore) Get(userID int64) (UserState, bool) {
	s.Lock()
	defer s.Unlock()
	state, ok := s.m[userID]
	return state, ok
}

func (s *UserStateStore) Delete(userID int64) {
	s.Lock()
	defer s.Unlock()
	delete(s.m, userID)
}

var UserStates = NewUserStateStore()

func handleCommand(message *tgbotapi.Message) {
	cat, _ := commandCategories[message.Command()]
	switch message.Command() {
	case "start":
		msg := tgbotapi.NewMessage(message.Chat.ID, "Welcome! Use /movie [movie name] [year] to search for movies.")
		bot.Send(msg)
	case "ktv":
		fallthrough
	case "tv":
		args := message.CommandArguments()
		if args == "" {
			us := UserState{ChatID: message.Chat.ID, State: "input", Category: cat, CreatedAt: time.Now()}
			UserStates.Set(message.From.ID, us)

			msg := tgbotapi.NewMessage(message.Chat.ID, "Please provide the name and year.")
			bot.Send(msg)
			return
		} else {
			doTVCommand(message, cat, args)
		}
	case "km":
		fallthrough
	case "movie":
		args := message.CommandArguments()
		if args == "" {
			us := UserState{ChatID: message.Chat.ID, State: "input", Category: cat, CreatedAt: time.Now()}
			UserStates.Set(message.From.ID, us)

			msg := tgbotapi.NewMessage(message.Chat.ID, "Please provide the name and year.")
			bot.Send(msg)
			return
		} else {
			doMovieCommand(message, cat, args)
		}
	default:
		msg := tgbotapi.NewMessage(message.Chat.ID, "I don't know that command. Use /movie, /tv, /km (kids movies), or /ktv (kids TV) to search.")
		bot.Send(msg)
	}
}

func doTVCommand(message *tgbotapi.Message, cat string, args string) {
	name, year := parseMovieCommand(args)
	omdbResults, err := lookupSeries(name, year)
	if err != nil {
		log.Printf("OMDB search failed: %v", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "No results found.")
		bot.Send(msg)
		return
	}

	totalSeasons, err := strconv.Atoi(omdbResults.TotalSeasons)
	if err != nil {
		omdbItems := []OMDBSearchResult{
			{
				Title:  omdbResults.Title,
				Year:   omdbResults.Year,
				Type:   "series",
				ImdbID: omdbResults.ImdbID,
			},
		}
		sendOMDBResultsAsButtons(message.Chat.ID, cat, name, omdbResults.Year, omdbItems)
	}
	var buttons [][]tgbotapi.InlineKeyboardButton
	for i := 0; i < totalSeasons+1; i++ {
		s := addLeadingZero(i)
		if s == "00" {
			s = "00 - Specials"
		}
		buttonText := fmt.Sprintf("S%s", s)
		button := tgbotapi.NewInlineKeyboardButtonData(buttonText, fmt.Sprintf("tvimdb:%s:%s", omdbResults.ImdbID, s))
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(button))
	}

	// Add the cancel button as the 10th button
	cancelButton := tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "cancel")
	buttons = append(buttons, []tgbotapi.InlineKeyboardButton{cancelButton})

	msg, err := bot.SendMessageWithButtons(message.Chat.ID, omdbResults.Title, buttons)
	if err != nil {
		log.Printf("Error sending message with buttons: %v", err)
	}

	if _, err := queries.InsertMessageData(context.Background(), db.InsertMessageDataParams{
		MessageID: msg.MessageID,
		UserID:    msg.From.ID,
		Category:  cat,
		Search:    args,
		Year:      omdbResults.Year,
	}); err != nil {
		log.Printf("Error inserting message data: %v", err)
	}

}

func parseTVCommand(args string) (string, string, string) {
	words := strings.Fields(args)
	if len(words) < 2 {
		return "", "", ""
	}

	var name, season, year string
	nameEnd := len(words) - 2

	// Check if the last word is a year
	if _, err := strconv.Atoi(words[len(words)-1]); err == nil {
		year = words[len(words)-1]
		nameEnd--
	}

	// Check if the second last word (or last if year was found) is a season
	if strings.HasPrefix(strings.ToLower(words[nameEnd]), "s") {
		season = words[nameEnd]
		nameEnd--
	}

	// Join the remaining words as the episode name
	name = strings.Join(words[:nameEnd+1], " ")

	return name, season, year
}

func doMovieCommand(message *tgbotapi.Message, cat string, args string) {
	name, year := parseMovieCommand(args)
	if year == "" {
		userStates[message.Chat.ID] = name
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Please provide the year for: %s", name))
		bot.Send(msg)
	} else {
		omdbResults, err := searchOMDB(name, year, cat)
		if err != nil {
			log.Printf("OMDB search failed: %v", err)
			var errorMsg string
			if err.Error() == "no suitable results found, please try a more specific search" {
				errorMsg = "No bueno. The search was too broad. Please try a more specific search with both title and year."
			} else {
				errorMsg = "No bueno. Couldn't find any matching results."
			}
			msg := tgbotapi.NewMessage(message.Chat.ID, errorMsg)
			bot.Send(msg)
			return
		}
		sendOMDBResultsAsButtons(message.Chat.ID, cat, name, year, omdbResults)
	}
}

func handleInput(message *tgbotapi.Message) {
	state, ok := UserStates.Get(message.From.ID)
	if !ok {
		return
	}
	doMovieCommand(message, state.Category, message.Text)
	UserStates.Delete(message.From.ID)
}

func sendOMDBResultsAsButtons(chatID int64, category, search, year string, items []OMDBSearchResult) {
	if len(items) == 0 {
		msg := tgbotapi.NewMessage(chatID, "No results found.")
		bot.Send(msg)
		return
	}

	var buttons [][]tgbotapi.InlineKeyboardButton
	for _, item := range items {
		buttonText := fmt.Sprintf("%s (%s)", item.Title, item.Year)
		button := tgbotapi.NewInlineKeyboardButtonData(buttonText, fmt.Sprintf("imdb:%s", item.ImdbID))
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(button))
	}

	// Add the cancel button as the 10th button
	cancelButton := tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "cancel")
	buttons = append(buttons, []tgbotapi.InlineKeyboardButton{cancelButton})

	msg, err := bot.SendMessageWithButtons(chatID, "IMDB Results:", buttons)
	if err != nil {
		log.Printf("Error sending message with buttons: %v", err)
	}

	if _, err := queries.InsertMessageData(context.Background(), db.InsertMessageDataParams{
		MessageID: msg.MessageID,
		UserID:    msg.From.ID,
		Category:  category,
		Search:    search,
		Year:      year,
	}); err != nil {
		log.Printf("Error inserting message data: %v", err)
	}
}

func fatalf(chatID int64, format string, args ...interface{}) {
	sendErrorMessage(chatID, "Closing down to catastrophic error: "+fmt.Sprintf(format, args...))
	log.Fatalf(format, args...)
}
