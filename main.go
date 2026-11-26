package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/cmplx"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	SR        = 44100.0
	BlockSize = 1024 // ~23.2 ms per frame
)

type Config struct {
	Pair         string            `json:"pair"`
	Trigger      float64           `json:"trigger"`
	LeftHz       float64           `json:"left_hz"`
	RightHz      float64           `json:"right_hz"`
	ScrapeFlt    float64           `json:"scrape_flt"`
	ChordWinMs   int               `json:"chord_win_ms"`
	ProseChords  map[string]string `json:"prose_chords"`
	CodeChords   map[string]string `json:"code_chords"`
	WordPhrases  []string          `json:"word_phrases"`
}

func defaultConfig() Config {
	return Config{
		Pair:       "Small Coin (L) + Large Coin (R)",
		Trigger:    0.032,
		LeftHz:     9200,
		RightHz:    5800,
		ScrapeFlt:  0.26,
		ChordWinMs: 360,
		// Gestures per hand:
		//   L_TAP / R_TAP   = Crisp single tap
		//   L_LINE / R_LINE = Straight linear scrape (steady pitch/RMS envelope)
		//   L_CIRC / R_CIRC = Circular swirl (rhythmic RMS/pitch modulation)
		//   L_CURV / R_CURV = Curved arc/hook (pitch sweep / tapered RMS)
		//   DUAL_TAP        = Both tap simultaneously
		//   DUAL_SHAPES     = Overlapping multi-hand continuous shapes (e.g. Line + Curve)
		ProseChords: map[string]string{
			"L_TAP": "the ", "R_TAP": "and ", "DUAL_TAP": "I ",
			"L_LINE": "to ", "R_LINE": "of ",
			"L_CIRC": "in ", "R_CIRC": "that ",
			"L_CURV": "you ", "R_CURV": "for ",
			"L_LINE+R_CURV": "with ", "L_CURV+R_LINE": "from ",
			"L_CIRC+R_TAP": "this ", "R_CIRC+L_TAP": "have ",
			"L_LINE+R_CIRC": "which ", "L_CIRV+R_CIRC": "about ",
			"DUAL_SHAPES": "something ", "L_TAP+R_TAP": "is ", "R_TAP+L_TAP": "it ",
			"L_CIRC+R_CIRC": "[CODE]",
		},
		CodeChords: map[string]string{
			"L_TAP": "def ", "R_TAP": "return ", "DUAL_TAP": "()",
			"L_LINE": " = ", "R_LINE": ":\n    ",
			"L_CIRC": "for ", "R_CIRC": " in ",
			"L_CURV": "if ", "R_CURV": "else:\n    ",
			"L_LINE+R_CURV": "import ", "L_CURV+R_LINE": "print(",
			"L_CIRC+R_CIRC": "[PROSE]",
		},
		WordPhrases: []string{
			"the and to of in that you for ",
			"I have this with you from that ",
			"the something is for you and I ",
		},
	}
}

func cfgPath() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".coinchord.json")
}

func loadCfg() Config {
	c := defaultConfig()
	if b, err := os.ReadFile(cfgPath()); err == nil {
		_ = json.Unmarshal(b, &c)
	} else {
		saveCfg(c)
	}
	return c
}

func saveCfg(c Config) {
	b, _ := json.MarshalIndent(c, "", "  ")
	_ = os.WriteFile(cfgPath(), b, 0644)
}

func fft(x []complex128) []complex128 {
	n := len(x)
	if n <= 1 {
		return x
	}
	even, odd := make([]complex128, n/2), make([]complex128, n/2)
	for i := 0; i < n/2; i++ {
		even[i], odd[i] = x[2*i], x[2*i+1]
	}
	even, odd = fft(even), fft(odd)
	out := make([]complex128, n)
	for k := 0; k < n/2; k++ {
		t := cmplx.Rect(1, -2*math.Pi*float64(k)/float64(n)) * odd[k]
		out[k], out[k+n/2] = even[k]+t, even[k]-t
	}
	return out
}

type FrameFeat struct {
	RMS, Cent, Flat, BandRatio float64
}

func analyze(buf []int16) FrameFeat {
	in := make([]complex128, BlockSize)
	var rms float64
	for i, s := range buf {
		v := float64(s) / 32768.0
		rms += v * v
		w := 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(BlockSize-1)))
		in[i] = complex(v*w, 0)
	}
	rms = math.Sqrt(rms / float64(BlockSize))
	spec := fft(in)
	var num, den, logSum, hiE, loE float64
	var cnt int
	for k := 1; k < BlockSize/2; k++ {
		f := float64(k) * SR / float64(BlockSize)
		if f >= 1800 && f <= 18500 {
			m := cmplx.Abs(spec[k]) + 1e-12
			num += f * m
			den += m
			logSum += math.Log(m)
			cnt++
			if f >= 7500 {
				hiE += m
			} else {
				loE += m
			}
		}
	}
	var cent, flt, ratio float64
	if den > 0 && cnt > 0 {
		cent = num / den
		flt = math.Exp(logSum/float64(cnt)) / (den / float64(cnt))
		ratio = hiE / (loE + 1e-9)
	}
	return FrameFeat{rms, cent, flt, ratio}
}

func openMic() (io.ReadCloser, *exec.Cmd, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("rec", "-q", "-t", "raw", "-r", "44100", "-e", "signed-integer", "-b", "16", "-c", "1", "-")
	} else {
		cmd = exec.Command("arecord", "-q", "-f", "S16_LE", "-r", "44100", "-c", "1", "-t", "raw")
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	return out, cmd, cmd.Start()
}

func readFrame(r io.Reader) (FrameFeat, error) {
	raw := make([]byte, BlockSize*2)
	if _, err := io.ReadFull(r, raw); err != nil {
		return FrameFeat{}, err
	}
	s := make([]int16, BlockSize)
	for i := 0; i < BlockSize; i++ {
		s[i] = int16(binary.LittleEndian.Uint16(raw[i*2:]))
	}
	return analyze(s), nil
}

func median(a []float64) float64 {
	s := append([]float64(nil), a...)
	sort.Float64s(s)
	return s[len(s)/2]
}

// classifyShape distinguishes TAP vs LINE vs CURVE vs CIRCLE + hand side (L/R/DUAL)
func classifyShape(fs []FrameFeat, c Config) string {
	n := len(fs)
	cents, flats, rms := make([]float64, n), make([]float64, n), make([]float64, n)
	var sumFlt, sumRatio float64
	for i, f := range fs {
		cents[i], flats[i], rms[i] = f.Cent, f.Flat, f.RMS
		sumFlt += f.Flat
		sumRatio += f.BandRatio
	}
	medCent, meanFlt := median(cents), sumFlt/float64(n)
	durMs := float64(n) * (float64(BlockSize) / SR) * 1000.0

	// Determine hand side from pitch centroid
	mid, gap := (c.LeftHz+c.RightHz)/2, math.Abs(c.LeftHz-c.RightHz)
	side := "R"
	if math.Abs(medCent-c.LeftHz) < math.Abs(medCent-c.RightHz) {
		side = "L"
	}

	// Short crisp impact -> TAP
	if durMs <= 125 && meanFlt <= c.ScrapeFlt {
		if math.Abs(medCent-mid) < gap*0.18 && n >= 2 {
			return "DUAL_TAP"
		}
		return side + "_TAP"
	}

	// Continuous friction -> analyze trajectory envelope for LINE, CURVE, CIRCLE, or DUAL_SHAPES
	// 1. Check if BOTH high-band (L) and low-band (R) friction are simultaneously strong
	meanRatio := sumRatio / float64(n)
	if n >= 6 && meanRatio > 0.55 && meanRatio < 1.65 && meanFlt > c.ScrapeFlt*1.05 {
		return "DUAL_SHAPES"
	}

	// 2. Count RMS peaks & velocity reversals to separate Circle vs Curve vs Straight Line
	peaks := 0
	for i := 1; i < n-1; i++ {
		if rms[i] > rms[i-1]*1.08 && rms[i] > rms[i+1]*1.08 {
			peaks++
		}
	}
	// Pitch drift from start to end of stroke (curved arcs change contact angle/speed)
	q := max(1, n/4)
	var startC, endC float64
	for i := 0; i < q; i++ {
		startC += cents[i]
		endC += cents[n-1-i]
	}
	pitchDrift := math.Abs(endC-startC) / float64(q)

	shape := "LINE"
	if peaks >= 2 || durMs > 340 {
		shape = "CIRC" // Circular swirl creates cyclic speed/pressure ripples
	} else if pitchDrift > 480 {
		shape = "CURV" // Arc/hook sweeps pitch centroid as coin angle curves
	}
	return side + "_" + shape
}

func nextChord(r io.Reader, c Config) string {
	var act []FrameFeat
	var q []string
	lastT := time.Now()
	win := time.Duration(c.ChordWinMs) * time.Millisecond
	for {
		f, err := readFrame(r)
		if err != nil {
			return ""
		}
		now := time.Now()
		if f.RMS > c.Trigger {
			act = append(act, f)
		} else if len(act) > 0 {
			g := classifyShape(act, c)
			act = nil
			if len(q) > 0 && now.Sub(lastT) > win {
				res := strings.Join(q, "+")
				return res
			}
			q = append(q, g)
			lastT = now
			if len(q) == 2 {
				return strings.Join(q, "+")
			}
		} else if len(q) > 0 && now.Sub(lastT) > win {
			return strings.Join(q, "+")
		}
	}
}

func setup(in *bufio.Reader) {
	c := loadCfg()
	r, cmd, err := openMic()
	if err != nil {
		fmt.Println("Mic error:", err)
		return
	}
	defer func() { _ = r.Close(); _ = cmd.Process.Kill() }()

	fmt.Println("\nMeasuring ambient noise for 2s...")
	var noise []float64
	for i := 0; i < 86; i++ {
		f, _ := readFrame(r)
		noise = append(noise, f.RMS)
	}
	sort.Float64s(noise)
	c.Trigger = math.Max(noise[int(float64(len(noise))*0.9)]*3.8, 0.014)

	cap5 := func(msg string) []FrameFeat {
		fmt.Println(msg)
		var out []FrameFeat
		var act []FrameFeat
		for len(out) < 5 {
			f, _ := readFrame(r)
			if f.RMS > c.Trigger {
				act = append(act, f)
			} else if len(act) > 0 {
				var cs, fls []float64
				for _, x := range act {
					cs, fls = append(cs, x.Cent), append(fls, x.Flat)
				}
				mc, mf := median(cs), median(fls)
				out = append(out, FrameFeat{Cent: mc, Flat: mf})
				fmt.Printf("  [%d/5] Pitch: %.0fHz | Flatness: %.3f\n", len(out), mc, mf)
				act = nil
			}
		}
		return out
	}

	l := cap5("\n1) Tap LEFT striker 5 times:")
	rHits := cap5("\n2) Tap RIGHT striker 5 times:")
	s := cap5("\n3) Draw a straight LINE scrape 5 times:")
	var lc, rc, tf, sf []float64
	for i := 0; i < 5; i++ {
		lc, rc = append(lc, l[i].Cent), append(rc, rHits[i].Cent)
		tf = append(tf, l[i].Flat, rHits[i].Flat)
		sf = append(sf, s[i].Flat)
	}
	c.LeftHz, c.RightHz = median(lc), median(rc)
	c.ScrapeFlt = (median(tf) + median(sf)) / 2
	saveCfg(c)
	fmt.Printf("\nSaved to %s! (Edit JSON anytime to add custom chords)\n", cfgPath())
}

// Whole-word & rhythmic phrase trainer (trains muscle memory for words, not letters)
func trainWords() {
	c := loadCfg()
	r, cmd, err := openMic()
	if err != nil {
		fmt.Println("Mic error:", err)
		return
	}
	defer func() { _ = r.Close(); _ = cmd.Process.Kill() }()

	// Build reverse lookup: word -> chord
	wordToChord := map[string]string{}
	for k, v := range c.ProseChords {
		if !strings.HasPrefix(v, "[") {
			wordToChord[strings.TrimSpace(v)] = k
		}
	}

	fmt.Println("\n=== WHOLE-WORD & SHAPE CHORD TRAINER ===")
	fmt.Println("Shapes: TAP (dot), LINE (straight drag), CURV (hook/arc), CIRC (swirl)")
	fmt.Println("Combos: L_LINE+R_CURV = 'with', L_CURV+R_LINE = 'from', DUAL_SHAPES = 'something'\n")

	for pIdx, phrase := range c.WordPhrases {
		words := strings.Fields(phrase)
		fmt.Printf("Phrase %d/%d: \"%s\"\n", pIdx+1, len(c.WordPhrases), strings.TrimSpace(phrase))
		start := time.Now()
		for wIdx := 0; wIdx < len(words); {
			w := words[wIdx]
			target := wordToChord[w]
			fmt.Printf("  Word [%d/%d]: %-10s  Chord: %-15s -> ", wIdx+1, len(words), "\""+w+"\"", target)
			got := nextChord(r, c)
			if got == target || c.ProseChords[got] == w+" " {
				fmt.Println("OK!")
				wIdx++
			} else {
				fmt.Printf("heard %s (try again)\n", got)
			}
		}
		sec := math.Max(time.Since(start).Seconds(), 0.1)
		wpm := int((float64(len(phrase)) / 5.0) / (sec / 60.0))
		fmt.Printf("Phrase complete in %.2fs -> %d WPM!\n\n", sec, wpm)
	}
}

func live(mode string) {
	c := loadCfg()
	r, cmd, err := openMic()
	if err != nil {
		fmt.Println("Mic error:", err)
		return
	}
	defer func() { _ = r.Close(); _ = cmd.Process.Kill() }()

	fmt.Printf("\n=== LIVE [%s] (L_CIRC+R_CIRC toggles PROSE/CODE, Ctrl+C quits) ===\n> ", mode)
	for {
		ch := nextChord(r, c)
		m := c.ProseChords
		if mode == "CODE" {
			m = c.CodeChords
		}
		if txt, ok := m[ch]; ok {
			if txt == "[CODE]" {
				mode = "CODE"
				fmt.Print("\n[CODE]\n> ")
			} else if txt == "[PROSE]" {
				mode = "PROSE"
				fmt.Print("\n[PROSE]\n> ")
			} else {
				fmt.Print(txt)
			}
		} else {
			fmt.Printf("[%s?]", ch)
		}
	}
}

func main() {
	in := bufio.NewReader(os.Stdin)
	for {
		_ = loadCfg()
		fmt.Printf("\n=== COINCHORD | Config: %s ===\n", cfgPath())
		fmt.Println("1) Setup & Calibrate\n2) Whole-Word & Shape Trainer\n3) Live Prose Mode\n4) Live Code Mode\n5) Exit")
		fmt.Print("Select [1-5]: ")
		opt, _ := in.ReadString('\n')
		switch strings.TrimSpace(opt) {
		case "1":
			setup(in)
		case "2":
			trainWords()
		case "3":
			live("PROSE")
		case "4":
			live("CODE")
		case "5":
			return
		}
	}
}

