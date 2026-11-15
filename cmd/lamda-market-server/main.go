// lamda-market-server serves the shared market behind Cloudflare Access/Tunnel.
package main

import (
	"context"
	"github.com/tm-LBenson/lamda-engine/internal/market"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func main() {
	root := env("MARKET_DATA", "/data")
	client := env("MARKET_CLIENT", "forever")
	access, err := market.NewAccess(os.Getenv("CF_ACCESS_ISSUER"), os.Getenv("CF_ACCESS_AUDIENCE"))
	if err != nil {
		log.Fatal(err)
	}
	shared, err := market.Open(filepath.Join(root, "shared.sqlite"), client)
	if err != nil {
		log.Fatal(err)
	}
	defer shared.Close()
	profiles, err := market.OpenProfiles(filepath.Join(root, "profiles"), client)
	if err != nil {
		log.Fatal(err)
	}
	defer profiles.Close()
	handler, err := market.NewSharedServer(shared, profiles, access, os.Getenv("MARKET_HOST"), os.Getenv("MARKET_OWNER_EMAIL"))
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{Addr: env("MARKET_LISTEN", ":8080"), Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(shutdown)
	}()
	log.Print("LamdaMarket shared server listening")
	if err = server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
