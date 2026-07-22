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
		ID:                 roomID,
		Players:            []Player{},
		Status:             StatusWaiting,
		Mode:               ModeNormal,
		Roles:              map[string]Role{},
		MissionResults:     []bool{},
		MessageCount:       map[string]int{},
		ChatMessages:       []ChatMessage{},
		TeamVotes:          map[string]bool{},
		MissionVotes:       map[string]string{},
		AutoTeamVotes:      map[string]bool{},
		AutoMissionVotes:   map[string]bool{},
		VoteHistory:        []VoteRecord{},
		SignalHistory:      []SignalRecord{},
		NominationHistory:  []NominationRecord{},
		Stances:            map[int]map[string]StanceRecord{},
		SuspicionEvents:    []string{},
		DiscussionFocus:    []string{},
		AutoManagedActions: map[string][]AutoManagedAction{},
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

// JoinPlayer adds a new stable player identity or restores an existing seat.
func (s *Store) JoinPlayer(roomID, playerID, name string, mode Mode) (Player, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	room, ok := s.rooms[roomID]
	if !ok {
		return Player{}, false, errors.New("房间不存在")
	}
	if mode == "" {
		mode = ModeNormal
	}
	if room.Mode != mode && len(room.Players) > 0 {
		return Player{}, false, fmt.Errorf("房间已是%s模式，不能用%s模式加入", room.Mode, mode)
	}
	for i := range room.Players {
		if room.Players[i].ID != playerID {
			continue
		}
		room.Players[i].Disconnected = false
		if room.Status == StatusWaiting {
			room.Players[i].Name = name
		}
		return room.Players[i], true, nil
	}
	if room.Status != StatusWaiting {
		return Player{}, false, errors.New("游戏已开始")
	}
	if len(room.Players) >= MaxPlayers {
		return Player{}, false, errors.New("房间已满")
	}
	if len(room.Players) == 0 {
		room.Mode = mode
	}
	player := Player{
		ID:       playerID,
		Name:     name,
		Position: len(room.Players),
	}
	room.Players = append(room.Players, player)
	return player, false, nil
}

func (s *Store) SetPlayerDisconnected(roomID, playerID string, disconnected bool) (*Room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	room, ok := s.rooms[roomID]
	if !ok {
		return nil, errors.New("房间不存在")
	}
	for i := range room.Players {
		if room.Players[i].ID != playerID {
			continue
		}
		room.Players[i].Disconnected = disconnected
		return cloneRoom(room), nil
	}
	return nil, errors.New("玩家不在房间中")
}

// LeavePlayer permanently releases a waiting or finished-room seat. Active
// games keep their participant roster so result and reconnect state remain
// consistent.
func (s *Store) LeavePlayer(roomID, playerID string) (*Room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	room, ok := s.rooms[roomID]
	if !ok {
		return nil, errors.New("房间不存在")
	}
	if room.Status == StatusPlaying {
		return nil, errors.New("游戏进行中，暂不能退出席位")
	}
	playerIndex := -1
	for index := range room.Players {
		if room.Players[index].ID == playerID {
			playerIndex = index
			break
		}
	}
	if playerIndex < 0 {
		return nil, errors.New("玩家不在房间中")
	}
	room.Players = append(room.Players[:playerIndex], room.Players[playerIndex+1:]...)
	if len(room.Players) == 0 {
		delete(s.rooms, roomID)
		return nil, nil
	}
	for index := range room.Players {
		room.Players[index].Position = index
	}
	return cloneRoom(room), nil
}

// SetDebugPausedByHost pauses or resumes the active solo-debug countdown.
func (s *Store) SetDebugPausedByHost(roomID, playerID string, paused bool) (*Room, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	room, ok := s.rooms[roomID]
	if !ok {
		return nil, 0, errors.New("房间不存在")
	}
	hostID := ""
	for _, player := range room.Players {
		if player.IsAI || player.Eliminated || player.Disconnected {
			continue
		}
		hostID = player.ID
		break
	}
	if room.Mode != ModeSolo || hostID != playerID {
		return nil, 0, errors.New("只有单人调试房主可以暂停游戏")
	}
	if room.Status != StatusPlaying {
		return nil, 0, errors.New("当前没有进行中的调试对局")
	}

	if paused && !room.DebugPaused {
		room.DebugRemainingMS = max(0, room.PhaseDeadline-time.Now().UnixMilli())
		room.DebugPaused = true
		room.PhaseDeadline = 0
	}
	if !paused && room.DebugPaused {
		room.DebugPaused = false
		room.PhaseDeadline = time.Now().Add(time.Duration(room.DebugRemainingMS) * time.Millisecond).UnixMilli()
	}
	remainingSeconds := int((room.DebugRemainingMS + 999) / 1000)
	return cloneRoom(room), remainingSeconds, nil
}

// RefreshDebugPauseByHost captures the new deadline after a paused room is
// manually advanced into another phase.
func (s *Store) RefreshDebugPauseByHost(roomID, playerID string) (*Room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	room, ok := s.rooms[roomID]
	if !ok {
		return nil, errors.New("房间不存在")
	}
	hostID := ""
	for _, player := range room.Players {
		if player.IsAI || player.Eliminated || player.Disconnected {
			continue
		}
		hostID = player.ID
		break
	}
	if room.Mode != ModeSolo || hostID != playerID || !room.DebugPaused {
		return nil, errors.New("当前不是已暂停的单人调试房间")
	}
	room.DebugRemainingMS = max(0, room.PhaseDeadline-time.Now().UnixMilli())
	room.PhaseDeadline = 0
	return cloneRoom(room), nil
}

func (s *Store) ResetRoomByHost(roomID, playerID string) (*Room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	room, ok := s.rooms[roomID]
	if !ok {
		return nil, errors.New("房间不存在")
	}
	hostID := ""
	for _, player := range room.Players {
		if player.Eliminated || player.IsAI || player.Disconnected {
			continue
		}
		hostID = player.ID
		break
	}
	if hostID == "" || hostID != playerID {
		return nil, errors.New("只有当前房主可以重置房间")
	}

	reset := newRoom(roomID)
	s.rooms[roomID] = reset
	return cloneRoom(reset), nil
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
	return appendAIPlayer(room), nil
}

func (s *Store) FillWithAIByHost(roomID, playerID string, target int) (*Room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	room, ok := s.rooms[roomID]
	if !ok {
		return nil, errors.New("房间不存在")
	}
	if room.Status != StatusWaiting {
		return nil, errors.New("只能在等待阶段补位")
	}
	hostID := ""
	for _, player := range room.Players {
		if player.IsAI || player.Eliminated || player.Disconnected {
			continue
		}
		hostID = player.ID
		break
	}
	if hostID != playerID {
		return nil, errors.New("只有当前房主可以开启 AI 补位")
	}
	if target > MaxPlayers {
		target = MaxPlayers
	}
	for len(room.Players) < target {
		appendAIPlayer(room)
	}
	return cloneRoom(room), nil
}

func appendAIPlayer(room *Room) Player {
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
	return player
}

func (s *Store) PrepareRematchByHost(roomID, playerID string) (*Room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	room, ok := s.rooms[roomID]
	if !ok {
		return nil, errors.New("房间不存在")
	}
	hostID := ""
	for _, player := range room.Players {
		if player.IsAI || player.Eliminated || player.Disconnected {
			continue
		}
		hostID = player.ID
		break
	}
	if hostID != playerID {
		return nil, errors.New("只有当前房主可以发起再来一局")
	}
	mode := room.Mode
	players := make([]Player, 0, len(room.Players))
	for _, player := range room.Players {
		if player.IsAI || player.Disconnected {
			continue
		}
		player.Position = len(players)
		player.Eliminated = false
		players = append(players, player)
	}
	reset := newRoom(roomID)
	reset.Mode = mode
	reset.Players = players
	s.rooms[roomID] = reset
	return cloneRoom(reset), nil
}

func (s *Store) ResetRoom(roomID string) *Room {
	s.mu.Lock()
	defer s.mu.Unlock()

	room := newRoom(roomID)
	s.rooms[roomID] = room
	return cloneRoom(room)
}

func newRoom(roomID string) *Room {
	return &Room{
		ID:                 roomID,
		Players:            []Player{},
		Status:             StatusWaiting,
		Mode:               ModeNormal,
		Roles:              map[string]Role{},
		MissionResults:     []bool{},
		MessageCount:       map[string]int{},
		ChatMessages:       []ChatMessage{},
		TeamVotes:          map[string]bool{},
		MissionVotes:       map[string]string{},
		AutoTeamVotes:      map[string]bool{},
		AutoMissionVotes:   map[string]bool{},
		VoteHistory:        []VoteRecord{},
		SignalHistory:      []SignalRecord{},
		NominationHistory:  []NominationRecord{},
		Stances:            map[int]map[string]StanceRecord{},
		SuspicionEvents:    []string{},
		DiscussionFocus:    []string{},
		AutoManagedActions: map[string][]AutoManagedAction{},
	}
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
	clone.VoteHistory = cloneVoteHistory(room.VoteHistory)
	clone.SignalHistory = append([]SignalRecord(nil), room.SignalHistory...)
	clone.NominationHistory = cloneNominationHistory(room.NominationHistory)
	clone.SuspicionEvents = append([]string(nil), room.SuspicionEvents...)
	clone.DiscussionFocus = append([]string(nil), room.DiscussionFocus...)
	clone.AutoManagedActions = cloneAutoManagedActions(room.AutoManagedActions)
	clone.Stances = cloneStances(room.Stances)
	clone.Roles = copyMap(room.Roles)
	clone.MessageCount = copyMap(room.MessageCount)
	clone.TeamVotes = copyMap(room.TeamVotes)
	clone.MissionVotes = copyMap(room.MissionVotes)
	clone.AutoTeamVotes = copyMap(room.AutoTeamVotes)
	clone.AutoMissionVotes = copyMap(room.AutoMissionVotes)
	if room.DebugMissionResult != nil {
		debugResult := *room.DebugMissionResult
		clone.DebugMissionResult = &debugResult
	}
	return &clone
}

func cloneAutoManagedActions(input map[string][]AutoManagedAction) map[string][]AutoManagedAction {
	if input == nil {
		return nil
	}
	output := make(map[string][]AutoManagedAction, len(input))
	for playerID, actions := range input {
		output[playerID] = append([]AutoManagedAction(nil), actions...)
	}
	return output
}

func cloneVoteHistory(input []VoteRecord) []VoteRecord {
	if input == nil {
		return nil
	}
	output := make([]VoteRecord, len(input))
	for index, record := range input {
		output[index] = record
		output[index].Votes = copyMap(record.Votes)
		output[index].Team = append([]string(nil), record.Team...)
	}
	return output
}

func cloneNominationHistory(input []NominationRecord) []NominationRecord {
	if input == nil {
		return nil
	}
	output := make([]NominationRecord, len(input))
	for index, record := range input {
		output[index] = record
		output[index].Team = append([]string(nil), record.Team...)
		output[index].TeamNames = append([]string(nil), record.TeamNames...)
	}
	return output
}

func cloneStances(input map[int]map[string]StanceRecord) map[int]map[string]StanceRecord {
	if input == nil {
		return nil
	}
	output := make(map[int]map[string]StanceRecord, len(input))
	for round, records := range input {
		output[round] = copyMap(records)
	}
	return output
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
