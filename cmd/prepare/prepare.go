package prepare

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/tanq16/raikiri/internal/thumbnails"
	u "github.com/tanq16/raikiri/utils"
)

var Cmd = &cobra.Command{
	Use:   "prepare",
	Short: "Generate thumbnails and metadata for media files",
}

func requireFFmpeg() {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		u.PrintFatal("ffmpeg not found in PATH", err)
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		u.PrintFatal("ffprobe not found in PATH", err)
	}
}

func requireTMDB() {
	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		u.PrintFatal("TMDB_API_KEY environment variable is required", nil)
	}
	thumbnails.TmdbAPIKey = apiKey
}

func getCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		u.PrintFatal("error getting current working directory", err)
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
				u.PrintFatal(fmt.Sprintf("video file not found: %s", videoPath), nil)
			}
			u.PrintInfo(fmt.Sprintf("processing: %s", filepath.Base(videoPath)))
			if err := thumbnails.CreateVideoThumbnail(videoPath); err != nil {
				u.PrintFatal("error creating thumbnail", err)
			}
			u.PrintSuccess("complete")
			return
		}

		if thumbnailsFlags.current {
			u.PrintInfo("starting video thumbnail generation (current directory)")
			thumbnails.ProcessVideo(cwd)
		} else {
			u.PrintInfo("starting recursive video thumbnail generation")
			thumbnails.ProcessVideos(cwd)
		}
		u.PrintSuccess("complete")
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
			u.PrintInfo("starting manual show processing")
			thumbnails.ProcessShowManual(getCwd())
		} else {
			u.PrintInfo("starting automatic show processing")
			thumbnails.ProcessShowsAuto(getCwd())
		}
		u.PrintSuccess("complete")
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
			u.PrintInfo("starting manual movie processing")
			thumbnails.ProcessMovieManual(getCwd())
		} else {
			u.PrintInfo("starting automatic movie processing")
			thumbnails.ProcessMoviesAuto(getCwd())
		}
		u.PrintSuccess("complete")
	},
}

func init() {
	thumbnailsCmd.Flags().BoolVar(&thumbnailsFlags.current, "current", false, "Only process the current directory (non-recursive)")
	showsCmd.Flags().BoolVar(&showsFlags.manual, "manual", false, "Interactive matching for a single show")
	moviesCmd.Flags().BoolVar(&moviesFlags.manual, "manual", false, "Interactive matching for a single movie")

	Cmd.AddCommand(thumbnailsCmd, showsCmd, moviesCmd)
}
