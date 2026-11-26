package audio

import (
	"fmt"
	"math"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"coinchord/internal/dsp"
)

// MicState is what the incoming signal says about the microphone.
type MicState uint8

const (
	MicWaiting  MicState = iota // no audio yet
	MicSilent                   // exact digital silence: muted or wrong device
	MicClipping                 // samples hit full scale: gain too high
	MicLive                     // usable signal
)

const (
	window       = 128 // frames kept, ~3 s
	silentFrames = 43  // ~1 s of exact zeros
	clipLevel    = 0.98
	clipShare    = 0.1 // share of clipped frames that means "too loud"
	loudNoise    = 0.2 // a noise floor this loud drowns every gesture
	noiseRank    = 0.2 // noise floor = this quantile of recent loudness
)

// Monitor keeps recent loudness to judge the microphone and the room.
type Monitor struct {
	rms, peak []float64
	zeros     int // consecutive silent frames
}

// Add records one frame.
func (m *Monitor) Add(f dsp.Frame) {
	m.rms = append(m.rms, f.RMS)
	m.peak = append(m.peak, f.Peak)
	if len(m.rms) > window {
		m.rms, m.peak = m.rms[1:], m.peak[1:]
	}
	m.zeros++
	if f.Peak > 0 {
		m.zeros = 0
	}
}

// State judges the microphone from recent frames.
func (m Monitor) State() MicState {
	clipped := 0
	for _, p := range m.peak {
		if p >= clipLevel {
			clipped++
		}
	}
	switch {
	case len(m.rms) == 0:
		return MicWaiting
	case m.zeros >= silentFrames:
		return MicSilent
	case float64(clipped) >= clipShare*float64(len(m.peak)) || m.Noise() >= loudNoise:
		return MicClipping
	}
	return MicLive
}

// Noise is the room noise floor: a low quantile of recent loudness, so strokes do not raise it.
func (m Monitor) Noise() float64 {
	return m.Quantile(noiseRank)
}

// History returns recent loudness, oldest first.
func (m Monitor) History() []float64 { return slices.Clone(m.rms) }

// DB converts a 0..1 amplitude to dBFS, floored at -100.
func DB(x float64) float64 { return math.Max(-100, 20*math.Log10(math.Max(x, 1e-5))) }

// Level is the system input volume of the default microphone.
type Level struct {
	Volume float64 // 1 = 100%
	Muted  bool
}

// MicAction changes the default microphone.
type MicAction uint8

const (
	Unmute MicAction = iota
	Quieter
	Louder
)

// Control applies a mic action with wpctl (PipeWire) or pactl (PulseAudio).
func Control(a MicAction) error {
	wp := map[MicAction][]string{
		Unmute:  {"set-mute", "@DEFAULT_AUDIO_SOURCE@", "0"},
		Quieter: {"set-volume", "@DEFAULT_AUDIO_SOURCE@", "5%-"},
		Louder:  {"set-volume", "@DEFAULT_AUDIO_SOURCE@", "5%+"},
	}[a]
	if err := exec.Command("wpctl", wp...).Run(); err == nil {
		return nil
	}
	pa := map[MicAction][]string{
		Unmute:  {"set-source-mute", "@DEFAULT_SOURCE@", "0"},
		Quieter: {"set-source-volume", "@DEFAULT_SOURCE@", "-5%"},
		Louder:  {"set-source-volume", "@DEFAULT_SOURCE@", "+5%"},
	}[a]
	if err := exec.Command("pactl", pa...).Run(); err != nil {
		return fmt.Errorf("could not change the mic (needs wpctl or pactl): %w", err)
	}
	return nil
}

// ReadLevel asks wpctl for the default microphone volume.
func ReadLevel() (Level, error) {
	out, err := exec.Command("wpctl", "get-volume", "@DEFAULT_AUDIO_SOURCE@").Output()
	if err != nil {
		return Level{}, err
	}
	return parseLevel(string(out))
}

// parseLevel reads "Volume: 0.40 [MUTED]".
func parseLevel(s string) (Level, error) {
	f := strings.Fields(s)
	if len(f) < 2 || f[0] != "Volume:" {
		return Level{}, fmt.Errorf("unexpected wpctl output %q", s)
	}
	v, err := strconv.ParseFloat(f[1], 64)
	return Level{v, strings.Contains(s, "[MUTED]")}, err
}

// Quantile returns the q (0..1) quantile of recent loudness.
func (m Monitor) Quantile(q float64) float64 {
	if len(m.rms) == 0 {
		return 0
	}
	s := slices.Clone(m.rms)
	slices.Sort(s)
	return s[int(q*float64(len(s)-1))]
}

// Frames is how many frames are in the window.
func (m Monitor) Frames() int { return len(m.rms) }

// TriggerFor sets the stroke trigger above the loud end of quiet-room noise.
// Measure it while the room is quiet: strokes in the window raise it.
func TriggerFor(m Monitor) float64 { return math.Max(m.Quantile(0.9)*triggerOverNoise, minTrigger) }

const (
	triggerOverNoise = 3.8   // ~11.6 dB above the noise peaks
	minTrigger       = 0.006 // below this, digital noise alone could trigger
)
