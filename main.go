package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"git.icyphox.sh/legit/config"
	"git.icyphox.sh/legit/routes"
	"git.icyphox.sh/legit/ssh"
)

//go:generate templ generate

type server interface {
	ListenAndServe() error
	Shutdown(context.Context) error
}

func main() {
	var cfg string
	flag.StringVar(&cfg, "config", "./config.yaml", "path to config file")
	flag.Parse()

	c, err := config.Read(cfg)
	if err != nil {
		log.Fatal(err)
	}

	mux := routes.Handlers(c)
	addr := fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)

	servers := []server{
		&http.Server{
			Addr:    addr,
			Handler: mux,
		},
	}
	log.Printf("Starting HTTP server on %s", addr)
	if c.SSH.Port > 0 && len(c.SSH.Identity) > 0 {
		log.Printf("Starting SSH server on %s:%d with %d authorized keys", c.SSH.Host, c.SSH.Port, len(c.SSH.Identity))
		servers = append(servers, ssh.NewServer(c))
	}
	for _, s := range servers {
		go func(s server) {
			if err := s.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				log.Fatalf("Server error on %s: %v", addr, err)
			}
		}(s)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownRelease()

	log.Printf("Signal received, shutting down")
	for _, s := range servers {
		if err := s.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, ssh.ErrServerClosed) {
			log.Fatalf("Server shutdown error: %v", err)
		}
	}
	log.Println("Shutdown complete")
}
