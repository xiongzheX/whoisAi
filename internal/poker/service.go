package poker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"whoisai/internal/platform"
)

type room struct {
	target                     int
	table                      *Table
	version                    uint64
	sessionID                  string
	deadline, moveAfter, botAt time.Time
	seen                       map[string]time.Time
	updated                    time.Time
}

// Service serializes room mutations and ticks; snapshots are built under the same lock.
type Service struct {
	mu    sync.Mutex
	store *platform.MemoryStore
	rooms map[string]*room
	now   func() time.Time
}

func NewService(store *platform.MemoryStore) *Service {
	return &Service{store: store, rooms: map[string]*room{}, now: time.Now}
}

// Run stops when ctx is cancelled. The caller owns and waits for its goroutine.
func (s *Service) Run(ctx context.Context) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.tick(now)
		}
	}
}
func memberID(code, token string) string {
	sum := sha256.Sum256([]byte(code + ":" + token))
	return "poker_" + hex.EncodeToString(sum[:16])
}
func isMember(party *platform.PartyRoom, id string) bool {
	for _, m := range party.Members {
		if m.ID == id && m.ConnectionStatus != platform.MemberLeft {
			return true
		}
	}
	return false
}
func (s *Service) setTurn(r *room, now time.Time, action bool) {
	r.moveAfter = now
	if action {
		r.moveAfter = now.Add(1200 * time.Millisecond)
	}
	r.deadline = time.Time{}
	r.botAt = time.Time{}
	if r.table.Actor < 0 {
		return
	}
	r.deadline = r.moveAfter.Add(20 * time.Second)
	if r.table.Players[r.table.Actor].Bot {
		style := botStyles[r.table.Actor%len(botStyles)]
		delay := float64(800+r.table.random(1400)) * style.pace
		r.botAt = r.moveAfter.Add(time.Duration(delay) * time.Millisecond)
	}
}
func (s *Service) afterAction(code string, r *room, now time.Time, timedOut bool) {
	r.version++
	r.updated = now
	r.table.LastAction.Sequence = r.version
	r.table.LastAction.TimedOut = timedOut
	if timedOut {
		r.table.record(r.table.Players[r.table.LastAction.Seat].Name + " · 超时自动" + r.table.LastAction.Text)
	}
	s.setTurn(r, now, true)
	if r.table.Phase == "done" {
		summary, _ := json.Marshal(map[string]any{"result": r.table.Result, "payouts": r.table.Payouts})
		_, _ = s.store.FinishSession(code, summary)
	}
}
func (s *Service) tick(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for code, r := range s.rooms {
		party, ok := s.store.RoomByCode(code)
		if !ok {
			delete(s.rooms, code)
			continue
		}
		for _, m := range party.Members {
			if now.Sub(r.seen[m.ID]) > 15*time.Second && m.ConnectionStatus == platform.MemberOnline {
				_, _ = s.store.SetConnection(code, m.ID, platform.MemberOffline)
			}
		}
		if r.table != nil && r.table.Phase != "done" {
			if !r.botAt.IsZero() && !now.Before(r.botAt) {
				kind, amount := r.table.botAction()
				if r.table.act(kind, amount) == nil {
					s.afterAction(code, r, now, false)
				}
			} else if !r.deadline.IsZero() && !now.Before(r.deadline) {
				kind := "call"
				if r.table.options().Call > 0 {
					kind = "fold"
				}
				if r.table.act(kind, 0) == nil {
					s.afterAction(code, r, now, true)
				}
			}
		} else {
			// Reclaim abandoned lobby seats and transfer the host using the shared platform rules.
			for _, m := range party.Members {
				if now.Sub(r.seen[m.ID]) > 90*time.Second {
					_, _ = s.store.LeaveRoom(code, m.ID)
					delete(r.seen, m.ID)
					r.table = nil
					r.version++
				}
			}
		}
	}
}

type request struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Target    int    `json:"target"`
	Kind      string `json:"kind"`
	Amount    int    `json:"amount"`
	Version   uint64 `json:"version"`
	SessionID string `json:"sessionId"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
func (s *Service) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" && req.Method != "POST" {
		w.Header().Set("Allow", "GET, POST")
		writeJSON(w, 405, map[string]string{"error": "不支持的方法"})
		return
	}
	token := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
	if len(token) < 24 || len(token) > 100 {
		writeJSON(w, 401, map[string]string{"error": "缺少玩家身份，请重新加入"})
		return
	}
	input := request{}
	operation := strings.TrimPrefix(req.URL.Path, "/api/poker/")
	if req.Method == "POST" {
		req.Body = http.MaxBytesReader(w, req.Body, 4096)
		dec := json.NewDecoder(req.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&input); err != nil {
			writeJSON(w, 400, map[string]string{"error": "参数格式错误"})
			return
		}
		if err := dec.Decode(&struct{}{}); err != io.EOF {
			writeJSON(w, 400, map[string]string{"error": "参数格式错误"})
			return
		}
	} else {
		if operation != "state" {
			writeJSON(w, 405, map[string]string{"error": "请使用 POST"})
			return
		}
		input.Code = req.URL.Query().Get("code")
	}
	if req.Method == "POST" && operation == "state" {
		writeJSON(w, 405, map[string]string{"error": "请使用 GET"})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	code, id, err := s.handle(operation, input, token, now)
	if err != nil {
		status := 400
		if errors.Is(err, platform.ErrRoomNotFound) {
			status = 404
		}
		if errors.Is(err, platform.ErrMemberNotFound) {
			status = 403
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	if operation == "leave" {
		writeJSON(w, 200, map[string]bool{"left": true})
		return
	}
	writeJSON(w, 200, s.snapshot(code, id, now))
}
func (s *Service) handle(op string, in request, token string, now time.Time) (string, string, error) {
	code := strings.TrimSpace(in.Code)
	id := memberID(code, token)
	if op == "create" {
		name := strings.TrimSpace(in.Name)
		if utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 12 {
			return "", "", errors.New("昵称需为 1–12 个字")
		}
		if in.Target < 2 || in.Target > 9 {
			return "", "", errors.New("总人数必须为 2–9 人")
		}
		code = platform.GenerateRoomCode("texas-holdem")
		id = memberID(code, token)
		_, err := s.store.CreateRoom(platform.CreateRoomInput{Code: code, HostID: id, HostName: name, GameID: "texas-holdem"})
		if err != nil {
			return "", "", err
		}
		s.rooms[code] = &room{target: in.Target, version: 1, seen: map[string]time.Time{id: now}, updated: now}
		return code, id, nil
	}
	r, ok := s.rooms[code]
	if !ok {
		return "", "", platform.ErrRoomNotFound
	}
	party, ok := s.store.RoomByCode(code)
	if !ok {
		return "", "", platform.ErrRoomNotFound
	}
	if op == "join" {
		name := strings.TrimSpace(in.Name)
		if utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 12 {
			return "", "", errors.New("昵称需为 1–12 个字")
		}
		if !isMember(party, id) && (r.table != nil || len(party.Members) >= r.target) {
			return "", "", errors.New("本桌已开局或真人席位已满，请等待房主回到等待房")
		}
		if _, _, err := s.store.JoinRoom(code, id, name); err != nil {
			return "", "", err
		}
		r.seen[id] = now
		r.version++
		return code, id, nil
	}
	if !isMember(party, id) {
		return "", "", platform.ErrMemberNotFound
	}
	r.seen[id] = now
	_, _ = s.store.SetConnection(code, id, platform.MemberOnline)
	switch op {
	case "state":
	case "configure":
		if party.HostMemberID != id {
			return "", "", platform.ErrNotHost
		}
		if r.table != nil {
			return "", "", errors.New("请先回到等待房")
		}
		if in.Target < max(2, len(party.Members)) || in.Target > 9 {
			return "", "", errors.New("总人数不能少于真人数，且必须在 2–9 人之间")
		}
		r.target = in.Target
		r.version++
	case "start":
		if party.HostMemberID != id {
			return "", "", platform.ErrNotHost
		}
		if r.table != nil {
			return "", "", errors.New("已经开局")
		}
		if in.Version != r.version {
			return "", "", errors.New("房间成员或设置已变化，请确认后重试")
		}
		if len(party.Members) > r.target {
			return "", "", errors.New("真人数超过设定总人数")
		}
		players := []Player{}
		for _, m := range party.Members {
			if now.Sub(r.seen[m.ID]) > 15*time.Second {
				return "", "", errors.New("请等待断线玩家重连，或等待其席位释放")
			}
			players = append(players, Player{ID: m.ID, Name: m.DisplayName, Chips: 1000})
		}
		names := []string{"桃桃", "阿栗", "慢慢", "团团", "小橘", "绵绵", "咕咕", "豆包"}
		for len(players) < r.target {
			i := len(players)
			players = append(players, Player{ID: fmt.Sprintf("bot-%d", i), Name: names[(i-1)%len(names)], Bot: true, Chips: 1000})
		}
		r.table = newTable(players)
		if err := s.startHand(code, r, party, now); err != nil {
			r.table = nil
			return "", "", err
		}
	case "next":
		if party.HostMemberID != id {
			return "", "", platform.ErrNotHost
		}
		if r.table == nil || r.table.Phase != "done" || in.Version != r.version || in.SessionID != r.sessionID {
			return "", "", errors.New("请等待本局结束并刷新状态")
		}
		if err := s.startHand(code, r, party, now); err != nil {
			return "", "", err
		}
	case "lobby":
		if party.HostMemberID != id {
			return "", "", platform.ErrNotHost
		}
		if r.table != nil && r.table.Phase != "done" {
			return "", "", errors.New("本局未结束，不能重置牌桌")
		}
		r.table = nil
		r.sessionID = ""
		r.version++
	case "action":
		if r.table == nil || r.table.Actor < 0 || r.table.Players[r.table.Actor].ID != id {
			return "", "", errors.New("尚未轮到你")
		}
		if in.SessionID != r.sessionID || in.Version != r.version {
			return "", "", errors.New("操作已过期，请等待最新状态")
		}
		if now.Before(r.moveAfter) || !now.Before(r.deadline) {
			return "", "", errors.New("不在操作时间内")
		}
		if err := r.table.act(in.Kind, in.Amount); err != nil {
			return "", "", err
		}
		s.afterAction(code, r, now, false)
	case "leave":
		if r.table != nil && r.table.Phase != "done" {
			return "", "", errors.New("本局进行中，请等待结束；断线后仍按限时自动操作")
		}
		if _, err := s.store.LeaveRoom(code, id); err != nil {
			return "", "", err
		}
		delete(r.seen, id)
		r.table = nil
		r.sessionID = ""
		r.version++
	default:
		return "", "", errors.New("未知操作")
	}
	r.updated = now
	return code, id, nil
}
func (s *Service) startHand(code string, r *room, party *platform.PartyRoom, now time.Time) error {
	alive := 0
	participants := []platform.ParticipantInput{}
	for i, p := range r.table.Players {
		if p.Chips <= 0 {
			continue
		}
		alive++
		kind := platform.ParticipantHuman
		member := p.ID
		if p.Bot {
			kind = platform.ParticipantBot
			member = ""
		}
		participants = append(participants, platform.ParticipantInput{ParticipantKey: p.ID, MemberID: member, DisplayName: p.Name, Kind: kind, Seat: i})
	}
	if alive < 2 {
		return errors.New("剩余玩家不足，请回到等待房重新开桌")
	}
	settings, _ := json.Marshal(map[string]any{"target": r.target, "authority": "server"})
	session, err := s.store.StartSession(platform.StartSessionInput{RoomCode: code, HostID: party.HostMemberID, Mode: "poker", Settings: settings, Participants: participants})
	if err != nil {
		return err
	}
	if err := r.table.start(); err != nil {
		_, _ = s.store.AbandonSession(code)
		return err
	}
	r.sessionID = session.ID
	r.version++
	s.setTurn(r, now, false)
	if r.table.Phase == "done" {
		summary, _ := json.Marshal(map[string]any{"result": r.table.Result, "payouts": r.table.Payouts})
		_, _ = s.store.FinishSession(code, summary)
	}
	return nil
}
func (s *Service) snapshot(code, id string, now time.Time) map[string]any {
	r := s.rooms[code]
	party, _ := s.store.RoomByCode(code)
	state := map[string]any{"code": code, "playerId": id, "hostId": party.HostMemberID, "members": party.Members, "target": r.target, "version": r.version, "sessionId": r.sessionID, "serverTime": now.UnixMilli(), "deadline": int64(0), "moveAfter": r.moveAfter.UnixMilli(), "table": nil}
	if r.table == nil {
		return state
	}
	t := r.table
	players := []map[string]any{}
	own := -1
	for i, p := range t.Players {
		cards := make([]*Card, len(p.Cards))
		if p.ID == id || t.Revealed && !p.Folded {
			for j := range p.Cards {
				c := p.Cards[j]
				cards[j] = &c
			}
		}
		if p.ID == id {
			own = i
		}
		players = append(players, map[string]any{"id": p.ID, "name": p.Name, "bot": p.Bot, "chips": p.Chips, "cards": cards, "bet": p.Bet, "total": p.Total, "folded": p.Folded, "action": p.Action})
	}
	state["selfSeat"] = own
	if !r.deadline.IsZero() {
		state["deadline"] = r.deadline.UnixMilli()
	}
	var options *Options
	if own == t.Actor {
		options = t.options()
	}
	state["table"] = map[string]any{"players": players, "dealer": t.Dealer, "actor": t.Actor, "hand": t.Hand, "phase": t.Phase, "board": t.Board, "pot": t.Pot, "lastPot": t.LastPot, "payouts": t.Payouts, "revealed": t.Revealed, "result": t.Result, "log": t.Log, "currentBet": t.CurrentBet, "lastAction": t.LastAction, "options": options}
	return state
}
