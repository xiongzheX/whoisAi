/**
 * 任务执行逻辑处理器
 * 包含任务阶段、谜题讨论、投票、揭晓等逻辑
 */

// 简易日志函数（utils.js 移除此功能）
function logGame(roomId, msg) {
  console.log(`[GAME] ${roomId}: ${msg}`);
}
function logWarn(roomId, msg) {
  console.warn(`[WARN] ${roomId}: ${msg}`);
}


const { rooms } = require('./roomService');
const {
  ROLES, ROLE_FACTION, GAME_CONSTANTS
} = require('./gameData');
const { rewriteMessage, randomStyle, shouldPossess, selectPossessedPlayer, distortPuzzle, getPossessedHint } = require('./possessionEngine');
const { pickPuzzle, distortPuzzle: dpDistort } = require('./puzzleBank');
const {
  getHumanPlayers,
  timer,
  rotateLeader
} = require('./utils');

/**
 * 创建任务处理器
 * @param {Object} options - 配置选项
 * @param {Object} options.io - Socket.IO 实例
 * @returns {Object} 任务处理器函数
 */
function createMissionHandlers({ io }) {
  
  /**
   * 执行阶段入口 → 直接发题
   * @param {string} roomId - 房间ID
   */
  function startMissionPhase(roomId) {
    const room = rooms[roomId];
    if (!room) return;

    room.currentPhase = 'mission';
    room.missionVotes = {};
    room.puzzleAnswers = {};    // 答案 { [playerId]: 'A'|'B'|'C' }
    room.puzzleChatCount = {};  // 聊天计数
    room.puzzleChatLog = [];    // 聊天记录（用于揭晓）

    // 选题
    const puzzle = pickPuzzle(room.currentRound);
    room.currentPuzzle = puzzle;

    // 决定是否附身（50% 概率，仅在有小队时）
    if (shouldPossess(0.5) && room.proposedTeam.length > 0) {
      const teamIds = room.proposedTeam.filter(id => {
        const p = room.players.find(pp => pp.id === id);
        return p && !p.isAI;
      });
      room.possessedPlayer = selectPossessedPlayer(teamIds);
      room.possessionStyle = randomStyle();

      // 生成扭曲版题目
      if (room.possessedPlayer) {
        room.distortedPuzzle = dpDistort(puzzle);
        const possessed = room.players.find(p => p.id === room.possessedPlayer);
        console.log(`房间 ${roomId} 第 ${room.currentRound} 轮执行阶段：${possessed?.name} 被附身（${room.possessionStyle}，逻辑扭曲）`);
      }
    } else {
      room.possessedPlayer = null;
      room.possessionStyle = null;
      room.distortedPuzzle = null;
    }

    // 通知所有玩家进入执行阶段
    const proposedTeamNames = room.proposedTeam.map(id => {
      const p = room.players.find(pp => pp.id === id);
      return p ? p.name : '???';
    });

    io.to(roomId).emit('phaseChange', {
      phase: 'mission',
      missionSubPhase: 'puzzle',
      roundNumber: room.currentRound,
      totalRounds: GAME_CONSTANTS.MAX_ROUNDS,
      proposedTeam: room.proposedTeam,
      proposedTeamNames,
      missionResults: room.missionResults,
      timeLimit: timer(room, GAME_CONSTANTS.PUZZLE_DISCUSS_TIME) + timer(room, GAME_CONSTANTS.PUZZLE_VOTE_TIME),
    });

    // 给小队成员发题
    room.proposedTeam.forEach(memberId => {
      const member = room.players.find(p => p.id === memberId);
      if (!member || member.isAI) return;

      const isPossessed = memberId === room.possessedPlayer;
      const puzzleToSend = isPossessed && room.distortedPuzzle
        ? room.distortedPuzzle
        : puzzle;

      io.to(memberId).emit('missionPuzzle', {
        puzzle: {
          id: puzzleToSend.id,
          title: puzzleToSend.title,
          scenario: puzzleToSend.scenario,
          options: puzzleToSend.options,
        },
        isPossessed,
        possessedHint: isPossessed ? getPossessedHint() : null,
        canSabotage: room.roles[memberId] === ROLES.INFILTRATOR,
        teamNames: proposedTeamNames,
      });
    });

    // 给非小队成员发旁观信息
    const nonTeam = room.players.filter(p =>
      !room.proposedTeam.includes(p.id) && !p.isAI
    );
    nonTeam.forEach(p => {
      io.to(p.id).emit('missionSpectate', {
        teamNames: proposedTeamNames,
        puzzleTitle: puzzle.title,
        roundNumber: room.currentRound,
      });
    });

    // 进入讨论子阶段（1秒后）
    if (room.currentPhaseTimer) clearTimeout(room.currentPhaseTimer);
    room.currentPhaseTimer = setTimeout(() => {
      startMissionDiscuss(roomId);
    }, 1000);
  }

  /**
   * 子阶段：讨论（25秒）
   * @param {string} roomId - 房间ID
   */
  function startMissionDiscuss(roomId) {
    const room = rooms[roomId];
    if (!room || room.currentPhase !== 'mission') return;

    room.missionSubPhase = 'discuss';
    room.puzzleChatCount = {};

    // 通知进入讨论
    io.to(roomId).emit('missionSubPhase', {
      subPhase: 'discuss',
      timeLimit: timer(room, GAME_CONSTANTS.PUZZLE_DISCUSS_TIME),
      maxMessages: GAME_CONSTANTS.PUZZLE_MAX_MESSAGES,
      maxChars: GAME_CONSTANTS.PUZZLE_MAX_CHARS,
    });

    // AI 玩家自动发言（1-2条）
    const aiMembers = room.proposedTeam.filter(id => {
      const p = room.players.find(pp => pp.id === id);
      return p && p.isAI;
    });
    aiMembers.forEach((aiId, idx) => {
      setTimeout(() => {
        if (room.currentPhase !== 'mission' || room.missionSubPhase !== 'discuss') return;
        const aiPlayer = room.players.find(p => p.id === aiId);
        if (!aiPlayer) return;

        // AI 根据是否被附身生成不同发言
        let aiMessage;
        if (aiId === room.possessedPlayer && room.distortedPuzzle) {
          aiMessage = `我看到的是${room.distortedPuzzle.correctAnswer}`;
        } else {
          const correctAnswer = room.currentPuzzle.correctAnswer;
          aiMessage = `我觉得是${correctAnswer}`;
        }

        room.puzzleChatLog.push({
          playerId: aiId,
          playerName: aiPlayer.name,
          message: aiMessage,
        });

        io.to(roomId).emit('puzzleChatBroadcast', {
          playerId: aiId,
          playerName: aiPlayer.name,
          message: aiMessage,
        });
      }, (timer(room, 3) * 1000) + idx * (timer(room, 5) * 1000)); // 错开发言时间（测试模式加速）
    });

    // 25秒后进入投票
    if (room.currentPhaseTimer) clearTimeout(room.currentPhaseTimer);
    room.currentPhaseTimer = setTimeout(() => {
      startMissionVote(roomId);
    }, timer(room, GAME_CONSTANTS.PUZZLE_DISCUSS_TIME) * 1000);
  }

  /**
   * 处理谜题讨论消息
   * @param {string} roomId - 房间ID
   * @param {string} playerId - 玩家ID
   * @param {string} message - 消息内容
   */
  function handlePuzzleChat(roomId, playerId, message) {
    const room = rooms[roomId];
    if (!room || room.currentPhase !== 'mission' || room.missionSubPhase !== 'discuss') return;

    // 只有小队成员能发言
    if (!room.proposedTeam.includes(playerId)) return;

    const player = room.players.find(p => p.id === playerId);
    if (!player || player.isAI || player.eliminated) return;

    // 检查条数限制
    const count = room.puzzleChatCount[playerId] || 0;
    if (count >= GAME_CONSTANTS.PUZZLE_MAX_MESSAGES) {
      io.to(playerId).emit('error', `谜题讨论已发送 ${GAME_CONSTANTS.PUZZLE_MAX_MESSAGES} 条消息`);
      return;
    }

    // 检查长度
    const trimmed = (message || '').trim();
    if (!trimmed) return;
    if (trimmed.length > GAME_CONSTANTS.PUZZLE_MAX_CHARS) {
      io.to(playerId).emit('error', `消息不能超过 ${GAME_CONSTANTS.PUZZLE_MAX_CHARS} 字`);
      return;
    }

    room.puzzleChatCount[playerId] = count + 1;

    // 附身改写（风格层）
    let displayed = trimmed;
    if (playerId === room.possessedPlayer && room.possessionStyle) {
      try {
        displayed = rewriteMessage(trimmed, room.possessionStyle);
      } catch (err) {
        console.error('谜题讨论附身改写失败:', err);
      }
    }

    // 记录日志
    room.puzzleChatLog.push({
      playerId,
      playerName: player.name,
      original: trimmed,
      displayed,
      isPossessed: playerId === room.possessedPlayer,
    });

    // 广播给所有人（包括非小队成员）
    io.to(roomId).emit('puzzleChatBroadcast', {
      playerId,
      playerName: player.name,
      message: displayed,
      messagesLeft: GAME_CONSTANTS.PUZZLE_MAX_MESSAGES - room.puzzleChatCount[playerId],
    });
  }

  /**
   * 子阶段：投票（10秒）
   * @param {string} roomId - 房间ID
   */
  function startMissionVote(roomId) {
    const room = rooms[roomId];
    if (!room || room.currentPhase !== 'mission') return;

    room.missionSubPhase = 'vote';

    io.to(roomId).emit('missionSubPhase', {
      subPhase: 'vote',
      timeLimit: timer(room, GAME_CONSTANTS.PUZZLE_VOTE_TIME),
    });

    // AI 玩家自动投票
    const aiMembers = room.proposedTeam.filter(id => {
      const p = room.players.find(pp => pp.id === id);
      return p && p.isAI;
    });
    aiMembers.forEach(aiId => {
      setTimeout(() => {
        if (room.currentPhase !== 'mission' || room.missionSubPhase !== 'vote') return;
        const aiPlayer = room.players.find(p => p.id === aiId);
        if (!aiPlayer) return;

        // AI 投票逻辑
        let answer;
        const isInf = room.roles[aiId] === ROLES.INFILTRATOR;
        if (aiId === room.possessedPlayer && room.distortedPuzzle) {
          // 被附身的 AI 投扭曲版答案
          answer = room.distortedPuzzle.correctAnswer;
        } else if (isInf && Math.random() < 0.7) {
          // 渗透者 70% 概率破坏（选错误答案）
          const options = Object.keys(room.currentPuzzle.options);
          const wrongOptions = options.filter(o => o !== room.currentPuzzle.correctAnswer);
          answer = wrongOptions[Math.floor(Math.random() * wrongOptions.length)];
        } else {
          // 正常投票
          answer = room.currentPuzzle.correctAnswer;
        }

        room.puzzleAnswers[aiId] = answer;
        checkAllPuzzleVoted(roomId);
      }, (timer(room, 2) * 1000) + Math.random() * (timer(room, 3) * 1000));
    });

    // 10秒超时
    if (room.currentPhaseTimer) clearTimeout(room.currentPhaseTimer);
    room.currentPhaseTimer = setTimeout(() => {
      // 超时未投票 → 默认选正确答案
      room.proposedTeam.forEach(id => {
        if (room.puzzleAnswers[id] === undefined) {
          room.puzzleAnswers[id] = room.currentPuzzle.correctAnswer;
        }
      });
      startMissionReveal(roomId);
    }, timer(room, GAME_CONSTANTS.PUZZLE_VOTE_TIME) * 1000);
  }

  /**
   * 处理谜题投票
   * @param {string} roomId - 房间ID
   * @param {string} voterId - 投票者ID
   * @param {string} answer - 答案
   */
  function handlePuzzleVote(roomId, voterId, answer) {
    const room = rooms[roomId];
    if (!room || room.currentPhase !== 'mission' || room.missionSubPhase !== 'vote') return;

    if (!room.proposedTeam.includes(voterId)) return;
    const voter = room.players.find(p => p.id === voterId);
    if (!voter || voter.isAI) return;

    // 验证答案有效性
    const validAnswers = Object.keys(room.currentPuzzle.options);
    if (!validAnswers.includes(answer)) return;

    room.puzzleAnswers[voterId] = answer;
    checkAllPuzzleVoted(roomId);
  }

  /**
   * 检查所有人是否已投票
   * @param {string} roomId - 房间ID
   */
  function checkAllPuzzleVoted(roomId) {
    const room = rooms[roomId];
    if (!room) return;

    const allVoted = room.proposedTeam.every(id =>
      room.puzzleAnswers[id] !== undefined ||
      room.players.find(p => p.id === id)?.isAI
    );

    if (allVoted) {
      if (room.currentPhaseTimer) clearTimeout(room.currentPhaseTimer);
      startMissionReveal(roomId);
    }
  }

  /**
   * 子阶段：揭晓（8秒）
   * @param {string} roomId - 房间ID
   */
  function startMissionReveal(roomId) {
    const room = rooms[roomId];
    if (!room || room.currentPhase !== 'mission') return;

    room.missionSubPhase = 'reveal';

    const correctAnswer = room.currentPuzzle.correctAnswer;
    const explanation = room.currentPuzzle.explanation;

    // 统计结果
    const votes = {};
    let wrongCount = 0;
    room.proposedTeam.forEach(id => {
      const p = room.players.find(pp => pp.id === id);
      if (!p) return;
      const answer = room.puzzleAnswers[id] || correctAnswer;
      votes[id] = {
        name: p.name,
        answer,
        isCorrect: answer === correctAnswer,
      };
      if (answer !== correctAnswer) wrongCount++;
    });

    // 有错误答案 = 任务失败
    const missionSuccess = wrongCount === 0;
    room.missionResults.push(missionSuccess);

    if (missionSuccess) {
      room.missionSuccesses++;
    } else {
      room.missionFailures++;
    }

    // 提取推理摘要（每人最后一句聊天）
    const justifications = {};
    room.proposedTeam.forEach(id => {
      const p = room.players.find(pp => pp.id === id);
      if (!p) return;
      const msgs = room.puzzleChatLog.filter(m => m.playerId === id);
      justifications[id] = {
        name: p.name,
        lastMessage: msgs.length > 0 ? msgs[msgs.length - 1].displayed : '(未发言)',
      };
    });

    console.log(`房间 ${roomId} 第 ${room.currentRound} 轮任务${missionSuccess ? '成功' : '失败'}（${wrongCount} 票错误）`);

    // 发送揭晓信息
    io.to(roomId).emit('missionReveal', {
      roundNumber: room.currentRound,
      correctAnswer,
      correctLabel: room.currentPuzzle.options[correctAnswer],
      explanation,
      votes,
      justifications,
      success: missionSuccess,
      sabotageCount: wrongCount,
      missionResults: room.missionResults,
      missionSuccesses: room.missionSuccesses,
      missionFailures: room.missionFailures,
      // 信号员额外信息
      hadPossession: !!room.possessedPlayer,
    });

    // 结算（8秒后）
    if (room.currentPhaseTimer) clearTimeout(room.currentPhaseTimer);
    room.currentPhaseTimer = setTimeout(() => {
      // 发送最终结果（兼容旧客户端）
      io.to(roomId).emit('missionResult', {
        roundNumber: room.currentRound,
        success: missionSuccess,
        sabotageCount: wrongCount,
        missionResults: room.missionResults,
        missionSuccesses: room.missionSuccesses,
        missionFailures: room.missionFailures,
      });

      // 检查胜负
      if (room.missionSuccesses >= GAME_CONSTANTS.MISSIONS_TO_WIN) {
        // 通过事件触发游戏结束，避免循环依赖
        io.to(roomId).emit('gameOver', { winner: 'engineer' });
        return;
      }
      if (room.missionFailures >= GAME_CONSTANTS.MISSIONS_TO_WIN) {
        // 通过事件触发游戏结束，避免循环依赖
        io.to(roomId).emit('gameOver', { winner: 'infiltrator' });
        return;
      }

      // 下一轮
      room.rejectStreak = 0;
      // 通过事件触发新一轮，避免循环依赖
      io.to(roomId).emit('nextRound', { roomId });
    }, timer(room, GAME_CONSTANTS.PUZZLE_REVEAL_TIME) * 1000);
  }

  return {
    startMissionPhase,
    startMissionDiscuss,
    handlePuzzleChat,
    startMissionVote,
    handlePuzzleVote,
    checkAllPuzzleVoted,
    startMissionReveal
  };
}

module.exports = { createMissionHandlers };