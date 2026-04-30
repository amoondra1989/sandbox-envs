package main

import (
	"log"
	"net/http"
	"os"

	"github.com/amoondra1989/sandbox-envs/internal/api"
	"github.com/amoondra1989/sandbox-envs/internal/sandbox"
)

func main() {
	addr := os.Getenv("SANDBOX_LISTEN")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}

	backend := sandbox.NewPodman("", nil)
	handler := api.NewServer(backend)

	log.Printf("sandbox-envs listening on http://%s (no auth — localhost only)", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}
