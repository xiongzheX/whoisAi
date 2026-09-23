package poker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"sync"
	"time"
)

// Records keeps completed hands separate from the in-memory live table.
// SaveHand must atomically save every participant and be idempotent by hand ID.
type Records interface {
	Durable() bool
	Player(context.Context, string, string) error
	SaveHand(context.Context, HandRecord) error
	History(context.Context, string) (History, error)
}

type HandRecord struct {
	ID       string       `json:"id"`
	RoomCode string       `json:"roomCode"`
	Hand     int          `json:"hand"`
	EndedAt  time.Time    `json:"endedAt"`
	Result   string       `json:"result"`
	Players  []HandPlayer `json:"-"`
}
type HandPlayer struct {
	Seat                         int
	ProfileID                    string
	Name                         string
	Bot                          bool
	StartChips, EndChips, Payout int
}
type HistoryEntry struct {
	HandRecord
	StartChips int `json:"startChips"`
	EndChips   int `json:"endChips"`
	Net        int `json:"net"`
}
type History struct {
	Name      string         `json:"name"`
	Hands     int            `json:"hands"`
	Wins      int            `json:"wins"`
	Net       int            `json:"net"`
	LastChips *int           `json:"lastChips"`
	Recent    []HistoryEntry `json:"recent"`
}

func profileID(token string) string {
	hash := sha256.Sum256([]byte("poker-profile:" + token))
	return hex.EncodeToString(hash[:])
}

// MemoryRecords supports the same history UI in local, non-persistent mode.
type MemoryRecords struct {
	mu      sync.Mutex
	players map[string]string
	hands   map[string]HandRecord
}

func NewMemoryRecords() *MemoryRecords {
	return &MemoryRecords{players: make(map[string]string), hands: make(map[string]HandRecord)}
}
func (*MemoryRecords) Durable() bool { return false }
func (m *MemoryRecords) Player(ctx context.Context, id, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.players[id] = name
	return nil
}
func validRecord(h HandRecord) error {
	if h.ID == "" || h.RoomCode == "" || h.Hand < 1 || h.EndedAt.IsZero() || len(h.Players) < 2 || len(h.Players) > 9 {
		return errors.New("invalid hand record")
	}
	seats := map[int]bool{}
	profiles := map[string]bool{}
	start, end := 0, 0
	for _, p := range h.Players {
		if seats[p.Seat] || p.Seat < 0 || p.Seat > 8 || p.StartChips <= 0 || p.EndChips < 0 || p.Payout < 0 {
			return errors.New("invalid hand participant")
		}
		seats[p.Seat] = true
		if !p.Bot {
			if p.ProfileID == "" || profiles[p.ProfileID] {
				return errors.New("invalid player identity")
			}
			profiles[p.ProfileID] = true
		}
		start += p.StartChips
		end += p.EndChips
	}
	if start != end {
		return errors.New("hand chips are not conserved")
	}
	return nil
}
func (m *MemoryRecords) SaveHand(ctx context.Context, h HandRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validRecord(h); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.hands[h.ID]; ok {
		return nil
	}
	h.Players = append([]HandPlayer(nil), h.Players...)
	m.hands[h.ID] = h
	return nil
}
func (m *MemoryRecords) History(ctx context.Context, id string) (History, error) {
	if err := ctx.Err(); err != nil {
		return History{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	history := History{Name: m.players[id], Recent: []HistoryEntry{}}
	for _, h := range m.hands {
		for _, p := range h.Players {
			if !p.Bot && p.ProfileID == id {
				e := HistoryEntry{HandRecord: h, StartChips: p.StartChips, EndChips: p.EndChips, Net: p.EndChips - p.StartChips}
				e.Players = nil
				history.Recent = append(history.Recent, e)
				history.Hands++
				history.Net += e.Net
				if e.Net > 0 {
					history.Wins++
				}
			}
		}
	}
	sort.Slice(history.Recent, func(i, j int) bool {
		a, b := history.Recent[i], history.Recent[j]
		if a.EndedAt.Equal(b.EndedAt) {
			return a.ID > b.ID
		}
		return a.EndedAt.After(b.EndedAt)
	})
	if len(history.Recent) > 0 {
		chips := history.Recent[0].EndChips
		history.LastChips = &chips
	}
	if len(history.Recent) > 20 {
		history.Recent = history.Recent[:20]
	}
	return history, nil
}
