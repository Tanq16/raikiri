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

	u.PrintGeneric("")
	u.PrintInfo("Possible Matches")
	maxDisplay := min(5, len(results))
	for i, r := range results {
		if i >= maxDisplay {
			break
		}
		date := "N/A"
		if len(r.ReleaseDate) >= 4 {
			date = r.ReleaseDate[:4]
		}
		u.PrintGeneric(fmt.Sprintf("  %d. %s (%s) - ID: %d", i+1, r.Title, date, r.ID))
	}
	manualOptionNum := maxDisplay + 1
	u.PrintGeneric(fmt.Sprintf("  %d. Enter TMDB ID Manually", manualOptionNum))

	u.PrintGeneric("")
	input, err := u.PromptInput("Select option (or 'q' to quit)", "")
	if err != nil {
		u.PrintError("input error", err)
		return
	}

	if input == "q" {
		return
	}

	var tmdbID int
	choice, atoiErr := strconv.Atoi(input)
	if atoiErr == nil && choice > 0 && choice <= maxDisplay {
		tmdbID = results[choice-1].ID
	} else if atoiErr == nil && choice == manualOptionNum {
		manualInput, err := u.PromptInput("Enter TMDB ID", "")
		if err != nil {
			u.PrintError("input error", err)
			return
		}
		tmdbID, atoiErr = strconv.Atoi(manualInput)
		if atoiErr != nil {
			u.PrintError("invalid ID", nil)
			return
		}
	} else {
		u.PrintError("invalid selection", nil)
		return
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
