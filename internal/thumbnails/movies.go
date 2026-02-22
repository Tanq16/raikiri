package thumbnails

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/tanq16/raikiri/internal/utils"
)

// ProcessMoviesAuto auto-matches all subdirectories to TMDB movies.
func ProcessMoviesAuto(rootDir string) {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		utils.PrintFatal(fmt.Sprintf("%v", err))
	}

	regexNameYear := regexp.MustCompile(`^(.*) \((\d{4})\)?$`)

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		folderName := entry.Name()
		fullPath := filepath.Join(rootDir, folderName)
		utils.PrintInfo(fmt.Sprintf("Processing Folder: %s", folderName))

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
			utils.PrintError(fmt.Sprintf("TMDB Error: %v", err))
			continue
		}
		if len(results) == 0 {
			if queryYear != "" {
				results, _ = searchMovie(queryName, "")
			}
			if len(results) == 0 {
				utils.PrintWarn("No matches found.")
				continue
			}
		}

		best := results[0]
		utils.PrintSuccess(fmt.Sprintf("Match: %s (%s) [ID:%d]", best.Title, best.ReleaseDate, best.ID))

		details, err := getMovieDetails(best.ID)
		if err != nil {
			utils.PrintError(fmt.Sprintf("Failed to get details: %v", err))
			continue
		}

		if details.PosterPath != "" {
			url := imageBaseURL + details.PosterPath
			dest := filepath.Join(fullPath, ".thumbnail.jpg")
			if err := downloadFile(url, dest); err == nil {
				utils.PrintSuccess("Movie Poster: OK")
			}
		}
	}
}

// ProcessMovieManual interactively matches the current directory to a TMDB movie.
func ProcessMovieManual(currentDir string) {
	dirName := filepath.Base(currentDir)
	utils.PrintInfo(fmt.Sprintf("Processing Directory: %s", dirName))

	cleanName := strings.ReplaceAll(dirName, "-", " ")
	cleanName = strings.ReplaceAll(cleanName, ".", " ")

	results, err := searchMovie(cleanName, "")
	if err != nil {
		utils.PrintFatal(fmt.Sprintf("Search failed: %v", err))
	}

	fmt.Println("\n--- Possible Matches ---")
	maxDisplay := min(5, len(results))
	for i, r := range results {
		if i >= maxDisplay {
			break
		}
		date := "N/A"
		if len(r.ReleaseDate) >= 4 {
			date = r.ReleaseDate[:4]
		}
		fmt.Printf("%d. %s (%s) - ID: %d\n", i+1, r.Title, date, r.ID)
	}
	manualOptionNum := maxDisplay + 1
	fmt.Printf("%d. Enter TMDB ID Manually\n", manualOptionNum)

	reader := getReader()
	fmt.Print("\nSelect option (or 'q' to quit): ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "q" {
		return
	}

	var tmdbID int
	choice, err := strconv.Atoi(input)
	if err == nil && choice > 0 && choice <= maxDisplay {
		tmdbID = results[choice-1].ID
	} else if err == nil && choice == manualOptionNum {
		fmt.Print("Enter TMDB ID: ")
		manualInput, _ := reader.ReadString('\n')
		tmdbID, err = strconv.Atoi(strings.TrimSpace(manualInput))
		if err != nil {
			utils.PrintError("Invalid ID")
			return
		}
	} else {
		utils.PrintError("Invalid selection")
		return
	}

	details, err := getMovieDetails(tmdbID)
	if err != nil {
		utils.PrintFatal(fmt.Sprintf("Failed to get details: %v", err))
	}

	fmt.Printf("\nSelected: %s\n", details.Title)
	fmt.Print("Apply Movie Poster? [Y/n]: ")
	ans, _ := reader.ReadString('\n')
	if strings.TrimSpace(strings.ToLower(ans)) != "n" {
		if details.PosterPath != "" {
			err := downloadFile(imageBaseURL+details.PosterPath, filepath.Join(currentDir, ".thumbnail.jpg"))
			if err != nil {
				utils.PrintError(fmt.Sprintf("Error downloading movie poster: %v", err))
			} else {
				utils.PrintSuccess("Movie Poster applied.")
			}
		}
	}
}
