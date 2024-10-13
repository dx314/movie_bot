package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/dx314/movie_beacon_bot/db"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

func storeNZBInfo(nzbUUID string, info db.NzbInfo) error {
	ctx := context.Background()

	return queries.UpsertNZBInfo(ctx, db.UpsertNZBInfoParams{
		ID:          nzbUUID,
		Url:         info.Url,
		Name:        info.Name,
		Category:    info.Category,
		SabnzbdID:   info.SabnzbdID,
		ChatID:      info.ChatID,
		MessageID:   info.MessageID,
		Status:      info.Status,
		LastUpdated: info.LastUpdated,
		Selected:    info.Selected,
	})
}

func getNZBInfo(nzbUUID string) (db.NzbInfo, error) {
	ctx := context.Background()
	return queries.GetNZBInfo(ctx, nzbUUID)
}

func deleteNZBInfo(nzbUUID string) error {
	return queries.DeleteNZBInfo(context.Background(), nzbUUID)
}

var x = 0

func monitorDownloadProgress(nzbUUID string) {
	x++
	fmt.Printf("MONITORING THREAD %d\n", x)
	if !manageMonitorState(nzbUUID) {
		// A monitor is already running for this item
		fmt.Printf("RELEASING MONITOR %d\n", x)
		return
	}
	defer func() {
		fmt.Printf("RELEASING MONITOR %d\n", x)
		releaseMonitorState(nzbUUID)
	}()

	for {
		nzbInfo, err := getNZBInfo(nzbUUID)
		if err != nil {
			log.Printf("Error getting NZB info: %v", err)
			return
		}

		status, progress, err := getSABnzbdProgress(nzbInfo.SabnzbdID)
		if err != nil {
			log.Printf("Error getting SABnzbd progress: %v", err)

			updateNZBStatus(nzbUUID, "Failed", fmt.Sprintf("Error monitoring '%s': %v", nzbInfo.Name, err))
			return
		}

		if status == "Deleted" {
			timeSinceLastUpdate := time.Now().Unix() - int64(nzbInfo.LastUpdated)
			if timeSinceLastUpdate > 150 { // 5 minutes in seconds
				message := fmt.Sprintf("%s download has been removed from queue.", nzbInfo.Name)
				err = editMessage(nzbInfo.ChatID, nzbInfo.MessageID, message)
				if err != nil {
					log.Printf("Error editing message %s: %v", nzbInfo.MessageID, err)
				} else {
					log.Printf("Message for %s edited successfully", nzbInfo.Name)
				}
				if err := deleteNZBInfo(nzbUUID); err != nil {
					log.Printf("Error deleting NZB info from database: %v", err)
				}
				return
			}
		} else {
			progressMsg := fmt.Sprintf("NZB: %s\nStatus: %s\n%s", nzbInfo.Name, status, progress)
			updateNZBStatus(nzbUUID, status, progressMsg)
		}

		if status == "Completed" || status == "Failed" {
			return
		}

		time.Sleep(6 * time.Second)
	}
}

func getSABnzbdProgress(nzbID string) (string, string, error) {
	if nzbID == "" {
		return "Unknown", "", errors.New("NZB ID not provided")
	}
	apiURL := fmt.Sprintf("%s/api?output=json&apikey=%s&mode=queue&nzo_ids=%s", sabnzbdAPI, sabnzbdAPIKey, nzbID)

	resp, err := http.Get(apiURL)
	if err != nil {
		return "", "", fmt.Errorf("failed to get SABnzbd queue: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("bad status from SABnzbd API: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response body: %v", err)
	}

	var result struct {
		Queue struct {
			Slots []struct {
				Status          string `json:"status"`
				Filename        string `json:"filename"`
				PercentComplete string `json:"percentage"`
				SizeMB          string `json:"mb"`
				SizeLeft        string `json:"mbleft"`
			} `json:"slots"`
		} `json:"queue"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println(string(body))
		return "", "", fmt.Errorf("failed to decode SABnzbd response: %v", err)
	}

	for _, slot := range result.Queue.Slots {
		if slot.Status == "Downloading" || slot.Status == "Queued" {
			totalSize, _ := strconv.ParseFloat(slot.SizeMB, 64)
			sizeLeft, _ := strconv.ParseFloat(slot.SizeLeft, 64)
			downloaded := totalSize - sizeLeft
			percentage, _ := strconv.ParseFloat(strings.TrimRight(slot.PercentComplete, "%"), 64)

			progress := fmt.Sprintf("Progress: %.2f MB / %.2f MB (%.1f%%)", downloaded, totalSize, percentage)
			return slot.Status, progress, nil
		}
	}

	// If not found in queue, check history
	status, progress, err := checkSABnzbdHistory(nzbID)
	if err != nil {
		return "", "", err
	}

	// If not found in history either, it might have been deleted
	if status == "Unknown" {
		return "Deleted", "Download has been removed from queue", nil
	}

	return status, progress, nil
}

func addNZBToSABnzbd(nzbURL, category string) (string, error) {
	fmt.Println("#############")
	fmt.Println("ADDING TO :" + category)
	fmt.Println("#############")

	apiURL := fmt.Sprintf("%s/api?output=json&apikey=%s&mode=addurl&name=%s&cat=%s",
		sabnzbdAPI, sabnzbdAPIKey, url.QueryEscape(nzbURL), url.QueryEscape(category))

	log.Printf("Adding NZB to SABnzbd: %s", apiURL)

	resp, err := http.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("failed to add NZB to SABnzbd: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status from SABnzbd API: %s", resp.Status)
	}

	var result struct {
		Status bool     `json:"status"`
		NzoIDs []string `json:"nzo_ids"`
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println(string(body))
		return "", fmt.Errorf("failed to decode SABnzbd response: %v", err)
	}

	if !result.Status || len(result.NzoIDs) == 0 {
		return "", fmt.Errorf("SABnzbd failed to add NZB")
	}

	return result.NzoIDs[0], nil
}

func calculateSimilarity(s1, s2 string) float64 {
	pairs1 := make(map[string]int)
	pairs2 := make(map[string]int)

	// Create character pairs
	for i := 0; i < len(s1)-1; i++ {
		pair := s1[i : i+2]
		pairs1[pair]++
	}

	for i := 0; i < len(s2)-1; i++ {
		pair := s2[i : i+2]
		pairs2[pair]++
	}

	// Count matching pairs
	matchingPairs := 0
	for pair, count := range pairs1 {
		if count2, found := pairs2[pair]; found {
			if count2 < count {
				matchingPairs += count2
			} else {
				matchingPairs += count
			}
		}
	}

	// Calculate similarity
	totalPairs := len(s1) + len(s2) - 2
	if totalPairs == 0 {
		return 0
	}
	similarity := float64(2*matchingPairs) / float64(totalPairs)

	log.Printf("Similarity calculation details:")
	log.Printf("String 1: %s", s1)
	log.Printf("String 2: %s", s2)
	log.Printf("Matching pairs: %d", matchingPairs)
	log.Printf("Total pairs: %d", totalPairs)
	log.Printf("Calculated similarity: %f", similarity)

	return similarity
}

// Define NZBGeek category IDs
var nzbGeekCategories = map[string]string{
	"movies":      "2000",
	"tv":          "5000",
	"kids_movies": "2000",
	"kids_tv":     "5000",
}

type SearchResult struct {
	Items          []Item
	TotalFound     int
	FilteredCount  int
	RemainingCount int
}

type RSS struct {
	Channel struct {
		Items []Item `xml:"item"`
	} `xml:"channel"`
}

type Item struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Enclosure   Enclosure `xml:"enclosure"`
	PubDate     string    `xml:"pubDate"`
}

func searchNZBGeek(imdbID string, category string) (SearchResult, error) {
	apiKey := os.Getenv("NZBGEEK_API_KEY")
	baseURL := "https://api.nzbgeek.info/api"

	imdbID = strings.TrimPrefix(imdbID, "tt")

	categoryID := nzbGeekCategories[category]
	if categoryID == "" {
		categoryID = "2000" // Default to movies if category is not found
	}

	fullURL := fmt.Sprintf("%s?apikey=%s&t=search&cat=%s&imdbid=%s&limit=50", baseURL, apiKey, categoryID, imdbID)

	log.Println("Fetching from NZBGeek:", fullURL)

	resp, err := http.Get(fullURL)
	if err != nil {
		return SearchResult{}, fmt.Errorf("error fetching from NZBGeek: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return SearchResult{}, fmt.Errorf("error reading response body: %v", err)
	}

	var rss RSS
	err = xml.Unmarshal(body, &rss)
	if err != nil {
		return SearchResult{}, fmt.Errorf("error decoding XML: %v", err)
	}

	totalFound := len(rss.Channel.Items)

	// Sort items by publication date (most recent first)
	sort.Slice(rss.Channel.Items, func(i, j int) bool {
		timeI, _ := time.Parse(time.RFC1123Z, rss.Channel.Items[i].PubDate)
		timeJ, _ := time.Parse(time.RFC1123Z, rss.Channel.Items[j].PubDate)
		return timeI.After(timeJ)
	})

	// Prepare the result
	result := SearchResult{
		TotalFound:     totalFound,
		FilteredCount:  0,
		RemainingCount: totalFound,
	}

	// Return the 10 most recent items
	if len(rss.Channel.Items) > 9 {
		result.Items = rss.Channel.Items[:9]
	} else {
		result.Items = rss.Channel.Items
	}

	return result, nil
}

// resumeDownloadMonitoring resumes monitoring of all incomplete downloads
func resumeDownloadMonitoring() {
	ctx := context.Background()
	log.Println("Resuming download monitoring...")

	incompleteDownloads, err := queries.GetIncompleteDownloads(ctx)
	if err != nil {
		log.Printf("Error getting incomplete downloads: %v", err)
		return
	}

	for _, dbNzbInfo := range incompleteDownloads {
		go monitorDownloadProgress(dbNzbInfo.ID)
	}

	log.Printf("Resumed monitoring for %d incomplete downloads", len(incompleteDownloads))
}

func updateNZBStatus(nzbUUID, status, message string) error {
	ctx := context.Background()

	// Fetch the current NZB info
	currentInfo, err := queries.GetNZBInfo(ctx, nzbUUID)
	if err != nil {
		return fmt.Errorf("failed to get NZB info: %v", err)
	}

	// Update the status and last updated time
	currentInfo.Status = status
	currentInfo.LastUpdated = time.Now().Unix()

	// Prepare the update parameters
	updateParams := db.UpsertNZBInfoParams{
		ID:          nzbUUID,
		Url:         currentInfo.Url,
		Name:        currentInfo.Name,
		Category:    currentInfo.Category,
		SabnzbdID:   currentInfo.SabnzbdID,
		ChatID:      currentInfo.ChatID,
		MessageID:   currentInfo.MessageID,
		Status:      status,
		LastUpdated: currentInfo.LastUpdated,
		Selected:    currentInfo.Selected,
	}

	// Update the NZB info in the database
	err = queries.UpsertNZBInfo(ctx, updateParams)
	if err != nil {
		return fmt.Errorf("failed to update NZB info: %v", err)
	}

	// Edit the message
	editMessage(currentInfo.ChatID, int(currentInfo.MessageID), message)

	return nil
}
