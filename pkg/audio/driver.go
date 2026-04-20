package audio

import (
	"context"
	"encoding/binary"
	"io"
	"math"
	"time"

	"github.com/ebitengine/oto/v3"
)

type Driver struct {
	context *oto.Context
	player  *oto.Player
	ready   chan struct{}
}

func NewDriver(rate int) (*Driver, error) {
	op := &oto.NewContextOptions{
		SampleRate:   rate,
		ChannelCount: 1,
		Format:       oto.FormatSignedInt16LE,
	}

	otoCtx, ready, err := oto.NewContext(op)
	if err != nil {
		return nil, err
	}

	d := &Driver{
		context: otoCtx,
		ready:   ready,
	}

	<-ready
	return d, nil
}

func (d *Driver) Play(ctx context.Context, source io.Reader) error {
	d.player = d.context.NewPlayer(source)
	d.player.Play()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if !d.player.IsPlaying() {
				return nil
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (d *Driver) Close() error {
	return d.player.Close()
}

// Float32ToS16 converts a slice of float32 samples [-1.0, 1.0] to 16-bit signed PCM.
func Float32ToS16(samples []float32) []byte {
	out := make([]byte, len(samples)*2)
	for i, s := range samples {
		val := int16(math.Max(-32768, math.Min(32767, float64(s)*32767)))
		binary.LittleEndian.PutUint16(out[i*2:], uint16(val))
	}
	return out
}
