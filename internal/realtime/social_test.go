package realtime

import (
	"strings"
	"testing"

	"whoisai/internal/game"
)

func TestAIDiscussionMessageUsesMissionHistory(t *testing.T) {
	t.Parallel()

	room := socialTestRoom()
	room.MissionResults = []bool{false}
	room.VoteHistory = []game.VoteRecord{{
		Approved: true,
		Team:     []string{"human", "ai-red"},
	}}
	room.ProposedTeam = []string{"human", "ai-red", "ai-blue"}

	got := aiDiscussionMessage(room, "ai-red")
	if !strings.Contains(got, "上轮失败队") {
		t.Errorf("aiDiscussionMessage(ai-red) = %q, want failed-team evidence", got)
	}
}

func TestAIStanceDecisionSuspectsFailedTeamMember(t *testing.T) {
	t.Parallel()

	room := socialTestRoom()
	room.MissionResults = []bool{false}
	room.VoteHistory = []game.VoteRecord{{
		Approved: true,
		Team:     []string{"human", "ai-red"},
	}}
	room.ProposedTeam = []string{"human", "ai-blue"}

	_, suspectID, reason := aiStanceDecision(room, "ai-blue")
	if suspectID != "human" && suspectID != "ai-red" {
		t.Errorf("aiStanceDecision(ai-blue) suspectID = %q, want a failed-team member", suspectID)
	}
	if !strings.Contains(reason, "失败") {
		t.Errorf("aiStanceDecision(ai-blue) reason = %q, want mission-history reasoning", reason)
	}
}

func TestDebugAdvanceRoomAllowsTestHost(t *testing.T) {
	t.Parallel()

	store := game.NewStore()
	store.CreateRoom("test-tools")
	if _, err := store.AddPlayer("test-tools", "host", "房主", game.ModeTest); err != nil {
		t.Fatalf("AddPlayer(host) error: %v", err)
	}
	server := &Server{store: store}

	room, err := server.debugAdvanceRoom("test-tools", "host")
	if err != nil {
		t.Fatalf("debugAdvanceRoom(test host) error: %v", err)
	}
	if room.Mode != game.ModeTest {
		t.Errorf("debugAdvanceRoom(test host) mode = %q, want %q", room.Mode, game.ModeTest)
	}
}

func socialTestRoom() *game.Room {
	return &game.Room{
		Players: []game.Player{
			{ID: "human", Name: "队长", Position: 0},
			{ID: "ai-red", Name: "AI_小红", Position: 1, IsAI: true},
			{ID: "ai-blue", Name: "AI_小蓝", Position: 2, IsAI: true},
			{ID: "ai-green", Name: "AI_小绿", Position: 3, IsAI: true},
		},
		CurrentLeader: "human",
	}
}
