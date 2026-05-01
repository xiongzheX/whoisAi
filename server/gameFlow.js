/**
 * 谁是AI v3 — 游戏流程（阿瓦隆式社交推理 + AI 附身）
 *
 * 每轮流程：
 *   1. 提名阶段（队长提名小队）
 *   2. 讨论阶段（受限打字聊天，有人可能被 AI 附身）
 *   3. 全员投票（同意/反对这个小队）
 *   4. 执行阶段（小队成员讨论危机场景并选择行动方案）
 *   5. 结算（检查胜负条件）
 *
 * 游戏配置（6人局）：
 *   - 小队人数：[2, 3, 4, 3]
 *   - 最大轮次：4轮
 *   - 胜负条件：3次成功/失败
 *   - 聊天限制：每轮4条消息，每条30字
 *   - AI附身：50%概率，4种改写风格
 */

const { createPhaseHandlers, setMissionHandlers } = require('./phaseHandlers');
const { createAIHandlers } = require('./aiHandlers');
const { createMissionHandlers } = require('./missionHandlers');
const { timer, addAIToRoom } = require('./utils');

/**
 * 创建游戏流程控制器
 * @param {Object} options - 配置选项
 * @param {Object} options.io - Socket.IO 实例
 * @returns {Object} 游戏流程API
 */
function createGameFlow({ io }) {
  
  // 创建各个模块的处理器
  const phaseHandlers = createPhaseHandlers({ io });
  const aiHandlers = createAIHandlers({ io });
  const missionHandlers = createMissionHandlers({
    io,
    onGameOver: (roomId, winner) => phaseHandlers.endGame(roomId, winner),
    onNextRound: (roomId) => phaseHandlers.startRound(roomId)
  });
  
  // 设置任务处理器依赖，解决循环依赖
  setMissionHandlers(missionHandlers);
  
  // 包装 addAIToRoom 函数，注入 io 参数
  const wrappedAddAIToRoom = (roomId) => {
    return addAIToRoom(roomId, io);
  };
  
  // 注册事件监听器，处理模块间的通信
  io.on('connection', (socket) => {
    // 监听游戏结束事件
    socket.on('gameOver', ({ winner }) => {
      const roomId = socket.roomId;
      if (roomId) {
        phaseHandlers.endGame(roomId, winner);
      }
    });
    
    // 监听新一轮事件
    socket.on('nextRound', ({ roomId }) => {
      if (roomId) {
        phaseHandlers.startRound(roomId);
      }
    });
    
    // 监听团队投票完成事件
    socket.on('teamVoteComplete', ({ roomId }) => {
      if (roomId) {
        // 这里需要访问 phaseHandlers 中的 resolveTeamVote 函数
        // 但由于它是私有的，我们需要通过其他方式处理
        console.log(`房间 ${roomId} 团队投票完成`);
      }
    });
    
    // 监听谜题投票完成事件
    socket.on('puzzleVoteComplete', ({ roomId }) => {
      if (roomId) {
        // 这里需要访问 missionHandlers 中的 startMissionReveal 函数
        // 但由于它是私有的，我们需要通过其他方式处理
        console.log(`房间 ${roomId} 谜题投票完成`);
      }
    });
  });
  
  // 返回组合后的API
  return {
    // 主要游戏流程函数
    startGame: phaseHandlers.startGame,
    startRound: phaseHandlers.startRound,
    handleProposeMission: phaseHandlers.handleProposeMission,
    handleChat: phaseHandlers.handleChat,
    handleTeamVote: phaseHandlers.handleTeamVote,
    
    // 任务相关函数
    handleMissionVote: missionHandlers.handlePuzzleVote,  // 兼容旧事件名
    handlePuzzleVote: missionHandlers.handlePuzzleVote,
    handlePuzzleChat: missionHandlers.handlePuzzleChat,
    
    // 工具函数
    addAIToRoom: wrappedAddAIToRoom,
    
    // 任务阶段函数（供内部使用）
    startMissionPhase: missionHandlers.startMissionPhase,
    startMissionDiscuss: missionHandlers.startMissionDiscuss,
    startMissionVote: missionHandlers.startMissionVote,
    startMissionReveal: missionHandlers.startMissionReveal,
    
    // AI 调度函数
    scheduleAITeamVotes: aiHandlers.scheduleAITeamVotes,
    scheduleAIMissionVotes: aiHandlers.scheduleAIMissionVotes,
    scheduleAIPuzzleChats: aiHandlers.scheduleAIPuzzleChats,
    
    // 游戏结束函数
    endGame: phaseHandlers.endGame,
    
    // 获取各个处理器（用于测试或高级用法）
    getPhaseHandlers: () => phaseHandlers,
    getAIHandlers: () => aiHandlers,
    getMissionHandlers: () => missionHandlers
  };
}

module.exports = { createGameFlow };
