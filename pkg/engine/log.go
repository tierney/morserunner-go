package engine

import (
	"time"
)

type Qso struct {
	Timestamp time.Time
	Call      string
	RstSent   int
	RstRecv   int
	NrSent    int
	NrRecv    int
	ExchSent  string
	ExchRecv  string
	Points    int
	Mult      string
}

type Log struct {
	Qsos     []Qso
	Filename string
}

func NewLog(filename string) *Log {
	return &Log{
		Filename: filename,
		Qsos:     make([]Qso, 0),
	}
}

func (l *Log) AddQso(qso Qso) {
	l.Qsos = append(l.Qsos, qso)
}

func (l *Log) TotalPoints() int {
	total := 0
	for _, qso := range l.Qsos {
		total += qso.Points
	}
	return total
}

func (l *Log) TotalMults() int {
	mults := make(map[string]bool)
	for _, qso := range l.Qsos {
		if qso.Mult != "" {
			mults[qso.Mult] = true
		}
	}
	return len(mults)
}

func (l *Log) Score() int {
	return l.TotalPoints() * l.TotalMults()
}
