package game

type Mode string

const (
	ModeNormal Mode = "normal"
	ModeTest   Mode = "test"
	ModeSolo   Mode = "solo"
)

type Status string

const (
	StatusWaiting  Status = "waiting"
	StatusPlaying  Status = "playing"
	StatusFinished Status = "finished"
)

type Phase string

const (
	PhasePropose  Phase = "propose"
	PhaseDiscuss  Phase = "discuss"
	PhaseTeamVote Phase = "team_vote"
	PhaseMission  Phase = "mission"
)

type Role string

const (
	RoleEngineer     Role = "engineer"
	RoleInfiltrator  Role = "infiltrator"
	RoleSignalKeeper Role = "signal_keeper"
	RoleObserver     Role = "observer"
	RoleProtector    Role = "protector"
	RoleDisruptor    Role = "disruptor"
)

var RoleLabels = map[Role]string{
	RoleEngineer:     "🛡️ 守护者",
	RoleInfiltrator:  "🦠 渗透者",
	RoleSignalKeeper: "📡 侦测者",
	RoleObserver:     "👁️ 观察者",
	RoleProtector:    "🛡️ 护卫",
	RoleDisruptor:    "🎭 伪装者",
}

var roleDistribution = map[int]map[Role]int{
	5: {RoleEngineer: 2, RoleInfiltrator: 1, RoleSignalKeeper: 1, RoleObserver: 1},
	6: {RoleEngineer: 3, RoleInfiltrator: 1, RoleSignalKeeper: 1, RoleObserver: 1},
	7: {RoleEngineer: 3, RoleInfiltrator: 2, RoleSignalKeeper: 1, RoleObserver: 1},
	8: {RoleEngineer: 3, RoleInfiltrator: 2, RoleSignalKeeper: 1, RoleObserver: 1, RoleProtector: 1},
}

const (
	MinPlayers          = 5
	MaxPlayers          = 8
	MaxRounds           = 5
	MissionsToWin       = 3
	MaxMessagesPerRound = 6
	MaxMissionMessages  = 3
	MaxCharsPerMessage  = 50
)

type Player struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	IsAI         bool   `json:"isAI"`
	Position     int    `json:"position"`
	Eliminated   bool   `json:"eliminated"`
	Disconnected bool   `json:"disconnected,omitempty"`
}

type Room struct {
	ID                 string
	Players            []Player
	Status             Status
	Mode               Mode
	Roles              map[string]Role
	CurrentRound       int
	CurrentPhase       Phase
	MissionSubPhase    string
	MissionResults     []bool
	MissionSuccesses   int
	MissionFailures    int
	CurrentLeader      string
	ProposedTeam       []string
	RejectStreak       int
	MessageCount       map[string]int
	ChatMessages       []ChatMessage
	PossessedPlayer    string
	PossessionStyle    Style
	TeamVotes          map[string]bool
	MissionVotes       map[string]string
	VoteHistory        []VoteRecord
	SignalHistory      []SignalRecord
	NominationReason   string
	NominationHistory  []NominationRecord
	Stances            map[int]map[string]StanceRecord
	SuspicionEvents    []string
	DiscussionFocus    []string
	AutoManagedActions map[string][]AutoManagedAction
	StartedAtMillis    int64
	EndedAtMillis      int64
	PhaseDeadline      int64
	AutoTeamVotes      map[string]bool
	AutoMissionVotes   map[string]bool
	DebugMissionResult *bool
	DebugPaused        bool
	DebugRemainingMS   int64
}

type ChatMessage struct {
	PlayerID        string `json:"playerId"`
	PlayerName      string `json:"playerName"`
	Original        string `json:"original,omitempty"`
	Displayed       string `json:"displayed"`
	Possessed       bool   `json:"possessed"`
	Round           int    `json:"round"`
	Channel         string `json:"channel"`
	CreatedAtMillis int64  `json:"createdAt"`
}

type AutoManagedAction struct {
	Round           int    `json:"round"`
	Action          string `json:"action"`
	Message         string `json:"message"`
	CreatedAtMillis int64  `json:"createdAt"`
}

type PublicChatMessage struct {
	PlayerID        string `json:"playerId"`
	PlayerName      string `json:"playerName"`
	Displayed       string `json:"displayed"`
	Round           int    `json:"round"`
	Channel         string `json:"channel"`
	CreatedAtMillis int64  `json:"createdAt"`
}

type PublicVoteRecord struct {
	Round        int                   `json:"round"`
	Votes        map[string]PlayerVote `json:"votes"`
	Approved     bool                  `json:"approved"`
	Team         []string              `json:"team"`
	ApproveCount int                   `json:"approveCount"`
	RejectCount  int                   `json:"rejectCount"`
}

type PublicGameSnapshot struct {
	RoomID            string                          `json:"roomId"`
	Status            Status                          `json:"status"`
	Mode              Mode                            `json:"mode"`
	Players           []Player                        `json:"players"`
	CurrentRound      int                             `json:"currentRound"`
	CurrentPhase      Phase                           `json:"currentPhase"`
	MissionSubPhase   string                          `json:"missionSubPhase"`
	MissionTitle      string                          `json:"missionTitle,omitempty"`
	MissionResults    []bool                          `json:"missionResults"`
	MissionSuccesses  int                             `json:"missionSuccesses"`
	MissionFailures   int                             `json:"missionFailures"`
	CurrentLeader     string                          `json:"currentLeader"`
	ProposedTeam      []string                        `json:"proposedTeam"`
	RejectStreak      int                             `json:"rejectStreak"`
	ChatMessages      []PublicChatMessage             `json:"chatMessages"`
	VoteHistory       []PublicVoteRecord              `json:"voteHistory"`
	NominationReason  string                          `json:"nominationReason"`
	NominationHistory []NominationRecord              `json:"nominationHistory"`
	Stances           map[int]map[string]StanceRecord `json:"stances"`
	SuspicionEvents   []string                        `json:"suspicionEvents"`
	DiscussionFocus   []string                        `json:"discussionFocus"`
	PhaseDeadline     int64                           `json:"phaseDeadline"`
	DebugPaused       bool                            `json:"debugPaused"`
	DebugRemainingMS  int64                           `json:"debugRemainingMs"`
	StartedAtMillis   int64                           `json:"startedAt"`
	EndedAtMillis     int64                           `json:"endedAt"`
}

type PrivateGameSnapshot struct {
	PlayerID             string              `json:"playerId"`
	Role                 Role                `json:"role"`
	RoleLabel            string              `json:"roleLabel"`
	RoleDescription      string              `json:"roleDescription"`
	Possessed            bool                `json:"possessed"`
	SignalHistory        []SignalRecord      `json:"signalHistory,omitempty"`
	TeamVoteSubmitted    bool                `json:"teamVoteSubmitted"`
	MissionVoteSubmitted bool                `json:"missionVoteSubmitted"`
	MissionAction        string              `json:"missionAction,omitempty"`
	MessagesLeft         int                 `json:"messagesLeft"`
	MissionMessagesLeft  int                 `json:"missionMessagesLeft"`
	AutoManagedActions   []AutoManagedAction `json:"autoManagedActions"`
	Mission              *PrivateMission     `json:"mission,omitempty"`
}

type PrivateMission struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Scenario    string   `json:"scenario"`
	TeamNames   []string `json:"teamNames"`
	CanSabotage bool     `json:"canSabotage"`
	IsPossessed bool     `json:"isPossessed"`
}

type VoteRecord struct {
	Round        int                   `json:"round"`
	Votes        map[string]PlayerVote `json:"votes"`
	Approved     bool                  `json:"approved"`
	Team         []string              `json:"team"`
	ApproveCount int                   `json:"approveCount"`
	RejectCount  int                   `json:"rejectCount"`
	HasPossess   bool                  `json:"hasPossession"`
}

type PlayerVote struct {
	VoterName   string `json:"voterName"`
	Approved    bool   `json:"approved"`
	AutoManaged bool   `json:"autoManaged,omitempty"`
}

type SignalRecord struct {
	Round         int  `json:"round"`
	HasPossession bool `json:"hasPossession"`
}

type NominationRecord struct {
	Round      int      `json:"round"`
	LeaderID   string   `json:"leaderId"`
	LeaderName string   `json:"leaderName"`
	Team       []string `json:"team"`
	TeamNames  []string `json:"teamNames"`
	Reason     string   `json:"reason"`
}

type StanceRecord struct {
	Round       int    `json:"round"`
	PlayerID    string `json:"playerId"`
	PlayerName  string `json:"playerName"`
	TrustID     string `json:"trustId"`
	TrustName   string `json:"trustName"`
	SuspectID   string `json:"suspectId"`
	SuspectName string `json:"suspectName"`
	Reason      string `json:"reason"`
}

type RoleReveal struct {
	Name      string `json:"name"`
	Role      Role   `json:"role"`
	RoleLabel string `json:"roleLabel"`
	Faction   string `json:"faction"`
	IsWinner  bool   `json:"isWinner"`
}
