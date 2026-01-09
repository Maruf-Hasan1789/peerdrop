package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
)

func main() {
	port := flag.Int("port", 9000, "port")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("Port : %v\n", *port)
	d := discovery.New()

	//start registering
	go func() {
		if err := d.Register(ctx, *port); err != nil {
			log.Fatal(err)
		}
	}()

	//start browsing
	go func() {
		d.StartPersistentBrowse(ctx)
	}()

	waitForShutDown()
	cancel()
}

func waitForShutDown() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}
