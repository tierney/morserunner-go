package engine

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"strings"
	"time"
)

// Contest represents the main simulation engine, managing stations, audio mixing, and rules.
type Contest struct {
	Rate           int
	Keyer          *Keyer
	Mixer          *Mixer
	Stations       []*Operator
	MyCall         string
	MyWpm          int
	NoiseLevel     float64
	QRMLevel       float64
	TestTone       bool
	Bandwidth      float64
	LIDs           bool
	RIT            float64
	QSBEnabled     bool
	FlutterEnabled bool
	Log            *Log
	Rules          ContestRules
	MyNR           int
	UserEnv        []float32
	UserPos        int
}

// NewContest initializes a new competition environment with default settings.
func NewContest(rate int) *Contest {
	return &Contest{
		Rate:       rate,
		Keyer:      NewKeyer(rate),
		Mixer:      NewMixer(rate, 600.0),
		MyWpm:      30,
		MyCall:     "W7SST",
		NoiseLevel: 0.05,
		Bandwidth:  500.0,
		Log:        NewLog("contest.log"),
		Rules:      &WPXRules{},
		MyNR:       1,
	}
}

func (c *Contest) AddStation(call string) {
	st := NewStation(call, c.Rate)
	op := NewOperator(st)
	c.Stations = append(c.Stations, op)
}

func (c *Contest) NextBlock(blockSize int) []float32 {
	baseband := make([]complex128, blockSize)

	// Add noise
	if c.NoiseLevel > 0 {
		for i := 0; i < blockSize; i++ {
			baseband[i] = complex((rand.Float64()-0.5)*c.NoiseLevel, (rand.Float64()-0.5)*c.NoiseLevel)
		}
	}

	// Add QRM (random carrier tones)
	if c.QRMLevel > 0 {
		for i := 0; i < blockSize; i++ {
			// Simplified QRM: a few wandering carriers
			baseband[i] += complex(c.QRMLevel*math.Sin(float64(i)*0.01), c.QRMLevel*math.Cos(float64(i)*0.01))
		}
	}

	// Test tone (pure carrier at baseband 0Hz, Mixer will shift it to Pitch)
	if c.TestTone {
		for i := 0; i < blockSize; i++ {
			baseband[i] += complex(0.5, 0)
		}
	}

	// Add User Sidetone
	if c.UserEnv != nil {
		end := c.UserPos + blockSize
		if end > len(c.UserEnv) {
			end = len(c.UserEnv)
		}
		for i := 0; i < end-c.UserPos; i++ {
			baseband[i] += complex(float64(c.UserEnv[c.UserPos+i]), 0)
		}
		c.UserPos = end
		if c.UserPos >= len(c.UserEnv) {
			c.UserEnv = nil
			c.UserPos = 0
		}
	}

	// Mix active stations
	for _, op := range c.Stations {
		op.Station.Tick(blockSize)
		if op.Station.State == StSending {
			env := op.Station.GetBlock(blockSize)

			// Apply QSB/Flutter
			if c.QSBEnabled {
				op.Station.Qsb.Apply(env)
			}
			if c.FlutterEnabled {
				op.Station.Flutter.Apply(env)
			}

			for i, e := range env {
				if e > 0 {
					// Modulate with BFO offset and RIT
					phase := float64(i) * (op.Station.Bfo - 2.0*math.Pi*c.RIT/float64(c.Rate))
					baseband[i] += complex(float64(e)*math.Cos(phase), float64(e)*math.Sin(phase))
				}
			}
		} else if op.State == OsNeedQso && rand.Float64() < 0.0005 {
			// Tail-Ending Nuance: Inactive stations might jump in randomly
			morse := c.Keyer.Encode(op.Station.Call)
			op.Station.Envelope = c.Keyer.GenerateEnvelope(morse)
			op.Station.State = StSending
			op.Station.SendPos = 0
		}
	}

	// Add Noise (scaled by Bandwidth)
	bwScale := math.Sqrt(c.Mixer.Bandwidth / 3000.0)
	noiseLvl := c.NoiseLevel * bwScale
	for i := range baseband {
		n := (rand.Float64()*2 - 1) * noiseLvl
		baseband[i] += complex(n, 0)
	}

	// Final mix to audio output
	return c.Mixer.Mix(baseband)
}

func (c *Contest) ProcessUserTX(msg string) {
	msg = strings.ToUpper(strings.TrimSpace(msg))
	parts := strings.Split(msg, " ")
	if len(parts) == 0 {
		return
	}

	// Simple heuristic for msg type
	msgType := MsgNone
	inputCall := ""

	if strings.Contains(msg, "CQ") {
		msgType = MsgCQ
		fmt.Println("CQ detected! Stations are responding...")
	} else if strings.Contains(msg, "TU") {
		msgType = MsgTU
	} else if len(parts[0]) >= 1 {
		// Treat any non-command as a potential callsign
		msgType = MsgHisCall
		inputCall = parts[0]
	}

	// Generate sidetone for user transmission
	morse := c.Keyer.Encode(msg)
	c.UserEnv = c.Keyer.GenerateEnvelope(morse)
	c.UserPos = 0

	// Pass 1: Find best confidence
	maxConf := 0
	for _, op := range c.Stations {
		conf := Confidence(op.Station.Call, inputCall)
		if conf > maxConf {
			maxConf = conf
		}
	}

	// Pass 2: Process and reply only if we are the best candidate
	for _, op := range c.Stations {
		conf := Confidence(op.Station.Call, inputCall)

		// If we are significantly worse than the best match, ignore the message
		effectiveMsg := msgType
		if conf < maxConf && maxConf > 50 {
			effectiveMsg = MsgNone
		}

		op.MsgReceived(effectiveMsg, inputCall)

		reply := op.GetReply(c.Rules, c.LIDs)

		// Logic for responding:
		// 1. If it's a CQ, everyone in the pile-up responds.
		// 2. If it's a callsign, only the "Best Match" responds.
		canReply := (msgType == MsgCQ) || (conf >= maxConf && conf > 30)

		if reply != MsgNone && canReply {
			text := op.GetExchangeText(c.Rules, c.LIDs)
			if reply == MsgMyCall {
				text = op.Station.Call
			} else if reply == MsgTU {
				text = "TU"
			}

			log.Printf("Station %s (Conf: %d%%) responding with: %s", op.Station.Call, conf, text)
			morse := c.Keyer.Encode(text)
			op.Station.Envelope = c.Keyer.GenerateEnvelope(morse)
			op.Station.State = StSending
			op.Station.SendPos = 0
		}

		if op.State == OsDone {
			// Log QSO
			q := Qso{
				Timestamp: time.Now(),
				Call:      op.Station.Call,
				Points:    c.Rules.Point(Qso{Call: op.Station.Call}),
				Mult:      c.Rules.Multiplier(Qso{Call: op.Station.Call}),
			}
			c.Log.AddQso(q)
			c.MyNR++
		}
	}
}

func (c *Contest) StartPileup(count int) {
	calls := []string{"K7ABC", "W6XYZ", "N1ABC", "G4AAA", "JA1YAA", "DL1ZZZ", "F5ABC"}
	for i := 0; i < count; i++ {
		call := calls[rand.Intn(len(calls))]
		c.AddStation(call)

		op := c.Stations[len(c.Stations)-1]
		morse := c.Keyer.Encode(op.Station.Call)
		op.Station.Envelope = c.Keyer.GenerateEnvelope(morse)
		op.Station.State = StSending
	}
}
