//go:build !windows

package audio

import (
	"context"
	"io"
	"os/exec"
	"runtime"
)

// openMic records with arecord (Linux, alsa-utils) or rec (macOS, sox).
func openMic(ctx context.Context) (io.ReadCloser, error) {
	cmd := exec.CommandContext(ctx, "arecord", "-q", "-f", "S16_LE", "-r", "44100", "-c", "1", "-t", "raw")
	if runtime.GOOS == "darwin" {
		cmd = exec.CommandContext(ctx, "rec", "-q", "-t", "raw", "-r", "44100", "-e", "signed-integer", "-b", "16", "-c", "1", "-")
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	return out, cmd.Start()
}
