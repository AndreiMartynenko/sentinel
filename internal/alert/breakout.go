package alert

import (
	"math"
	"time"

	"realtime-market-engine/internal/candle"
)

type BreakoutDirection string

const (
	BreakoutUp   BreakoutDirection = "up"
	BreakoutDown BreakoutDirection = "down"
)

type BreakoutEvent struct {
	Type      string            `json:"type"`
	Symbol    string            `json:"symbol"`
	Dir       BreakoutDirection `json:"dir"`
	Price     float64           `json:"price"`
	Level     float64           `json:"level"`
	Pct       float64           `json:"pct"`
	Lookback  string            `json:"lookback"`
	CandleEnd time.Time         `json:"candleEnd"`
	Timestamp time.Time         `json:"timestamp"`
}

type BreakoutDetector struct {
	lookback time.Duration
	pct      float64
	cooldown time.Duration

	candles []candle.Candle

	lastSignalAt time.Time
}

func NewBreakoutDetector(lookback time.Duration, pct float64, cooldown time.Duration) *BreakoutDetector {
	if lookback <= 0 {
		lookback = 5 * time.Minute
	}
	if pct < 0 {
		pct = 0
	}
	if cooldown < 0 {
		cooldown = 0
	}
	return &BreakoutDetector{lookback: lookback, pct: pct, cooldown: cooldown}
}

func (d *BreakoutDetector) Push(c candle.Candle) (BreakoutEvent, bool) {
	d.candles = append(d.candles, c)
	cut := c.End.Add(-d.lookback)

	start := 0
	for start < len(d.candles) && d.candles[start].End.Before(cut) {
		start++
	}
	if start > 0 {
		d.candles = d.candles[start:]
	}

	if len(d.candles) < 2 {
		return BreakoutEvent{}, false
	}

	if d.cooldown > 0 && !d.lastSignalAt.IsZero() {
		if c.End.Sub(d.lastSignalAt) < d.cooldown {
			return BreakoutEvent{}, false
		}
	}

	high := -math.MaxFloat64
	low := math.MaxFloat64

	for i := 0; i < len(d.candles)-1; i++ {
		if d.candles[i].High > high {
			high = d.candles[i].High
		}
		if d.candles[i].Low < low {
			low = d.candles[i].Low
		}
	}

	price := c.Close
	upLevel := high * (1 + d.pct)
	downLevel := low * (1 - d.pct)

	if price > upLevel {
		d.lastSignalAt = c.End
		return BreakoutEvent{
			Type:      "breakout",
			Symbol:    c.Symbol,
			Dir:       BreakoutUp,
			Price:     price,
			Level:     high,
			Pct:       d.pct,
			Lookback:  d.lookback.String(),
			CandleEnd: c.End,
			Timestamp: c.Timestamp,
		}, true
	}

	if price < downLevel {
		d.lastSignalAt = c.End
		return BreakoutEvent{
			Type:      "breakout",
			Symbol:    c.Symbol,
			Dir:       BreakoutDown,
			Price:     price,
			Level:     low,
			Pct:       d.pct,
			Lookback:  d.lookback.String(),
			CandleEnd: c.End,
			Timestamp: c.Timestamp,
		}, true
	}

	return BreakoutEvent{}, false
}
