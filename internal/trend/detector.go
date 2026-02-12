package trend

import (
	"fmt"
	"math"
	"time"

	"realtime-market-engine/internal/types"
)

type Direction string

const (
	DirectionUp   Direction = "up"
	DirectionDown Direction = "down"
)

type TrendChange struct {
	Type      string    `json:"type"`
	Symbol    string    `json:"symbol"`
	Trend     Direction `json:"trend"`
	FastEMA   float64   `json:"fastEma"`
	SlowEMA   float64   `json:"slowEma"`
	Price     float64   `json:"price"`
	Timestamp time.Time `json:"timestamp"`
}

type EMACrossoverDetector struct {
	fastN int
	slowN int

	alphaFast float64
	alphaSlow float64

	fastEMA float64
	slowEMA float64
	hasEMA  bool

	trend         Direction
	hasTrend      bool
	pendingTrend  Direction
	pendingCount  int
	confirmTicks  int
	minRelDiff    float64
	cooldown      time.Duration
	lastChangeAt  time.Time
	lastSymbol    string
	lastTimestamp time.Time
}

func (d *EMACrossoverDetector) Ready() bool {
	return d.hasEMA
}

func (d *EMACrossoverDetector) EMAs() (fast float64, slow float64, ok bool) {
	if !d.hasEMA {
		return 0, 0, false
	}
	return d.fastEMA, d.slowEMA, true
}

func (d *EMACrossoverDetector) CurrentDirection() (Direction, bool) {
	if !d.hasEMA {
		return "", false
	}
	if d.fastEMA >= d.slowEMA {
		return DirectionUp, true
	}
	return DirectionDown, true
}

func NewEMACrossoverDetector(fastN, slowN, confirmTicks int, minRelDiff float64, cooldown time.Duration) *EMACrossoverDetector {
	if fastN <= 0 {
		fastN = 20
	}
	if slowN <= 0 {
		slowN = 50
	}
	if fastN >= slowN {
		slowN = fastN + 1
	}
	if confirmTicks <= 0 {
		confirmTicks = 1
	}
	if minRelDiff < 0 {
		minRelDiff = 0
	}
	if cooldown < 0 {
		cooldown = 0
	}

	return &EMACrossoverDetector{
		fastN:        fastN,
		slowN:        slowN,
		alphaFast:    2.0 / (float64(fastN) + 1.0),
		alphaSlow:    2.0 / (float64(slowN) + 1.0),
		confirmTicks: confirmTicks,
		minRelDiff:   minRelDiff,
		cooldown:     cooldown,
	}
}

func (d *EMACrossoverDetector) Push(ev types.PriceEvent) (TrendChange, bool) {
	d.lastSymbol = ev.Symbol
	d.lastTimestamp = ev.Timestamp

	price := ev.Price
	if math.IsNaN(price) || math.IsInf(price, 0) {
		return TrendChange{}, false
	}

	if !d.hasEMA {
		d.fastEMA = price
		d.slowEMA = price
		d.hasEMA = true
		return TrendChange{}, false
	}

	d.fastEMA = d.alphaFast*price + (1.0-d.alphaFast)*d.fastEMA
	d.slowEMA = d.alphaSlow*price + (1.0-d.alphaSlow)*d.slowEMA

	if d.cooldown > 0 && !d.lastChangeAt.IsZero() {
		if ev.Timestamp.Sub(d.lastChangeAt) < d.cooldown {
			return TrendChange{}, false
		}
	}

	var current Direction
	if d.fastEMA >= d.slowEMA {
		current = DirectionUp
	} else {
		current = DirectionDown
	}

	relSep := 0.0
	if price != 0 {
		relSep = math.Abs(d.fastEMA-d.slowEMA) / math.Abs(price)
	}
	if relSep < d.minRelDiff {
		return TrendChange{}, false
	}

	if !d.hasTrend {
		d.trend = current
		d.hasTrend = true
		d.pendingCount = 0
		return TrendChange{}, false
	}

	if current == d.trend {
		d.pendingCount = 0
		return TrendChange{}, false
	}

	if d.pendingCount == 0 || d.pendingTrend != current {
		d.pendingTrend = current
		d.pendingCount = 1
	} else {
		d.pendingCount++
	}

	if d.pendingCount < d.confirmTicks {
		return TrendChange{}, false
	}

	d.trend = current
	d.pendingCount = 0
	d.lastChangeAt = ev.Timestamp

	return TrendChange{
		Type:      "trend_change",
		Symbol:    ev.Symbol,
		Trend:     current,
		FastEMA:   d.fastEMA,
		SlowEMA:   d.slowEMA,
		Price:     price,
		Timestamp: ev.Timestamp,
	}, true
}

func (d *EMACrossoverDetector) String() string {
	if !d.hasEMA {
		return "EMA(n/a)"
	}
	if !d.hasTrend {
		return fmt.Sprintf("EMA fast=%.6f slow=%.6f", d.fastEMA, d.slowEMA)
	}
	return fmt.Sprintf("EMA fast=%.6f slow=%.6f trend=%s", d.fastEMA, d.slowEMA, d.trend)
}
