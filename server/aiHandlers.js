/**
 * AI 相关逻辑处理器
 * 包含 AI 投票策略、行为逻辑等
 */

const { rooms } = require('./roomService');
const {
  ROLES, ROLE_FACTION, GAME_CONSTANTS
} = require('./gameData');
const {
  getHumanPlayers,
  getAIPlayers,
  timer
} = require('./utils');

/**
 * 创建 AI 处理器
 * @param {Object} options - 配置选项
 * @param {Object} options.io - Socket.IO 实例
 * @returns {Object} AI 处理器函数
 */
function createAIHandlers({ io }) {
  
  /**
   * AI 投票策略 - 决定 AI 是否同意小队
   * @param {Object} room - 房间对象
   * @param {Object} ai - AI 玩家对象
   * @returns {boolean} 是否同意
   */
  function calculateAITeamVote(room, ai) {
    const aiRole = room.roles[ai.id];
    const isGood = aiRole && ROLE_FACTION[aiRole] === 'good';
    const missionSuccesses = room.missionSuccesses || 0;
    const missionFailures = room.missionFailures || 0;
    const rejectStreak = room.rejectStreak || 0;
    
    let approveChance = 0.6; // 基础同意率
    
    // 根据角色调整
    if (isGood) {
      approveChance = 0.75; // 好人更倾向同意
    } else {
      approveChance = 0.55; // 坏人稍微倾向反对
    }
    
    // 根据游戏状态调整
    if (missionFailures >= 2) {
      // 坏人快赢了，好人更警惕
      if (isGood) approveChance -= 0.2;
      else approveChance += 0.1;
    }
    if (missionSuccesses >= 2) {
      // 好人快赢了，坏人更反对
      if (!isGood) approveChance -= 0.2;
    }
    
    // 连续否决后更倾向同意
    if (rejectStreak >= 3) {
      approveChance += 0.15;
    }
    
    // 小队中包含自己更倾向同意
    if (room.proposedTeam && room.proposedTeam.includes(ai.id)) {
      approveChance += 0.2;
    }
    
    // 限制范围
    approveChance = Math.max(0.2, Math.min(0.9, approveChance));
    
    return Math.random() < approveChance;
  }

  /**
   * AI 投票策略 - 决定 AI 在谜题中的答案
   * @param {Object} room - 房间对象
   * @param {string} aiId - AI 玩家ID
   * @returns {string} 答案选项 (A/B/C)
   */
  function calculateAIMissionVote(room, aiId) {
    const isInf = room.roles[aiId] === ROLES.INFILTRATOR;
    
    if (aiId === room.possessedPlayer && room.distortedPuzzle) {
      // 被附身的 AI 投扭曲版答案
      return room.distortedPuzzle.correctAnswer;
    } else if (isInf && Math.random() < 0.7) {
      // 渗透者 70% 概率破坏（选错误答案）
      const options = Object.keys(room.currentPuzzle.options);
      const wrongOptions = options.filter(o => o !== room.currentPuzzle.correctAnswer);
      return wrongOptions[Math.floor(Math.random() * wrongOptions.length)];
    } else {
      // 正常投票
      return room.currentPuzzle.correctAnswer;
    }
  }

  /**
   * AI 生成谜题讨论消息
   * @param {Object} room - 房间对象
   * @param {string} aiId - AI 玩家ID
   * @returns {string} 消息内容
   */
  function generateAIPuzzleChat(room, aiId) {
    if (aiId === room.possessedPlayer && room.distortedPuzzle) {
      return `我看到的是${room.distortedPuzzle.correctAnswer}`;
    } else {
      const correctAnswer = room.currentPuzzle.correctAnswer;
      return `我觉得是${correctAnswer}`;
    }
  }

  /**
   * 处理 AI 团队投票
   * @param {string} roomId - 房间ID
   * @param {Object} ai - AI 玩家对象
   * @param {Function} callback - 投票完成回调
   */
  function handleAITeamVote(roomId, ai, callback) {
    const room = rooms[roomId];
    if (!room || room.currentPhase !== 'team_vote') return;
    if (room.teamVotes[ai.id] !== undefined) return;
    
    const approve = calculateAITeamVote(room, ai);
    room.teamVotes[ai.id] = approve;
    
    console.log(`房间 ${roomId} AI ${ai.name} 投票: ${approve ? '同意' : '反对'}`);
    
    if (callback) callback(roomId);
  }

  /**
   * 处理 AI 谜题投票
   * @param {string} roomId - 房间ID
   * @param {string} aiId - AI 玩家ID
   * @param {Function} callback - 投票完成回调
   */
  function handleAIMissionVote(roomId, aiId, callback) {
    const room = rooms[roomId];
    if (!room || room.currentPhase !== 'mission' || room.missionSubPhase !== 'vote') return;
    if (room.puzzleAnswers[aiId] !== undefined) return;
    
    const answer = calculateAIMissionVote(room, aiId);
    room.puzzleAnswers[aiId] = answer;
    
    console.log(`房间 ${roomId} AI ${aiId} 投票: ${answer}`);
    
    if (callback) callback(roomId);
  }

  /**
   * 处理 AI 谜题讨论发言
   * @param {string} roomId - 房间ID
   * @param {string} aiId - AI 玩家ID
   * @param {Function} callback - 发言完成回调
   */
  function handleAIPuzzleChat(roomId, aiId, callback) {
    const room = rooms[roomId];
    if (!room || room.currentPhase !== 'mission' || room.missionSubPhase !== 'discuss') return;
    
    const aiPlayer = room.players.find(p => p.id === aiId);
    if (!aiPlayer) return;
    
    const message = generateAIPuzzleChat(room, aiId);
    
    room.puzzleChatLog.push({
      playerId: aiId,
      playerName: aiPlayer.name,
      message: message,
    });
    
    io.to(roomId).emit('puzzleChatBroadcast', {
      playerId: aiId,
      playerName: aiPlayer.name,
      message: message,
    });
    
    console.log(`房间 ${roomId} AI ${aiPlayer.name} 发言: ${message}`);
    
    if (callback) callback(roomId);
  }

  /**
   * 安排 AI 团队投票
   * @param {string} roomId - 房间ID
   */
  function scheduleAITeamVotes(roomId) {
    const room = rooms[roomId];
    if (!room) return;
    
    const aiVoters = room.players.filter(p => p.isAI && !p.eliminated);
    aiVoters.forEach(ai => {
      const delayTime = (timer(room, 2) * 1000) + Math.random() * (timer(room, 3) * 1000);
      setTimeout(() => {
        handleAITeamVote(roomId, ai, (roomId) => {
          const room = rooms[roomId];
          if (!room) return;
          
          // 检查是否所有人都投票了
          const allVoted = room.players.filter(p => !p.eliminated)
            .every(p => room.teamVotes[p.id] !== undefined);
          if (allVoted) {
            if (room.currentPhaseTimer) clearTimeout(room.currentPhaseTimer);
            // 通过事件触发投票结算，避免循环依赖
            io.to(roomId).emit('teamVoteComplete', { roomId });
          }
        });
      }, delayTime);
    });
  }

  /**
   * 安排 AI 谜题投票
   * @param {string} roomId - 房间ID
   */
  function scheduleAIMissionVotes(roomId) {
    const room = rooms[roomId];
    if (!room) return;
    
    const aiMembers = room.proposedTeam.filter(id => {
      const p = room.players.find(pp => pp.id === id);
      return p && p.isAI;
    });
    
    aiMembers.forEach(aiId => {
      setTimeout(() => {
        handleAIMissionVote(roomId, aiId, (roomId) => {
          const room = rooms[roomId];
          if (!room) return;
          
          // 检查所有人是否已投票
          const allVoted = room.proposedTeam.every(id =>
            room.puzzleAnswers[id] !== undefined ||
            room.players.find(p => p.id === id)?.isAI
          );
          
          if (allVoted) {
            if (room.currentPhaseTimer) clearTimeout(room.currentPhaseTimer);
            // 通过事件触发揭晓，避免循环依赖
            io.to(roomId).emit('puzzleVoteComplete', { roomId });
          }
        });
      }, (timer(room, 2) * 1000) + Math.random() * (timer(room, 3) * 1000));
    });
  }

  /**
   * 安排 AI 谜题讨论发言
   * @param {string} roomId - 房间ID
   */
  function scheduleAIPuzzleChats(roomId) {
    const room = rooms[roomId];
    if (!room) return;
    
    const aiMembers = room.proposedTeam.filter(id => {
      const p = room.players.find(pp => pp.id === id);
      return p && p.isAI;
    });
    
    aiMembers.forEach((aiId, idx) => {
      setTimeout(() => {
        handleAIPuzzleChat(roomId, aiId);
      }, (timer(room, 3) * 1000) + idx * (timer(room, 5) * 1000)); // 错开发言时间（测试模式加速）
    });
  }

  return {
    calculateAITeamVote,
    calculateAIMissionVote,
    generateAIPuzzleChat,
    handleAITeamVote,
    handleAIMissionVote,
    handleAIPuzzleChat,
    scheduleAITeamVotes,
    scheduleAIMissionVotes,
    scheduleAIPuzzleChats
  };
}

module.exports = { createAIHandlers };