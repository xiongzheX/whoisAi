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
	ID               string
	Players          []Player
	Status           Status
	Mode             Mode
	Roles            map[string]Role
	CurrentRound     int
	CurrentPhase     Phase
	MissionResults   []bool
	MissionSuccesses int
	MissionFailures  int
	CurrentLeader    string
	ProposedTeam     []string
	RejectStreak     int
	MessageCount     map[string]int
	ChatMessages     []ChatMessage
	PossessedPlayer  string
	PossessionStyle  Style
	TeamVotes        map[string]bool
	MissionVotes     map[string]string
	VoteHistory      []VoteRecord
	SignalHistory    []SignalRecord
}

type ChatMessage struct {
	PlayerID   string `json:"playerId"`
	PlayerName string `json:"playerName"`
	Original   string `json:"original,omitempty"`
	Displayed  string `json:"displayed"`
	Possessed  bool   `json:"possessed"`
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
	VoterName string `json:"voterName"`
	Approved  bool   `json:"approved"`
}

type SignalRecord struct {
	Round         int  `json:"round"`
	HasPossession bool `json:"hasPossession"`
}

type RoleReveal struct {
	Name      string `json:"name"`
	Role      Role   `json:"role"`
	RoleLabel string `json:"roleLabel"`
	Faction   string `json:"faction"`
	IsWinner  bool   `json:"isWinner"`
}
