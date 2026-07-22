package platform

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPAPIGames(t *testing.T) {
	t.Parallel()

	registry := DefaultRegistry()
	api := NewHTTPAPI(registry, NewMemoryStore(registry))
	request := httptest.NewRequest(http.MethodGet, "/api/platform/games", nil)
	recorder := httptest.NewRecorder()

	api.Games(recorder, request)

	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Fatalf("Games() status = %d, want %d", got, want)
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"id":"who-is-ai"`) ||
		!strings.Contains(body, `"id":"bean-sprint"`) ||
		!strings.Contains(body, `"id":"dumpling-sumo"`) {
		t.Errorf("Games() body = %s, want registered games", body)
	}
}

func TestHTTPAPIRoomNotFound(t *testing.T) {
	t.Parallel()

	registry := DefaultRegistry()
	api := NewHTTPAPI(registry, NewMemoryStore(registry))
	request := httptest.NewRequest(http.MethodGet, "/api/platform/rooms/missing", nil)
	recorder := httptest.NewRecorder()

	api.Room(recorder, request)

	if got, want := recorder.Code, http.StatusNotFound; got != want {
		t.Errorf("Room(missing) status = %d, want %d", got, want)
	}
}

func TestHTTPAPIWaitingRooms(t *testing.T) {
	t.Parallel()

	registry := DefaultRegistry()
	store := NewMemoryStore(registry)
	if _, err := store.CreateRoom(CreateRoomInput{
		Code: "race-friends", HostID: "host", HostName: "小蓝", GameID: "bean-sprint",
	}); err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}
	api := NewHTTPAPI(registry, store)
	request := httptest.NewRequest(http.MethodGet, "/api/platform/rooms?gameId=bean-sprint", nil)
	recorder := httptest.NewRecorder()

	api.Rooms(recorder, request)

	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Fatalf("Rooms(bean-sprint) status = %d, want %d", got, want)
	}
	var response struct {
		Rooms []WaitingRoomSummary `json:"rooms"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Decode(Rooms(bean-sprint)) error = %v", err)
	}
	if got, want := len(response.Rooms), 1; got != want {
		t.Fatalf("len(Rooms(bean-sprint)) = %d, want %d", got, want)
	}
	if got, want := response.Rooms[0].Code, "race-friends"; got != want {
		t.Errorf("Rooms(bean-sprint)[0].Code = %q, want %q", got, want)
	}
}

func TestHTTPAPIWaitingRoomsRequiresGame(t *testing.T) {
	t.Parallel()

	registry := DefaultRegistry()
	api := NewHTTPAPI(registry, NewMemoryStore(registry))
	request := httptest.NewRequest(http.MethodGet, "/api/platform/rooms", nil)
	recorder := httptest.NewRecorder()

	api.Rooms(recorder, request)

	if got, want := recorder.Code, http.StatusBadRequest; got != want {
		t.Errorf("Rooms(no game) status = %d, want %d", got, want)
	}
}
