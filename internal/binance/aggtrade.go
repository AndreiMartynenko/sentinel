package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"realtime-market-engine/internal/types"

	"github.com/gorilla/websocket"
)

type aggTradeMessage struct {
	EventType string `json:"e"`
	EventTime int64  `json:"E"`
	Symbol    string `json:"s"`
	Price     string `json:"p"`
	TradeTime int64  `json:"T"`
}

func StartAggTradeListener(ctx context.Context, symbol string) (<-chan types.PriceEvent, error) {
	s := strings.ToLower(symbol)
	url := fmt.Sprintf("wss://stream.binance.com:9443/ws/%s@aggTrade", s)

	ch := make(chan types.PriceEvent, 100)

	go func() {
		defer close(ch)

		backoff := 200 * time.Millisecond
		for {
			if ctx.Err() != nil {
				return
			}

			conn, _, err := websocket.DefaultDialer.Dial(url, nil)
			if err != nil {
				log.Printf("binance dial error: %v", err)
				select {
				case <-time.After(backoff):
					if backoff < 5*time.Second {
						backoff *= 2
					}
					continue
				case <-ctx.Done():
					return
				}
			}

			backoff = 200 * time.Millisecond
			log.Printf("connected to Binance aggTrade: %s", symbol)

			readDone := make(chan struct{})
			go func() {
				defer close(readDone)
				defer conn.Close()

				for {
					_, message, err := conn.ReadMessage()
					if err != nil {
						log.Printf("binance read error: %v", err)
						return
					}

					var msg aggTradeMessage
					if err := json.Unmarshal(message, &msg); err != nil {
						log.Printf("binance json error: %v", err)
						continue
					}

					price, err := strconv.ParseFloat(msg.Price, 64)
					if err != nil {
						log.Printf("binance price parse error: %v", err)
						continue
					}

					ev := types.PriceEvent{
						Symbol:    msg.Symbol,
						Price:     price,
						Timestamp: time.UnixMilli(msg.TradeTime),
						Source:    "binance",
					}

					select {
					case ch <- ev:
					case <-ctx.Done():
						return
					}
				}
			}()

			select {
			case <-ctx.Done():
				_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				<-readDone
				return
			case <-readDone:
				continue
			}
		}
	}()

	return ch, nil
}
