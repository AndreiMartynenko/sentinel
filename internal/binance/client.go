package binance

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"realtime-market-engine/internal/types"

	"github.com/gorilla/websocket"
)

// Client - connects and reads
func StartBTCListener() (<-chan types.PriceEvent, error) {
	// 1. URL for BTC/USDT 1m
	url := "wss://stream.binance.com:9443/ws/btcusdt@kline_1m"

	// 2. Connecting
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, fmt.Errorf("Can't connect: %w", err)
	}

	// 3. Channel to send data
	ch := make(chan types.PriceEvent, 10)

	// 4. Start reading at the background
	go func() {
		defer conn.Close()
		defer close(ch)

		log.Println("✅ Connect to Binance WebSocket")

		for {
			// Reading the message
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("❌ Read error: %v", err)

				log.Printf("📨 Raw message: %s", string(message))
				return
			}

			// Parsing JSON
			var msg KlineMessage
			if err := json.Unmarshal(message, &msg); err != nil {
				log.Printf("❌ Parsing error JSON: %v", err)
				continue // Trying next message
			}

			// Show the log for fixing (remove later on)
			log.Printf("📨 Received: %s = %s (closed: %v)",
				msg.Data.Kline.Symbol,
				msg.Data.Kline.Close,
				msg.Data.Kline.IsClosed)

			// If the candle didn't clos, skip it
			if !msg.Data.Kline.IsClosed {
				continue
			}

			// Convert price from string to int
			price, err := strconv.ParseFloat(msg.Data.Kline.Close, 64)
			if err != nil {
				log.Printf("❌ Price converting error: %v", err)
				continue
			}

			// Creating en event
			event := types.PriceEvent{
				Symbol:    msg.Data.Kline.Symbol,
				Price:     price,
				Timestamp: time.UnixMilli(msg.Data.Kline.CloseTime),
				Source:    "binance",
			}

			// Sending to a channel
			ch <- event
		}
	}()

	return ch, nil
}
