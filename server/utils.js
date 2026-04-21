/**
 * 游戏工具函数
 */

const { rooms } = require('./roomService');
const { ROLE_DESCRIPTIONS } = require('./gameData');

/**
 * 获取房间中的真人玩家
 */
function getHumanPlayers(room) {
  return room.players.filter(p => !p.isAI);
}

/**
 * 获取房间中的AI玩家
 */
function getAIPlayers(room) {
  return room.players.filter(p => p.isAI);
}

/**
 * 获取存活的玩家
 */
function getAlivePlayers(room) {
  return room.players.filter(p => !p.eliminated);
}

/**
 * 获取存活的真人玩家
 */
function getAliveHumanPlayers(room) {
  return room.players.filter(p => !p.isAI && !p.eliminated);
}

/**
 * 根据ID获取玩家
 */
function getPlayerById(room, playerId) {
  return room.players.find(p => p.id === playerId);
}

/**
 * 检查玩家是否在房间中
 */
function isPlayerInRoom(room, playerId) {
  return room.players.some(p => p.id === playerId);
}

/**
 * 获取玩家在房间中的位置
 */
function getPlayerPosition(room, playerId) {
  const index = room.players.findIndex(p => p.id === playerId);
  return index >= 0 ? index : -1;
}

/**
 * 安全地发送Socket消息
 */
function safeEmit(socket, event, data) {
  try {
    if (socket && socket.connected) {
      socket.emit(event, data);
      return true;
    }
  } catch (err) {
    console.error(`发送 ${event} 失败:`, err);
  }
  return false;
}

/**
 * 安全地广播到房间
 */
function safeBroadcastToRoom(io, roomId, event, data) {
  try {
    io.to(roomId).emit(event, data);
    return true;
  } catch (err) {
    console.error(`广播 ${event} 到房间 ${roomId} 失败:`, err);
  }
  return false;
}

/**
 * 格式化时间（秒转分钟:秒）
 */
function formatTime(seconds) {
  const mins = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return `${mins}:${secs.toString().padStart(2, '0')}`;
}

/**
 * 生成随机ID
 */
function generateId(prefix = '') {
  const timestamp = Date.now();
  const random = Math.random().toString(36).substr(2, 9);
  return `${prefix}${timestamp}_${random}`;
}

/**
 * 延迟执行
 */
function delay(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

/**
 * 带超时的Promise
 */
function withTimeout(promise, timeoutMs, errorMessage = '操作超时') {
  return Promise.race([
    promise,
    new Promise((_, reject) => 
      setTimeout(() => reject(new Error(errorMessage)), timeoutMs)
    )
  ]);
}

module.exports = {
  getHumanPlayers,
  getAIPlayers,
  getAlivePlayers,
  getAliveHumanPlayers,
  getPlayerById,
  isPlayerInRoom,
  getPlayerPosition,
  safeEmit,
  safeBroadcastToRoom,
  formatTime,
  generateId,
  delay,
  withTimeout,
  timer,
  getRoleDescription,
  rotateLeader,
  addAIToRoom
};

/**
 * 测试模式加速计时器（秒）
 * @param {Object} room - 房间对象
 * @param {number} normalSeconds - 正常时间（秒）
 * @returns {number} 实际时间（秒）
 */
function timer(room, normalSeconds) {
  const isTest = room && (room.mode === 'test' || room.mode === 'solo');
  if (!isTest) return normalSeconds;
  // 测试/单人模式：各阶段压缩到 3~8 秒
  if (normalSeconds >= 30) return 8;
  if (normalSeconds >= 15) return 5;
  if (normalSeconds >= 8) return 3;
  return Math.max(2, normalSeconds);
}

/**
 * 获取角色描述（信号员额外获得角色列表信息）
 * @param {string} role - 角色
 * @param {Object} allRoles - 所有角色
 * @param {string} playerId - 玩家ID
 * @returns {string} 角色描述
 */
function getRoleDescription(role, allRoles, playerId) {
  const base = ROLE_DESCRIPTIONS[role];
  if (role === 'signal_keeper') {
    // 信号员知道自己是信号员
    return base;
  }
  return base;
}

/**
 * 队长顺移
 * @param {string} roomId - 房间ID
 */
function rotateLeader(roomId) {
  const room = rooms[roomId];
  if (!room) return;

  // 优先选择真人玩家，如果没有则选择 AI 玩家
  let alivePlayers = room.players.filter(p => !p.isAI && !p.eliminated);
  if (alivePlayers.length === 0) {
    alivePlayers = room.players.filter(p => !p.eliminated);
  }
  if (alivePlayers.length === 0) return;

  const currentIdx = alivePlayers.findIndex(p => p.id === room.currentLeader);
  const nextIdx = (currentIdx + 1) % alivePlayers.length;
  room.currentLeader = alivePlayers[nextIdx].id;
}

/**
 * 添加 AI 到房间（测试模式）
 * @param {string} roomId - 房间ID
 * @param {Object} io - Socket.IO 实例
 * @returns {Object|null} AI 玩家对象
 */
function addAIToRoom(roomId, io) {
  const room = rooms[roomId];
  if (!room) return null;
  const { addAIPlayer } = require('./roomService');
  const ai = addAIPlayer(roomId);
  if (ai) {
    console.log(`添加AI: ${ai.name}, room.mode: ${room.mode}`);
    io.to(roomId).emit('playerJoined', {
      players: room.players.map(p => ({
        id: p.id,
        name: p.name,
        isAI: p.isAI,
        position: p.position,
        eliminated: !!p.eliminated
      })),
      count: room.players.length,
      mode: room.mode || 'normal'
    });
  }
  return ai;
}
