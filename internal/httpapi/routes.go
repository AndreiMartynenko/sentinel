package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"realtime-market-engine/internal/store"

	"github.com/gorilla/websocket"
)

type Routes struct {
	store    *store.PriceStore
	hub      *Hub
	upgrader websocket.Upgrader
}

func NewRoutes(store *store.PriceStore, hub *Hub) *Routes {
	return &Routes{
		store: store,
		hub:   hub,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (rt *Routes) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /", rt.index)
	mux.HandleFunc("GET /health", rt.health)
	mux.HandleFunc("GET /prices/", rt.priceBySymbol)
	mux.HandleFunc("GET /ws", rt.ws)
}

func (rt *Routes) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html>
<html>
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>realtime-market-engine</title>
  </head>
  <body style="font-family: ui-sans-serif, system-ui, -apple-system; padding: 16px;">
    <h2>BTCUSDT Live</h2>
    <div id="status">Connecting…</div>
    <pre id="out" style="background:#111;color:#eee;padding:12px;border-radius:8px;overflow:auto;max-height:70vh;"></pre>
    <script>
      const status = document.getElementById('status');
      const out = document.getElementById('out');
      const scheme = location.protocol === 'https:' ? 'wss' : 'ws';
      const ws = new WebSocket(scheme + '://' + location.host + '/ws');
      ws.onopen = () => status.textContent = 'Connected';
      ws.onclose = () => status.textContent = 'Disconnected';
      ws.onerror = () => status.textContent = 'Error';
      ws.onmessage = (ev) => {
        out.textContent = ev.data + "\n" + out.textContent;
      };
    </script>
  </body>
</html>`))
}

func (rt *Routes) health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (rt *Routes) priceBySymbol(w http.ResponseWriter, r *http.Request) {
	symbol := strings.TrimPrefix(r.URL.Path, "/prices/")
	if symbol == "" {
		http.Error(w, "symbol required", http.StatusBadRequest)
		return
	}

	ev, ok := rt.store.Get(symbol)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Symbol    string    `json:"symbol"`
		Price     float64   `json:"price"`
		Timestamp time.Time `json:"timestamp"`
		Source    string    `json:"source"`
	}{
		Symbol:    ev.Symbol,
		Price:     ev.Price,
		Timestamp: ev.Timestamp,
		Source:    ev.Source,
	})
}

func (rt *Routes) ws(w http.ResponseWriter, r *http.Request) {
	conn, err := rt.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	c := &client{send: make(chan []byte, 256)}
	rt.hub.register <- c

	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		defer conn.Close()

		for msg := range c.send {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}

	rt.hub.unregister <- c
	<-writeDone
}
