package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"runner_race/internal/storage"
)

func TestCreateRunnerAndMatch(t *testing.T) {
	h := NewHandler(storage.NewMemoryStore())
	routes := h.Routes()

	a := createRunner(t, routes, "小蓝兔", 11)
	b := createRunner(t, routes, "红豆", 12)

	body := map[string]any{
		"runnerAId": a,
		"runnerBId": b,
		"seed":      99,
	}
	resp := postJSON(t, routes, "/api/matches", body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create match status = %d", resp.Code)
	}

	var result struct {
		MatchID string `json:"matchId"`
		Winner  string `json:"winner"`
		Frames  []any  `json:"frames"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.MatchID == "" || result.Winner == "" || len(result.Frames) == 0 {
		t.Fatalf("bad match result: %+v", result)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/matches/"+result.MatchID, nil)
	getResp := httptest.NewRecorder()
	routes.ServeHTTP(getResp, req)
	if getResp.Code != http.StatusOK {
		t.Fatalf("get match status = %d", getResp.Code)
	}
}

func TestCreateWrestlerAndSumoMatch(t *testing.T) {
	h := NewHandler(storage.NewMemoryStore())
	routes := h.Routes()

	a := createWrestler(t, routes, "团团", 21, "aggressive")
	b := createWrestler(t, routes, "圆圆", 22, "defensive")

	body := map[string]any{
		"wrestlerAId": a,
		"wrestlerBId": b,
		"seed":        1234,
	}
	resp := postJSON(t, routes, "/api/sumo/matches", body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create sumo match status = %d", resp.Code)
	}

	var result struct {
		MatchID string `json:"matchId"`
		Winner  string `json:"winner"`
		Reason  string `json:"reason"`
		Frames  []any  `json:"frames"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.MatchID == "" || result.Winner == "" || result.Reason == "" || len(result.Frames) == 0 {
		t.Fatalf("bad sumo match result: %+v", result)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/sumo/matches/"+result.MatchID, nil)
	getResp := httptest.NewRecorder()
	routes.ServeHTTP(getResp, req)
	if getResp.Code != http.StatusOK {
		t.Fatalf("get sumo match status = %d", getResp.Code)
	}
}

func createRunner(t *testing.T, routes http.Handler, name string, seed int64) string {
	t.Helper()
	body := map[string]any{
		"name": name,
		"seed": seed,
		"stats": map[string]int{
			"burst":     8,
			"speed":     7,
			"stamina":   5,
			"stability": 6,
			"reaction":  7,
			"grit":      3,
		},
		"strategy": map[string]string{
			"start":  "normal",
			"middle": "steady",
			"sprint": "normal",
		},
	}
	resp := postJSON(t, routes, "/api/runners", body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create runner status = %d", resp.Code)
	}
	var out struct {
		Runner struct {
			ID string `json:"id"`
		} `json:"runner"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Runner.ID == "" {
		t.Fatal("empty runner id")
	}
	return out.Runner.ID
}

func createWrestler(t *testing.T, routes http.Handler, name string, seed int64, style string) string {
	t.Helper()
	body := map[string]any{
		"name": name,
		"seed": seed,
		"stats": map[string]int{
			"power":    8,
			"weight":   7,
			"balance":  6,
			"footwork": 5,
			"stamina":  5,
			"spirit":   5,
		},
		"style": style,
	}
	resp := postJSON(t, routes, "/api/sumo/wrestlers", body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create wrestler status = %d", resp.Code)
	}
	var out struct {
		Wrestler struct {
			ID string `json:"id"`
		} `json:"wrestler"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Wrestler.ID == "" {
		t.Fatal("empty wrestler id")
	}
	return out.Wrestler.ID
}

func postJSON(t *testing.T, routes http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	routes.ServeHTTP(resp, req)
	return resp
}
