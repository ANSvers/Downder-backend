package ffmpeg

import (
	"context"
	"downder-backend/internal/domain"
	"fmt"
	"os/exec"
	"strings"
)

// ตัวแปรคำสั่งจากแอปเรา --> โปรแกรม FFmpeg ใน Docker จัดการตัดต่อวิดีโอ
func ProcessMedia(ctx context.Context, inputPath string, outputPath string, opts domain.TrimOptions) error {
	var args []string

	if opts.StartTime != "" {
		args = append(args, "-ss", opts.StartTime)
	}

	args = append(args, "-i", inputPath)

	if opts.EndTime != "" {
		args = append(args, "-to", opts.EndTime)
	}

	format := strings.ToLower(opts.Format)
	if format == "mp3" {
		bitrate := opts.Bitrate
		if bitrate == "" {
			bitrate = "192k"
		}
		args = append(args, "-vn", "-acodec", "libmp3lame", "-b:a", bitrate)
	} else if format == "webm" {
		args = append(args, "-c:v", "libvpx-vp9", "-c:a", "libopus")
	} else {
		args = append(args, "-c", "copy")
	}

	args = append(args, outputPath)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w", err)
	}

	return nil
}
