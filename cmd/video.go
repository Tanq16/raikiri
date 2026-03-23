package cmd

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/tanq16/raikiri/internal/video"
)

var videoInfoCmd = &cobra.Command{
	Use:   "video-info <file>",
	Short: "Display detailed information about a video file using ffprobe",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := video.RunVideoInfo(args[0]); err != nil {
			log.Fatalf("ERROR [video-info] %v", err)
		}
	},
}

var videoEncodeFlags struct {
	quality string
	faster  bool
}

var videoEncodeCmd = &cobra.Command{
	Use:   "video-encode <file>",
	Short: "Smart encode video to H.265 with automatic stream selection",
	Long: `Probes the input file, selects the best audio stream (rejecting commentary),
keeps all subtitles, picks the right container (MP4 or MKV), and encodes
video to libx265 with the chosen quality tier.

Auto-halves frame rates above 30 fps (60→30, 59.94→29.97, 50→25).
Uses preset slow by default for best compression; use --faster for preset medium.

Output file is generated automatically as <basename>.h265.<mp4|mkv>.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		opts := video.EncodeOptions{
			Quality: videoEncodeFlags.quality,
			Faster:  videoEncodeFlags.faster,
		}
		if err := video.RunEncode(args[0], opts); err != nil {
			log.Fatalf("ERROR [video-encode] %v", err)
		}
	},
}

func init() {
	videoEncodeCmd.Flags().StringVarP(&videoEncodeFlags.quality, "quality", "q", "medium", "Quality tier: very-high, high, medium, low")
	videoEncodeCmd.Flags().BoolVar(&videoEncodeFlags.faster, "faster", false, "Use preset medium instead of slow for faster encoding")

	rootCmd.AddCommand(videoInfoCmd)
	rootCmd.AddCommand(videoEncodeCmd)
}
