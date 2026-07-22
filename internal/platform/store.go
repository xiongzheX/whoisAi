package platform

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

type CreateRoomInput struct {
	Code     string
	HostID   string
	HostName string
	GameID   string
}

type ParticipantInput struct {
	ParticipantKey string
	MemberID       string
	DisplayName    string
	Kind           ParticipantKind
	Seat           int
}

type StartSessionInput struct {
	RoomCode     string
	HostID       string
	Mode         string
	Settings     json.RawMessage
	Participants []ParticipantInput
}

const OfflineSeatGrace = 90 * time.Second

// MemoryStore mirrors the production relational model while the application
// still runs without a database. Returned values are defensive copies.
type MemoryStore struct {
	mu          sync.RWMutex
	registry    *Registry
	roomsByCode map[string]*PartyRoom
	roomsByID   map[string]*PartyRoom
	sessions    map[string]*GameSession
	snapshots   map[string]*Snapshot
	events      map[string][]EventRecord
	actionIDs   map[string]map[string]struct{}
	nextEventID uint64
}

func NewMemoryStore(registry *Registry) *MemoryStore {
	if registry == nil {
		registry = DefaultRegistry()
	}
	return &MemoryStore{
		registry:    registry,
		roomsByCode: make(map[string]*PartyRoom),
		roomsByID:   make(map[string]*PartyRoom),
		sessions:    make(map[string]*GameSession),
		snapshots:   make(map[string]*Snapshot),
		events:      make(map[string][]EventRecord),
		actionIDs:   make(map[string]map[string]struct{}),
	}
}

func (s *MemoryStore) CreateRoom(input CreateRoomInput) (*PartyRoom, error) {
	if err := s.validateCreateRoom(input); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createRoomLocked(input)
}

// CreateGeneratedRoom atomically reserves a short, shareable room code. The
// generated code is only an identifier; callers still own game-specific setup.
func (s *MemoryStore) CreateGeneratedRoom(hostID, hostName, gameID string) (*PartyRoom, error) {
	input := CreateRoomInput{HostID: hostID, HostName: hostName, GameID: gameID}
	if err := s.validateCreateRoom(CreateRoomInput{
		Code: "generated", HostID: hostID, HostName: hostName, GameID: gameID,
	}); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for attempts := 0; attempts < 32; attempts++ {
		input.Code = generatedRoomCode(gameID)
		if _, exists := s.roomsByCode[input.Code]; exists {
			continue
		}
		return s.createRoomLocked(input)
	}
	return nil, errors.New("暂时无法生成房间码，请稍后再试")
}

func (s *MemoryStore) validateCreateRoom(input CreateRoomInput) error {
	if input.Code == "" || input.HostID == "" || input.HostName == "" {
		return errors.New("房间码、房主和昵称不能为空")
	}
	game, ok := s.registry.Game(input.GameID)
	if !ok {
		return ErrGameNotFound
	}
	if game.Status != GameActive {
		return ErrGameUnavailable
	}
	return nil
}

func (s *MemoryStore) createRoomLocked(input CreateRoomInput) (*PartyRoom, error) {
	if _, exists := s.roomsByCode[input.Code]; exists {
		return nil, ErrRoomExists
	}

	now := time.Now().UTC()
	room := &PartyRoom{
		ID:             newID("room"),
		Code:           input.Code,
		HostMemberID:   input.HostID,
		SelectedGameID: input.GameID,
		Status:         RoomOpen,
		Version:        1,
		Members: []RoomMember{{
			ID:               input.HostID,
			DisplayName:      input.HostName,
			Seat:             0,
			Role:             MemberHost,
			ConnectionStatus: MemberOnline,
			JoinedAt:         now,
			LastSeenAt:       now,
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.roomsByCode[room.Code] = room
	s.roomsByID[room.ID] = room
	return cloneRoom(room), nil
}

func (s *MemoryStore) RoomByCode(code string) (*PartyRoom, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	room, ok := s.roomsByCode[code]
	if !ok {
		return nil, false
	}
	return cloneRoom(room), true
}

func (s *MemoryStore) WaitingRooms(gameID string) ([]WaitingRoomSummary, error) {
	game, ok := s.registry.Game(gameID)
	if !ok {
		return nil, ErrGameNotFound
	}
	if game.Status != GameActive {
		return nil, ErrGameUnavailable
	}

	s.ExpireOfflineMembers(time.Now().UTC(), OfflineSeatGrace)
	s.mu.RLock()
	defer s.mu.RUnlock()

	rooms := make([]WaitingRoomSummary, 0)
	for _, room := range s.roomsByCode {
		if room.SelectedGameID != gameID || room.Status != RoomOpen || room.ActiveSessionID != "" {
			continue
		}

		playerCount := 0
		onlineCount := 0
		hostName := ""
		for _, member := range room.Members {
			if member.ConnectionStatus == MemberLeft {
				continue
			}
			playerCount++
			if member.ConnectionStatus == MemberOnline {
				onlineCount++
			}
			if member.ID == room.HostMemberID {
				hostName = member.DisplayName
			}
		}
		if onlineCount == 0 || playerCount >= game.MaxPlayers {
			continue
		}

		rooms = append(rooms, WaitingRoomSummary{
			Code:        room.Code,
			GameID:      room.SelectedGameID,
			HostName:    hostName,
			PlayerCount: playerCount,
			MaxPlayers:  game.MaxPlayers,
			OpenSeats:   game.MaxPlayers - playerCount,
			UpdatedAt:   room.UpdatedAt,
		})
	}
	sort.Slice(rooms, func(i, j int) bool {
		if !rooms[i].UpdatedAt.Equal(rooms[j].UpdatedAt) {
			return rooms[i].UpdatedAt.After(rooms[j].UpdatedAt)
		}
		return rooms[i].Code < rooms[j].Code
	})
	return rooms, nil
}

// LeaveRoom releases a seat immediately. A nil room means the final member
// left and the empty room was removed.
func (s *MemoryStore) LeaveRoom(code, memberID string) (*PartyRoom, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	room, ok := s.roomsByCode[code]
	if !ok {
		return nil, ErrRoomNotFound
	}
	if room.ActiveSessionID != "" {
		return nil, ErrActiveSession
	}

	memberIndex := -1
	for index := range room.Members {
		if room.Members[index].ID == memberID {
			memberIndex = index
			break
		}
	}
	if memberIndex < 0 {
		return nil, ErrMemberNotFound
	}
	room.Members = append(room.Members[:memberIndex], room.Members[memberIndex+1:]...)
	if len(room.Members) == 0 {
		delete(s.roomsByCode, room.Code)
		delete(s.roomsByID, room.ID)
		return nil, nil
	}
	if room.HostMemberID == memberID {
		promoteHost(room)
	}
	touchRoom(room, time.Now().UTC())
	return cloneRoom(room), nil
}

// ExpireOfflineMembers reclaims seats abandoned after an ungraceful browser
// close. Active matches are left untouched so a brief disconnect can resume.
func (s *MemoryStore) ExpireOfflineMembers(now time.Time, grace time.Duration) int {
	if grace < 0 {
		grace = 0
	}
	cutoff := now.Add(-grace)
	expired := 0

	s.mu.Lock()
	defer s.mu.Unlock()
	for code, room := range s.roomsByCode {
		if room.ActiveSessionID != "" {
			continue
		}
		originalCount := len(room.Members)
		kept := room.Members[:0]
		hostExpired := false
		for _, member := range room.Members {
			if member.ConnectionStatus == MemberOffline && !member.LastSeenAt.After(cutoff) {
				expired++
				hostExpired = hostExpired || member.ID == room.HostMemberID
				continue
			}
			kept = append(kept, member)
		}
		room.Members = kept
		if len(room.Members) == 0 {
			delete(s.roomsByCode, code)
			delete(s.roomsByID, room.ID)
			continue
		}
		if hostExpired {
			promoteHost(room)
		}
		if hostExpired || len(room.Members) != originalCount {
			touchRoom(room, now)
		}
	}
	return expired
}

func (s *MemoryStore) JoinRoom(code, memberID, displayName string) (*PartyRoom, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	room, ok := s.roomsByCode[code]
	if !ok {
		return nil, false, ErrRoomNotFound
	}
	if room.Status == RoomClosed {
		return nil, false, errors.New("房间已关闭")
	}

	now := time.Now().UTC()
	for index := range room.Members {
		member := &room.Members[index]
		if member.ID != memberID {
			continue
		}
		member.DisplayName = displayName
		member.ConnectionStatus = MemberOnline
		member.LastSeenAt = now
		touchRoom(room, now)
		return cloneRoom(room), true, nil
	}
	if room.ActiveSessionID != "" {
		return nil, false, ErrActiveSession
	}
	game, ok := s.registry.Game(room.SelectedGameID)
	if !ok {
		return nil, false, ErrGameNotFound
	}
	activeMembers := 0
	for _, member := range room.Members {
		if member.ConnectionStatus != MemberLeft {
			activeMembers++
		}
	}
	if activeMembers >= game.MaxPlayers {
		return nil, false, ErrRoomFull
	}

	room.Members = append(room.Members, RoomMember{
		ID:               memberID,
		DisplayName:      displayName,
		Seat:             nextSeat(room.Members),
		Role:             MemberPlayer,
		ConnectionStatus: MemberOnline,
		JoinedAt:         now,
		LastSeenAt:       now,
	})
	touchRoom(room, now)
	return cloneRoom(room), false, nil
}

func (s *MemoryStore) SetConnection(code, memberID string, status ConnectionStatus) (*PartyRoom, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	room, ok := s.roomsByCode[code]
	if !ok {
		return nil, ErrRoomNotFound
	}
	now := time.Now().UTC()
	for index := range room.Members {
		if room.Members[index].ID != memberID {
			continue
		}
		room.Members[index].ConnectionStatus = status
		room.Members[index].LastSeenAt = now
		touchRoom(room, now)
		return cloneRoom(room), nil
	}
	return nil, ErrMemberNotFound
}

func (s *MemoryStore) SelectGame(code, hostID, gameID string) (*PartyRoom, error) {
	game, ok := s.registry.Game(gameID)
	if !ok {
		return nil, ErrGameNotFound
	}
	if game.Status != GameActive {
		return nil, ErrGameUnavailable
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	room, ok := s.roomsByCode[code]
	if !ok {
		return nil, ErrRoomNotFound
	}
	if room.HostMemberID != hostID {
		return nil, ErrNotHost
	}
	if room.ActiveSessionID != "" {
		return nil, ErrActiveSession
	}
	activeMembers := 0
	for _, member := range room.Members {
		if member.ConnectionStatus != MemberLeft {
			activeMembers++
		}
	}
	if activeMembers > game.MaxPlayers {
		return nil, fmt.Errorf("%s 最多支持 %d 名玩家，当前房间有 %d 人", game.Name, game.MaxPlayers, activeMembers)
	}
	room.SelectedGameID = gameID
	touchRoom(room, time.Now().UTC())
	return cloneRoom(room), nil
}

func (s *MemoryStore) StartSession(input StartSessionInput) (*GameSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	room, ok := s.roomsByCode[input.RoomCode]
	if !ok {
		return nil, ErrRoomNotFound
	}
	if room.HostMemberID != input.HostID {
		return nil, ErrNotHost
	}
	if room.ActiveSessionID != "" {
		return nil, ErrActiveSession
	}
	game, ok := s.registry.Game(room.SelectedGameID)
	if !ok {
		return nil, ErrGameNotFound
	}
	if game.Status != GameActive {
		return nil, ErrGameUnavailable
	}
	if len(input.Participants) < game.MinPlayers || len(input.Participants) > game.MaxPlayers {
		return nil, fmt.Errorf("%s 需要 %d-%d 名参与者", game.Name, game.MinPlayers, game.MaxPlayers)
	}
	if len(input.Settings) > 0 && !json.Valid(input.Settings) {
		return nil, errors.New("游戏设置不是有效 JSON")
	}
	participants, err := buildParticipants(input.Participants)
	if err != nil {
		return nil, err
	}
	members := make(map[string]struct{}, len(room.Members))
	for _, member := range room.Members {
		if member.ConnectionStatus != MemberLeft {
			members[member.ID] = struct{}{}
		}
	}
	for _, participant := range participants {
		if participant.Kind != ParticipantHuman {
			continue
		}
		if _, exists := members[participant.MemberID]; !exists {
			return nil, fmt.Errorf("真人参与者 %q 不在房间中", participant.ParticipantKey)
		}
	}

	now := time.Now().UTC()
	sequence := s.nextSequence(room.ID)
	session := &GameSession{
		ID:           newID("session"),
		RoomID:       room.ID,
		RoomCode:     room.Code,
		GameID:       room.SelectedGameID,
		Sequence:     sequence,
		Status:       SessionRunning,
		Mode:         input.Mode,
		Settings:     cloneJSON(input.Settings),
		Participants: participants,
		CreatedAt:    now,
		StartedAt:    timePointer(now),
	}
	s.sessions[session.ID] = session
	room.ActiveSessionID = session.ID
	room.Status = RoomInGame
	touchRoom(room, now)
	return cloneSession(session), nil
}

func (s *MemoryStore) Session(id string) (*GameSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[id]
	if !ok {
		return nil, false
	}
	return cloneSession(session), true
}

func (s *MemoryStore) ActiveSession(code string) (*GameSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	room, ok := s.roomsByCode[code]
	if !ok || room.ActiveSessionID == "" {
		return nil, false
	}
	return cloneSession(s.sessions[room.ActiveSessionID]), true
}

func (s *MemoryStore) FinishSession(code string, summary json.RawMessage) (*GameSession, error) {
	return s.endSession(code, SessionFinished, summary)
}

func (s *MemoryStore) AbandonSession(code string) (*GameSession, error) {
	return s.endSession(code, SessionAbandoned, nil)
}

func (s *MemoryStore) CommitSnapshot(
	sessionID string,
	expectedVersion uint64,
	publicState json.RawMessage,
	privateState map[string]json.RawMessage,
) (*Snapshot, error) {
	if !json.Valid(publicState) {
		return nil, errors.New("公共状态不是有效 JSON")
	}
	for participantID, state := range privateState {
		if !json.Valid(state) {
			return nil, fmt.Errorf("参与者 %q 的私密状态不是有效 JSON", participantID)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}
	if session.StateVersion != expectedVersion {
		return nil, ErrVersionConflict
	}
	if session.Status != SessionRunning && session.Status != SessionConfirming {
		return nil, errors.New("游戏会话不接受新快照")
	}

	now := time.Now().UTC()
	snapshot := &Snapshot{
		SessionID:    sessionID,
		Version:      expectedVersion + 1,
		PublicState:  cloneJSON(publicState),
		PrivateState: clonePrivateState(privateState),
		ServerTime:   now,
		CreatedAt:    now,
	}
	session.StateVersion = snapshot.Version
	s.snapshots[sessionID] = snapshot
	return cloneSnapshot(snapshot), nil
}

func (s *MemoryStore) LatestSnapshot(sessionID, participantID string) (*Snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot, ok := s.snapshots[sessionID]
	if !ok {
		return nil, false
	}
	result := cloneSnapshot(snapshot)
	if participantID == "" {
		result.PrivateState = nil
		return result, true
	}
	state, exists := result.PrivateState[participantID]
	result.PrivateState = nil
	if exists {
		result.PrivateState = map[string]json.RawMessage{participantID: state}
	}
	return result, true
}

func (s *MemoryStore) AppendEvent(event EventRecord) (EventRecord, error) {
	if event.SessionID == "" || event.Type == "" {
		return EventRecord{}, errors.New("事件会话和类型不能为空")
	}
	if len(event.Payload) > 0 && !json.Valid(event.Payload) {
		return EventRecord{}, errors.New("事件载荷不是有效 JSON")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[event.SessionID]; !ok {
		return EventRecord{}, ErrSessionNotFound
	}
	if event.ActionID != "" {
		key := event.ActorParticipantID + "\x00" + event.ActionID
		ids := s.actionIDs[event.SessionID]
		if ids == nil {
			ids = make(map[string]struct{})
			s.actionIDs[event.SessionID] = ids
		}
		if _, exists := ids[key]; exists {
			return EventRecord{}, ErrDuplicateAction
		}
		ids[key] = struct{}{}
	}
	s.nextEventID++
	event.ID = s.nextEventID
	event.Payload = cloneJSON(event.Payload)
	event.CreatedAt = time.Now().UTC()
	s.events[event.SessionID] = append(s.events[event.SessionID], event)
	return event, nil
}

func (s *MemoryStore) Events(sessionID string) []EventRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	records := s.events[sessionID]
	result := make([]EventRecord, len(records))
	for index, record := range records {
		result[index] = record
		result[index].Payload = cloneJSON(record.Payload)
	}
	return result
}

func (s *MemoryStore) endSession(
	code string,
	status SessionStatus,
	summary json.RawMessage,
) (*GameSession, error) {
	if len(summary) > 0 && !json.Valid(summary) {
		return nil, errors.New("结算摘要不是有效 JSON")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	room, ok := s.roomsByCode[code]
	if !ok {
		return nil, ErrRoomNotFound
	}
	if room.ActiveSessionID == "" {
		return nil, ErrSessionNotFound
	}
	session := s.sessions[room.ActiveSessionID]
	now := time.Now().UTC()
	session.Status = status
	session.EndedAt = timePointer(now)
	session.ResultSummary = cloneJSON(summary)
	room.ActiveSessionID = ""
	room.Status = RoomOpen
	touchRoom(room, now)
	return cloneSession(session), nil
}

func (s *MemoryStore) nextSequence(roomID string) uint64 {
	var sequence uint64
	for _, session := range s.sessions {
		if session.RoomID == roomID && session.Sequence > sequence {
			sequence = session.Sequence
		}
	}
	return sequence + 1
}

func buildParticipants(inputs []ParticipantInput) ([]SessionParticipant, error) {
	participants := make([]SessionParticipant, 0, len(inputs))
	keys := make(map[string]struct{}, len(inputs))
	seats := make(map[int]struct{}, len(inputs))
	for _, input := range inputs {
		if input.ParticipantKey == "" || input.DisplayName == "" {
			return nil, errors.New("参与者标识和昵称不能为空")
		}
		if input.Kind != ParticipantHuman && input.Kind != ParticipantBot {
			return nil, fmt.Errorf("参与者 %q 类型无效", input.ParticipantKey)
		}
		if input.Kind == ParticipantHuman && input.MemberID == "" {
			return nil, fmt.Errorf("真人参与者 %q 缺少房间成员标识", input.ParticipantKey)
		}
		if _, exists := keys[input.ParticipantKey]; exists {
			return nil, fmt.Errorf("参与者标识 %q 重复", input.ParticipantKey)
		}
		if _, exists := seats[input.Seat]; exists {
			return nil, fmt.Errorf("参与者座位 %d 重复", input.Seat)
		}
		keys[input.ParticipantKey] = struct{}{}
		seats[input.Seat] = struct{}{}
		participants = append(participants, SessionParticipant{
			ID:             newID("participant"),
			MemberID:       input.MemberID,
			DisplayName:    input.DisplayName,
			Kind:           input.Kind,
			Seat:           input.Seat,
			ParticipantKey: input.ParticipantKey,
		})
	}
	return participants, nil
}

func touchRoom(room *PartyRoom, now time.Time) {
	room.Version++
	room.UpdatedAt = now
}

func nextSeat(members []RoomMember) int {
	used := make(map[int]struct{}, len(members))
	for _, member := range members {
		used[member.Seat] = struct{}{}
	}
	for seat := 0; ; seat++ {
		if _, exists := used[seat]; !exists {
			return seat
		}
	}
}

func promoteHost(room *PartyRoom) {
	if len(room.Members) == 0 {
		room.HostMemberID = ""
		return
	}
	nextHost := 0
	for index := 1; index < len(room.Members); index++ {
		current := room.Members[index]
		candidate := room.Members[nextHost]
		if current.ConnectionStatus == MemberOnline && candidate.ConnectionStatus != MemberOnline ||
			current.ConnectionStatus == candidate.ConnectionStatus && current.JoinedAt.Before(candidate.JoinedAt) {
			nextHost = index
		}
	}
	for index := range room.Members {
		room.Members[index].Role = MemberPlayer
	}
	room.Members[nextHost].Role = MemberHost
	room.HostMemberID = room.Members[nextHost].ID
}

func generatedRoomCode(gameID string) string {
	prefixes := map[string]string{
		"who-is-ai":     "party",
		"bean-sprint":   "race",
		"dumpling-sumo": "sumo",
	}
	prefix := prefixes[gameID]
	if prefix == "" {
		prefix = "room"
	}
	var data [3]byte
	if _, err := rand.Read(data[:]); err != nil {
		panic(fmt.Sprintf("generate room code: %v", err))
	}
	return fmt.Sprintf("%s-%X", prefix, data)
}

// GenerateRoomCode returns a human-shareable candidate. CreateRoom remains the
// authority that detects the unlikely collision.
func GenerateRoomCode(gameID string) string {
	return generatedRoomCode(gameID)
}

func newID(prefix string) string {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		panic(fmt.Sprintf("generate %s id: %v", prefix, err))
	}
	return prefix + "_" + hex.EncodeToString(data[:])
}

func cloneRoom(room *PartyRoom) *PartyRoom {
	if room == nil {
		return nil
	}
	clone := *room
	clone.Members = append([]RoomMember(nil), room.Members...)
	return &clone
}

func cloneSession(session *GameSession) *GameSession {
	if session == nil {
		return nil
	}
	clone := *session
	clone.Settings = cloneJSON(session.Settings)
	clone.ResultSummary = cloneJSON(session.ResultSummary)
	clone.Participants = append([]SessionParticipant(nil), session.Participants...)
	if session.StartedAt != nil {
		clone.StartedAt = timePointer(*session.StartedAt)
	}
	if session.EndedAt != nil {
		clone.EndedAt = timePointer(*session.EndedAt)
	}
	return &clone
}

func cloneSnapshot(snapshot *Snapshot) *Snapshot {
	if snapshot == nil {
		return nil
	}
	clone := *snapshot
	clone.PublicState = cloneJSON(snapshot.PublicState)
	clone.PrivateState = clonePrivateState(snapshot.PrivateState)
	return &clone
}

func clonePrivateState(input map[string]json.RawMessage) map[string]json.RawMessage {
	if input == nil {
		return nil
	}
	output := make(map[string]json.RawMessage, len(input))
	for id, state := range input {
		output[id] = cloneJSON(state)
	}
	return output
}

func cloneJSON(input json.RawMessage) json.RawMessage {
	return append(json.RawMessage(nil), input...)
}

func timePointer(value time.Time) *time.Time {
	return &value
}
