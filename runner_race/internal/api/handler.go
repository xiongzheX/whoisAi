package api

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"runner_race/internal/runner"
	"runner_race/internal/storage"
	"runner_race/internal/sumo"
)

type Handler struct {
	store *storage.MemoryStore
}

func NewHandler(store *storage.MemoryStore) *Handler {
	return &Handler{store: store}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", h.health)
	mux.HandleFunc("GET /api/runners", h.listRunners)
	mux.HandleFunc("POST /api/runners", h.createRunner)
	mux.HandleFunc("POST /api/matches", h.createMatch)
	mux.HandleFunc("GET /api/matches/{id}", h.getMatch)
	mux.HandleFunc("GET /api/sumo/wrestlers", h.listWrestlers)
	mux.HandleFunc("POST /api/sumo/wrestlers", h.createWrestler)
	mux.HandleFunc("POST /api/sumo/matches", h.createSumoMatch)
	mux.HandleFunc("GET /api/sumo/matches/{id}", h.getSumoMatch)
	return withCORS(mux)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type createRunnerRequest struct {
	Name     string          `json:"name"`
	Stats    runner.Stats    `json:"stats"`
	Strategy runner.Strategy `json:"strategy"`
	Seed     *int64          `json:"seed,omitempty"`
}

func (h *Handler) createRunner(w http.ResponseWriter, r *http.Request) {
	var req createRunnerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body")
		return
	}

	seed := seedOrNow(req.Seed)
	rng := runner.NewRNG(seed)
	id := newID("runner", seed, len(h.store.ListRunners())+1)
	created := runner.Runner{
		ID:       id,
		Name:     strings.TrimSpace(req.Name),
		Stats:    req.Stats,
		Trait:    runner.RandomTrait(rng),
		Strategy: req.Strategy,
	}
	if err := runner.ValidateRunner(created); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_runner", err.Error())
		return
	}

	h.store.SaveRunner(created)
	writeJSON(w, http.StatusCreated, map[string]any{
		"runner": created,
		"seed":   seed,
	})
}

func (h *Handler) listRunners(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"runners": h.store.ListRunners()})
}

type createMatchRequest struct {
	RunnerAID string `json:"runnerAId"`
	RunnerBID string `json:"runnerBId"`
	Seed      *int64 `json:"seed,omitempty"`
}

func (h *Handler) createMatch(w http.ResponseWriter, r *http.Request) {
	var req createMatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body")
		return
	}
	if strings.TrimSpace(req.RunnerAID) == "" || strings.TrimSpace(req.RunnerBID) == "" {
		writeError(w, http.StatusBadRequest, "invalid_match", "runnerAId and runnerBId are required")
		return
	}
	if req.RunnerAID == req.RunnerBID {
		writeError(w, http.StatusBadRequest, "invalid_match", "runnerAId and runnerBId must be different")
		return
	}

	a, err := h.store.Runner(req.RunnerAID)
	if err != nil {
		writeLookupError(w, "runnerAId", err)
		return
	}
	b, err := h.store.Runner(req.RunnerBID)
	if err != nil {
		writeLookupError(w, "runnerBId", err)
		return
	}

	seed := seedOrNow(req.Seed)
	result, err := runner.Simulate(a, b, seed)
	if err != nil {
		writeError(w, http.StatusBadRequest, "simulate_failed", err.Error())
		return
	}
	result.MatchID = newID("match", seed, int(time.Now().UnixNano()%100000))
	h.store.SaveMatch(result)

	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) getMatch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	match, err := h.store.Match(id)
	if err != nil {
		writeLookupError(w, "match", err)
		return
	}
	writeJSON(w, http.StatusOK, match)
}

type createWrestlerRequest struct {
	Name  string     `json:"name"`
	Stats sumo.Stats `json:"stats"`
	Style string     `json:"style"`
	Seed  *int64     `json:"seed,omitempty"`
}

func (h *Handler) createWrestler(w http.ResponseWriter, r *http.Request) {
	var req createWrestlerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body")
		return
	}

	seed := seedOrNow(req.Seed)
	rng := sumo.NewRNG(seed)
	id := newID("wrestler", seed, len(h.store.ListWrestlers())+1)
	created := sumo.Wrestler{
		ID:    id,
		Name:  strings.TrimSpace(req.Name),
		Stats: req.Stats,
		Trait: sumo.RandomTrait(rng),
		Style: req.Style,
	}
	if err := sumo.ValidateWrestler(created); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_wrestler", err.Error())
		return
	}

	h.store.SaveWrestler(created)
	writeJSON(w, http.StatusCreated, map[string]any{
		"wrestler": created,
		"seed":     seed,
	})
}

func (h *Handler) listWrestlers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"wrestlers": h.store.ListWrestlers()})
}

type createSumoMatchRequest struct {
	WrestlerAID string `json:"wrestlerAId"`
	WrestlerBID string `json:"wrestlerBId"`
	Seed        *int64 `json:"seed,omitempty"`
}

func (h *Handler) createSumoMatch(w http.ResponseWriter, r *http.Request) {
	var req createSumoMatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body")
		return
	}
	if strings.TrimSpace(req.WrestlerAID) == "" || strings.TrimSpace(req.WrestlerBID) == "" {
		writeError(w, http.StatusBadRequest, "invalid_match", "wrestlerAId and wrestlerBId are required")
		return
	}
	if req.WrestlerAID == req.WrestlerBID {
		writeError(w, http.StatusBadRequest, "invalid_match", "wrestlerAId and wrestlerBId must be different")
		return
	}

	a, err := h.store.Wrestler(req.WrestlerAID)
	if err != nil {
		writeLookupError(w, "wrestlerAId", err)
		return
	}
	b, err := h.store.Wrestler(req.WrestlerBID)
	if err != nil {
		writeLookupError(w, "wrestlerBId", err)
		return
	}

	seed := seedOrNow(req.Seed)
	result, err := sumo.Simulate(a, b, seed)
	if err != nil {
		writeError(w, http.StatusBadRequest, "simulate_failed", err.Error())
		return
	}
	result.MatchID = newID("sumo_match", seed, int(time.Now().UnixNano()%100000))
	h.store.SaveSumoMatch(result)

	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) getSumoMatch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	match, err := h.store.SumoMatch(id)
	if err != nil {
		writeLookupError(w, "sumo match", err)
		return
	}
	writeJSON(w, http.StatusOK, match)
}

func seedOrNow(seed *int64) int64 {
	if seed != nil {
		return *seed
	}
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err == nil {
		return int64(binary.LittleEndian.Uint64(buf[:]))
	}
	return time.Now().UnixNano()
}

func newID(prefix string, seed int64, n int) string {
	if seed < 0 {
		seed = -seed
	}
	return fmt.Sprintf("%s_%x_%d", prefix, seed, n)
}

func writeLookupError(w http.ResponseWriter, field string, err error) {
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", field+" not found")
		return
	}
	writeError(w, http.StatusInternalServerError, "store_error", err.Error())
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
