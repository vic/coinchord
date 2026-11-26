// Command coinchord is an acoustic chord keyboard: everyday objects and a laptop mic.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"

	"coinchord/internal/audio"
	"coinchord/internal/config"
	"coinchord/internal/tui"
	"coinchord/internal/tutor"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "coinchord:", err)
		os.Exit(1)
	}
}

func run() error {
	dir, err := config.Dir()
	if err != nil {
		return err
	}
	input := flag.String("input", "", "read raw s16le mono 44100 Hz audio from this file instead of the mic")
	textFile := flag.String("text", "", "add this .txt file as a practice story")
	profile := flag.String("profile", filepath.Join(dir, "profile.json"), "profile file")
	stories := flag.String("stories", filepath.Join(dir, "stories"), "directory of .txt practice stories")
	flag.Parse()

	var notices []string
	prof, err := config.Load(*profile)
	if err != nil {
		notices = append(notices, err.Error())
	}
	all := slices.Clone(tutor.Builtin)
	user, err := tutor.Load(*stories)
	if err != nil {
		notices = append(notices, err.Error())
	}
	all = append(all, user...)
	if *textFile != "" {
		s, err := tutor.LoadFile(*textFile)
		if err != nil {
			return err
		}
		all = append(all, s)
	}

	var setup atomic.Pointer[audio.Setup]
	notice := ""
	if len(notices) > 0 {
		notice = notices[0]
	}
	m := tui.New(prof, tui.Options{ProfilePath: *profile, StoriesDir: *stories, Stories: all, Setup: &setup, Notice: notice})
	p := tea.NewProgram(m)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mic, err := audio.Open(ctx, *input)
	if err != nil {
		go p.Send(audio.Stopped{Err: fmt.Errorf("microphone: %w (install alsa-utils or sox)", err)}) // Send blocks until Run starts
	} else {
		defer mic.Close()
		go audio.Listen(mic, &setup, func(e any) { p.Send(e) })
	}
	_, err = p.Run()
	return err
}
