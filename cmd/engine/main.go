package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"realtime-market-engine/internal/binance"
	"realtime-market-engine/internal/httpapi"
	"realtime-market-engine/internal/store"
)

// better improvements
func main() {
	var httpAddr string
	flag.StringVar(&httpAddr, "http", ":8080", "HTTP listen address")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st := store.NewPriceStore()
	hub := httpapi.NewHub()
	go hub.Run(ctx)

	events, err := binance.StartAggTradeListener(ctx, "BTCUSDT")
	if err != nil {
		log.Fatalf("binance listener error: %v", err)
	}

	go func() {
		for ev := range events {
			st.Update(ev)
			hub.Publish(ev)
		}
	}()

	mux := http.NewServeMux()
	routes := httpapi.NewRoutes(st, hub)
	routes.Register(mux)

	srv := &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("engine listening on %s", httpAddr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("http server error: %v", err)
	}
}
