package game

import (
	"errors"
	"time"
)

func (s *Store) GameSnapshot(roomID, playerID string) (PublicGameSnapshot, PrivateGameSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	room, ok := s.rooms[roomID]
	if !ok {
		return PublicGameSnapshot{}, PrivateGameSnapshot{}, errors.New("房间不存在")
	}
	if _, ok := findPlayer(room, playerID); !ok {
		return PublicGameSnapshot{}, PrivateGameSnapshot{}, errors.New("玩家不在房间中")
	}

	public := PublicGameSnapshot{
		RoomID:            room.ID,
		Status:            room.Status,
		Mode:              room.Mode,
		Players:           append([]Player(nil), room.Players...),
		CurrentRound:      room.CurrentRound,
		CurrentPhase:      room.CurrentPhase,
		MissionSubPhase:   room.MissionSubPhase,
		MissionResults:    append([]bool(nil), room.MissionResults...),
		MissionSuccesses:  room.MissionSuccesses,
		MissionFailures:   room.MissionFailures,
		CurrentLeader:     room.CurrentLeader,
		ProposedTeam:      append([]string(nil), room.ProposedTeam...),
		RejectStreak:      room.RejectStreak,
		ChatMessages:      publicChatMessages(room.ChatMessages),
		VoteHistory:       publicVoteHistory(room.VoteHistory),
		NominationReason:  room.NominationReason,
		NominationHistory: cloneNominationHistory(room.NominationHistory),
		Stances:           cloneStances(room.Stances),
		SuspicionEvents:   append([]string(nil), room.SuspicionEvents...),
		DiscussionFocus:   append([]string(nil), room.DiscussionFocus...),
		PhaseDeadline:     room.PhaseDeadline,
		DebugPaused:       room.DebugPaused,
		DebugRemainingMS:  room.DebugRemainingMS,
		StartedAtMillis:   room.StartedAtMillis,
		EndedAtMillis:     room.EndedAtMillis,
	}
	if room.CurrentPhase == PhaseMission {
		public.MissionTitle = MissionScenarioForRound(room.CurrentRound).Title
	}

	private := PrivateGameSnapshot{
		PlayerID:             playerID,
		Role:                 room.Roles[playerID],
		RoleLabel:            RoleLabels[room.Roles[playerID]],
		RoleDescription:      RoleDescription(room.Roles[playerID]),
		Possessed:            room.PossessedPlayer == playerID,
		TeamVoteSubmitted:    mapContains(room.TeamVotes, playerID),
		MissionVoteSubmitted: mapContains(room.MissionVotes, playerID),
		MissionAction:        room.MissionVotes[playerID],
		MessagesLeft:         max(0, MaxMessagesPerRound-room.MessageCount[playerID]),
		MissionMessagesLeft:  max(0, MaxMissionMessages-room.MessageCount["mission:"+playerID]),
		AutoManagedActions:   append([]AutoManagedAction(nil), room.AutoManagedActions[playerID]...),
	}
	if room.Roles[playerID] == RoleSignalKeeper {
		private.SignalHistory = append([]SignalRecord(nil), room.SignalHistory...)
	}
	if room.CurrentPhase == PhaseMission && contains(room.ProposedTeam, playerID) {
		scenario := MissionScenarioForRound(room.CurrentRound)
		private.Mission = &PrivateMission{
			ID:          scenario.ID,
			Title:       scenario.Title,
			Scenario:    scenario.Scenario,
			TeamNames:   playerNames(room, room.ProposedTeam),
			CanSabotage: roleFaction(room.Roles[playerID]) == "evil",
			IsPossessed: room.PossessedPlayer == playerID,
		}
	}
	return public, private, nil
}

func publicChatMessages(messages []ChatMessage) []PublicChatMessage {
	result := make([]PublicChatMessage, 0, len(messages))
	for _, message := range messages {
		result = append(result, PublicChatMessage{
			PlayerID: message.PlayerID, PlayerName: message.PlayerName, Displayed: message.Displayed,
			Round: message.Round, Channel: message.Channel, CreatedAtMillis: message.CreatedAtMillis,
		})
	}
	return result
}

func publicVoteHistory(records []VoteRecord) []PublicVoteRecord {
	result := make([]PublicVoteRecord, 0, len(records))
	for _, record := range records {
		result = append(result, PublicVoteRecord{
			Round: record.Round, Votes: copyMap(record.Votes), Approved: record.Approved,
			Team: append([]string(nil), record.Team...), ApproveCount: record.ApproveCount,
			RejectCount: record.RejectCount,
		})
	}
	return result
}

func (s *Store) RecordAutoManaged(roomID, playerID, action, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	room, ok := s.rooms[roomID]
	if !ok {
		return errors.New("房间不存在")
	}
	if _, ok := findPlayer(room, playerID); !ok {
		return errors.New("玩家不在房间中")
	}
	if room.AutoManagedActions == nil {
		room.AutoManagedActions = make(map[string][]AutoManagedAction)
	}
	room.AutoManagedActions[playerID] = append(room.AutoManagedActions[playerID], AutoManagedAction{
		Round: room.CurrentRound, Action: action, Message: message, CreatedAtMillis: time.Now().UnixMilli(),
	})
	return nil
}

func mapContains[M ~map[string]V, V any](values M, key string) bool {
	_, ok := values[key]
	return ok
}
