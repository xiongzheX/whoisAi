package game

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type Store struct {
	mu    sync.RWMutex
	rooms map[string]*Room
}

func NewStore() *Store {
	return &Store{rooms: make(map[string]*Room)}
}

func (s *Store) CreateRoom(roomID string) *Room {
	s.mu.Lock()
	defer s.mu.Unlock()

	if room, ok := s.rooms[roomID]; ok && room.Status == StatusPlaying {
		return cloneRoom(room)
	}

	room := &Room{
		ID:             roomID,
		Players:        []Player{},
		Status:         StatusWaiting,
		Mode:           ModeNormal,
		Roles:          map[string]Role{},
		MissionResults: []bool{},
		MessageCount:   map[string]int{},
		ChatMessages:   []ChatMessage{},
		TeamVotes:      map[string]bool{},
		MissionVotes:   map[string]string{},
		VoteHistory:    []VoteRecord{},
		SignalHistory:  []SignalRecord{},
	}
	s.rooms[roomID] = room
	return cloneRoom(room)
}

func (s *Store) Room(roomID string) (*Room, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	room, ok := s.rooms[roomID]
	if !ok {
		return nil, false
	}
	return cloneRoom(room), true
}

func (s *Store) AddPlayer(roomID, socketID, name string, mode Mode) (Player, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	room, ok := s.rooms[roomID]
	if !ok {
		return Player{}, errors.New("房间不存在")
	}
	if room.Status == StatusPlaying {
		return Player{}, errors.New("游戏已开始")
	}
	if len(room.Players) >= MaxPlayers {
		return Player{}, errors.New("房间已满")
	}
	for _, player := range room.Players {
		if player.ID == socketID {
			return Player{}, errors.New("已在房间中")
		}
	}
	if mode == "" {
		mode = ModeNormal
	}
	if len(room.Players) == 0 {
		room.Mode = mode
	} else if room.Mode != mode {
		return Player{}, fmt.Errorf("房间已是%s模式，不能用%s模式加入", room.Mode, mode)
	}
	player := Player{
		ID:       socketID,
		Name:     name,
		Position: len(room.Players),
	}
	room.Players = append(room.Players, player)
	return player, nil
}

func (s *Store) AddAIPlayer(roomID string) (Player, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	room, ok := s.rooms[roomID]
	if !ok {
		return Player{}, errors.New("房间不存在")
	}
	if len(room.Players) >= MaxPlayers {
		return Player{}, errors.New("房间已满")
	}
	used := map[string]bool{}
	for _, player := range room.Players {
		used[player.Name] = true
	}
	names := []string{"AI_小红", "AI_小蓝", "AI_小绿", "AI_小紫", "AI_小橙", "AI_小黄", "AI_小青", "AI_小粉"}
	name := fmt.Sprintf("AI_%d", len(room.Players)+1)
	for _, candidate := range names {
		if !used[candidate] {
			name = candidate
			break
		}
	}

	player := Player{
		ID:       fmt.Sprintf("ai_%d_%d", time.Now().UnixNano(), rand.Intn(10000)),
		Name:     name,
		IsAI:     true,
		Position: len(room.Players),
	}
	room.Players = append(room.Players, player)
	return player, nil
}

func (s *Store) ResetRoom(roomID string) *Room {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.rooms, roomID)
	room := &Room{
		ID:             roomID,
		Players:        []Player{},
		Status:         StatusWaiting,
		Mode:           ModeNormal,
		Roles:          map[string]Role{},
		MissionResults: []bool{},
		MessageCount:   map[string]int{},
		ChatMessages:   []ChatMessage{},
		TeamVotes:      map[string]bool{},
		MissionVotes:   map[string]string{},
		VoteHistory:    []VoteRecord{},
		SignalHistory:  []SignalRecord{},
	}
	s.rooms[roomID] = room
	return cloneRoom(room)
}

func cloneRoom(room *Room) *Room {
	if room == nil {
		return nil
	}
	clone := *room
	clone.Players = append([]Player(nil), room.Players...)
	clone.MissionResults = append([]bool(nil), room.MissionResults...)
	clone.ProposedTeam = append([]string(nil), room.ProposedTeam...)
	clone.ChatMessages = append([]ChatMessage(nil), room.ChatMessages...)
	clone.VoteHistory = append([]VoteRecord(nil), room.VoteHistory...)
	clone.SignalHistory = append([]SignalRecord(nil), room.SignalHistory...)
	clone.Roles = copyMap(room.Roles)
	clone.MessageCount = copyMap(room.MessageCount)
	clone.TeamVotes = copyMap(room.TeamVotes)
	clone.MissionVotes = copyMap(room.MissionVotes)
	return &clone
}

func copyMap[K comparable, V any](input map[K]V) map[K]V {
	if input == nil {
		return nil
	}
	output := make(map[K]V, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
