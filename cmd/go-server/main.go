package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	siosocket "github.com/zishang520/socket.io/socket"

	"whoisai/internal/ai"
	"whoisai/internal/envfile"
	"whoisai/internal/game"
	"whoisai/internal/realtime"
)

func main() {
	if err := envfile.Load(".env"); err != nil {
		log.Printf("failed to load .env: %v", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "3014"
	}

	store := game.NewStore()
	aiConfig := ai.LoadConfigFromEnv()
	if aiConfig.Enabled() {
		log.Printf("AI rewrite enabled via %s", aiConfig.BaseURL)
	} else {
		log.Printf("AI rewrite disabled, using local fallback")
	}
	service := game.NewService(store, game.WithMessageRewriter(ai.NewClient(aiConfig, log.Default())))
	io := siosocket.NewServer(nil, nil)
	realtime.New(io, store, service).Register()

	mux := http.NewServeMux()
	mux.HandleFunc("/favicon.ico", serveNoContent)
	mux.HandleFunc("/socket.io/socket.io.js", serveSocketIOClient)
	mux.HandleFunc("/socket.io/socket.io.js.map", serveSocketIOClient)
	mux.Handle("/socket.io/", io.ServeHandler(nil))
	mux.Handle("/", http.FileServer(http.Dir("client")))

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		log.Printf("Go server running at http://localhost:%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	io.Close(nil)
	_ = server.Close()
}

func serveNoContent(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func serveSocketIOClient(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	name := filepath.Base(r.URL.Path)
	if name != "socket.io.js" && name != "socket.io.js.map" {
		http.NotFound(w, r)
		return
	}
	if filepath.Ext(name) == ".map" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	}
	http.ServeFile(w, r, filepath.Join("node_modules", "socket.io", "client-dist", name))
}
