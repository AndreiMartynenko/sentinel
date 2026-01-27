package httpapi

import (
	"context"
	"encoding/json"

	"realtime-market-engine/internal/types"
)

type Hub struct {
	register   chan *client
	unregister chan *client
	broadcast  chan types.PriceEvent
	clients    map[*client]struct{}
}

func NewHub() *Hub {
	return &Hub{
		register:   make(chan *client),
		unregister: make(chan *client),
		broadcast:  make(chan types.PriceEvent, 1024),
		clients:    make(map[*client]struct{}),
	}
}

func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			for c := range h.clients {
				close(c.send)
				delete(h.clients, c)
			}
			return
		case c := <-h.register:
			h.clients[c] = struct{}{}
		case c := <-h.unregister:
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
		case ev := <-h.broadcast:
			b, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			for c := range h.clients {
				select {
				case c.send <- b:
				default:
					delete(h.clients, c)
					close(c.send)
				}
			}
		}
	}
}

func (h *Hub) Publish(ev types.PriceEvent) {
	select {
	case h.broadcast <- ev:
	default:
	}
}

type client struct {
	send chan []byte
}
