package main

type SabNZBResponse struct {
	History struct {
		TotalSize string `json:"total_size"`
		MonthSize string `json:"month_size"`
		WeekSize  string `json:"week_size"`
		DaySize   string `json:"day_size"`
		Slots     []struct {
			ID           int         `json:"id"`
			Completed    int         `json:"completed"`
			Name         string      `json:"name"`
			NzbName      string      `json:"nzb_name"`
			Category     string      `json:"category"`
			Pp           string      `json:"pp"`
			Script       string      `json:"script"`
			Report       string      `json:"report"`
			URL          string      `json:"url"`
			Status       string      `json:"status"`
			NzoID        string      `json:"nzo_id"`
			Storage      string      `json:"storage"`
			Path         string      `json:"path"`
			ScriptLog    string      `json:"script_log"`
			ScriptLine   string      `json:"script_line"`
			DownloadTime interface{} `json:"download_time"`
			PostprocTime int         `json:"postproc_time"`
			StageLog     []struct {
				Name    string   `json:"name"`
				Actions []string `json:"actions"`
			} `json:"stage_log"`
			Downloaded   int    `json:"downloaded"`
			Completeness any    `json:"completeness"`
			FailMessage  string `json:"fail_message"`
			URLInfo      string `json:"url_info"`
			Bytes        int    `json:"bytes"`
			Meta         any    `json:"meta"`
			Series       string `json:"series"`
			Md5Sum       string `json:"md5sum"`
			Password     any    `json:"password"`
			ActionLine   string `json:"action_line"`
			Size         string `json:"size"`
			Loaded       bool   `json:"loaded"`
			Retry        int    `json:"retry"`
		} `json:"slots"`
		Noofslots         int    `json:"noofslots"`
		LastHistoryUpdate int    `json:"last_history_update"`
		Version           string `json:"version"`
	} `json:"history"`
}

type OMDBTVSearchResponse struct {
	Title    string `json:"Title"`
	Year     string `json:"Year"`
	Rated    string `json:"Rated"`
	Released string `json:"Released"`
	Runtime  string `json:"Runtime"`
	Genre    string `json:"Genre"`
	Director string `json:"Director"`
	Writer   string `json:"Writer"`
	Actors   string `json:"Actors"`
	Plot     string `json:"Plot"`
	Language string `json:"Language"`
	Country  string `json:"Country"`
	Awards   string `json:"Awards"`
	Poster   string `json:"Poster"`
	Ratings  []struct {
		Source string `json:"Source"`
		Value  string `json:"Value"`
	} `json:"Ratings"`
	Metascore    string `json:"Metascore"`
	ImdbRating   string `json:"imdbRating"`
	ImdbVotes    string `json:"imdbVotes"`
	ImdbID       string `json:"imdbID"`
	Type         string `json:"Type"`
	TotalSeasons string `json:"totalSeasons"`
	Response     string `json:"Response"`
	Error        string `json:"Error"`
}
