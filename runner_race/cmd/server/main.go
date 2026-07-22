package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"runner_race/internal/api"
	"runner_race/internal/storage"
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	store := storage.NewMemoryStore()
	apiHandler := api.NewHandler(store).Routes()
	staticHandler := http.FileServer(http.Dir("web"))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			apiHandler.ServeHTTP(w, r)
			return
		}
		staticHandler.ServeHTTP(w, r)
	})

	log.Printf("runner race API listening on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}
