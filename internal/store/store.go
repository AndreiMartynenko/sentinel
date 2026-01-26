package store

import (
	"realtime-market-engine/types"
	"sync"
)

type PriceStore struct {
	mu     sync.RWMutex
	prices map[string]types.PriceEvent
}

func (s *Store) Update(event types.PriceEvent) {
	s.Lock()
	defer s.Unlock()
	s.prices[price.ID] = price

}

func (s *Store) Get(symbol string) (types.PriceEven, bool) {
	return
}
