package realtime

import (
	"encoding/json"
	"errors"
	"log"
	"time"

	siosocket "github.com/zishang520/socket.io/socket"

	"whoisai/internal/game"
)

type Server struct {
	io      *siosocket.Server
	store   *game.Store
	service *game.Service
}

type joinRoomPayload struct {
	RoomID   string `json:"roomId"`
	Name     string `json:"name"`
	Mode     string `json:"mode"`
	TestMode bool   `json:"testMode"`
}

type startGamePayload struct {
	RoomID string `json:"roomId"`
}

type proposeMissionPayload struct {
	RoomID    string   `json:"roomId"`
	MemberIDs []string `json:"memberIds"`
}

type teamVotePayload struct {
	RoomID  string `json:"roomId"`
	Approve bool   `json:"approve"`
}

type chatPayload struct {
	RoomID  string `json:"roomId"`
	Message string `json:"message"`
}

func New(io *siosocket.Server, store *game.Store, service *game.Service) *Server {
	return &Server{io: io, store: store, service: service}
}

func (s *Server) Register() {
	s.io.On("connection", func(clients ...any) {
		client := clients[0].(*siosocket.Socket)
		clientID := string(client.Id())
		log.Printf("玩家连接: %s", clientID)

		client.On("joinRoom", func(datas ...any) {
			var payload joinRoomPayload
			if err := decodePayload(datas, &payload); err != nil {
				emitError(client, "加入房间参数错误")
				return
			}
			s.handleJoinRoom(client, clientID, payload)
		})

		client.On("startGame", func(datas ...any) {
			var payload startGamePayload
			if err := decodePayload(datas, &payload); err != nil {
				emitError(client, "开始游戏参数错误")
				return
			}
			if err := s.emitGameStart(payload.RoomID, clientID); err != nil {
				emitError(client, err.Error())
			}
		})

		client.On("proposeMission", func(datas ...any) {
			var payload proposeMissionPayload
			if err := decodePayload(datas, &payload); err != nil {
				emitError(client, "提名格式错误")
				return
			}
			_, events, err := s.service.ProposeMission(payload.RoomID, clientID, payload.MemberIDs)
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
			_, events, err := s.service.TeamVote(payload.RoomID, clientID, payload.Approve)
			if err != nil {
				emitError(client, err.Error())
				return
			}
			s.emitEvents(events)
			s.scheduleAfterEvents(payload.RoomID, events)
		})

		client.On("missionVote", func(datas ...any) {
			var payload struct {
				RoomID  string `json:"roomId"`
				Answer  string `json:"answer"`
				Action  string `json:"action"`
				Success *bool  `json:"success"`
			}
			if err := decodePayload(datas, &payload); err != nil {
				emitError(client, "任务投票参数错误")
				return
			}
			action := payload.Action
			if action == "" {
				action = payload.Answer
			}
			if action == "" && payload.Success != nil && !*payload.Success {
				action = game.MissionActionSabotage
			}
			_, events, err := s.service.MissionVote(payload.RoomID, clientID, action)
			if err != nil {
				emitError(client, err.Error())
				return
			}
			s.emitEvents(events)
			s.scheduleAfterEvents(payload.RoomID, events)
		})

		client.On("chat", func(datas ...any) {
			var payload chatPayload
			if err := decodePayload(datas, &payload); err != nil {
				emitError(client, "消息参数错误")
				return
			}
			message, messagesLeft, err := s.service.Chat(payload.RoomID, clientID, payload.Message)
			if err != nil {
				emitError(client, err.Error())
				return
			}
			_ = s.io.To(siosocket.Room(payload.RoomID)).Emit("chat", map[string]any{
				"playerId":     message.PlayerID,
				"playerName":   message.PlayerName,
				"message":      message.Displayed,
				"original":     message.Original,
				"possessed":    message.Possessed,
				"messagesLeft": messagesLeft,
			})
		})

		client.On("resetRoom", func(datas ...any) {
			var payload startGamePayload
			if err := decodePayload(datas, &payload); err != nil {
				emitError(client, "重置房间参数错误")
				return
			}
			s.store.ResetRoom(payload.RoomID)
			_ = s.io.To(siosocket.Room(payload.RoomID)).Emit("roomReset")
		})

		client.On("disconnect", func(...any) {
			log.Printf("玩家断开: %s", clientID)
		})
	})
}

func (s *Server) handleJoinRoom(client *siosocket.Socket, clientID string, payload joinRoomPayload) {
	roomID, err := game.ValidateRoomID(payload.RoomID)
	if err != nil {
		emitError(client, err.Error())
		return
	}
	name, err := game.ValidatePlayerName(payload.Name)
	if err != nil {
		emitError(client, err.Error())
		return
	}
	mode := game.Mode(payload.Mode)
	if mode == "" && payload.TestMode {
		mode = game.ModeTest
	}
	if mode == "" {
		mode = game.ModeNormal
	}

	if _, ok := s.store.Room(roomID); !ok {
		s.store.CreateRoom(roomID)
	}
	if _, err := s.store.AddPlayer(roomID, clientID, name, mode); err != nil {
		emitError(client, err.Error())
		return
	}
	client.Join(siosocket.Room(roomID))
	client.SetData(roomID)

	room, _ := s.store.Room(roomID)
	s.broadcastPlayerJoined(roomID, room)

	room, _ = s.store.Room(roomID)
	if room.Mode == game.ModeTest || room.Mode == game.ModeSolo {
		go s.autofillAndStart(roomID)
	}
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
	_ = s.io.To(siosocket.Room(roomID)).Emit("gameStarted", map[string]any{
		"players": state.Players,
	})
	s.emitEvents(events)
	s.scheduleAfterEvents(roomID, events)
	return nil
}

func (s *Server) emitEvents(events []game.Event) {
	for _, event := range events {
		if event.SocketID != "" {
			_ = s.io.To(siosocket.Room(event.SocketID)).Emit(event.Name, event.Payload)
			continue
		}
		_ = s.io.To(siosocket.Room(event.RoomID)).Emit(event.Name, event.Payload)
	}
}

func (s *Server) scheduleAfterEvents(roomID string, events []game.Event) {
	room, ok := s.store.Room(roomID)
	if !ok || (room.Mode != game.ModeTest && room.Mode != game.ModeSolo) {
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
		switch payload["phase"] {
		case game.PhasePropose:
			go s.autoPropose(roomID)
		case game.PhaseDiscuss:
			go s.autoStartTeamVote(roomID, 12*time.Second)
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
			go s.autoStartMissionVote(roomID)
		case "vote":
			go s.autoMissionVote(roomID)
		}
	}
}

func (s *Server) autoPropose(roomID string) {
	time.Sleep(2 * time.Second)
	room, ok := s.store.Room(roomID)
	if !ok || room.Status != game.StatusPlaying || room.CurrentPhase != game.PhasePropose {
		return
	}
	teamSize := game.MissionTeamSize(humanCount(room), room.CurrentRound)
	pool := make([]string, 0, len(room.Players))
	for _, player := range room.Players {
		if !player.Eliminated {
			pool = append(pool, player.ID)
		}
	}
	if len(pool) < teamSize {
		return
	}
	team := rotatingTeam(pool, room.CurrentLeader, room.CurrentRound, teamSize)
	_, events, err := s.service.ProposeMission(roomID, room.CurrentLeader, team)
	if err != nil {
		return
	}
	s.emitEvents(events)
	s.scheduleAfterEvents(roomID, events)
}

func (s *Server) autoStartTeamVote(roomID string, delay time.Duration) {
	time.Sleep(delay)
	room, ok := s.store.Room(roomID)
	if !ok || room.CurrentPhase != game.PhaseDiscuss {
		return
	}
	_, events, err := s.service.StartTeamVote(roomID)
	if err != nil {
		return
	}
	s.emitEvents(events)

	room, ok = s.store.Room(roomID)
	if !ok || len(room.Players) == 0 {
		return
	}
	time.Sleep(500 * time.Millisecond)
	for _, player := range room.Players {
		if !player.IsAI && !player.Eliminated {
			_, events, err = s.service.TeamVote(roomID, player.ID, true)
			if err == nil {
				s.emitEvents(events)
				s.scheduleAfterEvents(roomID, events)
			}
			return
		}
	}
}

func (s *Server) autoMissionVote(roomID string) {
	time.Sleep(1 * time.Second)
	room, ok := s.store.Room(roomID)
	if !ok || room.CurrentPhase != game.PhaseMission || len(room.ProposedTeam) == 0 {
		return
	}
	voterID := ""
	for _, playerID := range room.ProposedTeam {
		if isAIPlayer(room, playerID) {
			voterID = playerID
			break
		}
	}
	if voterID == "" {
		return
	}
	_, events, err := s.service.MissionVote(roomID, voterID, game.MissionActionSupport)
	if err != nil {
		return
	}
	s.emitEvents(events)
	s.scheduleAfterEvents(roomID, events)
}

func (s *Server) autoStartMissionVote(roomID string) {
	time.Sleep(12 * time.Second)
	room, ok := s.store.Room(roomID)
	if !ok || room.CurrentPhase != game.PhaseMission {
		return
	}
	events := []game.Event{{
		Name:   "missionSubPhase",
		RoomID: roomID,
		Payload: map[string]any{
			"subPhase":  "vote",
			"timeLimit": 15,
		},
	}}
	s.emitEvents(events)
	s.scheduleAfterEvents(roomID, events)
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

func humanCount(room *game.Room) int {
	count := 0
	for _, player := range room.Players {
		if !player.IsAI && !player.Eliminated {
			count++
		}
	}
	return count
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
