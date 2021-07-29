package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	httphandler "github.com/alex-a-renoire/sigma-homework/pkg/httpserver/handler"
	"github.com/alex-a-renoire/sigma-homework/pkg/storage/inmemory"
)

type config struct {
	HTTPAddr string
}

func getCfg() config {
	HTTPAddr := os.Getenv("HTTP_ADDR")
	if HTTPAddr == "" {
		HTTPAddr = ":8081"
	}

	return config{
		HTTPAddr: HTTPAddr,
	}
}

func main() {
	cfg := getCfg()

	db := inmemory.New()

	sh := httphandler.New(db)

	srv := http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: sh.GetRouter(),
	}

	//graceful shutdown of server
	sigC := make(chan os.Signal, 1)
	defer close(sigC)
	go func() {
		<-sigC
		srv.Shutdown(context.TODO())
	}()
	signal.Notify(sigC, syscall.SIGINT, syscall.SIGTERM)

	//Start the server
	log.Print("Starting server...")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("error: http server failed: %s", err)
	}
}