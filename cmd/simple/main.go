package main

import (
	"fmt"
	"log"
	"time"

	"realtime-market-engine/internal/binance"
	"realtime-market-engine/internal/store"
)

func main() {
	log.Println("🎯 Starting simple tracker BTC")

	// 1. Creating a storage
	store := store.NewPriceStore()

	// 2. Starting a Binance listener
	events, err := binance.StartBTCListener()
	if err != nil {
		log.Fatalf("💥 Error: %v", err)
	}

	// 3. Goroutine: updating the storage
	go func() {
		for event := range events {
			store.Update(event)
			log.Printf("💾 Saved: BTC = $%.2f", event.Price)
		}
	}()

	// 4. Infinite loop: Showing what in the storage
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		event, found := store.Get("BTCUSDT")
		if found {
			age := time.Since(event.Timestamp)
			fmt.Printf("[%s] 🪙 BTC: $%.2f (data %.0f sec ago)\n",
				time.Now().Format("15:04:05"),
				event.Price,
				age.Seconds())
		} else {
			fmt.Printf("[%s] ⏳ Awaiting data from Binance...\n",
				time.Now().Format("15:04:05"))
		}
	}
}
