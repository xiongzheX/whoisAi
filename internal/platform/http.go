package platform

import (
	"encoding/json"
	"net/http"
	"strings"
)

type HTTPAPI struct {
	registry *Registry
	store    *MemoryStore
}

func NewHTTPAPI(registry *Registry, store *MemoryStore) *HTTPAPI {
	return &HTTPAPI{registry: registry, store: store}
}

func (a *HTTPAPI) Games(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"games": a.registry.Games()})
}

func (a *HTTPAPI) Rooms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	gameID := strings.TrimSpace(r.URL.Query().Get("gameId"))
	if gameID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少 gameId"})
		return
	}
	rooms, err := a.store.WaitingRooms(gameID)
	if err != nil {
		status := http.StatusBadRequest
		if err == ErrGameNotFound {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"rooms": rooms})
}

func (a *HTTPAPI) Room(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	code := strings.TrimPrefix(r.URL.Path, "/api/platform/rooms/")
	if code == "" || strings.Contains(code, "/") {
		http.NotFound(w, r)
		return
	}
	room, ok := a.store.RoomByCode(code)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "房间不存在"})
		return
	}
	response := map[string]any{"room": room}
	if session, exists := a.store.ActiveSession(code); exists {
		response["activeSession"] = session
	}
	writeJSON(w, http.StatusOK, response)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
