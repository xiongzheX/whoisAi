package realtime

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	siosocket "github.com/zishang520/socket.io/socket"

	"whoisai/internal/duel"
	"whoisai/internal/game"
	"whoisai/internal/platform"
)

type Server struct {
	io       *siosocket.Server
	store    *game.Store
	service  *game.Service
	platform *platform.MemoryStore
	registry *platform.Registry
	duels    *duel.Service
	mu       sync.Mutex
	session  map[string]string
}

type joinRoomPayload struct {
	RoomID      string `json:"roomId"`
	Name        string `json:"name"`
	Mode        string `json:"mode"`
	PlayerToken string `json:"playerToken"`
	TestMode    bool   `json:"testMode"`
	CreateNew   bool   `json:"createNew"`
}

type joinDuelRoomPayload struct {
	RoomID      string `json:"roomId"`
	Name        string `json:"name"`
	GameID      string `json:"gameId"`
	PlayerToken string `json:"playerToken"`
	CreateNew   bool   `json:"createNew"`
}

type duelReadyPayload struct {
	RoomID string          `json:"roomId"`
	GameID string          `json:"gameId"`
	Config json.RawMessage `json:"config"`
}

type startGamePayload struct {
	RoomID     string `json:"roomId"`
	FillWithAI bool   `json:"fillWithAI"`
}

type proposeMissionPayload struct {
	RoomID    string   `json:"roomId"`
	SessionID string   `json:"sessionId"`
	MemberIDs []string `json:"memberIds"`
	Reason    string   `json:"reason"`
}

type teamVotePayload struct {
	RoomID    string `json:"roomId"`
	SessionID string `json:"sessionId"`
	Approve   bool   `json:"approve"`
}

type chatPayload struct {
	RoomID    string `json:"roomId"`
	SessionID string `json:"sessionId"`
	Message   string `json:"message"`
}

func New(io *siosocket.Server, store *game.Store, service *game.Service) *Server {
	registry := platform.DefaultRegistry()
	return NewWithPlatform(io, store, service, platform.NewMemoryStore(registry), registry)
}

func NewWithPlatform(
	io *siosocket.Server,
	store *game.Store,
	service *game.Service,
	platformStore *platform.MemoryStore,
	registry *platform.Registry,
) *Server {
	return &Server{
		io:       io,
		store:    store,
		service:  service,
		platform: platformStore,
		registry: registry,
		duels:    duel.NewService(),
		session:  make(map[string]string),
	}
}

func (s *Server) Register() {
	s.io.On("connection", func(clients ...any) {
		client := clients[0].(*siosocket.Socket)
		clientID := string(client.Id())
		joinedRoomID := ""
		joinedPlayerID := ""
		joinedGameID := ""
		log.Printf("玩家连接: %s", clientID)

		client.On("joinRoom", func(datas ...any) {
			var payload joinRoomPayload
			if err := decodePayload(datas, &payload); err != nil {
				emitError(client, "加入房间参数错误")
				return
			}
			roomID, playerID, ok := s.handleJoinRoom(client, payload)
			if ok {
				joinedRoomID = roomID
				joinedPlayerID = playerID
				joinedGameID = "who-is-ai"
			}
		})

		client.On("joinDuelRoom", func(datas ...any) {
			var payload joinDuelRoomPayload
			if err := decodePayload(datas, &payload); err != nil {
				emitError(client, "加入双人房间参数错误")
				return
			}
			roomID, playerID, ok := s.handleJoinDuelRoom(client, payload)
			if ok {
				joinedRoomID = roomID
				joinedPlayerID = playerID
				joinedGameID = payload.GameID
			}
		})

		client.On("duelReady", func(datas ...any) {
			var payload duelReadyPayload
			if err := decodePayload(datas, &payload); err != nil {
				emitError(client, "准备参数错误")
				return
			}
			if payload.RoomID != joinedRoomID || payload.GameID != joinedGameID || !isDuelGame(payload.GameID) {
				emitError(client, "当前连接不属于该双人房间")
				return
			}
			room, ok := s.platform.RoomByCode(payload.RoomID)
			if !ok || room.SelectedGameID != payload.GameID {
				emitError(client, "房间游戏不匹配")
				return
			}
			state, allReady, err := s.duels.Ready(room, joinedPlayerID, payload.Config)
			if err != nil {
				emitError(client, err.Error())
				return
			}
			s.emitDuelState(payload.RoomID, state)
			if allReady {
				if err := s.startDuel(room); err != nil {
					emitError(client, err.Error())
				}
			}
		})

		client.On("duelCancelReady", func(datas ...any) {
			var payload duelReadyPayload
			if err := decodePayload(datas, &payload); err != nil {
				emitError(client, "取消准备参数错误")
				return
			}
			if payload.RoomID != joinedRoomID || payload.GameID != joinedGameID || !isDuelGame(payload.GameID) {
				emitError(client, "当前连接不属于该双人房间")
				return
			}
			room, ok := s.platform.RoomByCode(payload.RoomID)
			if !ok {
				emitError(client, platform.ErrRoomNotFound.Error())
				return
			}
			state, err := s.duels.CancelReady(room, joinedPlayerID)
			if err != nil {
				emitError(client, err.Error())
				return
			}
			s.emitDuelState(payload.RoomID, state)
		})

		client.On("leaveRoom", func(...any) {
			if joinedRoomID == "" || joinedPlayerID == "" {
				_ = client.Emit("roomLeft", map[string]any{"success": true})
				return
			}
			oldRoomID := joinedRoomID
			oldPlayerID := joinedPlayerID
			oldGameID := joinedGameID
			if oldGameID == "who-is-ai" {
				gameRoom, err := s.store.LeavePlayer(oldRoomID, oldPlayerID)
				if err != nil {
					emitError(client, err.Error())
					return
				}
				if gameRoom != nil {
					s.broadcastPlayerJoined(oldRoomID, gameRoom)
				}
			}
			partyRoom, err := s.platform.LeaveRoom(oldRoomID, oldPlayerID)
			if err != nil {
				emitError(client, err.Error())
				return
			}
			if isDuelGame(oldGameID) {
				s.duels.RemovePlayer(oldRoomID, oldPlayerID)
			}
			s.mu.Lock()
			if s.session[oldPlayerID] == clientID {
				delete(s.session, oldPlayerID)
			}
			s.mu.Unlock()
			client.Leave(siosocket.Room(oldRoomID))
			client.Leave(siosocket.Room(oldPlayerID))
			joinedRoomID = ""
			joinedPlayerID = ""
			joinedGameID = ""
			if partyRoom != nil {
				_ = s.io.To(siosocket.Room(oldRoomID)).Emit("partyState", map[string]any{"room": partyRoom})
				if isDuelGame(oldGameID) {
					s.emitDuelState(oldRoomID, s.duels.State(partyRoom))
				}
			}
			_ = client.Emit("roomLeft", map[string]any{"success": true, "roomId": oldRoomID})
		})

		client.On("startGame", func(datas ...any) {
			var payload startGamePayload
			if err := decodePayload(datas, &payload); err != nil {
				emitError(client, "开始游戏参数错误")
				return
			}
			if payload.FillWithAI {
				room, err := s.store.FillWithAIByHost(payload.RoomID, joinedPlayerID, game.MinPlayers)
				if err != nil {
					emitError(client, err.Error())
					return
				}
				s.broadcastPlayerJoined(payload.RoomID, room)
			}
			if err := s.emitGameStart(payload.RoomID, joinedPlayerID); err != nil {
				emitError(client, err.Error())
			}
		})

		client.On("selectGame", func(datas ...any) {
			var payload struct {
				RoomID string `json:"roomId"`
				GameID string `json:"gameId"`
			}
			if err := decodePayload(datas, &payload); err != nil {
				emitError(client, "选择游戏参数错误")
				return
			}
			room, err := s.platform.SelectGame(payload.RoomID, joinedPlayerID, payload.GameID)
			if err != nil {
				emitError(client, err.Error())
				return
			}
			_ = s.io.To(siosocket.Room(payload.RoomID)).Emit("partyState", map[string]any{"room": room})
		})

		client.On("proposeMission", func(datas ...any) {
			var payload proposeMissionPayload
			if err := decodePayload(datas, &payload); err != nil {
				emitError(client, "提名格式错误")
				return
			}
			if err := s.validateActiveSession(payload.RoomID, payload.SessionID); err != nil {
				emitError(client, err.Error())
				return
			}
			_, events, err := s.service.ProposeMissionWithReason(payload.RoomID, joinedPlayerID, payload.MemberIDs, payload.Reason)
			if err != nil {
				emitError(client, err.Error())
				return
			}
			s.emitEvents(events)
			s.scheduleAfterEvents(payload.RoomID, events)
		})

		client.On("submitStance", func(datas ...any) {
			var payload struct {
				RoomID    string `json:"roomId"`
				SessionID string `json:"sessionId"`
				TrustID   string `json:"trustId"`
				SuspectID string `json:"suspectId"`
				Reason    string `json:"reason"`
			}
			if err := decodePayload(datas, &payload); err != nil {
				emitError(client, "表态参数错误")
				return
			}
			if err := s.validateActiveSession(payload.RoomID, payload.SessionID); err != nil {
				emitError(client, err.Error())
				return
			}
			_, events, err := s.service.SubmitStance(payload.RoomID, joinedPlayerID, payload.TrustID, payload.SuspectID, payload.Reason)
			if err != nil {
				emitError(client, err.Error())
				return
			}
			s.emitEvents(events)
		})

		client.On("teamVote", func(datas ...any) {
			var payload teamVotePayload
			if err := decodePayload(datas, &payload); err != nil {
				emitError(client, "投票参数错误")
				return
			}
			if err := s.validateActiveSession(payload.RoomID, payload.SessionID); err != nil {
				emitError(client, err.Error())
				return
			}
			_, events, err := s.service.TeamVote(payload.RoomID, joinedPlayerID, payload.Approve)
			if err != nil {
				emitError(client, err.Error())
				return
			}
			s.emitEvents(events)
			s.commitAndEmitSnapshot(payload.RoomID)
			s.scheduleAfterEvents(payload.RoomID, events)
		})

		client.On("missionVote", func(datas ...any) {
			var payload struct {
				RoomID    string `json:"roomId"`
				SessionID string `json:"sessionId"`
				Answer    string `json:"answer"`
				Action    string `json:"action"`
				Success   *bool  `json:"success"`
			}
			if err := decodePayload(datas, &payload); err != nil {
				emitError(client, "任务投票参数错误")
				return
			}
			if err := s.validateActiveSession(payload.RoomID, payload.SessionID); err != nil {
				emitError(client, err.Error())
				return
			}
			action := payload.Action
			if action == "" {
				action = payload.Answer
			}
			if action == "" && payload.Success != nil && !*payload.Success {
				action = game.MissionActionSabotage
			}
			_, events, err := s.service.MissionVote(payload.RoomID, joinedPlayerID, action)
			if err != nil {
				emitError(client, err.Error())
				return
			}
			s.emitEvents(events)
			s.commitAndEmitSnapshot(payload.RoomID)
			s.scheduleAfterEvents(payload.RoomID, events)
		})

		client.On("chat", func(datas ...any) {
			var payload chatPayload
			if err := decodePayload(datas, &payload); err != nil {
				emitError(client, "消息参数错误")
				return
			}
			if err := s.validateActiveSession(payload.RoomID, payload.SessionID); err != nil {
				emitError(client, err.Error())
				return
			}
			message, messagesLeft, err := s.service.Chat(payload.RoomID, joinedPlayerID, payload.Message)
			if err != nil {
				emitError(client, err.Error())
				return
			}
			_ = s.io.To(siosocket.Room(payload.RoomID)).Emit("chat", map[string]any{
				"playerId":   message.PlayerID,
				"playerName": message.PlayerName,
				"message":    message.Displayed,
			})
			_ = s.io.To(siosocket.Room(joinedPlayerID)).Emit("chatReceipt", map[string]any{
				"messagesLeft": messagesLeft,
				"possessed":    message.Possessed,
			})
			s.commitAndEmitSnapshot(payload.RoomID)
		})

		client.On("puzzleChat", func(datas ...any) {
			var payload chatPayload
			if err := decodePayload(datas, &payload); err != nil {
				emitError(client, "任务发言参数错误")
				return
			}
			if err := s.validateActiveSession(payload.RoomID, payload.SessionID); err != nil {
				emitError(client, err.Error())
				return
			}
			message, messagesLeft, err := s.service.MissionChat(payload.RoomID, joinedPlayerID, payload.Message)
			if err != nil {
				emitError(client, err.Error())
				return
			}
			_ = s.io.To(siosocket.Room(payload.RoomID)).Emit("puzzleChatBroadcast", map[string]any{
				"playerId":     message.PlayerID,
				"playerName":   message.PlayerName,
				"message":      message.Displayed,
				"messagesLeft": messagesLeft,
			})
			s.commitAndEmitSnapshot(payload.RoomID)
		})

		client.On("resetRoom", func(datas ...any) {
			var payload startGamePayload
			if err := decodePayload(datas, &payload); err != nil {
				emitError(client, "重置房间参数错误")
				return
			}
			if _, err := s.store.ResetRoomByHost(payload.RoomID, joinedPlayerID); err != nil {
				emitError(client, err.Error())
				return
			}
			if _, ok := s.platform.ActiveSession(payload.RoomID); ok {
				_, _ = s.platform.AbandonSession(payload.RoomID)
			}
			_ = s.io.To(siosocket.Room(payload.RoomID)).Emit("roomReset")
		})

		client.On("rematch", func(datas ...any) {
			var payload startGamePayload
			if err := decodePayload(datas, &payload); err != nil {
				emitError(client, "再来一局参数错误")
				return
			}
			room, err := s.store.PrepareRematchByHost(payload.RoomID, joinedPlayerID)
			if err != nil {
				emitError(client, err.Error())
				return
			}
			_ = s.io.To(siosocket.Room(payload.RoomID)).Emit("roomReset", map[string]any{
				"players": room.Players,
				"mode":    room.Mode,
				"rematch": true,
			})
			if room.Mode == game.ModeTest || room.Mode == game.ModeSolo {
				go s.autofillAndStart(payload.RoomID)
			}
		})

		client.On("debugRequestState", func(datas ...any) {
			var payload startGamePayload
			if err := decodePayload(datas, &payload); err != nil {
				emitError(client, "调试状态参数错误")
				return
			}
			if err := s.emitDebugState(payload.RoomID, joinedPlayerID); err != nil {
				emitError(client, err.Error())
			}
		})

		client.On("debugSkipPhase", func(datas ...any) {
			var payload startGamePayload
			if err := decodePayload(datas, &payload); err != nil {
				emitError(client, "调试推进参数错误")
				return
			}
			events, err := s.debugSkipPhase(payload.RoomID, joinedPlayerID)
			if err != nil {
				emitError(client, err.Error())
				return
			}
			s.emitEvents(events)
			s.scheduleAfterEvents(payload.RoomID, events)
			if room, ok := s.store.Room(payload.RoomID); ok && room.DebugPaused {
				_, _ = s.store.RefreshDebugPauseByHost(payload.RoomID, joinedPlayerID)
			}
			_ = client.Emit("debugResponse", map[string]any{"success": true, "message": "已推进当前阶段"})
		})

		client.On("debugPause", func(datas ...any) {
			var payload struct {
				RoomID string `json:"roomId"`
				Paused bool   `json:"paused"`
			}
			if err := decodePayload(datas, &payload); err != nil {
				emitError(client, "调试暂停参数错误")
				return
			}
			room, remainingSeconds, err := s.store.SetDebugPausedByHost(payload.RoomID, joinedPlayerID, payload.Paused)
			if err != nil {
				emitError(client, err.Error())
				return
			}
			_ = client.Emit("debugPauseState", map[string]any{
				"paused":           room.DebugPaused,
				"remainingSeconds": remainingSeconds,
				"deadlineAt":       room.PhaseDeadline,
			})
			_ = s.emitDebugState(payload.RoomID, joinedPlayerID)
		})

		client.On("debugSetMissionResult", func(datas ...any) {
			var payload struct {
				RoomID  string `json:"roomId"`
				Success bool   `json:"success"`
			}
			if err := decodePayload(datas, &payload); err != nil {
				emitError(client, "调试任务结果参数错误")
				return
			}
			if _, err := s.service.SetDebugMissionResult(payload.RoomID, joinedPlayerID, payload.Success); err != nil {
				emitError(client, err.Error())
				return
			}
			message := "下一次任务将强制成功"
			if !payload.Success {
				message = "下一次任务将强制失败"
			}
			_ = client.Emit("debugResponse", map[string]any{"success": true, "message": message})
			_ = s.emitDebugState(payload.RoomID, joinedPlayerID)
		})

		client.On("disconnect", func(...any) {
			log.Printf("玩家断开: %s", clientID)
			if joinedRoomID == "" || joinedPlayerID == "" {
				return
			}
			s.mu.Lock()
			if s.session[joinedPlayerID] != clientID {
				s.mu.Unlock()
				return
			}
			delete(s.session, joinedPlayerID)
			s.mu.Unlock()
			partyRoom, platformErr := s.platform.SetConnection(joinedRoomID, joinedPlayerID, platform.MemberOffline)
			if room, err := s.store.SetPlayerDisconnected(joinedRoomID, joinedPlayerID, true); err == nil {
				s.broadcastPlayerJoined(joinedRoomID, room)
			}
			if platformErr == nil {
				_ = s.io.To(siosocket.Room(joinedRoomID)).Emit("partyState", map[string]any{"room": partyRoom})
				if isDuelGame(joinedGameID) {
					s.emitDuelState(joinedRoomID, s.duels.State(partyRoom))
				}
			}
		})
	})
}

func (s *Server) handleJoinRoom(client *siosocket.Socket, payload joinRoomPayload) (string, string, bool) {
	name, err := game.ValidatePlayerName(payload.Name)
	if err != nil {
		emitError(client, err.Error())
		return "", "", false
	}
	playerToken, err := game.ValidatePlayerToken(payload.PlayerToken)
	if err != nil {
		emitError(client, err.Error())
		return "", "", false
	}
	roomID := ""
	playerID := ""
	createdPartyRoom := false
	if payload.CreateNew {
		partyRoom, generatedPlayerID, createErr := s.createGeneratedPartyRoom("who-is-ai", playerToken, name)
		if createErr != nil {
			emitError(client, createErr.Error())
			return "", "", false
		}
		roomID = partyRoom.Code
		playerID = generatedPlayerID
		createdPartyRoom = true
		s.store.CreateRoom(roomID)
	} else {
		roomID, err = game.ValidateRoomID(payload.RoomID)
		if err != nil {
			emitError(client, err.Error())
			return "", "", false
		}
		if _, exists := s.store.Room(roomID); !exists {
			emitError(client, "房间不存在，请检查房间号或快速开房")
			return "", "", false
		}
		playerID = stablePlayerID(roomID, playerToken)
	}
	mode := game.Mode(payload.Mode)
	if mode == "" && payload.TestMode {
		mode = game.ModeTest
	}
	if mode == "" {
		mode = game.ModeNormal
	}

	_, reconnected, err := s.store.JoinPlayer(roomID, playerID, name, mode)
	if err != nil {
		if createdPartyRoom {
			_, _ = s.platform.LeaveRoom(roomID, playerID)
		}
		emitError(client, err.Error())
		return "", "", false
	}
	if !createdPartyRoom {
		err = s.joinPartyRoom(roomID, playerID, name)
	}
	if err != nil {
		emitError(client, err.Error())
		return "", "", false
	}
	client.Join(siosocket.Room(roomID))
	client.Join(siosocket.Room(playerID))
	client.SetData(roomID)
	_ = client.Emit("roomJoined", map[string]any{"roomId": roomID, "playerId": playerID})
	s.mu.Lock()
	s.session[playerID] = string(client.Id())
	s.mu.Unlock()

	room, _ := s.store.Room(roomID)
	s.broadcastPlayerJoined(roomID, room)
	if partyRoom, exists := s.platform.RoomByCode(roomID); exists {
		_ = client.Emit("partyState", map[string]any{"room": partyRoom})
	}

	room, _ = s.store.Room(roomID)
	if reconnected && room.Status != game.StatusWaiting {
		events, reconnectErr := s.service.ReconnectEvents(roomID, playerID)
		if reconnectErr == nil {
			s.emitEvents(events)
		}
		s.sendLatestSnapshot(roomID, playerID)
	}
	if !reconnected && (room.Mode == game.ModeTest || room.Mode == game.ModeSolo) {
		go s.autofillAndStart(roomID)
	}
	return roomID, playerID, true
}

func stablePlayerID(roomID, resumeToken string) string {
	digest := sha256.Sum256([]byte(roomID + "\x00" + resumeToken))
	return fmt.Sprintf("player_%x", digest[:12])
}

func (s *Server) handleJoinDuelRoom(
	client *siosocket.Socket,
	payload joinDuelRoomPayload,
) (string, string, bool) {
	name, err := game.ValidatePlayerName(payload.Name)
	if err != nil {
		emitError(client, err.Error())
		return "", "", false
	}
	token, err := game.ValidatePlayerToken(payload.PlayerToken)
	if err != nil {
		emitError(client, err.Error())
		return "", "", false
	}
	if !isDuelGame(payload.GameID) {
		emitError(client, "该游戏不支持双人房间")
		return "", "", false
	}
	definition, ok := s.registry.Game(payload.GameID)
	if !ok || definition.Status != platform.GameActive {
		emitError(client, platform.ErrGameUnavailable.Error())
		return "", "", false
	}

	roomID := ""
	playerID := ""
	var partyRoom *platform.PartyRoom
	if payload.CreateNew {
		partyRoom, playerID, err = s.createGeneratedPartyRoom(payload.GameID, token, name)
		if err == nil {
			roomID = partyRoom.Code
		}
	} else {
		roomID, err = game.ValidateRoomID(payload.RoomID)
		if err != nil {
			emitError(client, err.Error())
			return "", "", false
		}
		playerID = stablePlayerID(roomID, token)
		var exists bool
		partyRoom, exists = s.platform.RoomByCode(roomID)
		if !exists {
			emitError(client, "房间不存在，请检查邀请码或快速开房")
			return "", "", false
		}
		if partyRoom.SelectedGameID != payload.GameID {
			if partyRoom.HostMemberID != playerID {
				emitError(client, "该房间正在玩其他游戏，请让房主先切换")
				return "", "", false
			}
			partyRoom, err = s.platform.SelectGame(roomID, playerID, payload.GameID)
			if err != nil {
				emitError(client, err.Error())
				return "", "", false
			}
		}
		partyRoom, _, err = s.platform.JoinRoom(roomID, playerID, name)
	}
	if err != nil {
		emitError(client, err.Error())
		return "", "", false
	}

	client.Join(siosocket.Room(roomID))
	client.Join(siosocket.Room(playerID))
	client.SetData(roomID)
	s.mu.Lock()
	s.session[playerID] = string(client.Id())
	s.mu.Unlock()

	_ = client.Emit("roomJoined", map[string]any{
		"roomId": roomID, "playerId": playerID, "gameId": payload.GameID,
	})
	_ = s.io.To(siosocket.Room(roomID)).Emit("partyState", map[string]any{"room": partyRoom})
	s.emitDuelState(roomID, s.duels.State(partyRoom))
	return roomID, playerID, true
}

func (s *Server) createGeneratedPartyRoom(gameID, token, name string) (*platform.PartyRoom, string, error) {
	for attempts := 0; attempts < 32; attempts++ {
		roomID := platform.GenerateRoomCode(gameID)
		playerID := stablePlayerID(roomID, token)
		room, err := s.platform.CreateRoom(platform.CreateRoomInput{
			Code: roomID, HostID: playerID, HostName: name, GameID: gameID,
		})
		if errors.Is(err, platform.ErrRoomExists) {
			continue
		}
		return room, playerID, err
	}
	return nil, "", errors.New("暂时无法生成房间号，请稍后再试")
}

func (s *Server) startDuel(room *platform.PartyRoom) error {
	currentRoom, ok := s.platform.RoomByCode(room.Code)
	if !ok {
		return platform.ErrRoomNotFound
	}
	room = currentRoom
	if !duelPlayersReady(s.duels.State(room)) {
		return duel.ErrNotReady
	}
	participants := make([]platform.ParticipantInput, 0, 2)
	for _, member := range room.Members {
		if member.ConnectionStatus != platform.MemberOnline {
			continue
		}
		participants = append(participants, platform.ParticipantInput{
			ParticipantKey: member.ID,
			MemberID:       member.ID,
			DisplayName:    member.DisplayName,
			Kind:           platform.ParticipantHuman,
			Seat:           member.Seat,
		})
	}
	settings, err := s.duels.SessionSettings(room)
	if err != nil {
		return err
	}
	session, err := s.platform.StartSession(platform.StartSessionInput{
		RoomCode:     room.Code,
		HostID:       room.HostMemberID,
		Mode:         "normal",
		Settings:     settings,
		Participants: participants,
	})
	if err != nil {
		return err
	}

	state, err := s.duels.Start(room, session.ID)
	if err != nil {
		_, _ = s.platform.AbandonSession(room.Code)
		return err
	}
	publicState, err := json.Marshal(state)
	if err != nil {
		_, _ = s.platform.AbandonSession(room.Code)
		return fmt.Errorf("编码双人比赛状态: %w", err)
	}
	if _, err := s.platform.CommitSnapshot(session.ID, 0, publicState, nil); err != nil {
		_, _ = s.platform.AbandonSession(room.Code)
		return err
	}

	_ = s.io.To(siosocket.Room(room.Code)).Emit("duelStarted", map[string]any{
		"gameId": session.GameID, "sessionId": session.ID, "round": state.Round,
	})
	s.emitDuelState(room.Code, state)
	matchSummary := struct {
		Winner string `json:"winner"`
		Reason string `json:"reason"`
	}{}
	if state.Result != nil {
		_ = json.Unmarshal(state.Result.Match, &matchSummary)
	}
	summary, _ := json.Marshal(map[string]any{
		"gameId": session.GameID,
		"round":  state.Round,
		"winner": matchSummary.Winner,
		"reason": matchSummary.Reason,
	})
	if _, err := s.platform.FinishSession(room.Code, summary); err != nil {
		return err
	}
	if updatedRoom, ok := s.platform.RoomByCode(room.Code); ok {
		_ = s.io.To(siosocket.Room(room.Code)).Emit("partyState", map[string]any{"room": updatedRoom})
	}
	return nil
}

func (s *Server) emitDuelState(roomID string, state duel.State) {
	_ = s.io.To(siosocket.Room(roomID)).Emit("duelState", state)
}

func isDuelGame(gameID string) bool {
	return gameID == "bean-sprint" || gameID == "dumpling-sumo"
}

func duelPlayersReady(state duel.State) bool {
	if len(state.Players) != 2 {
		return false
	}
	for _, player := range state.Players {
		if !player.Ready || player.ConnectionStatus != platform.MemberOnline {
			return false
		}
	}
	return true
}

func (s *Server) autofillAndStart(roomID string) {
	time.Sleep(800 * time.Millisecond)
	room, ok := s.store.Room(roomID)
	if !ok || room.Status != game.StatusWaiting {
		return
	}
	targetTotal := 6
	for len(room.Players) < targetTotal {
		if _, err := s.store.AddAIPlayer(roomID); err != nil {
			break
		}
		room, _ = s.store.Room(roomID)
	}
	s.broadcastPlayerJoined(roomID, room)
	time.Sleep(2 * time.Second)
	starterID := firstHumanPlayerID(room)
	if starterID == "" {
		return
	}
	if err := s.emitGameStart(roomID, starterID); err != nil {
		_ = s.io.To(siosocket.Room(roomID)).Emit("error", err.Error())
	}
}

func (s *Server) emitGameStart(roomID, starterID string) error {
	state, events, err := s.service.StartGame(roomID, starterID)
	if err != nil {
		return err
	}
	session, err := s.ensurePlatformSession(state, starterID)
	if err != nil {
		return err
	}
	_ = s.io.To(siosocket.Room(roomID)).Emit("gameStarted", map[string]any{
		"players":         state.Players,
		"gameId":          session.GameID,
		"sessionId":       session.ID,
		"sessionSequence": session.Sequence,
	})
	s.emitEvents(events)
	s.scheduleAfterEvents(roomID, events)
	return nil
}

func (s *Server) emitEvents(events []game.Event) {
	roomIDs := map[string]bool{}
	finished := map[string]any{}
	for _, event := range events {
		roomIDs[event.RoomID] = true
		if event.Name == "gameFinished" {
			finished[event.RoomID] = event.Payload
		}
		if event.SocketID != "" {
			_ = s.io.To(siosocket.Room(event.SocketID)).Emit(event.Name, event.Payload)
			continue
		}
		_ = s.io.To(siosocket.Room(event.RoomID)).Emit(event.Name, event.Payload)
	}
	for roomID := range roomIDs {
		s.commitAndEmitSnapshot(roomID)
		s.emitSoloDebugState(roomID)
	}
	for roomID, payload := range finished {
		s.finishPlatformSession(roomID, payload)
	}
}

func (s *Server) joinPartyRoom(roomID, playerID, name string) error {
	room, exists := s.platform.RoomByCode(roomID)
	if !exists {
		_, err := s.platform.CreateRoom(platform.CreateRoomInput{
			Code: roomID, HostID: playerID, HostName: name, GameID: "who-is-ai",
		})
		return err
	}
	if room.SelectedGameID != "who-is-ai" {
		if room.HostMemberID != playerID {
			return errors.New("该房间正在玩其他游戏，请让房主先切换")
		}
		if _, err := s.platform.SelectGame(roomID, playerID, "who-is-ai"); err != nil {
			return err
		}
	}
	_, _, err := s.platform.JoinRoom(roomID, playerID, name)
	return err
}

func (s *Server) validateActiveSession(roomID, sessionID string) error {
	// Empty session IDs keep the previous protocol working during the migration.
	// New clients always send the ID, so delayed actions from an older match can
	// no longer mutate the active match.
	if sessionID == "" {
		return nil
	}
	session, exists := s.platform.ActiveSession(roomID)
	if !exists || session.ID != sessionID {
		return errors.New("游戏局已变化，请按当前房间状态继续")
	}
	return nil
}

func (s *Server) ensurePlatformSession(room *game.Room, starterID string) (*platform.GameSession, error) {
	if session, exists := s.platform.ActiveSession(room.ID); exists {
		return session, nil
	}
	participants := make([]platform.ParticipantInput, 0, len(room.Players))
	for _, player := range room.Players {
		kind := platform.ParticipantHuman
		memberID := player.ID
		if player.IsAI {
			kind = platform.ParticipantBot
			memberID = ""
		}
		participants = append(participants, platform.ParticipantInput{
			ParticipantKey: player.ID,
			MemberID:       memberID,
			DisplayName:    player.Name,
			Kind:           kind,
			Seat:           player.Position,
		})
	}
	settings, err := json.Marshal(map[string]any{"mode": room.Mode})
	if err != nil {
		return nil, err
	}
	return s.platform.StartSession(platform.StartSessionInput{
		RoomCode: room.ID, HostID: starterID, Mode: string(room.Mode),
		Settings: settings, Participants: participants,
	})
}

func (s *Server) finishPlatformSession(roomID string, payload any) {
	if _, exists := s.platform.ActiveSession(roomID); !exists {
		return
	}
	summary, err := json.Marshal(payload)
	if err != nil {
		log.Printf("encode game result for room %s: %v", roomID, err)
		return
	}
	if _, err := s.platform.FinishSession(roomID, summary); err != nil {
		log.Printf("finish game session for room %s: %v", roomID, err)
	}
}

func (s *Server) commitAndEmitSnapshot(roomID string) {
	session, exists := s.platform.ActiveSession(roomID)
	if !exists {
		return
	}
	room, exists := s.store.Room(roomID)
	if !exists {
		return
	}

	privateState := make(map[string]json.RawMessage)
	var publicState json.RawMessage
	for _, player := range room.Players {
		if player.IsAI {
			continue
		}
		public, private, err := s.store.GameSnapshot(roomID, player.ID)
		if err != nil {
			log.Printf("build game snapshot for room %s player %s: %v", roomID, player.ID, err)
			return
		}
		if publicState == nil {
			publicState, err = json.Marshal(public)
			if err != nil {
				log.Printf("encode public game snapshot for room %s: %v", roomID, err)
				return
			}
		}
		encoded, err := json.Marshal(private)
		if err != nil {
			log.Printf("encode private game snapshot for room %s player %s: %v", roomID, player.ID, err)
			return
		}
		privateState[player.ID] = encoded
	}
	if publicState == nil {
		return
	}
	if latest, ok := s.platform.LatestSnapshot(session.ID, ""); ok &&
		bytes.Equal(latest.PublicState, publicState) && samePrivateState(session.ID, privateState, s.platform) {
		return
	}

	snapshot, err := s.platform.CommitSnapshot(session.ID, session.StateVersion, publicState, privateState)
	if err != nil {
		log.Printf("commit game snapshot for room %s: %v", roomID, err)
		return
	}
	for _, player := range room.Players {
		if player.IsAI {
			continue
		}
		s.emitSnapshotToPlayer(session, snapshot, player.ID)
	}
}

func (s *Server) sendLatestSnapshot(roomID, playerID string) {
	session, exists := s.platform.ActiveSession(roomID)
	if !exists {
		return
	}
	snapshot, exists := s.platform.LatestSnapshot(session.ID, playerID)
	if !exists {
		s.commitAndEmitSnapshot(roomID)
		return
	}
	s.emitSnapshotToPlayer(session, snapshot, playerID)
}

func (s *Server) emitSnapshotToPlayer(session *platform.GameSession, snapshot *platform.Snapshot, playerID string) {
	var privateState json.RawMessage
	if snapshot.PrivateState != nil {
		privateState = snapshot.PrivateState[playerID]
	}
	var publicValue any
	if err := json.Unmarshal(snapshot.PublicState, &publicValue); err != nil {
		log.Printf("decode public snapshot %s: %v", session.ID, err)
		return
	}
	var privateValue any
	if len(privateState) > 0 {
		if err := json.Unmarshal(privateState, &privateValue); err != nil {
			log.Printf("decode private snapshot %s for player %s: %v", session.ID, playerID, err)
			return
		}
	}
	_ = s.io.To(siosocket.Room(playerID)).Emit("gameSnapshot", map[string]any{
		"sessionId":    session.ID,
		"gameId":       session.GameID,
		"version":      snapshot.Version,
		"serverNow":    snapshot.ServerTime.UnixMilli(),
		"publicState":  publicValue,
		"privateState": privateValue,
	})
}

func samePrivateState(
	sessionID string,
	next map[string]json.RawMessage,
	store *platform.MemoryStore,
) bool {
	for playerID, state := range next {
		latest, ok := store.LatestSnapshot(sessionID, playerID)
		if !ok || !bytes.Equal(latest.PrivateState[playerID], state) {
			return false
		}
	}
	return true
}

func (s *Server) scheduleAfterEvents(roomID string, events []game.Event) {
	room, ok := s.store.Room(roomID)
	if !ok {
		return
	}
	for _, event := range events {
		if event.Name != "phaseChange" {
			continue
		}
		payload, ok := event.Payload.(map[string]any)
		if !ok {
			continue
		}
		roundNumber := intPayload(payload, "roundNumber", room.CurrentRound)
		timeLimit := eventDelay(payload, time.Duration(intPayload(payload, "timeLimit", 15))*time.Second)
		switch payload["phase"] {
		case game.PhasePropose:
			s.scheduleAutoPropose(roomID, roundNumber, room.CurrentLeader, timeLimit)
		case game.PhaseDiscuss:
			s.scheduleStartTeamVote(roomID, roundNumber, teamKey(room.ProposedTeam), timeLimit)
			if roomHasAI(room) {
				s.scheduleAIConversation(roomID, roundNumber, teamKey(room.ProposedTeam))
			}
		case game.PhaseTeamVote:
			s.scheduleResolveTeamVote(roomID, roundNumber, teamKey(room.ProposedTeam), timeLimit)
		}
	}
	for _, event := range events {
		if event.Name != "missionSubPhase" {
			continue
		}
		payload, ok := event.Payload.(map[string]any)
		if !ok {
			continue
		}
		switch payload["subPhase"] {
		case "discuss":
			delay := eventDelay(payload, time.Duration(intPayload(payload, "timeLimit", 12))*time.Second)
			s.scheduleStartMissionVote(roomID, room.CurrentRound, delay)
		case "vote":
			delay := eventDelay(payload, time.Duration(intPayload(payload, "timeLimit", 15))*time.Second)
			if roomHasAI(room) {
				s.scheduleAIMissionVote(roomID, room.CurrentRound, 500*time.Millisecond)
			}
			s.scheduleResolveMissionVote(roomID, room.CurrentRound, delay)
		}
	}
}

func (s *Server) scheduleAIConversation(roomID string, roundNumber int, proposedTeam string) {
	go func() {
		if !s.waitRoomDelay(roomID, 900*time.Millisecond) {
			return
		}

		room, ok := s.store.Room(roomID)
		if !ok || room.CurrentPhase != game.PhaseDiscuss || room.CurrentRound != roundNumber ||
			teamKey(room.ProposedTeam) != proposedTeam {
			return
		}
		spoken := 0
		for _, player := range room.Players {
			if !player.IsAI || player.Eliminated || player.Disconnected {
				continue
			}
			message := aiDiscussionMessage(room, player.ID)
			chatMessage, _, err := s.service.Chat(roomID, player.ID, message)
			if err != nil {
				continue
			}
			_ = s.io.To(siosocket.Room(roomID)).Emit("chat", map[string]any{
				"playerId":   chatMessage.PlayerID,
				"playerName": chatMessage.PlayerName,
				"message":    chatMessage.Displayed,
			})
			spoken++
			if spoken == 2 {
				break
			}
		}
		s.submitAIStance(roomID, room)
	}()
}

func (s *Server) submitAIStance(roomID string, room *game.Room) {
	submitted := 0
	for _, player := range room.Players {
		if !player.IsAI || player.Eliminated || player.Disconnected {
			continue
		}
		trustID, suspectID, reason := aiStanceDecision(room, player.ID)
		if trustID == "" || suspectID == "" {
			continue
		}
		_, events, err := s.service.SubmitStance(
			roomID,
			player.ID,
			trustID,
			suspectID,
			reason,
		)
		if err == nil {
			s.emitEvents(events)
			submitted++
		}
		if submitted == 2 {
			return
		}
	}
}

func aiDiscussionMessage(room *game.Room, playerID string) string {
	leaderName := playerNameForDebug(room, room.CurrentLeader)
	teamNames := playerNamesForDebug(room, room.ProposedTeam)
	lastTeam, lastSuccess := lastResolvedMission(room)
	wasOnFailedTeam := !lastSuccess && containsString(lastTeam, playerID)
	variant := playerPosition(room, playerID) % 3

	if len(room.MissionResults) > 0 && !lastSuccess {
		if wasOnFailedTeam {
			return "我在上轮失败队里，这轮请重点核对我和其他留队成员的投票解释。"
		}
		if overlapsTeam(room.ProposedTeam, lastTeam) {
			return "上轮失败成员再次进入小队，" + leaderName + "需要解释为什么还值得冒险。"
		}
		return "这轮换掉了上轮失败成员，我暂时支持，但会看谁仍替旧队伍辩护。"
	}
	if containsString(room.ProposedTeam, playerID) {
		if variant == 0 {
			return "我在本轮小队里，愿意解释自己的投票；也请" + leaderName + "说明为什么选我。"
		}
		return "这支队是" + strings.Join(teamNames, "、") + "，先看是否有人只给结论、不说明依据。"
	}
	if variant == 1 {
		return "我没进队，想听" + leaderName + "逐个解释这组人比其他组合更可信的证据。"
	}
	return "先别急着站队：谁支持" + strings.Join(teamNames, "、") + "，最好把依据说清楚。"
}

func aiStanceDecision(room *game.Room, playerID string) (string, string, string) {
	trustID, suspectID := debugStanceTargets(room, playerID)
	lastTeam, lastSuccess := lastResolvedMission(room)
	if len(room.MissionResults) > 0 {
		if lastSuccess {
			trustID = firstOtherPlayer(lastTeam, playerID, "")
		} else {
			suspectID = firstOtherPlayer(lastTeam, playerID, trustID)
		}
	}
	if trustID == "" || suspectID == "" || trustID == suspectID {
		trustID, suspectID = debugStanceTargets(room, playerID)
	}
	reason := "先按本轮提名关系站队，任务结果出来后再校正。"
	if playerPosition(room, playerID)%2 == 1 {
		reason = "我先记录队长的组队关系，重点观察谁只跟票却不给理由。"
	}
	if len(room.MissionResults) > 0 && lastSuccess {
		reason = "上一轮成功小队提供了公开证据，暂时延续信任但不排除隐藏。"
		if playerPosition(room, playerID)%2 == 0 {
			reason = "成功只能降低嫌疑；我会继续核对这轮入队变化和投票理由。"
		}
	}
	if len(room.MissionResults) > 0 && !lastSuccess {
		reason = "上一轮任务失败，先从行动小队和重复入队关系中缩小嫌疑。"
		if playerPosition(room, playerID)%2 == 0 {
			reason = "失败队里至少有一条线索，这轮重点追问留队成员和支持票。"
		}
	}
	return trustID, suspectID, reason
}

func lastResolvedMission(room *game.Room) ([]string, bool) {
	if len(room.MissionResults) == 0 {
		return nil, true
	}
	for i := len(room.VoteHistory) - 1; i >= 0; i-- {
		if room.VoteHistory[i].Approved {
			return room.VoteHistory[i].Team, room.MissionResults[len(room.MissionResults)-1]
		}
	}
	return nil, room.MissionResults[len(room.MissionResults)-1]
}

func firstOtherPlayer(playerIDs []string, playerID, excludedID string) string {
	for _, candidate := range playerIDs {
		if candidate != playerID && candidate != excludedID {
			return candidate
		}
	}
	return ""
}

func overlapsTeam(left, right []string) bool {
	for _, playerID := range left {
		if containsString(right, playerID) {
			return true
		}
	}
	return false
}

func playerPosition(room *game.Room, playerID string) int {
	for _, player := range room.Players {
		if player.ID == playerID {
			return player.Position
		}
	}
	return 0
}

func playerNamesForDebug(room *game.Room, playerIDs []string) []string {
	names := make([]string, 0, len(playerIDs))
	for _, playerID := range playerIDs {
		names = append(names, playerNameForDebug(room, playerID))
	}
	return names
}

func debugStanceTargets(room *game.Room, playerID string) (string, string) {
	trustID := ""
	for _, candidate := range room.ProposedTeam {
		if candidate != playerID {
			trustID = candidate
			break
		}
	}
	if trustID == "" {
		for _, player := range room.Players {
			if player.ID != playerID && !player.Eliminated && !player.Disconnected {
				trustID = player.ID
				break
			}
		}
	}
	for _, player := range room.Players {
		if player.ID != playerID && player.ID != trustID && !player.Eliminated && !player.Disconnected {
			return trustID, player.ID
		}
	}
	return "", ""
}

func (s *Server) scheduleAutoPropose(roomID string, roundNumber int, leaderID string, delay time.Duration) {
	go func() {
		if !s.waitRoomDelay(roomID, delay) {
			return
		}

		room, ok := s.store.Room(roomID)
		if !ok || room.Status != game.StatusPlaying || room.CurrentPhase != game.PhasePropose ||
			room.CurrentRound != roundNumber || room.CurrentLeader != leaderID {
			return
		}
		pool := activePlayerIDs(room)
		teamSize := game.MissionTeamSize(len(pool), room.CurrentRound)
		if len(pool) < teamSize {
			return
		}
		team := rotatingTeam(pool, room.CurrentLeader, room.CurrentRound, teamSize)
		if (room.Mode == game.ModeTest || room.Mode == game.ModeSolo) && room.CurrentRound == 2 {
			team = scriptedFailureTeam(room, teamSize)
		}
		_, events, err := s.service.ProposeMissionWithReason(
			roomID,
			room.CurrentLeader,
			team,
			"系统按队长顺位与本轮任务人数自动组队",
		)
		if err != nil {
			return
		}
		s.emitEvents(events)
		s.scheduleAfterEvents(roomID, events)
	}()
}

func scriptedFailureTeam(room *game.Room, teamSize int) []string {
	team := make([]string, 0, teamSize)
	if room.CurrentLeader != "" {
		team = append(team, room.CurrentLeader)
	}
	for _, player := range room.Players {
		if room.Roles[player.ID] == game.RoleInfiltrator && !containsString(team, player.ID) {
			team = append(team, player.ID)
			break
		}
	}
	for _, player := range room.Players {
		if len(team) >= teamSize {
			break
		}
		if player.Eliminated || containsString(team, player.ID) {
			continue
		}
		team = append(team, player.ID)
	}
	return team
}

func (s *Server) scheduleStartTeamVote(roomID string, roundNumber int, proposedTeam string, delay time.Duration) {
	go func() {
		if !s.waitRoomDelay(roomID, delay) {
			return
		}

		room, ok := s.store.Room(roomID)
		if !ok || room.CurrentPhase != game.PhaseDiscuss || room.CurrentRound != roundNumber ||
			teamKey(room.ProposedTeam) != proposedTeam {
			return
		}
		_, events, err := s.service.StartTeamVote(roomID)
		if err != nil {
			return
		}
		s.emitEvents(events)
		s.scheduleAfterEvents(roomID, events)
	}()
}

func (s *Server) scheduleResolveTeamVote(roomID string, roundNumber int, proposedTeam string, delay time.Duration) {
	go func() {
		if !s.waitRoomDelay(roomID, delay) {
			return
		}

		room, ok := s.store.Room(roomID)
		if !ok || room.CurrentPhase != game.PhaseTeamVote || room.CurrentRound != roundNumber ||
			teamKey(room.ProposedTeam) != proposedTeam {
			return
		}
		for _, player := range room.Players {
			if player.Eliminated || room.TeamVotes[player.ID] {
				continue
			}
			if _, voted := room.TeamVotes[player.ID]; voted {
				continue
			}
			_, events, err := s.service.AutoTeamVote(roomID, player.ID, false)
			if err != nil {
				continue
			}
			if !player.IsAI {
				message := "投票超时，系统已代为反对"
				_ = s.store.RecordAutoManaged(roomID, player.ID, "team_vote", message)
				_ = s.io.To(siosocket.Room(player.ID)).Emit("autoManaged", map[string]any{
					"action":  "team_vote",
					"message": message,
				})
			}
			s.emitEvents(events)
			s.commitAndEmitSnapshot(roomID)
			s.scheduleAfterEvents(roomID, events)
			if len(events) > 0 {
				return
			}
		}
	}()
}

func (s *Server) scheduleStartMissionVote(roomID string, roundNumber int, delay time.Duration) {
	go func() {
		if !s.waitRoomDelay(roomID, delay) {
			return
		}

		room, ok := s.store.Room(roomID)
		if !ok || room.CurrentPhase != game.PhaseMission || room.CurrentRound != roundNumber ||
			room.MissionSubPhase != "discuss" {
			return
		}
		_, events, err := s.service.StartMissionVote(roomID)
		if err != nil {
			return
		}
		s.emitEvents(events)
		s.scheduleAfterEvents(roomID, events)
	}()
}

func (s *Server) scheduleAIMissionVote(roomID string, roundNumber int, delay time.Duration) {
	go func() {
		if !s.waitRoomDelay(roomID, delay) {
			return
		}

		room, ok := s.store.Room(roomID)
		if !ok || room.CurrentPhase != game.PhaseMission || room.CurrentRound != roundNumber ||
			room.MissionSubPhase != "vote" {
			return
		}
		for _, playerID := range room.ProposedTeam {
			if !isAIPlayer(room, playerID) {
				continue
			}
			_, events, err := s.service.MissionVote(roomID, playerID, game.MissionActionSupport)
			if err == nil {
				s.emitEvents(events)
				s.commitAndEmitSnapshot(roomID)
				s.scheduleAfterEvents(roomID, events)
			}
			return
		}
	}()
}

func (s *Server) scheduleResolveMissionVote(roomID string, roundNumber int, delay time.Duration) {
	go func() {
		if !s.waitRoomDelay(roomID, delay) {
			return
		}

		room, ok := s.store.Room(roomID)
		if !ok || room.CurrentPhase != game.PhaseMission || room.CurrentRound != roundNumber {
			return
		}
		for _, playerID := range room.ProposedTeam {
			if _, voted := room.MissionVotes[playerID]; voted {
				continue
			}
			_, events, err := s.service.MissionVote(roomID, playerID, game.MissionActionSupport)
			if err != nil {
				continue
			}
			if !isAIPlayer(room, playerID) {
				message := "行动超时，系统已代为执行任务"
				_ = s.store.RecordAutoManaged(roomID, playerID, "mission_vote", message)
				_ = s.io.To(siosocket.Room(playerID)).Emit("autoManaged", map[string]any{
					"action":  "mission_vote",
					"message": message,
				})
			}
			s.emitEvents(events)
			s.commitAndEmitSnapshot(roomID)
			s.scheduleAfterEvents(roomID, events)
			if len(events) > 0 {
				return
			}
		}
	}()
}

func (s *Server) debugSkipPhase(roomID, playerID string) ([]game.Event, error) {
	room, err := s.debugAdvanceRoom(roomID, playerID)
	if err != nil {
		return nil, err
	}
	if room.Status != game.StatusPlaying {
		return nil, errors.New("当前没有进行中的调试对局")
	}

	switch room.CurrentPhase {
	case game.PhasePropose:
		pool := activePlayerIDs(room)
		team := rotatingTeam(pool, room.CurrentLeader, room.CurrentRound, game.MissionTeamSize(len(pool), room.CurrentRound))
		_, events, proposeErr := s.service.ProposeMissionWithReason(roomID, room.CurrentLeader, team, "调试面板自动提名")
		return events, proposeErr
	case game.PhaseDiscuss:
		_, events, voteErr := s.service.StartTeamVote(roomID)
		return events, voteErr
	case game.PhaseTeamVote:
		return s.debugResolveTeamVote(room)
	case game.PhaseMission:
		if room.MissionSubPhase == "discuss" {
			_, events, voteErr := s.service.StartMissionVote(roomID)
			return events, voteErr
		}
		if room.MissionSubPhase == "vote" {
			return s.debugResolveMissionVote(room)
		}
	}
	return nil, errors.New("当前阶段暂不支持调试推进")
}

func (s *Server) debugAdvanceRoom(roomID, playerID string) (*game.Room, error) {
	room, ok := s.store.Room(roomID)
	if !ok {
		return nil, errors.New("房间不存在")
	}
	if (room.Mode != game.ModeSolo && room.Mode != game.ModeTest) || firstHumanPlayerID(room) != playerID {
		return nil, errors.New("只有测试或单人调试房主可以推进阶段")
	}
	return room, nil
}

func (s *Server) debugResolveTeamVote(room *game.Room) ([]game.Event, error) {
	allEvents := []game.Event{}
	for _, player := range room.Players {
		if player.Eliminated || player.Disconnected {
			continue
		}
		if _, voted := room.TeamVotes[player.ID]; voted {
			continue
		}
		_, events, err := s.service.AutoTeamVote(room.ID, player.ID, true)
		if err != nil {
			return allEvents, err
		}
		allEvents = append(allEvents, events...)
		if len(events) > 0 {
			break
		}
	}
	return allEvents, nil
}

func (s *Server) debugResolveMissionVote(room *game.Room) ([]game.Event, error) {
	allEvents := []game.Event{}
	for _, playerID := range room.ProposedTeam {
		if _, voted := room.MissionVotes[playerID]; voted {
			continue
		}
		_, events, err := s.service.MissionVote(room.ID, playerID, game.MissionActionSupport)
		if err != nil {
			return allEvents, err
		}
		allEvents = append(allEvents, events...)
		if len(events) > 0 {
			break
		}
	}
	return allEvents, nil
}

func (s *Server) soloDebugRoom(roomID, playerID string) (*game.Room, error) {
	room, ok := s.store.Room(roomID)
	if !ok {
		return nil, errors.New("房间不存在")
	}
	if room.Mode != game.ModeSolo || firstHumanPlayerID(room) != playerID {
		return nil, errors.New("只有单人调试房主可以使用调试面板")
	}
	return room, nil
}

func (s *Server) emitDebugState(roomID, playerID string) error {
	room, err := s.soloDebugRoom(roomID, playerID)
	if err != nil {
		return err
	}
	return s.io.To(siosocket.Room(playerID)).Emit("gameStateUpdate", debugStatePayload(room))
}

func (s *Server) emitSoloDebugState(roomID string) {
	room, ok := s.store.Room(roomID)
	if !ok || room.Mode != game.ModeSolo {
		return
	}
	for _, player := range room.Players {
		if player.IsAI || player.Disconnected || player.Eliminated {
			continue
		}
		_ = s.io.To(siosocket.Room(player.ID)).Emit("gameStateUpdate", debugStatePayload(room))
	}
}

func debugStatePayload(room *game.Room) map[string]any {
	players := make([]map[string]any, 0, len(room.Players))
	for _, player := range room.Players {
		entry := map[string]any{
			"id":           player.ID,
			"name":         player.Name,
			"isAI":         player.IsAI,
			"disconnected": player.Disconnected,
		}
		if player.IsAI {
			entry["role"] = game.RoleLabels[room.Roles[player.ID]]
			entry["voteIntent"] = debugVoteIntent(room, player.ID)
			entry["isPossessed"] = room.PossessedPlayer == player.ID
		}
		players = append(players, entry)
	}
	debugResult := "未指定"
	if room.DebugMissionResult != nil {
		debugResult = "失败"
		if *room.DebugMissionResult {
			debugResult = "成功"
		}
	}
	return map[string]any{
		"roomId":             room.ID,
		"status":             room.Status,
		"players":            players,
		"currentRound":       room.CurrentRound,
		"maxRounds":          game.MaxRounds,
		"currentPhase":       room.CurrentPhase,
		"missionSubPhase":    room.MissionSubPhase,
		"missionSuccesses":   room.MissionSuccesses,
		"missionFailures":    room.MissionFailures,
		"currentLeaderName":  playerNameForDebug(room, room.CurrentLeader),
		"proposedTeam":       room.ProposedTeam,
		"deadlineAt":         room.PhaseDeadline,
		"debugMissionResult": debugResult,
		"debugPaused":        room.DebugPaused,
	}
}

func (s *Server) waitRoomDelay(roomID string, delay time.Duration) bool {
	remaining := delay
	lastTick := time.Now()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for remaining > 0 {
		<-ticker.C
		now := time.Now()
		room, ok := s.store.Room(roomID)
		if !ok {
			return false
		}
		if !room.DebugPaused {
			remaining -= now.Sub(lastTick)
		}
		lastTick = now
	}
	return true
}

func debugVoteIntent(room *game.Room, playerID string) string {
	if vote, ok := room.TeamVotes[playerID]; ok {
		if vote {
			return "已同意小队"
		}
		return "已反对小队"
	}
	if action, ok := room.MissionVotes[playerID]; ok {
		if action == game.MissionActionSabotage {
			return "已秘密破坏"
		}
		return "已执行任务"
	}
	if room.Roles[playerID] == game.RoleInfiltrator {
		return "等待制造失败机会"
	}
	return "倾向支持可信小队"
}

func playerNameForDebug(room *game.Room, playerID string) string {
	for _, player := range room.Players {
		if player.ID == playerID {
			return player.Name
		}
	}
	return "--"
}

func rotatingTeam(pool []string, leaderID string, roundNumber, teamSize int) []string {
	if teamSize >= len(pool) {
		return append([]string(nil), pool...)
	}
	team := make([]string, 0, teamSize)
	if leaderID != "" {
		team = append(team, leaderID)
	}
	start := roundNumber - 1
	if start < 0 {
		start = 0
	}
	for i := 0; len(team) < teamSize && i < len(pool)*2; i++ {
		playerID := pool[(start+i)%len(pool)]
		if containsString(team, playerID) {
			continue
		}
		team = append(team, playerID)
	}
	return team
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func intPayload(payload map[string]any, key string, fallback int) int {
	value, ok := payload[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return int(parsed)
		}
	}
	return fallback
}

func eventDelay(payload map[string]any, fallback time.Duration) time.Duration {
	deadlineAt := int64Payload(payload, "deadlineAt", 0)
	if deadlineAt == 0 {
		return fallback
	}
	delay := time.Until(time.UnixMilli(deadlineAt))
	if delay < 0 {
		return 0
	}
	return delay
}

func int64Payload(payload map[string]any, key string, fallback int64) int64 {
	value, ok := payload[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func teamKey(playerIDs []string) string {
	return strings.Join(playerIDs, "\x00")
}

func activePlayerIDs(room *game.Room) []string {
	playerIDs := make([]string, 0, len(room.Players))
	for _, disconnected := range []bool{false, true} {
		for _, player := range room.Players {
			if player.Eliminated || player.Disconnected != disconnected {
				continue
			}
			playerIDs = append(playerIDs, player.ID)
		}
	}
	return playerIDs
}

func firstHumanPlayerID(room *game.Room) string {
	for _, player := range room.Players {
		if !player.IsAI && !player.Eliminated {
			return player.ID
		}
	}
	return ""
}

func isAIPlayer(room *game.Room, playerID string) bool {
	for _, player := range room.Players {
		if player.ID == playerID {
			return player.IsAI
		}
	}
	return false
}

func roomHasAI(room *game.Room) bool {
	for _, player := range room.Players {
		if player.IsAI && !player.Eliminated {
			return true
		}
	}
	return false
}

func (s *Server) broadcastPlayerJoined(roomID string, room *game.Room) {
	_ = s.io.To(siosocket.Room(roomID)).Emit("playerJoined", map[string]any{
		"players": room.Players,
		"count":   len(room.Players),
		"mode":    room.Mode,
	})
}

func decodePayload(datas []any, out any) error {
	if len(datas) == 0 {
		return errors.New("missing payload")
	}
	raw, err := json.Marshal(datas[0])
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func emitError(client *siosocket.Socket, message string) {
	_ = client.Emit("error", message)
}
