/**
 * 谁是AI v3 — 房间服务
 * 
 * 管理房间状态、玩家管理。
 * 游戏逻辑由 gameFlow.js 处理。
 */

const { rooms } = require('./state');
const { GAME_CONSTANTS } = require('./gameData');

/**
 * 创建房间（v3 状态）
 */
function createRoom(roomId) {
  // 不覆盖正在游戏中的房间
  if (rooms[roomId] && rooms[roomId].status === 'playing') {
    console.log(`房间 ${roomId} 正在游戏中，跳过创建`);
    return;
  }

  rooms[roomId] = {
    // 基本状态
    players: [],              // [{ id, name, isAI, position, eliminated }]
    status: 'waiting',        // 'waiting' | 'playing' | 'finished'
    mode: 'normal',           // 'normal' | 'test' | 'solo'

    // v3: 角色分配
    roles: {},                // { socketId: 'engineer' | 'infiltrator' | 'signal_keeper' }

    // v3: 轮次状态
    currentRound: 0,
    currentPhase: null,       // 'propose' | 'discuss' | 'team_vote' | 'mission'
    missionResults: [],       // [true, false, true, ...] 每轮任务结果
    missionSuccesses: 0,      // 好人胜利计数
    missionFailures: 0,       // 坏人胜利计数

    // v3: 提名阶段
    currentLeader: null,      // 当前队长 socketId
    proposedTeam: [],         // 当前提名的小队
    rejectStreak: 0,          // 连续否决次数（5 次 = 坏人赢）

    // v3: 讨论阶段
    messageCount: {},         // { socketId: number } 每人已发消息数
    chatMessages: [],         // 本轮聊天记录 [{ playerId, playerName, original, displayed, possessed }]

    // v3: AI 附身
    possessedPlayer: null,    // 本轮被附身的玩家 socketId（null=无人被附身）
    possessionStyle: null,    // 附身风格 ('polite'|'verbose'|'neutral'|'awkward')

    // v3: 投票
    teamVotes: {},            // { socketId: boolean } 对小队的投票
    missionVotes: {},         // { socketId: boolean } 小队成员的任务投票
    
    // v3: 投票历史
    voteHistory: [],          // [{ round, votes: { voterId: { name, approved } }, approved, team: [] }]
    
    // v3: 信号员历史
    signalHistory: [],        // [{ round, hasPossession }]

    // v3: 题库
    topics: [],               // 本轮争议话题

    // 计时器
    currentPhaseTimer: null,
    phaseStartTime: null,

    // 加入锁（防止竞态）
    _joining: false,

    // 测试模式定时器
    testModeTimer: null
  };

  console.log(`房间 ${roomId} 已创建`);
}

/**
 * 重置房间
 */
function resetRoom(roomId) {
  if (rooms[roomId]) {
    if (rooms[roomId].currentPhaseTimer) {
      clearTimeout(rooms[roomId].currentPhaseTimer);
    }
    if (rooms[roomId].testModeTimer) {
      clearTimeout(rooms[roomId].testModeTimer);
    }
    // 清理重连定时器
    if (rooms[roomId].reconnectTimers) {
      for (const timerId of Object.values(rooms[roomId].reconnectTimers)) {
        clearTimeout(timerId);
      }
    }
    // 删除现有房间，然后创建新房间
    delete rooms[roomId];
  }
  // 创建新房间
  createRoom(roomId);
}

/**
 * 清除房间
 */
function clearRoom(roomId) {
  resetRoom(roomId);
}

/**
 * 添加玩家
 * @returns {{ success?: boolean, error?: string }}
 */
function addPlayer(roomId, socketId, name, mode) {
  const room = rooms[roomId];
  if (!room) return { error: '房间不存在' };
  if (room.status === 'playing') return { error: '游戏已开始' };

  // 添加加入锁，防止竞态条件
  if (room._joining) return { error: '房间正在处理加入请求，请稍后重试' };
  room._joining = true;

  try {
    const maxPlayers = GAME_CONSTANTS.MAX_PLAYERS;
    if (room.players.length >= maxPlayers) return { error: '房间已满' };
    if (room.players.find(p => p.id === socketId)) return { error: '已在房间中' };

    room.players.push({
      id: socketId,
      name: name || `玩家${room.players.length + 1}`,
      isAI: false,
      position: room.players.length,
      eliminated: false
    });
    room.mode = mode || 'normal';
    return { success: true };
  } finally {
    room._joining = false;
  }
}

/**
 * 添加 AI 玩家（测试模式用）
 * v3 中 AI 不参与附身，只填补空位
 */
function addAIPlayer(roomId) {
  try {
    const room = rooms[roomId];
    if (!room) return null;

    // 检查房间人数限制
    const maxPlayers = GAME_CONSTANTS.MAX_PLAYERS;
    if (room.players.length >= maxPlayers) {
      console.log(`房间 ${roomId} 已满，无法添加 AI 玩家`);
      return null;
    }

    const aiNames = [
      'AI_小红', 'AI_小蓝', 'AI_小绿', 'AI_小紫',
      'AI_小橙', 'AI_小黄', 'AI_小青', 'AI_小粉'
    ];
    const usedNames = room.players.map(p => p.name);
    const availableName = aiNames.find(n => !usedNames.includes(n))
      || `AI_${room.players.length + 1}`;

    const aiPlayer = {
      id: `ai_${Date.now()}_${Math.random().toString(36).slice(2, 6)}`,
      name: availableName,
      isAI: true,
      position: room.players.length,
      eliminated: false
    };
    room.players.push(aiPlayer);
    return aiPlayer;
  } catch (err) {
    console.error(`添加 AI 玩家失败:`, err);
    return null;
  }
}

/**
 * 移除玩家
 */
function removePlayer(roomId, socketId) {
  const room = rooms[roomId];
  if (!room) return;
  room.players = room.players.filter(p => p.id !== socketId);
}

/**
 * 获取存活玩家（未淘汰 + 非 AI）
 */
function getAlivePlayers(roomId) {
  const room = rooms[roomId];
  if (!room) return [];
  return room.players.filter(p => !p.eliminated && !p.isAI);
}

/**
 * 获取玩家信息
 */
function getPlayer(roomId, socketId) {
  const room = rooms[roomId];
  if (!room) return null;
  return room.players.find(p => p.id === socketId) || null;
}

/**
 * 获取玩家列表（用于广播的精简格式）
 */
function getPlayerList(room) {
  return room.players.map(p => ({
    id: p.id,
    name: p.name,
    isAI: !!p.isAI,
    position: p.position,
    eliminated: !!p.eliminated
  }));
}

module.exports = {
  rooms,
  createRoom,
  resetRoom,
  clearRoom,
  addPlayer,
  addAIPlayer,
  removePlayer,
  getAlivePlayers,
  getPlayer,
  getPlayerList
};
