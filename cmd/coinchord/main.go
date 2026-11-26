// Command coinchord is an acoustic chord keyboard: everyday objects and a
// laptop mic. It serves its interface on a local web page.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"

	"coinchord/internal/app"
	"coinchord/internal/audio"
	"coinchord/internal/config"
	"coinchord/internal/eval"
	"coinchord/internal/server"
	"coinchord/internal/tutor"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "coinchord:", err)
		os.Exit(1)
	}
}

// onOff is a flag that only accepts "on" or "off".
type onOff bool

func (o *onOff) String() string { return map[bool]string{true: "on", false: "off"}[bool(*o)] }

func (o *onOff) Set(s string) error {
	switch s {
	case "on":
		*o = true
	case "off":
		*o = false
	default:
		return errors.New(`want "on" or "off"`)
	}
	return nil
}

func run() error {
	dir, err := config.Dir()
	if err != nil {
		return err
	}
	var output onOff
	flag.Var(&output, "output", "start with chords typing into your real windows: on or off")
	listen := flag.String("listen", "127.0.0.1:7317", "address of the web interface (loopback only)")
	open := flag.Bool("open", true, "open the interface in your browser")
	input := flag.String("input", "", "read raw s16le mono 44100 Hz audio from this file instead of the mic")
	textFile := flag.String("text", "", "add this .txt file as a practice text")
	profile := flag.String("profile", filepath.Join(dir, "profile.json"), "profile file")
	stories := flag.String("stories", filepath.Join(dir, "stories"), "directory of .txt practice texts")
	record := flag.String("record", "", "save the mic audio and calibration labels in this directory, for -eval")
	evalFile := flag.String("eval", "", "score the detector on a recorded .raw file (labels next to it) and exit")
	flag.Parse()

	notice := ""
	prof, err := config.Load(*profile)
	if err != nil {
		notice = err.Error()
	}
	if *evalFile != "" {
		return evaluate(*evalFile, prof)
	}
	all := slices.Clone(tutor.Builtin)
	user, err := tutor.Load(*stories)
	if err != nil {
		notice = err.Error()
	}
	all = append(all, user...)
	if *textFile != "" {
		s, err := tutor.LoadFile(*textFile)
		if err != nil {
			return err
		}
		all = append(all, s)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	opts := app.Options{ProfilePath: *profile, StoriesDir: *stories, Stories: all, Output: bool(output), Notice: notice}
	mic, err := audio.Open(ctx, *input)
	if err == nil && *record != "" {
		var rec *recording
		if rec, err = startRecording(*record); err != nil {
			return err
		}
		defer rec.Close()
		mic, opts.Labels = rec.tee(mic), rec.labels
		fmt.Println("Recording to", rec.raw.Name())
	}
	eng := app.New(prof, opts)
	go eng.Run(ctx)

	if err != nil {
		eng.Emit(audio.Stopped{Err: fmt.Errorf("%w (install alsa-utils on Linux or sox on macOS)", err)})
	} else {
		defer mic.Close()
		go audio.Listen(mic, eng.Setup(), eng.Emit)
	}

	h, err := server.Handler(eng)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		return err
	}
	url := "http://" + ln.Addr().String()
	fmt.Println("CoinChord is ready at", url)
	if *open {
		openBrowser(url)
	}
	srv := &http.Server{Handler: h}
	go func() { <-ctx.Done(); srv.Close() }()
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func openBrowser(url string) {
	cmd := map[string][]string{"darwin": {"open"}, "windows": {"rundll32", "url.dll,FileProtocolHandler"}}[runtime.GOOS]
	if cmd == nil {
		cmd = []string{"xdg-open"}
	}
	if err := exec.Command(cmd[0], append(cmd[1:], url)...).Start(); err != nil {
		fmt.Println("Open", url, "in your browser.")
	}
}

// recording saves raw mic audio and the calibration labels side by side.
type recording struct{ raw, labels *os.File }

func startRecording(dir string) (*recording, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	base := filepath.Join(dir, time.Now().Format("2006-01-02-150405"))
	raw, err := os.Create(base + ".raw")
	if err != nil {
		return nil, err
	}
	labels, err := os.Create(base + ".labels.jsonl")
	if err != nil {
		raw.Close()
		return nil, err
	}
	return &recording{raw, labels}, nil
}

func (r *recording) tee(mic io.ReadCloser) io.ReadCloser {
	return struct {
		io.Reader
		io.Closer
	}{io.TeeReader(mic, r.raw), mic}
}

func (r *recording) Close() error { return errors.Join(r.raw.Close(), r.labels.Close()) }

// evaluate replays a recording with the profile's kit and tuning.
func evaluate(rawPath string, prof config.Profile) error {
	raw, err := os.Open(rawPath)
	if err != nil {
		return err
	}
	defer raw.Close()
	lf, err := os.Open(strings.TrimSuffix(rawPath, ".raw") + ".labels.jsonl")
	if err != nil {
		return err
	}
	defer lf.Close()
	labels, err := eval.ReadLabels(lf)
	if err != nil {
		return err
	}
	fmt.Print(eval.Run(raw, labels, audio.Setup{Kit: prof.Kit, Tuning: prof.Tuning}))
	return nil
}
