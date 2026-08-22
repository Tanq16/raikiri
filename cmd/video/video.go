package video

import (
	"context"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	iv "github.com/tanq16/raikiri/internal/video"
	u "github.com/tanq16/raikiri/utils"
)

var InfoCmd = &cobra.Command{
	Use:   "video-info <file>",
	Short: "Display detailed information about a video file using ffprobe",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := iv.RunVideoInfo(args[0]); err != nil {
			u.PrintFatal("failed to get video info", err)
		}
	},
}

var encodeFlags struct {
	quality string
	slower  bool
}

var EncodeCmd = &cobra.Command{
	Use:   "video-encode <file>",
	Short: "Smart encode video to H.265 with automatic stream selection",
	Long: `Probes the input file, selects the best audio stream (rejecting commentary),
keeps all subtitles, picks the right container (MP4 or MKV), and encodes
video to libx265 with the chosen quality tier.

Auto-halves frame rates above 30 fps (60→30, 59.94→29.97, 50→25).
Uses preset medium by default; use --slower for preset slow (better compression, longer encode).

Output file is generated automatically as <basename>.h265.<mp4|mkv>.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		opts := iv.EncodeOptions{
			Quality: encodeFlags.quality,
			Slower:  encodeFlags.slower,
		}
		if err := iv.RunEncode(ctx, args[0], opts); err != nil {
			u.PrintFatal("video encoding failed", err)
		}
	},
}

func init() {
	EncodeCmd.Flags().StringVarP(&encodeFlags.quality, "quality", "q", "medium", "Quality tier: very-high, high, medium, low")
	EncodeCmd.Flags().BoolVar(&encodeFlags.slower, "slower", false, "Use preset slow for better compression (longer encode)")
}
