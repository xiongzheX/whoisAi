/**
 * 游戏阶段处理器
 * 包含提名、讨论、投票、执行等阶段逻辑
 */

const { rooms } = require('./roomService');
const {
  ROLES, ROLE_LABELS, ROLE_DESCRIPTIONS, ROLE_FACTION,
  assignRoles, GAME_CONSTANTS, getMissionTeamSize
} = require('./gameData');
const { getTopics } = require('./topicBank');
const { rewriteMessage, randomStyle, shouldPossess, selectPossessedPlayer, distortPuzzle, getPossessedHint } = require('./possessionEngine');
const { pickPuzzle, distortPuzzle: dpDistort } = require('./puzzleBank');
const {
  getHumanPlayers,
  getAIPlayers,
  getAlivePlayers,
  getAliveHumanPlayers,
  getPlayerById,
  safeBroadcastToRoom,
  delay,
  timer,
  getRoleDescription,
  rotateLeader,
  addAIToRoom
} = require('./utils');

/**
 * 创建阶段处理器
 * @param {Object} options - 配置选项
 * @param {Object} options.io - Socket.IO 实例
 * @returns {Object} 阶段处理器函数
 */
function createPhaseHandlers({ io }) {
  
  /**
   * 开始游戏
   * @param {string} roomId - 房间ID
   */
  function startGame(roomId) {
    try {
      const room = rooms[roomId];
      if (!room || room.status === 'playing') return;

      const humanPlayers = getHumanPlayers(room);

      // 测试/单人模式：放宽人数限制（AI 填补）
      const isTestMode = room.mode === 'test' || room.mode === 'solo';
      if (!isTestMode && humanPlayers.length < GAME_CONSTANTS.MIN_PLAYERS) {
        console.log(`房间 ${roomId} 人数不足 ${GAME_CONSTANTS.MIN_PLAYERS}，无法开始`);
        return;
      }
      if (isTestMode && humanPlayers.length < 1) {
        console.log(`房间 ${roomId} 无真人玩家，无法开始`);
        return;
      }

      room.status = 'playing';
      room.currentRound = 0;
      room.missionResults = [];
      room.missionSuccesses = 0;
      room.missionFailures = 0;

      // 分配角色（只给真人玩家）
      const humanIds = humanPlayers.map(p => p.id);
      room.roles = assignRoles(humanIds);

      // 通知每个玩家他们的角色
      humanPlayers.forEach(player => {
        try {
          const role = room.roles[player.id];
          io.to(player.id).emit('rolesRevealed', {
            role,
            roleLabel: ROLE_LABELS[role],
            roleDescription: getRoleDescription(role, room.roles, player.id),
            players: room.players.map(p => ({
              id: p.id,
              name: p.name,
              position: p.position
            }))
          });
        } catch (err) {
          console.error(`发送角色信息给玩家 ${player.id} 失败:`, err);
        }
      });

      console.log(`房间 ${roomId} 游戏开始！角色分配完成`);

      // 生成话题
      room.topics = getTopics(GAME_CONSTANTS.MAX_ROUNDS);

      // 选择第一个队长（随机）
      if (humanPlayers.length > 0) {
        const leaderIdx = Math.floor(Math.random() * humanPlayers.length);
        room.currentLeader = humanPlayers[leaderIdx].id;
      } else {
        // 如果没有真人玩家，选择第一个 AI 玩家作为队长
        const aiPlayers = getAIPlayers(room);
        if (aiPlayers.length > 0) {
          room.currentLeader = aiPlayers[0].id;
        } else {
          console.error(`房间 ${roomId} 没有玩家，无法开始游戏`);
          return;
        }
      }

      // 开始第一轮
      startRound(roomId);
    } catch (err) {
      console.error(`房间 ${roomId} 开始游戏失败:`, err);
    }
  }

  /**
   * 开始新一轮
   * @param {string} roomId - 房间ID
   */
  function startRound(roomId) {
    try {
      const room = rooms[roomId];
      if (!room || room.status !== 'playing') return;

      room.currentRound++;

      // 检查是否超过最大轮次
      if (room.currentRound > GAME_CONSTANTS.MAX_ROUNDS) {
        endGame(roomId, 'infiltrator'); // 时间到了，坏人赢
        return;
      }

      // 重置轮次状态
      room.proposedTeam = [];
      room.teamVotes = {};
      room.missionVotes = {};
      room.messageCount = {};
      room.chatMessages = [];

      // 随机决定是否有人被附身（50% 概率）
      const alivePlayers = getHumanPlayers(room);
      if (shouldPossess(GAME_CONSTANTS.POSSESSION_CHANCE)) {
        room.possessedPlayer = selectPossessedPlayer(alivePlayers.map(p => p.id));
        room.possessionStyle = room.possessedPlayer ? randomStyle() : null;
      } else {
        room.possessedPlayer = null;
        room.possessionStyle = null;
      }

      if (room.possessedPlayer) {
        const possessed = alivePlayers.find(p => p.id === room.possessedPlayer);
        console.log(`房间 ${roomId} 第 ${room.currentRound} 轮：${possessed ? possessed.name : '未知'} 被附身（${room.possessionStyle}）`);
      } else {
        console.log(`房间 ${roomId} 第 ${room.currentRound} 轮：无人被附身`);
      }

      // 通知侦测者
      const signalKeeper = alivePlayers.find(p => room.roles[p.id] === ROLES.SIGNAL_KEEPER);
      if (signalKeeper) {
        try {
          const hasPossession = !!room.possessedPlayer;
          
          // 保存侦测者历史
          room.signalHistory.push({
            round: room.currentRound,
            hasPossession
          });
          
          io.to(signalKeeper.id).emit('signalCheck', {
            hasPossession,
            roundNumber: room.currentRound,
            signalHistory: room.signalHistory
          });
        } catch (err) {
          console.error(`通知侦测者失败:`, err);
        }
      }

      // 通知被附身者
      if (room.possessedPlayer) {
        try {
          io.to(room.possessedPlayer).emit('possessionAlert', {
            roundNumber: room.currentRound
          });
        } catch (err) {
          console.error(`通知被附身者失败:`, err);
        }
      }

      // 进入提名阶段
      startProposePhase(roomId);
    } catch (err) {
      console.error(`房间 ${roomId} 开始新一轮失败:`, err);
    }
  }

  /**
   * 开始提名阶段
   * @param {string} roomId - 房间ID
   */
  function startProposePhase(roomId) {
    const room = rooms[roomId];
    if (!room) return;

    console.log(`房间 ${roomId} 开始提名阶段，currentLeader=${room.currentLeader}`);
    const leader = room.players.find(p => p.id === room.currentLeader);
    console.log(`房间 ${roomId} 队长信息：leader=${leader ? leader.name : '无'}, leader.id=${leader?.id}`);
    const humanPlayers = getHumanPlayers(room);
    const teamSize = getMissionTeamSize(
      humanPlayers.length,
      room.currentRound - 1
    );

    room.currentPhase = 'propose';

    // 清理旧定时器
    if (room.currentPhaseTimer) clearTimeout(room.currentPhaseTimer);

    io.to(roomId).emit('phaseChange', {
      phase: 'propose',
      roundNumber: room.currentRound,
      totalRounds: GAME_CONSTANTS.MAX_ROUNDS,
      leader: leader ? { id: leader.id, name: leader.name } : null,
      teamSize,
      missionResults: room.missionResults,
      timeLimit: timer(room, GAME_CONSTANTS.PROPOSE_TIME)
    });

    // AI 领导者自动提名（或测试/单人模式下自动提名）
    const isTestMode = room.mode === 'test' || room.mode === 'solo';
    console.log(`房间 ${roomId} 提名阶段：队长=${leader?.name || '无'}, isAI=${leader?.isAI}, isTestMode=${isTestMode}`);
    if (leader && (leader.isAI || isTestMode)) {
      setTimeout(() => {
        if (room.currentPhase !== 'propose' || room.currentLeader !== leader.id) return;
        const alivePlayers = room.players.filter(p => !p.eliminated && !p.isAI);
        const allPlayers = room.players.filter(p => !p.eliminated);
        // 随机选 teamSize 个非 AI 玩家（如果不够则包含 AI）
        const pool = alivePlayers.length >= teamSize ? alivePlayers : allPlayers;
        const shuffled = [...pool].sort(() => Math.random() - 0.5);
        const memberIds = shuffled.slice(0, teamSize).map(p => p.id);
        console.log(`房间 ${roomId} ${leader.isAI ? 'AI队长' : '测试模式'} ${leader.name} 自动提名：${memberIds.map(id => room.players.find(p => p.id === id)?.name).join(', ')}`);
        handleProposeMission(roomId, leader.id, memberIds);
      }, 2000);
    }

    // 提名超时 → 队长顺移
    room.currentPhaseTimer = setTimeout(() => {
      console.log(`房间 ${roomId} 提名超时，队长顺移`);
      rotateLeader(roomId);
      startProposePhase(roomId);
    }, timer(room, GAME_CONSTANTS.PROPOSE_TIME) * 1000);
  }

  /**
   * 处理队长提名小队
   * @param {string} roomId - 房间ID
   * @param {string} proposerId - 提名者ID
   * @param {Array} memberIds - 成员ID数组
   */
  function handleProposeMission(roomId, proposerId, memberIds) {
    const room = rooms[roomId];
    if (!room || room.currentPhase !== 'propose') return;

    // 只有队长能提名
    if (room.currentLeader !== proposerId) {
      console.log(`非队长 ${proposerId} 尝试提名`);
      return;
    }

    const humanPlayers = getHumanPlayers(room);
    const teamSize = getMissionTeamSize(
      humanPlayers.length,
      room.currentRound - 1
    );

    // 验证小队人数
    if (!memberIds || memberIds.length !== teamSize) {
      io.to(proposerId).emit('error', `需要提名 ${teamSize} 人`);
      return;
    }

    // 验证成员都存在且未淘汰
    // 在测试模式下，允许提名AI玩家
    const isTestMode = room.mode === 'test' || room.mode === 'solo';
    const alivePlayers = isTestMode 
      ? room.players.filter(p => !p.eliminated)
      : room.players.filter(p => !p.eliminated && !p.isAI);
    
    for (const id of memberIds) {
      if (!alivePlayers.find(p => p.id === id)) {
        io.to(proposerId).emit('error', '提名了无效的玩家');
        return;
      }
    }

    room.proposedTeam = memberIds;
    const memberNames = memberIds.map(id => {
      const p = room.players.find(pp => pp.id === id);
      return p ? p.name : '???';
    });
    const leader = room.players.find(p => p.id === proposerId);

    console.log(`房间 ${roomId} 队长 ${leader?.name} 提名：${memberNames.join(', ')}`);

    // 清提名超时
    if (room.currentPhaseTimer) clearTimeout(room.currentPhaseTimer);

    // 广播提名结果，进入讨论
    io.to(roomId).emit('missionProposed', {
      leaderId: leader?.id,
      leaderName: leader?.name || '???',
      memberIds,
      memberNames
    });

    startDiscussPhase(roomId);
  }

  /**
   * 开始讨论阶段
   * @param {string} roomId - 房间ID
   */
  function startDiscussPhase(roomId) {
    const room = rooms[roomId];
    if (!room) return;

    room.currentPhase = 'discuss';
    room.chatMessages = [];
    room.messageCount = {};

    io.to(roomId).emit('phaseChange', {
      phase: 'discuss',
      roundNumber: room.currentRound,
      totalRounds: GAME_CONSTANTS.MAX_ROUNDS,
      proposedTeam: room.proposedTeam,
      missionResults: room.missionResults,
      maxMessages: GAME_CONSTANTS.MAX_MESSAGES_PER_ROUND,
      maxChars: GAME_CONSTANTS.MAX_CHARS_PER_MESSAGE,
      timeLimit: timer(room, GAME_CONSTANTS.DISCUSS_TIME)
    });

    if (room.currentPhaseTimer) clearTimeout(room.currentPhaseTimer);
    room.currentPhaseTimer = setTimeout(() => {
      startTeamVotePhase(roomId);
    }, timer(room, GAME_CONSTANTS.DISCUSS_TIME) * 1000);
  }

  /**
   * 处理聊天消息（带限制 + 附身改写）
   * @param {string} roomId - 房间ID
   * @param {string} playerId - 玩家ID
   * @param {string} message - 消息内容
   */
  function handleChat(roomId, playerId, message) {
    try {
      const room = rooms[roomId];
      if (!room || room.currentPhase !== 'discuss') return;

      // 检查玩家存在且未淘汰
      const player = room.players.find(p => p.id === playerId);
      if (!player || player.eliminated || player.isAI) return;

      // 检查消息有效性
      if (!message || typeof message !== 'string') return;
      const trimmed = message.trim();
      if (trimmed.length === 0) return;
      if (trimmed.length > GAME_CONSTANTS.MAX_CHARS_PER_MESSAGE) {
        io.to(playerId).emit('error', `消息不能超过 ${GAME_CONSTANTS.MAX_CHARS_PER_MESSAGE} 字`);
        return;
      }

      // 检查条数限制
      const count = room.messageCount[playerId] || 0;
      if (count >= GAME_CONSTANTS.MAX_MESSAGES_PER_ROUND) {
        io.to(playerId).emit('error', `本轮已发送 ${GAME_CONSTANTS.MAX_MESSAGES_PER_ROUND} 条消息`);
        return;
      }

      room.messageCount[playerId] = count + 1;

      // AI 附身改写
      let displayed = trimmed;
      let isPossessed = false;
      if (room.possessedPlayer === playerId && room.possessionStyle) {
        try {
          displayed = rewriteMessage(trimmed, room.possessionStyle);
          isPossessed = true;
          console.log(`[附身改写] ${player.name}: "${trimmed}" → "${displayed}"`);
        } catch (err) {
          console.error(`附身改写失败:`, err);
          // 改写失败，使用原始消息
          displayed = trimmed;
        }
      }

      // 记录消息
      room.chatMessages.push({
        playerId,
        playerName: player.name,
        original: trimmed,
        displayed,
        possessed: isPossessed
      });

      // 广播改写后的消息
      io.to(roomId).emit('chat', {
        playerId,
        playerName: player.name,
        message: displayed,
        messagesLeft: GAME_CONSTANTS.MAX_MESSAGES_PER_ROUND - room.messageCount[playerId],
        // 附身者自己看到原始消息（用于对比）
        isPossessed // 仅附身者自己会注意到这个 flag（客户端处理）
      });
    } catch (err) {
      console.error(`处理聊天消息失败:`, err);
    }
  }

  /**
   * 开始全员投票阶段
   * @param {string} roomId - 房间ID
   */
  function startTeamVotePhase(roomId) {
    const room = rooms[roomId];
    if (!room) return;

    room.currentPhase = 'team_vote';
    room.teamVotes = {};

    io.to(roomId).emit('phaseChange', {
      phase: 'team_vote',
      roundNumber: room.currentRound,
      totalRounds: GAME_CONSTANTS.MAX_ROUNDS,
      proposedTeam: room.proposedTeam,
      proposedTeamNames: room.proposedTeam.map(id => {
        const p = room.players.find(pp => pp.id === id);
        return p ? p.name : '???';
      }),
      missionResults: room.missionResults,
      timeLimit: timer(room, GAME_CONSTANTS.TEAM_VOTE_TIME)
    });

    // AI 玩家自动投票（测试模式加速）
    const aiVoters = room.players.filter(p => p.isAI && !p.eliminated);
    aiVoters.forEach(ai => {
      const delayTime = (timer(room, 2) * 1000) + Math.random() * (timer(room, 3) * 1000);
      setTimeout(() => {
        if (room.currentPhase !== 'team_vote') return;
        if (room.teamVotes[ai.id] !== undefined) return;
        
        // 改进的AI投票策略
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
        
        room.teamVotes[ai.id] = Math.random() < approveChance;
        checkAllTeamVoted(roomId);
      }, delayTime);
    });

    if (room.currentPhaseTimer) clearTimeout(room.currentPhaseTimer);
    room.currentPhaseTimer = setTimeout(() => {
      // 超时未投票视为同意
      const alivePlayers = room.players.filter(p => !p.isAI && !p.eliminated);
      alivePlayers.forEach(p => {
        if (room.teamVotes[p.id] === undefined) {
          room.teamVotes[p.id] = true;
        }
      });
      resolveTeamVote(roomId);
    }, timer(room, GAME_CONSTANTS.TEAM_VOTE_TIME) * 1000);
  }

  /**
   * 检查所有人是否投票
   * @param {string} roomId - 房间ID
   */
  function checkAllTeamVoted(roomId) {
    const room = rooms[roomId];
    if (!room) return;
    const allVoted = room.players.filter(p => !p.eliminated)
      .every(p => room.teamVotes[p.id] !== undefined);
    if (allVoted) {
      if (room.currentPhaseTimer) clearTimeout(room.currentPhaseTimer);
      resolveTeamVote(roomId);
    }
  }

  /**
   * 处理投票
   * @param {string} roomId - 房间ID
   * @param {string} voterId - 投票者ID
   * @param {boolean} approve - 是否同意
   */
  function handleTeamVote(roomId, voterId, approve) {
    const room = rooms[roomId];
    if (!room || room.currentPhase !== 'team_vote') return;

    const voter = room.players.find(p => p.id === voterId);
    if (!voter || voter.isAI || voter.eliminated) return;

    room.teamVotes[voterId] = !!approve;
    console.log(`房间 ${roomId} ${voter.name} 投票: ${approve ? '同意' : '反对'}`);

    // 检查是否所有人都投票了
    const alivePlayers = room.players.filter(p => !p.isAI && !p.eliminated);
    const allVoted = alivePlayers.every(p => room.teamVotes[p.id] !== undefined);

    if (allVoted) {
      if (room.currentPhaseTimer) clearTimeout(room.currentPhaseTimer);
      resolveTeamVote(roomId);
    }
  }

  /**
   * 结算投票
   * @param {string} roomId - 房间ID
   */
  function resolveTeamVote(roomId) {
    const room = rooms[roomId];
    if (!room) return;

    let approveCount = 0;
    let rejectCount = 0;
    const voteDisplay = {};

    for (const [voterId, approved] of Object.entries(room.teamVotes)) {
      const voter = room.players.find(p => p.id === voterId);
      if (approved) approveCount++;
      else rejectCount++;
      voteDisplay[voterId] = {
        voterName: voter?.name || '???',
        approved
      };
    }

    const approved = approveCount > rejectCount;

    // 保存投票历史
    room.voteHistory.push({
      round: room.currentRound,
      votes: voteDisplay,
      approved,
      team: room.proposedTeam.map(id => {
        const p = room.players.find(pp => pp.id === id);
        return p ? p.name : '???';
      })
    });

    io.to(roomId).emit('teamVoteResult', {
      approved,
      approveCount,
      rejectCount,
      votes: voteDisplay, // 内部保存完整投票记录
      voteHistory: room.voteHistory.map(v => ({
        round: v.round,
        approved: v.approved,
        team: v.team,
        approveCount: Object.values(v.votes).filter(v => v.approved).length,
        rejectCount: Object.values(v.votes).filter(v => !v.approved).length
      })) // 发送匿名化的投票历史
    });

    if (approved) {
      console.log(`房间 ${roomId} 小队通过，进入执行阶段`);
      // 队长顺移
      rotateLeader(roomId);
      // 使用注入的任务处理器
      if (createPhaseHandlers.missionHandlers && createPhaseHandlers.missionHandlers.startMissionPhase) {
        createPhaseHandlers.missionHandlers.startMissionPhase(roomId);
      } else {
        console.error('任务处理器未注入，无法开始任务阶段');
      }
    } else {
      console.log(`房间 ${roomId} 小队被否决，队长顺移`);
      rotateLeader(roomId);
      // 5次否决 = 坏人赢
      room.rejectStreak = (room.rejectStreak || 0) + 1;
      if (room.rejectStreak >= 5) {
        endGame(roomId, 'infiltrator');
        return;
      }
      startRound(roomId);
    }
  }

  /**
   * 游戏结束
   * @param {string} roomId - 房间ID
   * @param {string} winner - 获胜方
   */
  function endGame(roomId, winner) {
    const room = rooms[roomId];
    if (!room) return;

    room.status = 'finished';
    if (room.currentPhaseTimer) clearTimeout(room.currentPhaseTimer);

    const winnerFaction = winner; // 'engineer' (守护者阵营，兼容旧 key) or 'infiltrator'

    // 构建角色揭示信息
    const rolesReveal = {};
    for (const [playerId, role] of Object.entries(room.roles)) {
      const player = room.players.find(p => p.id === playerId);
      if (player) {
        rolesReveal[playerId] = {
          name: player.name,
          role,
          roleLabel: ROLE_LABELS[role],
          faction: ROLE_FACTION[role],
          isWinner: (winnerFaction === 'engineer' && ROLE_FACTION[role] === 'good') ||
                    (winnerFaction === 'infiltrator' && ROLE_FACTION[role] === 'evil')
        };
      }
    }

    console.log(`房间 ${roomId} 游戏结束！${winnerFaction === 'engineer' ? '守护者' : '渗透者'}胜利`);

    io.to(roomId).emit('gameFinished', {
      winner: winnerFaction,
      winnerLabel: winnerFaction === 'engineer' ? '🛡️ 守护者阵营胜利' : '🦠 渗透者胜利',
      roles: rolesReveal,
      missionResults: room.missionResults,
      missionSuccesses: room.missionSuccesses,
      missionFailures: room.missionFailures,
      totalRounds: room.currentRound
    });
  }

  return {
    startGame,
    startRound,
    handleProposeMission,
    handleChat,
    handleTeamVote,
    endGame
  };
}

/**
 * 设置任务处理器依赖
 * @param {Object} missionHandlers - 任务处理器
 */
function setMissionHandlers(missionHandlers) {
  // 将任务处理器注入到阶段处理器中
  createPhaseHandlers.missionHandlers = missionHandlers;
}

module.exports = { createPhaseHandlers, setMissionHandlers };
