package game

import "math/rand"

func AssignRoles(playerIDs []string) map[string]Role {
	count := len(playerIDs)
	if count < 5 {
		count = 5
	}
	if count > 8 {
		count = 8
	}

	distribution := roleDistribution[count]
	roles := make([]Role, 0, len(playerIDs))
	for role, roleCount := range distribution {
		for i := 0; i < roleCount; i++ {
			roles = append(roles, role)
		}
	}
	rand.Shuffle(len(roles), func(i, j int) {
		roles[i], roles[j] = roles[j], roles[i]
	})

	assigned := make(map[string]Role, len(playerIDs))
	for i, id := range playerIDs {
		if i >= len(roles) {
			break
		}
		assigned[id] = roles[i]
	}
	return assigned
}

func RoleDescription(role Role) string {
	switch role {
	case RoleEngineer:
		return "你属于守护者阵营。目标是推动 3 次任务成功；首轮优先支持可信小队，观察谁在替高风险行动找理由。"
	case RoleInfiltrator:
		return "你是潜伏在队伍中的渗透者。目标是诱导 3 次任务失败；伪装成守护者，在讨论和执行阶段包装高风险行动。"
	case RoleSignalKeeper:
		return "你是侦测者。每轮开始时能感知是否有人被 AI 干扰；用线索引导讨论，但别太早暴露身份。"
	case RoleObserver:
		return "你是团队的观察者。通过分析投票模式找出可疑玩家；当前版本没有额外主动按钮。"
	case RoleProtector:
		return "你是护卫。守护者阵营的扩展身份；当前版本以发言和投票判断为主，没有额外主动按钮。"
	case RoleDisruptor:
		return "你是伪装者。渗透者阵营的扩展身份；当前版本通过发言、投票和任务行动制造误导。"
	default:
		return ""
	}
}
