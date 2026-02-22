package thumbnails

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const tmdbBaseURL = "https://api.themoviedb.org/3"
const imageBaseURL = "https://image.tmdb.org/t/p/w500"

// TmdbAPIKey is set from the TMDB_API_KEY environment variable.
var TmdbAPIKey string

type tmdbSearchResponse struct {
	Results []tmdbShow `json:"results"`
}

type tmdbShow struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	FirstAirDate string `json:"first_air_date"`
	PosterPath   string `json:"poster_path"`
}

type tmdbShowDetails struct {
	ID         int          `json:"id"`
	Name       string       `json:"name"`
	PosterPath string       `json:"poster_path"`
	Seasons    []tmdbSeason `json:"seasons"`
}

type tmdbSeason struct {
	SeasonNumber int    `json:"season_number"`
	Name         string `json:"name"`
	PosterPath   string `json:"poster_path"`
}

type tmdbMovieSearchResponse struct {
	Results []tmdbMovie `json:"results"`
}

type tmdbMovie struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	ReleaseDate string `json:"release_date"`
	PosterPath  string `json:"poster_path"`
}

type tmdbMovieDetails struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	PosterPath string `json:"poster_path"`
}

func getReader() *bufio.Reader {
	return bufio.NewReader(os.Stdin)
}

func askToOverwrite(path string) bool {
	fmt.Printf("Thumbnail already exists at: %s\nOverwrite? [y/N]: ", filepath.Base(path))
	reader := getReader()
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

func downloadFile(url string, destPath string) error {
	if url == "" {
		return fmt.Errorf("empty url")
	}

	if _, err := os.Stat(destPath); err == nil {
		if !askToOverwrite(destPath) {
			fmt.Println("Skipped.")
			return nil
		}
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	return err
}

func searchTV(query string, year string) ([]tmdbShow, error) {
	endpoint := fmt.Sprintf("%s/search/tv?api_key=%s&query=%s", tmdbBaseURL, TmdbAPIKey, url.QueryEscape(query))
	if year != "" {
		endpoint += fmt.Sprintf("&first_air_date_year=%s", year)
	}

	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result tmdbSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Results, nil
}

func getTVDetails(id int) (*tmdbShowDetails, error) {
	endpoint := fmt.Sprintf("%s/tv/%d?api_key=%s", tmdbBaseURL, id, TmdbAPIKey)
	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var details tmdbShowDetails
	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		return nil, err
	}
	return &details, nil
}

func searchMovie(query string, year string) ([]tmdbMovie, error) {
	endpoint := fmt.Sprintf("%s/search/movie?api_key=%s&query=%s", tmdbBaseURL, TmdbAPIKey, url.QueryEscape(query))
	if year != "" {
		endpoint += fmt.Sprintf("&year=%s", year)
	}

	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result tmdbMovieSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Results, nil
}

func getMovieDetails(id int) (*tmdbMovieDetails, error) {
	endpoint := fmt.Sprintf("%s/movie/%d?api_key=%s", tmdbBaseURL, id, TmdbAPIKey)
	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var details tmdbMovieDetails
	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		return nil, err
	}
	return &details, nil
}
