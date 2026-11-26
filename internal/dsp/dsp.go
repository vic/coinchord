// Package dsp turns raw 16-bit mono PCM blocks into spectral frame features.
package dsp

import (
	"encoding/binary"
	"io"
	"math"
	"math/cmplx"
)

const (
	SampleRate = 44100.0
	BlockSize  = 1024 // ~23.2 ms per frame
	BinHz      = SampleRate / BlockSize
	MinHz      = 1800.0 // below: voice and hum, ignored
	MaxHz      = 18500.0
)

// FrameMs is the duration of one frame in milliseconds.
const FrameMs = BlockSize / SampleRate * 1000

// Frame holds the features of one block.
type Frame struct {
	RMS  float64   // loudness 0..1
	Cent float64   // spectral centroid in Hz (MinHz..MaxHz band)
	Flat float64   // spectral flatness 0..1 (noise-like friction is high)
	Mag  []float32 // magnitude per bin, index k = k*BinHz
}

// Band sums magnitudes between lo and hi Hz.
func (f Frame) Band(lo, hi float64) float64 {
	var s float64
	for k := max(1, int(lo/BinHz)); k <= int(hi/BinHz) && k < len(f.Mag); k++ {
		s += float64(f.Mag[k])
	}
	return s
}

func fft(x []complex128) []complex128 {
	n := len(x)
	if n <= 1 {
		return x
	}
	even, odd := make([]complex128, n/2), make([]complex128, n/2)
	for i := range n / 2 {
		even[i], odd[i] = x[2*i], x[2*i+1]
	}
	even, odd = fft(even), fft(odd)
	out := make([]complex128, n)
	for k := range n / 2 {
		t := cmplx.Rect(1, -2*math.Pi*float64(k)/float64(n)) * odd[k]
		out[k], out[k+n/2] = even[k]+t, even[k]-t
	}
	return out
}

// Analyze computes features of one block of BlockSize samples.
func Analyze(buf []int16) Frame {
	in := make([]complex128, BlockSize)
	var rms float64
	for i, s := range buf {
		v := float64(s) / 32768.0
		rms += v * v
		w := 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(BlockSize-1)))
		in[i] = complex(v*w, 0)
	}
	spec := fft(in)
	mag := make([]float32, BlockSize/2)
	var num, den, logSum float64
	var cnt int
	for k := 1; k < BlockSize/2; k++ {
		m := cmplx.Abs(spec[k]) + 1e-12
		mag[k] = float32(m)
		if f := float64(k) * BinHz; f >= MinHz && f <= MaxHz {
			num += f * m
			den += m
			logSum += math.Log(m)
			cnt++
		}
	}
	return Frame{
		RMS:  math.Sqrt(rms / BlockSize),
		Cent: num / den,
		Flat: math.Exp(logSum/float64(cnt)) / (den / float64(cnt)),
		Mag:  mag,
	}
}

// Read reads one block of little-endian s16 samples and analyzes it.
func Read(r io.Reader) (Frame, error) {
	raw := make([]byte, BlockSize*2)
	if _, err := io.ReadFull(r, raw); err != nil {
		return Frame{}, err
	}
	s := make([]int16, BlockSize)
	for i := range s {
		s[i] = int16(binary.LittleEndian.Uint16(raw[i*2:]))
	}
	return Analyze(s), nil
}
