package store

import (
	"realtime-market-engine/internal/types"
	"sync"
	"testing"
	"time"
)

func TestStoreConcurrentAccess(t *testing.T) {
	store := NewPriceStore()

	var wg sync.WaitGroup
	symbols := []string{"BTCUSDT", "ETHUSDT"}

	// 10 goroutines write
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for _, symbol := range symbols {
				store.Update(types.PriceEvent{
					Symbol:    symbol,
					Price:     float64(idx * 1000),
					Timestamp: time.Now(),
					Source:    "test",
				})
			}
		}(i)
	}

	// 10 goroutine read
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, symbol := range symbols {
				_, _ = store.Get(symbol)
			}
		}()
	}

	wg.Wait()

	// If this test passes without panic and -reace doesn't show error -> all good.
}
