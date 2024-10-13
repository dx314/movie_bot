# Movie Bot

Movie Bot is a Telegram bot that helps users search for movies and TV shows, find NZB files, and manage downloads through SABnzbd.

## Features

- Search for movies and TV shows using OMDB API
- Search for NZB files on NZBGeek using IMDB ID
- Add NZB files to SABnzbd for downloading
- Monitor download progress and provide status updates
- Support for different categories: movies, TV shows, kids movies, and kids TV shows

## Requirements

- Go 1.16 or higher
- Telegram Bot API Token
- OMDB API Key
- NZBGeek API Key
- SABnzbd API Key and URL

## Installation

1. Clone the repository:
   ```
   git clone https://github.com/yourusername/movie-bot.git
   cd movie-bot
   ```

2. Install dependencies:
   ```
   go mod tidy
   ```

3. Set up environment variables:
   Create a `.env` file in the project root and add the following:
   ```
   TELEGRAM_BOT_TOKEN=your_telegram_bot_token
   OMDB_API_KEY=your_omdb_api_key
   NZBGEEK_API_KEY=your_nzbgeek_api_key
   SABNZBD_API_URL=your_sabnzbd_api_url
   SABNZBD_API_KEY=your_sabnzbd_api_key
   ```

4. Build the project:
   ```
   go build
   ```

5. Run the bot:
   ```
   ./movie-bot
   ```

## Usage

Start a conversation with the bot on Telegram and use the following commands:

- `/movie [movie name] [year]`: Search for a movie
- `/tv [TV show name] [year]`: Search for a TV show
- `/km [movie name] [year]`: Search for a kids movie
- `/ktv [TV show name] [year]`: Search for a kids TV show

If you don't provide the year, the bot will ask for it separately.

## Project Structure

- `main.go`: Main entry point and bot initialization
- `telegram.go`: Telegram bot message handling and user interactions
- `omdb.go`: OMDB API integration for movie and TV show searches
- `nzb.go`: NZBGeek integration and SABnzbd download management
- `helpers.go`: Utility functions and helpers

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
