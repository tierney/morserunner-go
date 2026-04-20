package engine

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
)

type StationState int

const (
	StListening StationState = iota
	StCopying
	StPreparingToSend
	StSending
)

type MsgType int

const (
	MsgNone MsgType = iota
	MsgCQ
	MsgNR
	MsgTU
	MsgMyCall
	MsgHisCall
	MsgB4
	MsgQm
	MsgAgn
)

type Station struct {
	Call      string
	Amplitude float64
	WpmS      int
	WpmC      int
	Envelope  []float32
	SendPos   int
	State     StationState
	Bfo       float64 // Offset from center frequency in radians/sample
	Qsb       *QsbModulator
	Flutter   *QsbModulator
}

func NewStation(call string, rate int) *Station {
	return &Station{
		Call:      call,
		Amplitude: 1.0,
		WpmS:      30,
		WpmC:      30,
		State:     StListening,
		Bfo:       (rand.Float64() - 0.5) * 400.0 * 2.0 * math.Pi / float64(rate), // +/- 200Hz
		Qsb:       NewQsbModulator(rate, 0.5),                                    // Default slow fading
		Flutter:   NewQsbModulator(rate, 50.0),                                   // Default fast flutter
	}
}

func (s *Station) GetBlock(blockSize int) []float32 {
	if s.State != StSending || s.Envelope == nil {
		return make([]float32, blockSize)
	}

	end := s.SendPos + blockSize
	if end > len(s.Envelope) {
		end = len(s.Envelope)
	}

	block := make([]float32, blockSize)
	copy(block, s.Envelope[s.SendPos:end])
	s.SendPos = end

	if s.SendPos >= len(s.Envelope) {
		s.State = StListening
		s.Envelope = nil
		s.SendPos = 0
	}

	return block
}

func (s *Station) Tick(blockSize int) {
	if s.State == StSending {
		return
	}

	if s.State == StPreparingToSend {
		// Wait a few blocks
		s.SendPos++
		if s.SendPos > 5 {
			s.State = StSending
			s.SendPos = 0
		}
		return
	}

	if s.State == StListening && s.Envelope == nil {
		// Wait about 3 seconds (approx 94 blocks at 16k/512)
		s.SendPos++
		if s.SendPos > 94 {
			s.State = StPreparingToSend
			s.SendPos = 0
		}
	}
}

type OperatorState int

const (
	OsNeedPrevEnd OperatorState = iota
	OsNeedQso
	OsNeedNr
	OsNeedCall
	OsNeedCallNr
	OsNeedEnd
	OsDone
	OsFailed
	OsNeedAgn
)

type Operator struct {
	Station    *Station
	State      OperatorState
	Patience   int
	Skills     int
	Reply      string
	MyNR       int
	HisCall    string
	Confidence int
	Location   string
}

func NewOperator(station *Station) *Operator {
	locs := []string{"OR", "WA", "CA", "BC", "ON", "QC", "TX", "FL", "NY"}
	return &Operator{
		Station:  station,
		State:    OsNeedQso,
		Skills:   rand.Intn(3) + 1,
		Patience: 5,
		MyNR:     rand.Intn(100) + 1,
		Location: locs[rand.Intn(len(locs))],
	}
}

func (o *Operator) MsgReceived(msg MsgType, inputCall string) {
	if msg == MsgCQ || msg == MsgTU {
		o.State = OsNeedQso
		o.Patience = 5
		return
	}

	conf := Confidence(o.Station.Call, inputCall)
	o.Confidence = conf

	switch o.State {
	case OsNeedQso:
		if conf > 80 {
			o.State = OsNeedNr
			o.HisCall = inputCall
		} else if conf > 50 {
			o.State = OsNeedCallNr
		}
	case OsNeedNr:
		if msg == MsgNR {
			o.State = OsNeedEnd
		}
	case OsNeedEnd:
		if msg == MsgTU {
			o.State = OsDone
		}
	}

	o.Patience--
	if o.Patience <= 0 {
		o.State = OsFailed
	}
}

func (o *Operator) GetReply(rules ContestRules, useLids bool) MsgType {
	// LIDs logic: occasionally send wrong exchange or cut numbers
	if useLids && rand.Float64() < 0.05 {
		// 5% chance to make a mistake
		o.State = OsNeedAgn // Force a re-send or something
	}

	switch o.State {
	case OsNeedQso:
		return MsgMyCall
	case OsNeedNr, OsNeedCallNr:
		return MsgNR
	case OsNeedEnd:
		return MsgTU
	default:
		return MsgNone
	}
}

func (o *Operator) GetExchangeText(rules ContestRules, useLids bool) string {
	text := ""
	if _, ok := rules.(*POTARules); ok {
		text = fmt.Sprintf("599 %s", o.Location)
	} else {
		text = fmt.Sprintf("599 %03d", o.MyNR)
	}

	if useLids {
		// Randomly use cut numbers (T for 0, N for 9, A for 1)
		text = strings.ReplaceAll(text, "0", "T")
		text = strings.ReplaceAll(text, "9", "N")
	}
	return text
}

// Partial match logic (simplified edit distance)
func (o *Operator) IsMyCall(pattern string) bool {
	pattern = strings.ToUpper(pattern)
	call := strings.ToUpper(o.Station.Call)

	if pattern == "" {
		return false
	}

	if pattern == call {
		return true
	}

	// Simple substring match for now
	return strings.Contains(call, pattern)
}
