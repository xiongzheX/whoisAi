/**
 * 谁是AI v3 — 游戏数据
 * 阿瓦隆式社交推理 + AI 附身干扰
 */

// ═══════════════════════════════════════
//  角色定义
// ═══════════════════════════════════════
const ROLES = {
  ENGINEER: 'engineer',           // 守护者（守护者阵营，兼容旧 key）
  INFILTRATOR: 'infiltrator',     // 渗透者（渗透者阵营）
  SIGNAL_KEEPER: 'signal_keeper', // 侦测者（守护者阵营，特殊）
  OBSERVER: 'observer',           // 观察者（守护者阵营，新增）
  PROTECTOR: 'protector',         // 护卫（守护者阵营，新增）
  DISRUPTOR: 'disruptor'          // 伪装者（渗透者阵营，新增）
};

const ROLE_LABELS = {
  [ROLES.ENGINEER]: '🛡️ 守护者',
  [ROLES.INFILTRATOR]: '🦠 渗透者',
  [ROLES.SIGNAL_KEEPER]: '📡 侦测者',
  [ROLES.OBSERVER]: '👁️ 观察者',
  [ROLES.PROTECTOR]: '🛡️ 护卫',
  [ROLES.DISRUPTOR]: '🎭 伪装者'
};

const ROLE_DESCRIPTIONS = {
  [ROLES.ENGINEER]:
    '你属于守护者阵营。你的目标是找出可信小队，推动 3 次任务成功。' +
    '在讨论中观察谁的发言风格异常——他们可能被 AI 附身了。',
  [ROLES.INFILTRATOR]:
    '你是潜伏在队伍中的渗透者。你的目标是让 3 次任务失败。' +
    '在讨论中你需要表现得像个守护者，投票时可投"同意"来伪装，' +
    '但在任务阶段应诱导小队选择高风险行动。小心不要暴露！',
  [ROLES.SIGNAL_KEEPER]:
    '你是团队的侦测者。你拥有特殊能力：每轮开始时，你能感知队伍中是否存在异常信号——' +
    '也就是说，你知道本轮是否有人被 AI 附身了（但你不知道是谁）。' +
    '利用这个信息引导讨论，但不要过早暴露你的身份，否则渗透者会针对你。' +
    '你的胜利条件和守护者相同。',
  [ROLES.OBSERVER]:
    '你是团队的观察者。你拥有特殊能力：每轮可以查看一名玩家的投票历史。' +
    '通过分析投票模式，找出可疑的玩家。你的胜利条件和守护者相同。',
  [ROLES.PROTECTOR]:
    '你是团队的护卫。你拥有特殊能力：每轮可以保护一名玩家不被 AI 附身。' +
    '保护关键玩家，降低发言被改写的风险。你的胜利条件和守护者相同。',
  [ROLES.DISRUPTOR]:
    '你是潜伏在团队中的伪装者。你拥有特殊能力：每轮可以改变一名玩家的投票。' +
    '利用这个能力制造混乱，帮助渗透者获胜。你的胜利条件和渗透者相同。'
};

const ROLE_FACTION = {
  [ROLES.ENGINEER]: 'good',
  [ROLES.INFILTRATOR]: 'evil',
  [ROLES.SIGNAL_KEEPER]: 'good',
  [ROLES.OBSERVER]: 'good',
  [ROLES.PROTECTOR]: 'good',
  [ROLES.DISRUPTOR]: 'evil'
};

// ═══════════════════════════════════════
//  角色分配（按人数）
// ═══════════════════════════════════════
const ROLE_DISTRIBUTIONS = {
  5: { engineers: 2, infiltrator: 1, signal_keeper: 1, observer: 1 },
  6: { engineers: 3, infiltrator: 1, signal_keeper: 1, observer: 1 },  // 调整为1个渗透者，平衡游戏
  7: { engineers: 3, infiltrator: 2, signal_keeper: 1, observer: 1 },
  8: { engineers: 3, infiltrator: 2, signal_keeper: 1, observer: 1, protector: 1 }
};

function getRoleDistribution(playerCount) {
  const count = Math.max(5, Math.min(8, playerCount));
  return ROLE_DISTRIBUTIONS[count];
}

/**
 * 为房间分配角色
 * @param {string[]} playerIds - 玩家 socketId 数组
 * @returns {Object} { [socketId]: role }
 */
function assignRoles(playerIds) {
  const dist = getRoleDistribution(playerIds.length);
  const roles = [];

  // 添加守护者
  for (let i = 0; i < dist.engineers; i++) roles.push(ROLES.ENGINEER);
  
  // 添加固定角色
  roles.push(ROLES.INFILTRATOR);
  roles.push(ROLES.SIGNAL_KEEPER);
  
  // 添加新角色
  if (dist.observer) roles.push(ROLES.OBSERVER);
  if (dist.protector) roles.push(ROLES.PROTECTOR);
  if (dist.disruptor) roles.push(ROLES.DISRUPTOR);

  // Fisher-Yates 洗牌
  for (let i = roles.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [roles[i], roles[j]] = [roles[j], roles[i]];
  }

  const result = {};
  playerIds.forEach((id, i) => {
    result[id] = roles[i];
  });
  return result;
}

// ═══════════════════════════════════════
//  游戏常量
// ═══════════════════════════════════════
const GAME_CONSTANTS = {
  // 时间（秒）
  PROPOSE_TIME: 15,              // 提名阶段
  DISCUSS_TIME: 45,              // 讨论阶段
  TEAM_VOTE_TIME: 20,            // 全员投票
  MISSION_VOTE_TIME: 10,         // 小队执行投票（旧，保留兼容）

  // 执行阶段子阶段时间（方向 C 重构）
  PUZZLE_DISCUSS_TIME: 25,       // 谜题讨论
  PUZZLE_VOTE_TIME: 10,          // 谜题投票
  PUZZLE_REVEAL_TIME: 8,         // 谜题揭晓

  // 谜题聊天限制
  PUZZLE_MAX_MESSAGES: 2,        // 谜题讨论每人最多 2 条
  PUZZLE_MAX_CHARS: 30,          // 谜题讨论每条最多 30 字
  
  // v3 游戏轮次
  MAX_ROUNDS: 5,                 // 最多 5 轮（从4轮增加到5轮）
  MISSIONS_TO_WIN: 3,            // 需要 3 次成功/失败决定胜负

  // 聊天限制
  MAX_MESSAGES_PER_ROUND: 6,     // 每人每轮最多 6 条消息
  MAX_CHARS_PER_MESSAGE: 50,     // 每条最多 50 字

  // AI 附身
  POSSESSION_CHANCE: 0.5,        // 每轮 50% 概率有人被附身

  // 小队人数（按轮次，6人局）
  MISSION_TEAM_SIZES: [2, 2, 2, 2],

  // 人数限制
  MIN_PLAYERS: 5,
  MAX_PLAYERS: 8
};

// ═══════════════════════════════════════
//  AI 附身风格（用于 possessionEngine）
// ═══════════════════════════════════════
const POSSESSION_STYLES = {
  polite: {
    label: '太礼貌',
    description: '把口语变正式，加"可能""或许"',
    // "我觉得不行" → "这个方案可能需要再考虑一下"
  },
  verbose: {
    label: '太完整',
    description: '在原句后追加多余解释',
    // "别选他" → "不建议选他，因为他的投票历史不够一致"
  },
  neutral: {
    label: '太中立',
    description: '把绝对化表述变模糊',
    // "他肯定是渗透者" → "他的行为确实有些可疑"
  },
  awkward: {
    label: '太新',
    description: '用不自然的正式措辞',
    // "笑死" → "这个确实很幽默"
  }
};

const POSSESSION_STYLE_KEYS = Object.keys(POSSESSION_STYLES);

// ═══════════════════════════════════════
//  任务小队人数配置（按轮次 + 人数）
// ═══════════════════════════════════════
const MISSION_TEAM_CONFIG = {
  5: [2, 3, 2, 3],  // 5人局
  6: [2, 3, 4, 3],  // 6人局
  7: [2, 3, 3, 4],  // 7人局
  8: [3, 4, 4, 4],  // 8人局
};

function getMissionTeamSize(playerCount, roundNumber) {
  const config = MISSION_TEAM_CONFIG[playerCount] || MISSION_TEAM_CONFIG[6];
  return config[Math.min(roundNumber, config.length - 1)];
}

module.exports = {
  ROLES,
  ROLE_LABELS,
  ROLE_DESCRIPTIONS,
  ROLE_FACTION,
  ROLE_DISTRIBUTIONS,
  getRoleDistribution,
  assignRoles,
  GAME_CONSTANTS,
  POSSESSION_STYLES,
  POSSESSION_STYLE_KEYS,
  MISSION_TEAM_CONFIG,
  getMissionTeamSize
};
