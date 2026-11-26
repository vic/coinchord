package eval

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"coinchord/internal/audio"
	"coinchord/internal/dsp"
	"coinchord/internal/gesture"
	"coinchord/internal/kits"
)

// session renders taps (L, R, L, R) one second apart with a fading ring.
func session() []byte {
	var b bytes.Buffer
	for i := range 4 * 44100 {
		t := float64(i) / dsp.SampleRate
		k := int(t)
		v := 0.0
		if dt := t - float64(k) - 0.3; dt >= 0 && dt < 0.06 {
			v = 0.15 * math.Sin(2*math.Pi*[]float64{9200, 5800}[k%2]*dt) * math.Exp(-dt*40)
		}
		_ = binary.Write(&b, binary.LittleEndian, int16(v*32767))
	}
	return b.Bytes()
}

func TestRun(t *testing.T) {
	k, err := kits.Presets[0].Kit()
	if err != nil {
		t.Fatal(err)
	}
	s := audio.Setup{Kit: k, Tuning: gesture.DefaultTuning()}
	labels, err := ReadLabels(strings.NewReader(
		`{"ms":360,"want":"L_TAP"}` + "\n" + `{"ms":1360,"want":"R_TAP"}` + "\n" +
			`{"ms":2360,"want":"R_LINE"}` + "\n" + `{"ms":3900,"want":"R_TAP"}` + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	r := Run(bytes.NewReader(session()), labels, s)
	// 2 right, 1 confused (an L tap labeled R_LINE), 1 label far from any stroke
	if r.Hits != 2 || r.Missed != 1 || r.Confused["R_LINE → L_TAP"] != 1 {
		t.Fatalf("%+v\n%s", r, r)
	}
	if _, err := ReadLabels(strings.NewReader(`{"ms":1,"want":"X_TAP"}`)); err == nil {
		t.Fatal("bad gesture must fail")
	}
}
