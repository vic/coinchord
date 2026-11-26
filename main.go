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
	BlockSize = 1024
)

type Cal struct {
	Pair      string  `json:"pair"`
	Trigger   float64 `json:"trigger"`
	LeftHz    float64 `json:"left_hz"`
	RightHz   float64 `json:"right_hz"`
	ScrapeFlt float64 `json:"scrape_flt"`
}

var proseMap = map[string]string{
	"L": "the ", "R": "and ", "LL": "to ", "RR": "of ", "S": "in ", "D": "I ",
	"L+R": "that ", "R+L": "you ", "L+S": "with ", "R+S": "for ",
	"LL+R": "is ", "RR+L": "it ", "S+S": "ing ", "D+S": ".\n", "LL+RR": "[CODE]",
}

var codeMap = map[string]string{
	"L": "def ", "R": "return ", "LL": "if ", "RR": "else:\n    ", "S": " = ", "D": "()",
	"L+R": "for ", "R+L": " in ", "L+S": "import ", "R+S": "print(",
	"LL+R": "True", "RR+L": "False", "S+S": ":\n    ", "D+S": "\n", "LL+RR": "[PROSE]",
}

func calPath() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".coinchord.json")
}

func loadCal() Cal {
	c := Cal{"Small Coin (L) + Large Coin (R)", 0.032, 9200, 5800, 0.28}
	if b, err := os.ReadFile(calPath()); err == nil {
		_ = json.Unmarshal(b, &c)
	}
	return c
}

func saveCal(c Cal) {
	b, _ := json.MarshalIndent(c, "", "  ")
	_ = os.WriteFile(calPath(), b, 0644)
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

func analyze(buf []int16) (rms, centroid, flatness float64) {
	in := make([]complex128, BlockSize)
	for i, s := range buf {
		v := float64(s) / 32768.0
		rms += v * v
		w := 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(BlockSize-1)))
		in[i] = complex(v*w, 0)
	}
	rms = math.Sqrt(rms / float64(BlockSize))
	spec := fft(in)
	var num, den, logSum float64
	var cnt int
	for k := 1; k < BlockSize/2; k++ {
		f := float64(k) * SR / float64(BlockSize)
		if f >= 1800 && f <= 18500 {
			m := cmplx.Abs(spec[k]) + 1e-12
			num += f * m
			den += m
			logSum += math.Log(m)
			cnt++
		}
	}
	if den > 0 && cnt > 0 {
		centroid = num / den
		flatness = math.Exp(logSum/float64(cnt)) / (den / float64(cnt))
	}
	return
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

func median(a []float64) float64 {
	s := append([]float64(nil), a...)
	sort.Float64s(s)
	return s[len(s)/2]
}

func mean(a []float64) float64 {
	var sum float64
	for _, v := range a {
		sum += v
	}
	return sum / float64(len(a))
}

func classify(cents, flats []float64, c Cal) string {
	cent, flt := median(cents), mean(flats)
	durMs := float64(len(cents)) * (float64(BlockSize) / SR) * 1000
	if durMs > 115 || flt > c.ScrapeFlt {
		return "S"
	}
	mid, gap := (c.LeftHz+c.RightHz)/2, math.Abs(c.LeftHz-c.RightHz)
	if math.Abs(cent-mid) < gap*0.16 && len(cents) >= 2 {
		return "D"
	}
	if math.Abs(cent-c.LeftHz) < math.Abs(cent-c.RightHz) {
		return "L"
	}
	return "R"
}

func readFrame(r io.Reader) (float64, float64, float64, error) {
	raw := make([]byte, BlockSize*2)
	if _, err := io.ReadFull(r, raw); err != nil {
		return 0, 0, 0, err
	}
	samples := make([]int16, BlockSize)
	for i := 0; i < BlockSize; i++ {
		samples[i] = int16(binary.LittleEndian.Uint16(raw[i*2:]))
	}
	rms, cent, flt := analyze(samples)
	return rms, cent, flt, nil
}

func setup(in *bufio.Reader) {
	c := loadCal()
	fmt.Println("\n=== STEP 1: OBJECT SELECTION & CALIBRATION ===")
	fmt.Println("1) Small Coin (L) + Large Coin (R)\n2) House Key (L) + Spoon/Pen (R)\n3) Fingernail (L) + Ring/Knuckle (R)")
	fmt.Print("Choose [1-3]: ")
	sel, _ := in.ReadString('\n')
	if strings.TrimSpace(sel) == "2" {
		c.Pair = "House Key (L) + Spoon/Pen (R)"
	} else if strings.TrimSpace(sel) == "3" {
		c.Pair = "Fingernail (L) + Ring/Knuckle (R)"
	}

	r, cmd, err := openMic()
	if err != nil {
		fmt.Println("Mic error (need arecord or sox):", err)
		return
	}
	defer func() { _ = r.Close(); _ = cmd.Process.Kill() }()

	fmt.Println("Measuring room noise for 2s (keep hands still)...")
	var noise []float64
	for i := 0; i < 86; i++ {
		rms, _, _, _ := readFrame(r)
		noise = append(noise, rms)
	}
	sort.Float64s(noise)
	c.Trigger = math.Max(noise[int(float64(len(noise))*0.9)]*3.8, 0.014)
	fmt.Printf("Trigger threshold: %.4f\n", c.Trigger)

	capture5 := func(msg string) ([]float64, []float64) {
		fmt.Println(msg)
		var outC, outF, actC, actF []float64
		for len(outC) < 5 {
			rms, cent, flt, _ := readFrame(r)
			if rms > c.Trigger {
				actC, actF = append(actC, cent), append(actF, flt)
			} else if len(actC) > 0 {
				mc, mf := median(actC), mean(actF)
				outC, outF = append(outC, mc), append(outF, mf)
				fmt.Printf("  [%d/5] Pitch: %.0f Hz | Friction: %.3f\n", len(outC), mc, mf)
				actC, actF = nil, nil
			}
		}
		return outC, outF
	}

	lc, lf := capture5("\n1) Tap LEFT object 5 times:")
	rc, rf := capture5("\n2) Tap RIGHT object 5 times:")
	_, sf := capture5("\n3) Scrape either object across desk 5 times:")
	c.LeftHz, c.RightHz = median(lc), median(rc)
	c.ScrapeFlt = (mean(append(lf, rf...)) + mean(sf)) / 2
	saveCal(c)
	fmt.Printf("\nSaved! L=%.0fHz R=%.0fHz Gap=%.0fHz\n", c.LeftHz, c.RightHz, math.Abs(c.LeftHz-c.RightHz))
}

func nextChord(r io.Reader, c Cal) string {
	var actC, actF, q []string
	var fC, fF []float64
	_ = actC
	_ = actF
	lastT := time.Now()
	for {
		rms, cent, flt, err := readFrame(r)
		if err != nil {
			return ""
		}
		now := time.Now()
		if rms > c.Trigger {
			fC, fF = append(fC, cent), append(fF, flt)
		} else if len(fC) > 0 {
			g := classify(fC, fF, c)
			fC, fF = nil, nil
			if len(q) > 0 && now.Sub(lastT) < 220*time.Millisecond && q[len(q)-1] == g && (g == "L" || g == "R") {
				q[len(q)-1] = g + g
			} else {
				if len(q) > 0 && now.Sub(lastT) > 330*time.Millisecond {
					res := strings.Join(q, "+")
					return res
				}
				q = append(q, g)
			}
			lastT = now
			if len(q) == 2 {
				return strings.Join(q, "+")
			}
		} else if len(q) > 0 && now.Sub(lastT) > 330*time.Millisecond {
			return strings.Join(q, "+")
		}
	}
}

func train() {
	c := loadCal()
	r, cmd, err := openMic()
	if err != nil {
		fmt.Println("Mic error:", err)
		return
	}
	defer func() { _ = r.Close(); _ = cmd.Process.Kill() }()

	lessons := [][3]string{
		{"Left Tap -> 'the '", "L", "Tap LEFT once"},
		{"Right Tap -> 'and '", "R", "Tap RIGHT once"},
		{"Desk Scrape -> 'in '", "S", "Slide object ~1 inch"},
		{"Left Double -> 'to '", "LL", "Double-bounce LEFT"},
		{"Right Double -> 'of '", "RR", "Double-bounce RIGHT"},
		{"Dual Hit -> 'I '", "D", "Tap BOTH together"},
		{"Chord 'that '", "L+R", "Left then Right"},
		{"Chord 'you '", "R+L", "Right then Left"},
		{"Chord 'with '", "L+S", "Left tap then scrape"},
	}
	fmt.Println("\n=== STEP 2: INTERACTIVE TRAINER ===")
	for i, l := range lessons {
		fmt.Printf("[%d/%d] %s (%s)\n", i+1, len(lessons), l[0], l[2])
		for {
			got := nextChord(r, c)
			if got == l[1] {
				fmt.Printf("  PASS (%s)\n", got)
				break
			}
			fmt.Printf("  MISS: heard %s, need %s\n", got, l[1])
		}
	}

	seq := [][2]string{{"R+L", "you "}, {"L+R", "that "}, {"LL+R", "is "}, {"R+S", "for "}, {"L", "the "}}
	fmt.Println("\nSPEED DRILL: Perform (R+L) (L+R) (LL+R) (R+S) (L)")
	var start time.Time
	var out strings.Builder
	for i := 0; i < len(seq); {
		got := nextChord(r, c)
		if start.IsZero() {
			start = time.Now()
		}
		if got == seq[i][0] {
			out.WriteString(seq[i][1])
			i++
			fmt.Printf("\r  Progress: %-24s", out.String())
		}
	}
	sec := math.Max(time.Since(start).Seconds(), 0.1)
	wpm := int((float64(out.Len()) / 5.0) / (sec / 60.0))
	fmt.Printf("\nDONE in %.2fs -> %d WPM!\n", sec, wpm)
}

func live(mode string) {
	c := loadCal()
	r, cmd, err := openMic()
	if err != nil {
		fmt.Println("Mic error:", err)
		return
	}
	defer func() { _ = r.Close(); _ = cmd.Process.Kill() }()

	fmt.Printf("\n=== LIVE [%s] (LL+RR toggles PROSE/CODE, Ctrl+C quits) ===\n> ", mode)
	for {
		ch := nextChord(r, c)
		m := proseMap
		if mode == "CODE" {
			m = codeMap
		}
		if txt, ok := m[ch]; ok {
			if txt == "[CODE]" {
				mode = "CODE"
				fmt.Print("\n[CODE MODE]\n> ")
			} else if txt == "[PROSE]" {
				mode = "PROSE"
				fmt.Print("\n[PROSE MODE]\n> ")
			} else {
				fmt.Print(txt)
			}
		}
	}
}

func main() {
	in := bufio.NewReader(os.Stdin)
	for {
		c := loadCal()
		fmt.Printf("\n=== COINCHORD (Go) | %s (L:%.0fHz R:%.0fHz) ===\n", c.Pair, c.LeftHz, c.RightHz)
		fmt.Println("1) Setup & Calibrate\n2) Interactive Trainer\n3) Live Prose Mode\n4) Live Code Mode\n5) Exit")
		fmt.Print("Select [1-5]: ")
		opt, _ := in.ReadString('\n')
		switch strings.TrimSpace(opt) {
		case "1":
			setup(in)
		case "2":
			train()
		case "3":
			live("PROSE")
		case "4":
			live("CODE")
		case "5":
			return
		}
	}
}

