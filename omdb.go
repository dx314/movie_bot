package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
)

const omdbBaseURL = "http://www.omdbapi.com/"

type OMDBSearchResult struct {
	Title  string `json:"Title"`
	Year   string `json:"Year"`
	ImdbID string `json:"imdbID"`
	Type   string `json:"Type"`
}

type SearchResponse struct {
	Search       []OMDBSearchResult `json:"Search"`
	TotalResults string             `json:"totalResults"`
	Response     string             `json:"Response"`
	Error        string             `json:"Error"`
}

var CategoryToType = map[string]string{
	"movie":       "movie",
	"tv":          "series",
	"kids_movies": "movie",
	"kids_tv":     "series",
}

func searchOMDB(title, year, category string) ([]OMDBSearchResult, error) {
	log.Printf("Searching OMDB for title: '%s', year: '%s', category: '%s'", title, year, category)

	apiKey := os.Getenv("OMDB_API_KEY")
	if apiKey == "" {
		log.Println("OMDB_API_KEY environment variable is not set")
		return nil, fmt.Errorf("OMDB_API_KEY environment variable is not set")
	}

	searchType := CategoryToType[category]
	log.Printf("Mapped category '%s' to OMDB search type: '%s'", category, searchType)

	// Try specific match first
	specificResult, err := trySpecificMatch(apiKey, title, year, searchType)
	if err == nil {
		log.Printf("Specific match found: %+v", specificResult)
		return []OMDBSearchResult{specificResult}, nil
	}
	log.Printf("Specific match failed: %v. Falling back to search.", err)

	// Fall back to search
	searchResults, err := performSearch(apiKey, title, year, searchType)
	if err != nil {
		if err.Error() == "too many results found, please provide more specific search terms" {
			log.Printf("Too many results found. Attempting to refine search.")
			// Try to refine the search by combining title and year
			refinedTitle := fmt.Sprintf("%s %s", title, year)
			searchResults, err = performSearch(apiKey, refinedTitle, "", searchType)
			if err != nil {
				log.Printf("Refined search failed: %v", err)
				return nil, fmt.Errorf("no suitable results found, please try a more specific search")
			}
		} else {
			log.Printf("Search failed: %v", err)
			return nil, fmt.Errorf("no results found")
		}
	}

	log.Printf("Search successful. Returning %d results.", len(searchResults))
	return searchResults, nil
}

func trySpecificMatch(apiKey, title, year, searchType string) (OMDBSearchResult, error) {
	params := url.Values{}
	params.Add("apikey", apiKey)
	params.Add("t", title)
	params.Add("y", year)
	params.Add("type", searchType)

	fullURL := omdbBaseURL + "?" + params.Encode()
	log.Printf("Trying specific match with URL: %s", fullURL)

	resp, err := http.Get(fullURL)
	if err != nil {
		return OMDBSearchResult{}, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Response string `json:"Response"`
		Error    string `json:"Error"`
		Title    string `json:"Title"`
		Year     string `json:"Year"`
		ImdbID   string `json:"imdbID"`
		Type     string `json:"Type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return OMDBSearchResult{}, fmt.Errorf("error decoding response: %v", err)
	}

	if result.Response == "False" {
		return OMDBSearchResult{}, fmt.Errorf(result.Error)
	}

	return OMDBSearchResult{
		Title:  result.Title,
		Year:   result.Year,
		ImdbID: result.ImdbID,
		Type:   result.Type,
	}, nil
}

func performSearch(apiKey, title, year, searchType string) ([]OMDBSearchResult, error) {
	params := url.Values{}
	params.Add("apikey", apiKey)
	params.Add("s", title)
	params.Add("y", year)
	params.Add("type", searchType)

	fullURL := omdbBaseURL + "?" + params.Encode()
	log.Printf("Performing search with URL: %s", fullURL)

	resp, err := http.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}

	if searchResp.Response == "False" {
		if searchResp.Error == "Too many results." {
			log.Printf("OMDB search returned too many results for query: %s %s", title, year)
			return nil, fmt.Errorf("too many results found, please provide more specific search terms")
		}
		if searchResp.Error != "" {
			return nil, fmt.Errorf(searchResp.Error)
		}
		return nil, fmt.Errorf("unknown error occurred during search")
	}

	if len(searchResp.Search) == 0 {
		return nil, fmt.Errorf("no results found")
	}

	return searchResp.Search, nil
}
