// Package duel owns the shared two-player ready/start/result lifecycle used by
// lightweight competitive games. PartyRoom and GameSession remain owned by the
// platform package.
package duel

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	runnerduel "runner_race/duel"
	"whoisai/internal/platform"
)

type Phase string

const (
	PhaseConfiguring Phase = "configuring"
	PhaseRunning     Phase = "running"
	PhaseFinished    Phase = "finished"
)

var (
	ErrNotParticipant = errors.New("你不在这个双人房间中")
	ErrNotReady       = errors.New("需要两名玩家都完成配置")
	ErrAlreadyRunning = errors.New("比赛已经开始")
)

type PlayerState struct {
	ID               string                    `json:"id"`
	Name             string                    `json:"name"`
	Seat             int                       `json:"seat"`
	IsHost           bool                      `json:"isHost"`
	Ready            bool                      `json:"ready"`
	ConnectionStatus platform.ConnectionStatus `json:"connectionStatus"`
}

func (s *Service) CancelReady(room *platform.PartyRoom, playerID string) (State, error) {
	if _, ok := roomMember(room, playerID); !ok {
		return State{}, ErrNotParticipant
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	stored := s.ensureRoom(room)
	if stored.phase == PhaseRunning {
		return State{}, ErrAlreadyRunning
	}
	delete(stored.configs, playerID)
	delete(stored.ready, playerID)
	stored.updatedAt = time.Now().UTC()
	return buildState(room, stored), nil
}

func (s *Service) RemovePlayer(roomCode, playerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored := s.rooms[roomCode]
	if stored == nil {
		return
	}
	delete(stored.configs, playerID)
	delete(stored.ready, playerID)
	if stored.phase == PhaseFinished {
		stored.phase = PhaseConfiguring
		stored.sessionID = ""
		stored.configs = make(map[string]json.RawMessage)
		stored.ready = make(map[string]bool)
		stored.result = nil
	}
	stored.updatedAt = time.Now().UTC()
}

type State struct {
	RoomCode  string             `json:"roomCode"`
	GameID    string             `json:"gameId"`
	Phase     Phase              `json:"phase"`
	Round     uint64             `json:"round"`
	SessionID string             `json:"sessionId,omitempty"`
	Players   []PlayerState      `json:"players"`
	Result    *runnerduel.Result `json:"result,omitempty"`
	UpdatedAt time.Time          `json:"updatedAt"`
}

type roomState struct {
	gameID    string
	phase     Phase
	round     uint64
	sessionID string
	configs   map[string]json.RawMessage
	ready     map[string]bool
	result    *runnerduel.Result
	updatedAt time.Time
}

type Service struct {
	mu    sync.RWMutex
	rooms map[string]*roomState
}

func NewService() *Service {
	return &Service{rooms: make(map[string]*roomState)}
}

func (s *Service) State(room *platform.PartyRoom) State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return buildState(room, s.rooms[room.Code])
}

func (s *Service) SessionSettings(room *platform.PartyRoom) (json.RawMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stored := s.rooms[room.Code]
	state := buildState(room, stored)
	if stored == nil || !bothReady(state.Players) {
		return nil, ErrNotReady
	}
	configs := make(map[string]json.RawMessage, len(stored.configs))
	for playerID, config := range stored.configs {
		configs[playerID] = append(json.RawMessage(nil), config...)
	}
	settings, err := json.Marshal(map[string]any{
		"schemaVersion": 1,
		"authority":     "server",
		"playerConfigs": configs,
	})
	if err != nil {
		return nil, fmt.Errorf("encode duel session settings: %w", err)
	}
	return settings, nil
}

func (s *Service) Ready(
	room *platform.PartyRoom,
	playerID string,
	config json.RawMessage,
) (State, bool, error) {
	member, ok := roomMember(room, playerID)
	if !ok {
		return State{}, false, ErrNotParticipant
	}
	if err := runnerduel.ValidateConfig(room.SelectedGameID, member.DisplayName, config); err != nil {
		return State{}, false, fmt.Errorf("选手配置无效: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	stored := s.ensureRoom(room)
	if stored.phase == PhaseRunning {
		return State{}, false, ErrAlreadyRunning
	}
	if stored.phase == PhaseFinished {
		stored.phase = PhaseConfiguring
		stored.sessionID = ""
		stored.configs = make(map[string]json.RawMessage)
		stored.ready = make(map[string]bool)
		stored.result = nil
	}
	stored.configs[playerID] = append(json.RawMessage(nil), config...)
	stored.ready[playerID] = true
	stored.updatedAt = time.Now().UTC()

	state := buildState(room, stored)
	return state, bothReady(state.Players), nil
}

func (s *Service) Start(room *platform.PartyRoom, sessionID string) (State, error) {
	s.mu.Lock()
	stored := s.ensureRoom(room)
	if stored.phase == PhaseRunning {
		s.mu.Unlock()
		return State{}, ErrAlreadyRunning
	}
	state := buildState(room, stored)
	if !bothReady(state.Players) {
		s.mu.Unlock()
		return State{}, ErrNotReady
	}

	inputs := make([]runnerduel.PlayerInput, 0, 2)
	for _, player := range state.Players {
		inputs = append(inputs, runnerduel.PlayerInput{
			ID:     player.ID,
			Name:   player.Name,
			Config: append(json.RawMessage(nil), stored.configs[player.ID]...),
		})
	}
	stored.phase = PhaseRunning
	stored.sessionID = sessionID
	stored.round++
	stored.updatedAt = time.Now().UTC()
	s.mu.Unlock()

	result, err := runnerduel.Simulate(room.SelectedGameID, inputs, randomSeed())
	if err != nil {
		s.mu.Lock()
		stored.phase = PhaseConfiguring
		stored.sessionID = ""
		stored.updatedAt = time.Now().UTC()
		s.mu.Unlock()
		return State{}, fmt.Errorf("比赛结算失败: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	stored.phase = PhaseFinished
	stored.result = cloneResult(result)
	stored.updatedAt = time.Now().UTC()
	return buildState(room, stored), nil
}

func (s *Service) ensureRoom(room *platform.PartyRoom) *roomState {
	stored, ok := s.rooms[room.Code]
	if ok && stored.gameID == room.SelectedGameID {
		return stored
	}
	stored = &roomState{
		gameID:    room.SelectedGameID,
		phase:     PhaseConfiguring,
		configs:   make(map[string]json.RawMessage),
		ready:     make(map[string]bool),
		updatedAt: time.Now().UTC(),
	}
	s.rooms[room.Code] = stored
	return stored
}

func buildState(room *platform.PartyRoom, stored *roomState) State {
	state := State{
		RoomCode:  room.Code,
		GameID:    room.SelectedGameID,
		Phase:     PhaseConfiguring,
		Players:   make([]PlayerState, 0, len(room.Members)),
		UpdatedAt: room.UpdatedAt,
	}
	if stored != nil && stored.gameID == room.SelectedGameID {
		state.Phase = stored.phase
		state.Round = stored.round
		state.SessionID = stored.sessionID
		state.Result = cloneResultPointer(stored.result)
		state.UpdatedAt = stored.updatedAt
	}
	for _, member := range room.Members {
		if member.ConnectionStatus == platform.MemberLeft {
			continue
		}
		ready := stored != nil && stored.gameID == room.SelectedGameID && stored.ready[member.ID]
		state.Players = append(state.Players, PlayerState{
			ID:               member.ID,
			Name:             member.DisplayName,
			Seat:             member.Seat,
			IsHost:           member.ID == room.HostMemberID,
			Ready:            ready,
			ConnectionStatus: member.ConnectionStatus,
		})
	}
	sort.Slice(state.Players, func(i, j int) bool { return state.Players[i].Seat < state.Players[j].Seat })
	return state
}

func roomMember(room *platform.PartyRoom, playerID string) (platform.RoomMember, bool) {
	for _, member := range room.Members {
		if member.ID == playerID && member.ConnectionStatus != platform.MemberLeft {
			return member, true
		}
	}
	return platform.RoomMember{}, false
}

func bothReady(players []PlayerState) bool {
	if len(players) != 2 {
		return false
	}
	return players[0].Ready && players[1].Ready &&
		players[0].ConnectionStatus == platform.MemberOnline &&
		players[1].ConnectionStatus == platform.MemberOnline
}

func randomSeed() int64 {
	var buffer [8]byte
	if _, err := rand.Read(buffer[:]); err == nil {
		return int64(binary.LittleEndian.Uint64(buffer[:]))
	}
	return time.Now().UnixNano()
}

func cloneResult(result runnerduel.Result) *runnerduel.Result {
	return &runnerduel.Result{
		GameID:  result.GameID,
		Players: append(json.RawMessage(nil), result.Players...),
		Match:   append(json.RawMessage(nil), result.Match...),
	}
}

func cloneResultPointer(result *runnerduel.Result) *runnerduel.Result {
	if result == nil {
		return nil
	}
	return cloneResult(*result)
}
