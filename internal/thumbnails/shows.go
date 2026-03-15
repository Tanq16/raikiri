package thumbnails

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ProcessShowsAuto auto-matches all subdirectories to TMDB TV shows.
func ProcessShowsAuto(rootDir string) {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		log.Fatalf("ERROR [thumbnails] %v", err)
	}

	regexNameYear := regexp.MustCompile(`^(.*) \((\d{4})\)?$`)

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		folderName := entry.Name()
		fullPath := filepath.Join(rootDir, folderName)
		log.Printf("INFO [thumbnails] processing folder: %s", folderName)

		match := regexNameYear.FindStringSubmatch(folderName)
		var queryName, queryYear string
		if match != nil {
			queryName = strings.TrimSpace(match[1])
			queryYear = match[2]
		} else {
			queryName = folderName
		}

		results, err := searchTV(queryName, queryYear)
		if err != nil {
			log.Printf("ERROR [thumbnails] TMDB error: %v", err)
			continue
		}
		if len(results) == 0 {
			if queryYear != "" {
				results, _ = searchTV(queryName, "")
			}
			if len(results) == 0 {
				log.Printf("WARN [thumbnails] no matches found")
				continue
			}
		}

		best := results[0]
		log.Printf("OK [thumbnails] match: %s (%s) [ID:%d]", best.Name, best.FirstAirDate, best.ID)

		details, err := getTVDetails(best.ID)
		if err != nil {
			log.Printf("ERROR [thumbnails] failed to get details: %v", err)
			continue
		}

		if details.PosterPath != "" {
			url := imageBaseURL + details.PosterPath
			dest := filepath.Join(fullPath, ".thumbnail.jpg")
			if err := downloadFile(url, dest); err == nil {
				log.Printf("OK [thumbnails] show poster: OK")
			}
		}

		localSeasons, _ := os.ReadDir(fullPath)
		for _, ls := range localSeasons {
			if !ls.IsDir() {
				continue
			}
			reNum := regexp.MustCompile(`(\d+)`)
			sMatch := reNum.FindString(ls.Name())
			if sMatch == "" {
				continue
			}
			sNum, _ := strconv.Atoi(sMatch)

			for _, ts := range details.Seasons {
				if ts.SeasonNumber == sNum && ts.PosterPath != "" {
					sUrl := imageBaseURL + ts.PosterPath
					sDest := filepath.Join(fullPath, ls.Name(), ".thumbnail.jpg")
					if err := downloadFile(sUrl, sDest); err == nil {
						log.Printf("OK [thumbnails] season %d poster: OK", sNum)
					}
					break
				}
			}
		}
	}
}

// ProcessShowManual interactively matches the current directory to a TMDB TV show.
func ProcessShowManual(currentDir string) {
	dirName := filepath.Base(currentDir)
	log.Printf("INFO [thumbnails] processing directory: %s", dirName)

	cleanName := strings.ReplaceAll(dirName, "-", " ")
	cleanName = strings.ReplaceAll(cleanName, ".", " ")

	results, err := searchTV(cleanName, "")
	if err != nil {
		log.Fatalf("ERROR [thumbnails] search failed: %v", err)
	}

	fmt.Println("\n--- Possible Matches ---")
	maxDisplay := min(5, len(results))
	for i, r := range results {
		if i >= maxDisplay {
			break
		}
		date := "N/A"
		if len(r.FirstAirDate) >= 4 {
			date = r.FirstAirDate[:4]
		}
		fmt.Printf("%d. %s (%s) - ID: %d\n", i+1, r.Name, date, r.ID)
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
			log.Printf("ERROR [thumbnails] invalid ID")
			return
		}
	} else {
		log.Printf("ERROR [thumbnails] invalid selection")
		return
	}

	details, err := getTVDetails(tmdbID)
	if err != nil {
		log.Fatalf("ERROR [thumbnails] failed to get details: %v", err)
	}

	fmt.Printf("\nSelected: %s\n", details.Name)
	fmt.Print("Apply Show Poster? [Y/n]: ")
	ans, _ := reader.ReadString('\n')
	if strings.TrimSpace(strings.ToLower(ans)) != "n" {
		if details.PosterPath != "" {
			err := downloadFile(imageBaseURL+details.PosterPath, filepath.Join(currentDir, ".thumbnail.jpg"))
			if err != nil {
				log.Printf("ERROR [thumbnails] error downloading show poster: %v", err)
			} else {
				log.Printf("OK [thumbnails] show poster applied")
			}
		}
	}

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
	sort.Strings(localFolders)

	log.Printf("INFO [thumbnails] found %d local folders, attempting to map to TMDB seasons", len(localFolders))

	for _, folder := range localFolders {
		reNum := regexp.MustCompile(`(\d+)`)
		sMatch := reNum.FindString(folder)
		if sMatch == "" {
			continue
		}
		sNum, _ := strconv.Atoi(sMatch)

		for _, ts := range details.Seasons {
			if ts.SeasonNumber == sNum && ts.PosterPath != "" {
				dest := filepath.Join(currentDir, folder, ".thumbnail.jpg")
				err := downloadFile(imageBaseURL+ts.PosterPath, dest)
				if err == nil {
					log.Printf("OK [thumbnails] season %d (%s): done", sNum, folder)
				}
				break
			}
		}
	}
}
