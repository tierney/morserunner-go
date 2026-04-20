package engine

type ContestType string

const (
	ContestWPX    ContestType = "WPX"
	ContestARRLDX ContestType = "ARRLDX"
	ContestPOTA   ContestType = "POTA"
)

type ContestRules interface {
	Name() string
	Exchange(myCall string, nr int) string
	Point(qso Qso) int
	Multiplier(qso Qso) string
}

type WPXRules struct{}

func (w *WPXRules) Name() string { return "CQ WPX" }
func (w *WPXRules) Exchange(myCall string, nr int) string {
	return "599 001" // Simple for now
}
func (w *WPXRules) Point(qso Qso) int { return 1 }
func (w *WPXRules) Multiplier(qso Qso) string {
	// Prefix logic
	if len(qso.Call) < 3 {
		return ""
	}
	// Simplified: first 3 chars
	return qso.Call[:3]
}

type ARRLDXRules struct{}

func (a *ARRLDXRules) Name() string { return "ARRL DX" }
func (a *ARRLDXRules) Exchange(myCall string, nr int) string {
	return "599 KW" // Simplified
}
func (a *ARRLDXRules) Point(qso Qso) int { return 3 }
func (a *ARRLDXRules) Multiplier(qso Qso) string {
	// DXCC logic (simplified: first 2 chars)
	if len(qso.Call) < 2 {
		return ""
	}
	return qso.Call[:2]
}

type POTARules struct {
	ParkID string
}

func (p *POTARules) Name() string { return "Parks on the Air (POTA)" }
func (p *POTARules) Exchange(myCall string, nr int) string {
	return "599 " + p.ParkID
}
func (p *POTARules) Point(qso Qso) int { return 1 }
func (p *POTARules) Multiplier(qso Qso) string {
	// Multiplier is unique Parks
	return qso.ExchRecv
}
