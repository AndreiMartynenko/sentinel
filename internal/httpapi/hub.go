package httpapi

import (
	"context"
	"encoding/json"

	"realtime-market-engine/internal/types"
)

type Hub struct {
	register   chan *client
	unregister chan *client
	broadcast  chan []byte
	clients    map[*client]struct{}
}

func NewHub() *Hub {
	return &Hub{
		register:   make(chan *client),
		unregister: make(chan *client),
		broadcast:  make(chan []byte, 1024),
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
		case b := <-h.broadcast:
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

func (h *Hub) PublishPrice(ev types.PriceEvent) {
	b, err := json.Marshal(ev)
	if err != nil {
		return
	}
	h.PublishJSON(b)
}

func (h *Hub) PublishJSON(b []byte) {
	select {
	case h.broadcast <- b:
	default:
	}
}

type client struct {
	send chan []byte
}
