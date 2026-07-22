package thumbnails

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	u "github.com/tanq16/raikiri/utils"
)

func ProcessMoviesAuto(rootDir string) {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		u.PrintFatal("error reading directory", err)
	}

	regexNameYear := regexp.MustCompile(`^(.*) \((\d{4})\)?$`)

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		folderName := entry.Name()
		fullPath := filepath.Join(rootDir, folderName)
		u.PrintInfo(fmt.Sprintf("processing folder: %s", folderName))

		match := regexNameYear.FindStringSubmatch(folderName)
		var queryName, queryYear string
		if match != nil {
			queryName = strings.TrimSpace(match[1])
			queryYear = match[2]
		} else {
			queryName = folderName
		}

		results, err := searchMovie(queryName, queryYear)
		if err != nil {
			u.PrintError("TMDB error", err)
			continue
		}
		if len(results) == 0 {
			if queryYear != "" {
				results, _ = searchMovie(queryName, "")
			}
			if len(results) == 0 {
				u.PrintWarn("no matches found", nil)
				continue
			}
		}

		best := results[0]
		u.PrintInfo(fmt.Sprintf("match: %s (%s) [ID:%d]", best.Title, best.ReleaseDate, best.ID))

		details, err := getMovieDetails(best.ID)
		if err != nil {
			u.PrintError("failed to get details", err)
			continue
		}

		if details.PosterPath != "" {
			url := imageBaseURL + details.PosterPath
			dest := filepath.Join(fullPath, ".thumbnail.jpg")
			if err := downloadFile(url, dest); err == nil {
				u.PrintSuccess("movie poster: OK")
			}
		}
	}
}

func ProcessMovieManual(currentDir string) {
	dirName := filepath.Base(currentDir)
	u.PrintInfo(fmt.Sprintf("processing directory: %s", dirName))

	cleanName := strings.ReplaceAll(dirName, "-", " ")
	cleanName = strings.ReplaceAll(cleanName, ".", " ")

	results, err := searchMovie(cleanName, "")
	if err != nil {
		u.PrintFatal("search failed", err)
	}

	maxDisplay := min(5, len(results))
	labels := make([]string, 0, maxDisplay+1)
	for i := range maxDisplay {
		r := results[i]
		date := "N/A"
		if len(r.ReleaseDate) >= 4 {
			date = r.ReleaseDate[:4]
		}
		labels = append(labels, fmt.Sprintf("%s (%s) - ID: %d", r.Title, date, r.ID))
	}
	labels = append(labels, "Enter TMDB ID Manually")

	idx, err := u.PromptSelect("Select a match", labels)
	if err != nil {
		u.PrintError("input error", err)
		return
	}
	if idx < 0 {
		return
	}

	var tmdbID int
	if idx == maxDisplay {
		manualInput, err := u.PromptInput("Enter TMDB ID", "")
		if err != nil {
			u.PrintError("input error", err)
			return
		}
		tmdbID, err = strconv.Atoi(manualInput)
		if err != nil {
			u.PrintError("invalid ID", nil)
			return
		}
	} else {
		tmdbID = results[idx].ID
	}

	details, err := getMovieDetails(tmdbID)
	if err != nil {
		u.PrintFatal("failed to get details", err)
	}

	u.PrintInfo(fmt.Sprintf("selected: %s", details.Title))
	ans, err := u.PromptInput("Apply Movie Poster?", "Y/n")
	if err != nil {
		u.PrintError("input error", err)
		return
	}
	if strings.ToLower(ans) != "n" {
		if details.PosterPath != "" {
			err := downloadFile(imageBaseURL+details.PosterPath, filepath.Join(currentDir, ".thumbnail.jpg"))
			if err != nil {
				u.PrintError("error downloading movie poster", err)
			} else {
				u.PrintSuccess("movie poster applied")
			}
		}
	}
}
