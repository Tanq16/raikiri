package thumbnails

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	u "github.com/tanq16/raikiri/utils"
)

const tmdbBaseURL = "https://api.themoviedb.org/3"
const imageBaseURL = "https://image.tmdb.org/t/p/w500"

var tmdbClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		// A custom Transport replaces DefaultTransport, so proxy support is not inherited.
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	},
}

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

func askToOverwrite(path string) bool {
	input, err := u.PromptInput(fmt.Sprintf("Thumbnail exists: %s. Overwrite?", filepath.Base(path)), "y/N")
	if err != nil {
		return false
	}
	input = strings.ToLower(input)
	return input == "y" || input == "yes"
}

func downloadFile(url string, destPath string) error {
	if url == "" {
		return fmt.Errorf("empty url")
	}

	if _, err := os.Stat(destPath); err == nil {
		if !askToOverwrite(destPath) {
			u.PrintInfo("skipped")
			return nil
		}
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := tmdbClient.Get(url)
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

// The api_key rides in the query string, so an error carrying the URL leaks it.
func redactQuery(err error) error {
	urlErr, ok := errors.AsType[*url.Error](err)
	if !ok {
		return err
	}
	safe := urlErr.URL
	if parsed, parseErr := url.Parse(urlErr.URL); parseErr == nil {
		parsed.RawQuery = ""
		safe = parsed.String()
	}
	return fmt.Errorf("%s %s: %w", urlErr.Op, safe, urlErr.Err)
}

func tmdbFetch[T any](endpoint string) (*T, error) {
	resp, err := tmdbClient.Get(endpoint)
	if err != nil {
		return nil, redactQuery(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tmdb returned %s", resp.Status)
	}

	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func searchTV(apiKey, query, year string) ([]tmdbShow, error) {
	endpoint := fmt.Sprintf("%s/search/tv?api_key=%s&query=%s", tmdbBaseURL, url.QueryEscape(apiKey), url.QueryEscape(query))
	if year != "" {
		endpoint += fmt.Sprintf("&first_air_date_year=%s", year)
	}

	result, err := tmdbFetch[tmdbSearchResponse](endpoint)
	if err != nil {
		return nil, err
	}
	return result.Results, nil
}

func getTVDetails(apiKey string, id int) (*tmdbShowDetails, error) {
	return tmdbFetch[tmdbShowDetails](fmt.Sprintf("%s/tv/%d?api_key=%s", tmdbBaseURL, id, url.QueryEscape(apiKey)))
}

func searchMovie(apiKey, query, year string) ([]tmdbMovie, error) {
	endpoint := fmt.Sprintf("%s/search/movie?api_key=%s&query=%s", tmdbBaseURL, url.QueryEscape(apiKey), url.QueryEscape(query))
	if year != "" {
		endpoint += fmt.Sprintf("&year=%s", year)
	}

	result, err := tmdbFetch[tmdbMovieSearchResponse](endpoint)
	if err != nil {
		return nil, err
	}
	return result.Results, nil
}

func getMovieDetails(apiKey string, id int) (*tmdbMovieDetails, error) {
	return tmdbFetch[tmdbMovieDetails](fmt.Sprintf("%s/movie/%d?api_key=%s", tmdbBaseURL, id, url.QueryEscape(apiKey)))
}
