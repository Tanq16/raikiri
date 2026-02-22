package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/tanq16/raikiri/internal/thumbnails"
	"github.com/tanq16/raikiri/internal/utils"
)

var prepareCmd = &cobra.Command{
	Use:   "prepare",
	Short: "Generate thumbnails and metadata for media files",
}

func requireFFmpeg() {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		utils.PrintFatal("`ffmpeg` not found in PATH.")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		utils.PrintFatal("`ffprobe` not found in PATH.")
	}
}

func requireTMDB() {
	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		utils.PrintFatal("TMDB_API_KEY environment variable is required.")
	}
	thumbnails.TmdbAPIKey = apiKey
}

func getCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		utils.PrintFatal(fmt.Sprintf("Error getting current working directory: %v", err))
	}
	return cwd
}

var thumbnailsFlags struct {
	current bool
}

var thumbnailsCmd = &cobra.Command{
	Use:   "thumbnails [file]",
	Short: "Generate video thumbnails using ffmpeg",
	Long: `Generate video thumbnails using ffmpeg.

By default, generates thumbnails recursively for all videos under the current directory.
Use --current to only process the current directory (non-recursive).
Pass a single file path as an argument to generate a thumbnail for that file only.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		requireFFmpeg()
		cwd := getCwd()

		if len(args) == 1 {
			videoPath := args[0]
			if !filepath.IsAbs(videoPath) {
				videoPath = filepath.Join(cwd, videoPath)
			}
			if _, err := os.Stat(videoPath); os.IsNotExist(err) {
				utils.PrintFatal(fmt.Sprintf("Video file not found: %s", videoPath))
			}
			utils.PrintInfo(fmt.Sprintf("Processing: %s", filepath.Base(videoPath)))
			if err := thumbnails.CreateVideoThumbnail(videoPath); err != nil {
				utils.PrintFatal(fmt.Sprintf("Error creating thumbnail: %v", err))
			}
			utils.PrintSuccess("Complete.")
			return
		}

		if thumbnailsFlags.current {
			utils.PrintInfo("Starting video thumbnail generation (current directory)...")
			thumbnails.ProcessVideo(cwd)
		} else {
			utils.PrintInfo("Starting recursive video thumbnail generation...")
			thumbnails.ProcessVideos(cwd)
		}
		utils.PrintSuccess("Complete.")
	},
}

var showsFlags struct {
	manual bool
}

var showsCmd = &cobra.Command{
	Use:   "shows",
	Short: "Download TV show posters from TMDB",
	Long: `Download TV show posters from TMDB.

By default, auto-matches all show subdirectories in the current directory.
Use --manual for interactive matching of the current directory to a specific show.`,
	Run: func(cmd *cobra.Command, args []string) {
		requireTMDB()
		if showsFlags.manual {
			utils.PrintInfo("Starting manual show processing...")
			thumbnails.ProcessShowManual(getCwd())
		} else {
			utils.PrintInfo("Starting automatic show processing...")
			thumbnails.ProcessShowsAuto(getCwd())
		}
		utils.PrintSuccess("Complete.")
	},
}

var moviesFlags struct {
	manual bool
}

var moviesCmd = &cobra.Command{
	Use:   "movies",
	Short: "Download movie posters from TMDB",
	Long: `Download movie posters from TMDB.

By default, auto-matches all movie subdirectories in the current directory.
Use --manual for interactive matching of the current directory to a specific movie.`,
	Run: func(cmd *cobra.Command, args []string) {
		requireTMDB()
		if moviesFlags.manual {
			utils.PrintInfo("Starting manual movie processing...")
			thumbnails.ProcessMovieManual(getCwd())
		} else {
			utils.PrintInfo("Starting automatic movie processing...")
			thumbnails.ProcessMoviesAuto(getCwd())
		}
		utils.PrintSuccess("Complete.")
	},
}

func init() {
	thumbnailsCmd.Flags().BoolVar(&thumbnailsFlags.current, "current", false, "Only process the current directory (non-recursive)")
	showsCmd.Flags().BoolVar(&showsFlags.manual, "manual", false, "Interactive matching for a single show")
	moviesCmd.Flags().BoolVar(&moviesFlags.manual, "manual", false, "Interactive matching for a single movie")

	prepareCmd.AddCommand(thumbnailsCmd, showsCmd, moviesCmd)
	rootCmd.AddCommand(prepareCmd)
}
