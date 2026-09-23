package poker

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func sampleRecord(id string, at time.Time) HandRecord {
	return HandRecord{ID: id, RoomCode: "test-room", Hand: 1, EndedAt: at, Result: "Alice 赢得 40 豆", Players: []HandPlayer{
		{Seat: 0, ProfileID: "alice", Name: "Alice", StartChips: 1000, EndChips: 1020, Payout: 40},
		{Seat: 1, ProfileID: "bob", Name: "Bob", StartChips: 1000, EndChips: 980},
	}}
}
func recordsContract(t *testing.T, store Records) {
	t.Helper()
	ctx := context.Background()
	for _, id := range []string{"alice", "bob"} {
		if err := store.Player(ctx, id, id); err != nil {
			t.Fatal(err)
		}
	}
	base := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	h := sampleRecord("hand-one", base)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := store.SaveHand(ctx, h); err != nil {
				t.Errorf("SaveHand: %v", err)
			}
		}()
	}
	wg.Wait()
	got, err := store.History(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if got.Hands != 1 || got.Wins != 1 || got.Net != 20 || got.LastChips == nil || *got.LastChips != 1020 {
		t.Fatalf("duplicate settlement changed totals: %+v", got)
	}
	other, err := store.History(ctx, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if other.Net != -20 || other.Wins != 0 {
		t.Fatalf("loser totals: %+v", other)
	}
	for i := 0; i < 24; i++ {
		h := sampleRecord(fmt.Sprint("more-", i), base.Add(time.Duration(i+1)*time.Second))
		if err := store.SaveHand(ctx, h); err != nil {
			t.Fatal(err)
		}
	}
	got, err = store.History(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if got.Hands != 25 || got.Net != 500 || len(got.Recent) != 20 || got.Recent[0].ID != "more-23" {
		t.Fatalf("history ordering/count/limit: %+v", got)
	}
	empty, err := store.History(ctx, "stranger")
	if err != nil || empty.Hands != 0 || len(empty.Recent) != 0 || empty.LastChips != nil {
		t.Fatalf("stranger history: %+v, %v", empty, err)
	}
	raw, _ := json.Marshal(got)
	for _, secret := range []string{"ProfileID", "Players", "start_chips", "cards", "deck"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("internal fields in public history: %s", raw)
		}
	}
	invalid := sampleRecord("invalid", base)
	invalid.Players[0].EndChips++
	if err := store.SaveHand(ctx, invalid); err == nil {
		t.Fatal("nonconserving settlement accepted")
	}
}
func TestMemoryRecords(t *testing.T) { recordsContract(t, NewMemoryRecords()) }

// Only run against a dedicated empty test database. Normal test runs need no DB.
func TestPostgresRecords(t *testing.T) {
	databaseURL := os.Getenv("POKER_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("POKER_TEST_DATABASE_URL not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal("cannot connect to test PostgreSQL")
	}
	defer admin.Close(context.Background())
	schema := "poker_test_" + strings.ToLower(rand.Text())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal("test URL must be a PostgreSQL URL")
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	databaseURL = parsed.String()
	db, err := OpenPostgresRecords(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	recordsContract(t, db)
	bad := sampleRecord("rollback-hand", time.Now())
	bad.Players[1].ProfileID = "missing-player"
	if err = db.SaveHand(ctx, bad); err == nil {
		t.Fatal("missing FK accepted")
	}
	var count int
	if err = db.pool.QueryRow(ctx, `SELECT count(*) FROM poker_hands WHERE id='rollback-hand'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed transaction left hand: %d, %v", count, err)
	}
	reopened, err := OpenPostgresRecords(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.History(ctx, "alice")
	if err != nil || got.Hands != 25 {
		t.Fatalf("reopen/migration lost history: %+v, %v", got, err)
	}
}
func TestConnectionErrorsDoNotLeakSecrets(t *testing.T) {
	_, err := OpenPostgresRecords(context.Background(), "postgres://user:TOPSECRET@bad host/db")
	if err == nil || strings.Contains(err.Error(), "TOPSECRET") {
		t.Fatalf("unsafe configuration error: %v", err)
	}
}

type uncertainRecords struct {
	*MemoryRecords
	fail bool
}

func (r *uncertainRecords) SaveHand(ctx context.Context, h HandRecord) error {
	if err := r.MemoryRecords.SaveHand(ctx, h); err != nil {
		return err
	}
	if r.fail {
		return ErrRecordsUnavailable
	}
	return nil
}
func (*uncertainRecords) Durable() bool { return true }
func TestPendingSettlementRetryAndHistoryIdentity(t *testing.T) {
	s, code, host := setup(t)
	records := &uncertainRecords{MemoryRecords: NewMemoryRecords(), fail: true}
	s.records = records
	_, _, _ = s.handle("configure", request{Code: code, Target: 2}, hostToken, s.now())
	_, guest, err := s.handle("join", request{Code: code, Name: "好友"}, guestToken, s.now())
	if err != nil {
		t.Fatal(err)
	}
	r := s.rooms[code]
	_, _, err = s.handle("start", request{Code: code, Version: r.version}, hostToken, s.now())
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = s.handle("action", request{Code: code, Kind: "fold", Version: r.version, SessionID: r.sessionID}, hostToken, s.now())
	if err != nil {
		t.Fatal(err)
	}
	if r.pending == nil || !s.snapshot(code, host, s.now())["settlementPending"].(bool) {
		t.Fatal("failed save not shown pending")
	}
	for _, op := range []string{"next", "lobby", "leave"} {
		if _, _, err = s.handle(op, request{Code: code, Version: r.version, SessionID: r.sessionID}, hostToken, s.now()); err == nil {
			t.Fatalf("%s discarded pending result", op)
		}
	}
	records.fail = false
	s.tick(s.now().Add(6 * time.Second))
	if r.pending != nil {
		t.Fatal("settlement retry did not finish")
	}
	got, _ := records.History(context.Background(), profileID(hostToken))
	if got.Hands != 1 || got.Net != -10 {
		t.Fatalf("retry double counted: %+v", got)
	}
	req := httptest.NewRequest("GET", "/api/poker/history?playerId="+guest, nil)
	req.Header.Set("Authorization", "Bearer "+hostToken)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	var response struct {
		History History `json:"history"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || response.History.Net != -10 {
		t.Fatalf("history identity bypass: %d %s", w.Code, w.Body.String())
	}
	// Room changes keep the same profile, with a new room-scoped member ID.
	nextCode, nextMember, err := s.handle("create", request{Name: "新昵称", Target: 2}, hostToken, s.now())
	if err != nil || nextMember == host || s.rooms[nextCode].profiles[nextMember] != r.profiles[host] {
		t.Fatal("profile did not survive room change")
	}
}
func TestUnavailableRecordsBlockRoomCreation(t *testing.T) {
	s, _, _ := setup(t)
	s.records = unavailableRecords{}
	_, _, err := s.handle("create", request{Name: "玩家", Target: 2}, hostToken, s.now())
	if !errors.Is(err, ErrRecordsUnavailable) {
		t.Fatalf("create error = %v", err)
	}
	req := httptest.NewRequest("GET", "/api/poker/history", nil)
	req.Header.Set("Authorization", "Bearer "+hostToken)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 503 {
		t.Fatalf("history status = %d, want 503", w.Code)
	}
}

type unavailableRecords struct{ Records }

func (unavailableRecords) Player(context.Context, string, string) error { return ErrRecordsUnavailable }
func (unavailableRecords) History(context.Context, string) (History, error) {
	return History{}, ErrRecordsUnavailable
}
