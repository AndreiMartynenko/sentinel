package types

import "time"

// Typr for price event
type PriceEvent struct {
	Symbol    string
	Price     float64
	Timestamp time.Time
	Source    string // "Binance"
}
