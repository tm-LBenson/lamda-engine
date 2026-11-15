// Standalone, local-only workspace for development and offline market research.
package main

import (
	"flag"
	"fmt"
	"github.com/tm-LBenson/lamda-engine/internal/market"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	db := flag.String("db", "market.sqlite", "SQLite file (use a separate preview directory)")
	client := flag.String("client", "forever", "forever or retail")
	port := flag.Int("port", 47643, "loopback port; 0 chooses a free port")
	flag.Parse()
	store, err := market.Open(*db, *client)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	l, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{Handler: store.LocalHandler(l.Addr().String()), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() { <-stop; server.Close() }()
	fmt.Printf("LamdaMarket: http://%s/market/\n", l.Addr())
	if err = server.Serve(l); err != http.ErrServerClosed {
		log.Print(err)
	}
}
