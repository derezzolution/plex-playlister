package main

import (
	"context"
	"embed"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/derezzolution/plex-playlister/http"
	"github.com/derezzolution/plex-playlister/service"
)

//go:embed LICENSE
var packageFS embed.FS

func main() {
	s := service.NewService(&packageFS)
	s.LogSummary()

	// Create a channel to listen for interrupt or termination signals
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	httpService := http.NewHttpService(s)
	httpService.Start()

	// Wait for a signal
	<-interrupt

	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shut down the server
	err := httpService.Stop(ctx)
	if err != nil {
		log.Fatalf("fatal server shutdown error: %v", err)
	}

	log.Println("http service shut down gracefully")
	os.Exit(0)
}
