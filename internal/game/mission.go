package game

type MissionScenario struct {
	ID          string
	Title       string
	Scenario    string
	Explanation string
}

var missionScenarios = []MissionScenario{
	{
		ID:          "contain-signal",
		Title:       "封锁异常信号",
		Scenario:    "行动小队进入异常信号区域，任务成败将检验这支队伍是否可信。",
		Explanation: "行动小队已完成秘密行动。具体行动者保持隐藏，请回到队长提名、投票记录和队内发言里寻找线索。",
	},
	{
		ID:          "escort-route",
		Title:       "护送数据核心",
		Scenario:    "行动小队护送数据核心穿过封锁区，任何渗透者都可能在途中留下破坏痕迹。",
		Explanation: "数据核心护送结束。系统只公开整体结果，不公开任何个人行动。",
	},
	{
		ID:          "crowd-evacuation",
		Title:       "重启安全协议",
		Scenario:    "行动小队负责重启安全协议。队伍是否干净，将直接反映在任务结果里。",
		Explanation: "安全协议重启流程结束。失败只说明小队内出现破坏行动，不说明具体是谁。",
	},
	{
		ID:          "supply-lock",
		Title:       "夺回控制节点",
		Scenario:    "行动小队接近关键控制节点。守护者会执行任务，渗透者可能选择秘密破坏。",
		Explanation: "控制节点行动结束。请结合本轮小队名单和支持票复盘。",
	},
}

func MissionScenarioForRound(roundNumber int) MissionScenario {
	if len(missionScenarios) == 0 {
		return MissionScenario{}
	}
	index := roundNumber - 1
	if index < 0 {
		index = 0
	}
	return missionScenarios[index%len(missionScenarios)]
}
