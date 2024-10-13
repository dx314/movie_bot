package main

import (
	"errors"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// calculateTotalTime takes downloadTime as an interface{} and postProcessTime as a string.
// It handles different types for downloadTime, such as string, int, or float64.
func calculateTotalTime(downloadTime, postProcessTime interface{}) (int, error) {
	// Helper function to convert interface{} to int
	convertToSeconds := func(value interface{}) (int, error) {
		switch v := value.(type) {
		case int:
			return v, nil
		case float64:
			return int(v), nil
		case string:
			secs, err := strconv.Atoi(v)
			if err != nil {
				return 0, fmt.Errorf("invalid time (string): %v", err)
			}
			return secs, nil
		default:
			return 0, errors.New("unsupported type for time")
		}
	}

	// Convert downloadTime to seconds
	downloadSecs, err := convertToSeconds(downloadTime)
	if err != nil {
		return 0, fmt.Errorf("invalid download time: %v", err)
	}

	// Convert postProcessTime to seconds
	postProcessSecs, err := convertToSeconds(postProcessTime)
	if err != nil {
		return 0, fmt.Errorf("invalid post-process time: %v", err)
	}

	// Return the total of download and post-process times
	return downloadSecs + postProcessSecs, nil
}

func calculateAge(pubDate string) string {
	t, err := time.Parse(time.RFC1123Z, pubDate)
	if err != nil {
		log.Printf("Error parsing date: %v", err)
		return "Unknown"
	}

	duration := time.Since(t)

	if duration.Hours() < 24 {
		return fmt.Sprintf("%.0f hours", duration.Hours())
	} else if duration.Hours() < 48 {
		return "1 day"
	} else {
		return fmt.Sprintf("%.0f days", duration.Hours()/24)
	}
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func sendErrorMessage(chatID int64, message string) {
	msg := tgbotapi.NewMessage(chatID, message)
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Error sending error message: %v", err)
	}
}

func parseMovieCommand(args string) (string, string) {
	year := yearRegex.FindString(args)
	movieName := yearRegex.ReplaceAllString(args, "")
	movieName = strings.TrimSpace(movieName)
	return movieName, year
}

type MovieInfo struct {
	Title      string
	Year       string
	Resolution string
	LastTag    string
}

func parseMovieTitle(s string) MovieInfo {
	// Keep original case for last tag extraction
	originalTitle := s

	// Convert to lowercase for consistent processing
	title := strings.ToLower(s)

	// Remove file extension if present
	title = regexp.MustCompile(`\.[^.]+$`).ReplaceAllString(title, "")

	// Extract year (allow for years between 1900 and 2099)
	yearRegex := regexp.MustCompile(`\b(19\d{2}|20\d{2})\b`)
	year := yearRegex.FindString(title)

	// Extract resolution (include more variations)
	resolutionRegex := regexp.MustCompile(`\b(4k|uhd|2160p|1080p|720p|480p|360p|240p|144p|sd)\b`)
	resolution := resolutionRegex.FindString(title)

	// Extract last tag (using original case)
	lastTag := ""
	parts := strings.FieldsFunc(originalTitle, func(r rune) bool {
		return r == '-' || r == '.'
	})
	if len(parts) > 1 {
		lastTag = parts[len(parts)-1]
	}

	// Clean up title
	cleanTitle := title

	// Remove year
	cleanTitle = yearRegex.ReplaceAllString(cleanTitle, "")

	// Remove resolution
	cleanTitle = resolutionRegex.ReplaceAllString(cleanTitle, "")

	// Remove common tags and formats
	tagsToRemove := []string{
		"bluray", "web-dl", "webrip", "brrip", "dvdrip", "hdtv", "multi", "internal",
		"x264", "x265", "h264", "h\\.265", "hevc", "xvid", "divx",
		"dts-hd", "dts", "dd5\\.1", "dd", "aac", "ac3", "eac3", "atmos",
		"remux", "proper", "repack", "extended", "theatrical",
		"hdr", "dolby", "vision", "dovi", "hybrid",
	}
	for _, tag := range tagsToRemove {
		cleanTitle = regexp.MustCompile(`\b`+tag+`\b`).ReplaceAllString(cleanTitle, "")
	}

	// Remove audio channel information
	cleanTitle = regexp.MustCompile(`\b\d+\.\d+\b`).ReplaceAllString(cleanTitle, "")

	// Remove special characters and extra spaces
	cleanTitle = regexp.MustCompile(`[^\w\s]`).ReplaceAllString(cleanTitle, " ")
	cleanTitle = regexp.MustCompile(`\s+`).ReplaceAllString(cleanTitle, " ")

	// Trim spaces and convert to title case
	cleanTitle = strings.TrimSpace(cleanTitle)
	cleanTitle = strings.Title(cleanTitle)

	return MovieInfo{
		Title:      cleanTitle,
		Year:       year,
		Resolution: resolution,
		LastTag:    lastTag,
	}
}
