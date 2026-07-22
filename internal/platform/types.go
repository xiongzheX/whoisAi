// Package platform owns the shared lifecycle of social rooms and game sessions.
// Individual game rules belong in their own packages and reference these types.
package platform

import (
	"encoding/json"
	"errors"
	"time"
)

type GameStatus string

const (
	GameActive     GameStatus = "active"
	GameComingSoon GameStatus = "coming_soon"
	GameDisabled   GameStatus = "disabled"
)

type GameDefinition struct {
	ID          string          `json:"id"`
	Slug        string          `json:"slug"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Status      GameStatus      `json:"status"`
	MinPlayers  int             `json:"minPlayers"`
	MaxPlayers  int             `json:"maxPlayers"`
	SupportsAI  bool            `json:"supportsAI"`
	Route       string          `json:"route"`
	Tags        []string        `json:"tags"`
	Manifest    json.RawMessage `json:"manifest,omitempty"`
	SortOrder   int             `json:"sortOrder"`
}

type RoomStatus string

const (
	RoomOpen   RoomStatus = "open"
	RoomInGame RoomStatus = "in_game"
	RoomClosed RoomStatus = "closed"
)

type MemberRole string

const (
	MemberHost   MemberRole = "host"
	MemberPlayer MemberRole = "player"
)

type ConnectionStatus string

const (
	MemberOnline  ConnectionStatus = "online"
	MemberOffline ConnectionStatus = "offline"
	MemberLeft    ConnectionStatus = "left"
)

type RoomMember struct {
	ID               string           `json:"id"`
	DisplayName      string           `json:"displayName"`
	Seat             int              `json:"seat"`
	Role             MemberRole       `json:"role"`
	ConnectionStatus ConnectionStatus `json:"connectionStatus"`
	Ready            bool             `json:"ready"`
	JoinedAt         time.Time        `json:"joinedAt"`
	LastSeenAt       time.Time        `json:"lastSeenAt"`
}

type PartyRoom struct {
	ID              string       `json:"id"`
	Code            string       `json:"code"`
	HostMemberID    string       `json:"hostMemberId"`
	SelectedGameID  string       `json:"selectedGameId"`
	ActiveSessionID string       `json:"activeSessionId,omitempty"`
	Status          RoomStatus   `json:"status"`
	Version         uint64       `json:"version"`
	Members         []RoomMember `json:"members"`
	CreatedAt       time.Time    `json:"createdAt"`
	UpdatedAt       time.Time    `json:"updatedAt"`
}

// WaitingRoomSummary is the public, joinable view used by the platform lobby.
// It intentionally excludes member IDs and resume credentials.
type WaitingRoomSummary struct {
	Code        string    `json:"code"`
	GameID      string    `json:"gameId"`
	HostName    string    `json:"hostName"`
	PlayerCount int       `json:"playerCount"`
	MaxPlayers  int       `json:"maxPlayers"`
	OpenSeats   int       `json:"openSeats"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type SessionStatus string

const (
	SessionCreated    SessionStatus = "created"
	SessionConfirming SessionStatus = "confirming"
	SessionRunning    SessionStatus = "running"
	SessionFinished   SessionStatus = "finished"
	SessionAbandoned  SessionStatus = "abandoned"
)

type ParticipantKind string

const (
	ParticipantHuman ParticipantKind = "human"
	ParticipantBot   ParticipantKind = "bot"
)

type SessionParticipant struct {
	ID             string          `json:"id"`
	MemberID       string          `json:"memberId,omitempty"`
	DisplayName    string          `json:"displayName"`
	Kind           ParticipantKind `json:"kind"`
	Seat           int             `json:"seat"`
	ParticipantKey string          `json:"participantKey"`
}

type GameSession struct {
	ID            string               `json:"id"`
	RoomID        string               `json:"roomId"`
	RoomCode      string               `json:"roomCode"`
	GameID        string               `json:"gameId"`
	Sequence      uint64               `json:"sequence"`
	Status        SessionStatus        `json:"status"`
	Mode          string               `json:"mode"`
	Settings      json.RawMessage      `json:"settings"`
	StateVersion  uint64               `json:"stateVersion"`
	Participants  []SessionParticipant `json:"participants"`
	ResultSummary json.RawMessage      `json:"resultSummary,omitempty"`
	CreatedAt     time.Time            `json:"createdAt"`
	StartedAt     *time.Time           `json:"startedAt,omitempty"`
	EndedAt       *time.Time           `json:"endedAt,omitempty"`
}

type Snapshot struct {
	SessionID    string                     `json:"sessionId"`
	Version      uint64                     `json:"version"`
	PublicState  json.RawMessage            `json:"publicState"`
	PrivateState map[string]json.RawMessage `json:"privateState,omitempty"`
	ServerTime   time.Time                  `json:"serverTime"`
	CreatedAt    time.Time                  `json:"createdAt"`
}

type EventScope string

const (
	EventPublic  EventScope = "public"
	EventPrivate EventScope = "private"
	EventSystem  EventScope = "system"
)

type EventRecord struct {
	ID                  uint64          `json:"id"`
	SessionID           string          `json:"sessionId"`
	Version             uint64          `json:"version,omitempty"`
	Round               int             `json:"round,omitempty"`
	ActorParticipantID  string          `json:"actorParticipantId,omitempty"`
	ActionID            string          `json:"actionId,omitempty"`
	Type                string          `json:"type"`
	Scope               EventScope      `json:"scope"`
	TargetParticipantID string          `json:"targetParticipantId,omitempty"`
	Payload             json.RawMessage `json:"payload"`
	CreatedAt           time.Time       `json:"createdAt"`
}

var (
	ErrGameNotFound    = errors.New("游戏不存在")
	ErrGameUnavailable = errors.New("游戏暂不可用")
	ErrRoomExists      = errors.New("房间已存在")
	ErrRoomNotFound    = errors.New("房间不存在")
	ErrRoomFull        = errors.New("房间已满")
	ErrMemberNotFound  = errors.New("房间成员不存在")
	ErrNotHost         = errors.New("只有房主可以执行该操作")
	ErrActiveSession   = errors.New("房间已有进行中的游戏")
	ErrSessionNotFound = errors.New("游戏会话不存在")
	ErrVersionConflict = errors.New("状态版本冲突")
	ErrDuplicateAction = errors.New("动作已处理")
)
