package thumbnails

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/tanq16/raikiri/handlers" // just for GetVideoDuration
)

const tmdbBaseURL = "https://api.themoviedb.org/3"
const imageBaseURL = "https://image.tmdb.org/t/p/w500"

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

func FormatDuration(seconds float64) string {
	hours := int(seconds) / 3600
	minutes := int(seconds) % 3600 / 60
	secs := int(seconds) % 60
	frac := seconds - float64(int(seconds))
	millis := int(frac * 1000)
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, secs, millis)
}

func CreateVideoThumbnail(filePath string) error {
	dir := filepath.Dir(filePath)
	filename := filepath.Base(filePath)
	thumbFilename := fmt.Sprintf(".%s.thumbnail.jpg", filename)
	thumbPath := filepath.Join(dir, thumbFilename)

	if _, err := os.Stat(thumbPath); err == nil {
		if !askToOverwrite(thumbPath) {
			return nil
		}
	}

	duration, err := handlers.GetVideoDuration(filePath)
	if err != nil {
		return fmt.Errorf("failed to get video duration: %w", err)
	}

	seekTime := duration / 2.0
	if seekTime >= duration {
		seekTime = max(0, duration-0.5)
	}
	seekTimeStr := FormatDuration(seekTime)

	cmd := exec.Command("ffmpeg", "-ss", seekTimeStr, "-i", filePath, "-vframes", "1", "-vf", "scale=400:-1", "-q:v", "3", "-y", thumbPath)
	return cmd.Run()
}

func ProcessVideos(rootDir string) {
	var filesToProcess []string
	videoExts := []string{".mp4", ".mkv", ".webm", ".mov", ".avi"}

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && !strings.HasPrefix(info.Name(), ".") {
			ext := strings.ToLower(filepath.Ext(info.Name()))
			if slices.Contains(videoExts, ext) {
				filesToProcess = append(filesToProcess, path)
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("Error walking directory: %v", err)
		return
	}

	fmt.Printf("Found %d video files in '%s'.\n", len(filesToProcess), rootDir)
	for i, filePath := range filesToProcess {
		fmt.Printf("[%d/%d] Processing: %s\n", i+1, len(filesToProcess), filepath.Base(filePath))
		err := CreateVideoThumbnail(filePath)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
		}
	}
}

func ProcessVideo(currentDir string) {
	var filesToProcess []string
	videoExts := []string{".mp4", ".mkv", ".webm", ".mov", ".avi"}

	entries, err := os.ReadDir(currentDir)
	if err != nil {
		log.Printf("Error reading directory: %v", err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if slices.Contains(videoExts, ext) {
			filePath := filepath.Join(currentDir, entry.Name())
			filesToProcess = append(filesToProcess, filePath)
		}
	}

	fmt.Printf("Found %d video files in '%s'.\n", len(filesToProcess), currentDir)
	for i, filePath := range filesToProcess {
		fmt.Printf("[%d/%d] Processing: %s\n", i+1, len(filesToProcess), filepath.Base(filePath))
		err := CreateVideoThumbnail(filePath)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
		}
	}
}

func ProcessShowsAuto(rootDir string) {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		log.Fatal(err)
	}

	regexNameYear := regexp.MustCompile(`^(.*) \((\d{4})\)?$`)

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		folderName := entry.Name()
		fullPath := filepath.Join(rootDir, folderName)
		fmt.Printf("\nProcessing Folder: %s\n", folderName)

		// Parse Name/Year
		match := regexNameYear.FindStringSubmatch(folderName)
		var queryName, queryYear string
		if match != nil {
			queryName = strings.TrimSpace(match[1])
			queryYear = match[2]
		} else {
			queryName = folderName // Fallback to raw folder name
		}

		// Search
		results, err := searchTV(queryName, queryYear)
		if err != nil {
			fmt.Printf("-> TMDB Error: %v\n", err)
			continue
		}
		if len(results) == 0 {
			// Try fallback without year
			if queryYear != "" {
				results, _ = searchTV(queryName, "")
			}
			if len(results) == 0 {
				fmt.Println("-> No matches found.")
				continue
			}
		}

		// Pick Top Result
		best := results[0]
		fmt.Printf("-> Match: %s (%s) [ID:%d]\n", best.Name, best.FirstAirDate, best.ID)

		details, err := getTVDetails(best.ID)
		if err != nil {
			fmt.Printf("-> Failed to get details: %v\n", err)
			continue
		}

		// Download Show Poster
		if details.PosterPath != "" {
			url := imageBaseURL + details.PosterPath
			dest := filepath.Join(fullPath, ".thumbnail.jpg")
			if err := downloadFile(url, dest); err == nil {
				fmt.Println("-> Show Poster: OK")
			}
		}

		// Download Season Posters (Auto-match local folders)
		localSeasons, _ := os.ReadDir(fullPath)
		for _, ls := range localSeasons {
			if !ls.IsDir() {
				continue
			}
			// Attempt to extract number from folder name like "Season 1", "S01", "1"
			// Simple heuristic: look for digits
			reNum := regexp.MustCompile(`(\d+)`)
			sMatch := reNum.FindString(ls.Name())
			if sMatch == "" {
				continue
			}
			sNum, _ := strconv.Atoi(sMatch)

			// Find matching TMDB season
			for _, ts := range details.Seasons {
				if ts.SeasonNumber == sNum && ts.PosterPath != "" {
					sUrl := imageBaseURL + ts.PosterPath
					sDest := filepath.Join(fullPath, ls.Name(), ".thumbnail.jpg")
					if err := downloadFile(sUrl, sDest); err == nil {
						fmt.Printf("-> Season %d Poster: OK\n", sNum)
					}
					break
				}
			}
		}
	}
}

func ProcessShowManual(currentDir string) {
	dirName := filepath.Base(currentDir)
	fmt.Printf("Processing Directory: %s\n", dirName)

	// Clean name for search
	cleanName := strings.ReplaceAll(dirName, "-", " ")
	cleanName = strings.ReplaceAll(cleanName, ".", " ")

	results, err := searchTV(cleanName, "")
	if err != nil {
		log.Fatalf("Search failed: %v", err)
	}

	fmt.Println("\n--- Possible Matches ---")
	for i, r := range results {
		if i >= 5 {
			break
		}
		date := "N/A"
		if len(r.FirstAirDate) >= 4 {
			date = r.FirstAirDate[:4]
		}
		fmt.Printf("%d. %s (%s) - ID: %d\n", i+1, r.Name, date, r.ID)
	}
	fmt.Printf("%d. Enter TMDB ID Manually\n", min(len(results), 5)+1)

	reader := getReader()
	fmt.Print("\nSelect option (or 'q' to quit): ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "q" {
		return
	}

	var tmdbID int
	choice, err := strconv.Atoi(input)
	if err == nil && choice > 0 && choice <= len(results) {
		tmdbID = results[choice-1].ID
	} else {
		fmt.Print("Enter TMDB ID: ")
		manualInput, _ := reader.ReadString('\n')
		tmdbID, err = strconv.Atoi(strings.TrimSpace(manualInput))
		if err != nil {
			fmt.Println("Invalid ID")
			return
		}
	}

	details, err := getTVDetails(tmdbID)
	if err != nil {
		log.Fatalf("Failed to get details: %v", err)
	}

	fmt.Printf("\nSelected: %s\n", details.Name)
	fmt.Print("Apply Show Poster? [Y/n]: ")
	ans, _ := reader.ReadString('\n')
	if strings.TrimSpace(strings.ToLower(ans)) != "n" {
		if details.PosterPath != "" {
			err := downloadFile(imageBaseURL+details.PosterPath, filepath.Join(currentDir, ".thumbnail.jpg"))
			if err != nil {
				fmt.Printf("Error downloading show poster: %v\n", err)
			} else {
				fmt.Println("-> Show Poster applied.")
			}
		}
	}

	// Manual Season matching
	fmt.Print("Apply Season Posters? [Y/n]: ")
	ans, _ = reader.ReadString('\n')
	if strings.TrimSpace(strings.ToLower(ans)) == "n" {
		return
	}

	localEntries, _ := os.ReadDir(currentDir)
	localFolders := []string{}
	for _, e := range localEntries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			localFolders = append(localFolders, e.Name())
		}
	}
	// Sort naturally-ish
	sort.Strings(localFolders)

	fmt.Printf("\nFound %d local folders. Attempting to map to TMDB seasons.\n", len(localFolders))

	for _, folder := range localFolders {
		// Heuristic match
		reNum := regexp.MustCompile(`(\d+)`)
		sMatch := reNum.FindString(folder)
		if sMatch == "" {
			continue
		}
		sNum, _ := strconv.Atoi(sMatch)

		for _, ts := range details.Seasons {
			if ts.SeasonNumber == sNum && ts.PosterPath != "" {
				dest := filepath.Join(currentDir, folder, ".thumbnail.jpg")
				// Quietly try to download seasons unless collision
				err := downloadFile(imageBaseURL+ts.PosterPath, dest)
				if err == nil {
					fmt.Printf("-> Season %d (%s): Done\n", sNum, folder)
				}
				break
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
