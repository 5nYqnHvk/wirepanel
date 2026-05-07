package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/wirepanel/wirepanel/agent/internal/client"
	"github.com/wirepanel/wirepanel/agent/internal/config"
)

func main() {
	cfg := config.Load()
	c := client.New(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		<-stop
		cancel()
	}()

	log.Printf("WirePanel Agent starting (id=%s, core=%s)", cfg.AgentID, cfg.CoreURL)
	if err := c.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("agent: %v", err)
	}
}
