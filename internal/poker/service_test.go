package poker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"whoisai/internal/platform"
)

const hostToken = "host-test-token-00000000000000001"
const guestToken = "guest-test-token-0000000000000001"

func setup(t *testing.T) (*Service, string, string) {
	t.Helper()
	s := NewService(platform.NewMemoryStore(platform.DefaultRegistry()))
	now := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	code, id, err := s.handle("create", request{Name: "房主", Target: 6}, hostToken, now)
	if err != nil {
		t.Fatal(err)
	}
	return s, code, id
}
func TestFillBotsPrivacyAndAuthority(t *testing.T) {
	s, code, host := setup(t)
	_, guest, err := s.handle("join", request{Code: code, Name: "好友"}, guestToken, s.now())
	if err != nil {
		t.Fatal(err)
	}
	r := s.rooms[code]
	if _, _, err = s.handle("configure", request{Code: code, Target: 9}, guestToken, s.now()); err == nil {
		t.Fatal("guest configured")
	}
	if _, _, err = s.handle("start", request{Code: code, Version: r.version}, guestToken, s.now()); err == nil {
		t.Fatal("guest started")
	}
	if _, _, err = s.handle("start", request{Code: code, Version: r.version}, hostToken, s.now()); err != nil {
		t.Fatal(err)
	}
	humans, bots := 0, 0
	for _, p := range r.table.Players {
		if p.Bot {
			bots++
		} else {
			humans++
		}
	}
	if humans != 2 || bots != 4 {
		t.Fatalf("humans %d bots %d", humans, bots)
	}
	for _, id := range []string{host, guest} {
		view := s.snapshot(code, id, s.now())["table"].(map[string]any)
		players := view["players"].([]map[string]any)
		for _, p := range players {
			cards := p["cards"].([]*Card)
			if p["id"] == id {
				if cards[0] == nil || cards[1] == nil {
					t.Fatal("own cards hidden")
				}
			} else {
				if cards[0] != nil || cards[1] != nil {
					t.Fatal("private cards leaked")
				}
			}
		}
		raw, _ := json.Marshal(s.snapshot(code, host, s.now()))
		if bytes.Contains(raw, []byte("deck")) {
			t.Fatal("deck leaked")
		}
	}
	if _, _, err = s.handle("join", request{Code: code, Name: "迟到"}, strings.Repeat("x", 32), s.now()); err == nil {
		t.Fatal("late join accepted")
	}
	if _, _, err = s.handle("action", request{Code: code, Kind: "call", Version: r.version, SessionID: r.sessionID}, hostToken, s.now()); err == nil {
		t.Fatal("out of turn accepted")
	}
	session, _ := s.store.ActiveSession(code)
	if len(session.Participants) != 6 {
		t.Fatal("platform participants missing")
	}
}
func TestHumanOnlyAndCapacity(t *testing.T) {
	s, code, _ := setup(t)
	_, _, _ = s.handle("configure", request{Code: code, Target: 2}, hostToken, s.now())
	_, _, _ = s.handle("join", request{Code: code, Name: "好友"}, guestToken, s.now())
	if _, _, err := s.handle("join", request{Code: code, Name: "挤进来"}, strings.Repeat("z", 32), s.now()); err == nil {
		t.Fatal("capacity exceeded")
	}
	r := s.rooms[code]
	_, _, err := s.handle("start", request{Code: code, Version: r.version}, hostToken, s.now())
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range r.table.Players {
		if p.Bot {
			t.Fatal("unexpected bot")
		}
	}
	version := r.version
	input := request{Code: code, Kind: "call", Version: version, SessionID: r.sessionID}
	if _, _, err = s.handle("action", input, hostToken, s.now()); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.handle("action", input, hostToken, s.now()); err == nil {
		t.Fatal("duplicate accepted")
	}
}
func TestReconnectAndServerTimeout(t *testing.T) {
	s, code, host := setup(t)
	_, _, _ = s.handle("configure", request{Code: code, Target: 2}, hostToken, s.now())
	_, _, _ = s.handle("join", request{Code: code, Name: "好友"}, guestToken, s.now())
	r := s.rooms[code]
	_, _, _ = s.handle("start", request{Code: code, Version: r.version}, hostToken, s.now())
	before := append([]Card{}, r.table.Players[0].Cards...)
	_, reconnected, err := s.handle("join", request{Code: code, Name: "房主"}, hostToken, s.now())
	if err != nil || host != reconnected {
		t.Fatal("reconnect failed")
	}
	if !reflect.DeepEqual(before, r.table.Players[0].Cards) {
		t.Fatal("cards changed")
	}
	s.tick(s.now().Add(21 * time.Second))
	if r.table.LastAction == nil || !r.table.LastAction.TimedOut || r.table.LastAction.Kind != "fold" {
		t.Fatal("timeout did not fold")
	}
	if r.table.Phase != "done" {
		t.Fatal("heads-up hand not ended")
	}
	if _, ok := s.store.ActiveSession(code); ok {
		t.Fatal("session not finished")
	}
}
func TestTickBotsAndHostTransfer(t *testing.T) {
	s, code, _ := setup(t)
	r := s.rooms[code]
	_, _, _ = s.handle("join", request{Code: code, Name: "好友"}, guestToken, s.now())
	_, _, _ = s.handle("start", request{Code: code, Version: r.version}, hostToken, s.now())
	now := s.now()
	for n := 0; n < 300 && r.table.Phase != "done"; n++ {
		now = now.Add(22 * time.Second)
		s.tick(now)
	}
	if r.table.Phase != "done" {
		t.Fatal("bots did not finish")
	}
	_, _, _ = s.handle("state", request{Code: code}, guestToken, now)
	s.tick(now.Add(time.Second))
	party, _ := s.store.RoomByCode(code)
	if party.HostMemberID != memberID(code, guestToken) {
		t.Fatal("host not transferred")
	}
}
func TestHTTPAuthenticationAndInput(t *testing.T) {
	s, code, _ := setup(t)
	tests := []struct {
		method, path, token, body string
		status                    int
	}{{"GET", "/api/poker/state?code=" + code, "", "", 401}, {"GET", "/api/poker/state?code=" + code, strings.Repeat("x", 32), "", 403}, {"POST", "/api/poker/action", hostToken, `{"code":"` + code + `","amount":1.5}`, 400}, {"GET", "/api/poker/create", hostToken, "", 405}, {"GET", "/api/poker/state?code=" + code, hostToken, "", 200}}
	for i, tt := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Authorization", "Bearer "+tt.token)
			w := httptest.NewRecorder()
			s.ServeHTTP(w, req)
			if w.Code != tt.status {
				t.Fatalf("got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestEveryHumanAndTargetCombination(t *testing.T) {
	for target := 2; target <= 9; target++ {
		for humans := 1; humans <= target; humans++ {
			t.Run(fmt.Sprintf("%d_humans_%d_seats", humans, target), func(t *testing.T) {
				s, code, _ := setup(t)
				if _, _, err := s.handle("configure", request{Code: code, Target: target}, hostToken, s.now()); err != nil {
					t.Fatal(err)
				}
				for i := 1; i < humans; i++ {
					if _, _, err := s.handle("join", request{Code: code, Name: fmt.Sprint("玩家", i)}, fmt.Sprintf("guest-token-%032d", i), s.now()); err != nil {
						t.Fatal(err)
					}
				}
				if humans > 2 {
					if _, _, err := s.handle("configure", request{Code: code, Target: humans - 1}, hostToken, s.now()); err == nil {
						t.Fatal("target below humans accepted")
					}
				}
				r := s.rooms[code]
				if _, _, err := s.handle("start", request{Code: code, Version: r.version}, hostToken, s.now()); err != nil {
					t.Fatal(err)
				}
				bots := 0
				for _, p := range r.table.Players {
					if p.Bot {
						bots++
					}
				}
				if len(r.table.Players) != target || bots != target-humans {
					t.Fatal("incorrect fill")
				}
			})
		}
	}
}

func TestBlindAllInFinishesSession(t *testing.T) {
	s, code, _ := setup(t)
	r := s.rooms[code]
	party, _ := s.store.RoomByCode(code)
	r.table = newTable([]Player{{ID: party.HostMemberID, Name: "房主", Chips: 5}, {ID: "bot-1", Name: "电脑", Bot: true, Chips: 5}})
	if err := s.startHand(code, r, party, s.now()); err != nil {
		t.Fatal(err)
	}
	if r.table.Phase != "done" {
		t.Fatal("blind all-in did not finish")
	}
	if _, ok := s.store.ActiveSession(code); ok {
		t.Fatal("finished hand left active session")
	}
}
