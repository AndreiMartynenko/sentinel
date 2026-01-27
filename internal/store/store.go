package store

import (
	"realtime-market-engine/internal/types"
	"sync"
)

type PriceStore struct {
	mu     sync.RWMutex
	prices map[string]types.PriceEvent
}

func NewPriceStore() *PriceStore {
	return &PriceStore{
		prices: make(map[string]types.PriceEvent),
	}
}

// To write the latest price. Like BTCUSDT = 88000 -> add it in our map
func (s *PriceStore) Update(event types.PriceEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prices[event.Symbol] = event

}

// To Get the latest current price
func (s *PriceStore) Get(symbol string) (types.PriceEvent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ev, ok := s.prices[symbol]
	return ev, ok
}
