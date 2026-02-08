package candle

import (
	"time"

	"realtime-market-engine/internal/types"
)

type Candle struct {
	Symbol    string    `json:"symbol"`
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Timestamp time.Time `json:"timestamp"`
}

type Aggregator struct {
	interval time.Duration

	hasCurrent bool
	current    Candle
}

func NewAggregator(interval time.Duration) *Aggregator {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &Aggregator{interval: interval}
}

func (a *Aggregator) Push(ev types.PriceEvent) (Candle, bool) {
	bucketStart := ev.Timestamp.Truncate(a.interval)
	bucketEnd := bucketStart.Add(a.interval)

	if !a.hasCurrent {
		a.current = Candle{
			Symbol:    ev.Symbol,
			Start:     bucketStart,
			End:       bucketEnd,
			Open:      ev.Price,
			High:      ev.Price,
			Low:       ev.Price,
			Close:     ev.Price,
			Timestamp: ev.Timestamp,
		}
		a.hasCurrent = true
		return Candle{}, false
	}

	if a.current.Start.Equal(bucketStart) {
		if ev.Price > a.current.High {
			a.current.High = ev.Price
		}
		if ev.Price < a.current.Low {
			a.current.Low = ev.Price
		}
		a.current.Close = ev.Price
		a.current.Timestamp = ev.Timestamp
		return Candle{}, false
	}

	completed := a.current

	a.current = Candle{
		Symbol:    ev.Symbol,
		Start:     bucketStart,
		End:       bucketEnd,
		Open:      ev.Price,
		High:      ev.Price,
		Low:       ev.Price,
		Close:     ev.Price,
		Timestamp: ev.Timestamp,
	}

	return completed, true
}
