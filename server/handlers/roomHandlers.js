/**
 * 谁是AI v3 — Socket 事件处理
 * 
 * 事件体系：
 *   客户端 → 服务端:
 *     joinRoom, startGame, proposeMission, teamVote, missionVote, chat, resetRoom
 *   服务端 → 客户端:
 *     playerJoined, rolesRevealed, phaseChange, missionProposed,
 *     chat, teamVoteResult, missionResult, possessionAlert, signalCheck,
 *     gameFinished, roomReset, error
 */

const {
  rooms, addPlayer, addAIPlayer, removePlayer,
  getPlayerList, getPlayer
} = require('../roomService');
const {
  validateRoomId,
  validatePlayerName,
  validateChatMessage,
  validateMemberIds,
  validateVote
} = require('../validator');
const gameData = require('../gameData');

function registerRoomHandlers({ socket, io, gameFlow }) {

  /**
   * 加入房间
   */
  socket.on('joinRoom', ({ roomId, name, mode, testMode }) => {
    try {
      // 输入验证
      const roomIdResult = validateRoomId(roomId);
      if (!roomIdResult.valid) {
        socket.emit('error', roomIdResult.error);
        return;
      }
      roomId = roomIdResult.value;
      
      const nameResult = validatePlayerName(name);
      if (!nameResult.valid) {
        socket.emit('error', nameResult.error);
        return;
      }
      name = nameResult.value;

      let room = rooms[roomId];
      if (!room) {
        const { createRoom } = require('../roomService');
        createRoom(roomId);
        room = rooms[roomId];
      }

      if (room.status === 'playing') {
        socket.emit('error', '游戏已开始，无法加入');
        return;
      }

      const selectedMode = mode || (testMode ? 'test' : 'normal');
      const result = addPlayer(roomId, socket.id, name, selectedMode);
      if (result.error) {
        socket.emit('error', result.error);
        return;
      }

      socket.join(roomId);
      console.log(`玩家 ${name} 加入房间 ${roomId}（${selectedMode}），当前 ${room.players.length} 人`);

      // 广播玩家列表
      io.to(roomId).emit('playerJoined', {
        players: getPlayerList(room),
        count: room.players.length,
        mode: selectedMode
      });

      // 测试/单人模式：补 AI 到 6 人，自动开始
      if (selectedMode === 'test' || selectedMode === 'solo') {
        // 清除之前的定时器
        if (room.testModeTimer) {
          clearTimeout(room.testModeTimer);
        }

        // 第一步：800ms 后补 AI
        room.testModeTimer = setTimeout(() => {
          try {
            const r = rooms[roomId];
            if (!r || r.status !== 'waiting') return;

            const humanCount = r.players.filter(p => !p.isAI).length;
            const aiCount = r.players.filter(p => p.isAI).length;
            const targetTotal = 6;
            const needAI = Math.max(0, targetTotal - humanCount - aiCount);

            console.log(`${selectedMode} mode：${humanCount}人 + ${aiCount}AI，需要补 ${needAI} 个AI`);

            for (let i = 0; i < needAI; i++) {
              gameFlow.addAIToRoom(roomId);
            }

            const finalCount = r.players.length;
            console.log(`${selectedMode} mode：${humanCount}人 + ${r.players.filter(p => p.isAI).length}AI = ${finalCount}人`);

            // 广播完整玩家列表（确保客户端看到满员状态）
            const { getPlayerList } = require('../roomService');
            io.to(roomId).emit('playerJoined', {
              players: getPlayerList(r),
              count: r.players.length,
              mode: selectedMode
            });

            // 第二步：再等 2 秒让玩家看到等待室，然后自动开始
            if (finalCount >= 5) {
              setTimeout(() => {
                const rr = rooms[roomId];
                if (!rr || rr.status !== 'waiting') return;
                console.log(`${selectedMode} mode：延迟自动开始！(${finalCount}人)`);
                gameFlow.startGame(roomId);
              }, 2000);
            } else {
              console.log(`${selectedMode} mode：人数不足，等待中... (${finalCount}人)`);
            }
          } catch (err) {
            console.error(`测试模式自动开始失败:`, err);
          }
        }, 800);
      }
    } catch (err) {
      console.error(`加入房间失败:`, err);
      socket.emit('error', '加入房间失败，请重试');
    }
  });

  /**
   * 开始游戏（手动）
   */
  socket.on('startGame', ({ roomId }) => {
    const room = rooms[roomId];
    if (!room) return;
    if (room.players.length < gameData.GAME_CONSTANTS.MIN_PLAYERS) {
      socket.emit('error', `至少需要 ${gameData.GAME_CONSTANTS.MIN_PLAYERS} 名玩家`);
      return;
    }
    gameFlow.startGame(roomId);
  });

  /**
   * v3: 队长提名小队
   */
  socket.on('proposeMission', ({ roomId, memberIds }) => {
    try {
      if (!Array.isArray(memberIds)) {
        socket.emit('error', '提名格式错误');
        return;
      }
      gameFlow.handleProposeMission(roomId, socket.id, memberIds);
    } catch (err) {
      console.error(`处理提名失败:`, err);
      socket.emit('error', '提名失败，请重试');
    }
  });

  /**
   * v3: 全员投票（同意/反对小队）
   */
  socket.on('teamVote', ({ roomId, approve }) => {
    try {
      gameFlow.handleTeamVote(roomId, socket.id, approve);
    } catch (err) {
      console.error(`处理投票失败:`, err);
      socket.emit('error', '投票失败，请重试');
    }
  });

  /**
   * v3: 小队成员任务投票（方向 C：选答案 A/B/C）
   * 兼容旧格式：{ success: true/false }
   */
  socket.on('missionVote', ({ roomId, answer, success }) => {
    try {
      // 兼容旧格式（success → answer）
      if (answer) {
        gameFlow.handlePuzzleVote(roomId, socket.id, answer);
      } else {
        gameFlow.handleMissionVote(roomId, socket.id, success);
      }
    } catch (err) {
      console.error(`处理任务投票失败:`, err);
      socket.emit('error', '任务投票失败，请重试');
    }
  });

  /**
   * 方向 C：谜题讨论消息（执行阶段小队聊天）
   */
  socket.on('puzzleChat', ({ roomId, message }) => {
    try {
      gameFlow.handlePuzzleChat(roomId, socket.id, message);
    } catch (err) {
      console.error(`处理谜题讨论消息失败:`, err);
      socket.emit('error', '发送消息失败，请重试');
    }
  });

  /**
   * v3: 受限聊天（由 gameFlow 处理限制 + 附身改写）
   */
  socket.on('chat', ({ roomId, message }) => {
    try {
      // 输入验证
      const messageResult = validateChatMessage(message);
      if (!messageResult.valid) {
        socket.emit('error', messageResult.error);
        return;
      }
      
      gameFlow.handleChat(roomId, socket.id, messageResult.value);
    } catch (err) {
      console.error(`处理聊天消息失败:`, err);
      socket.emit('error', '发送消息失败，请重试');
    }
  });

  /**
   * 重置房间
   */
  socket.on('resetRoom', ({ roomId }) => {
    try {
      const { resetRoom } = require('../roomService');
      resetRoom(roomId);
      io.to(roomId).emit('roomReset');
    } catch (err) {
      console.error(`重置房间失败:`, err);
      socket.emit('error', '重置房间失败');
    }
  });

  /**
   * 断开连接（支持断线重连）
   */
  socket.on('disconnect', () => {
    try {
      console.log('玩家断开:', socket.id);
      for (const roomId of Object.keys(rooms)) {
        const room = rooms[roomId];
        const player = room.players.find(p => p.id === socket.id);
        if (player) {
          // 标记为断线状态，而不是立即移除
          player.disconnected = true;
          player.disconnectTime = Date.now();
          
          // 通知其他玩家
          io.to(roomId).emit('playerJoined', {
            players: getPlayerList(room),
            count: room.players.filter(p => !p.disconnected).length,
            mode: room.mode
          });
          
          io.to(roomId).emit('playerDisconnected', {
            playerId: socket.id,
            playerName: player.name
          });
          
          // 设置重连超时（30秒）
          if (room.reconnectTimers) {
            clearTimeout(room.reconnectTimers[socket.id]);
          } else {
            room.reconnectTimers = {};
          }
          
          room.reconnectTimers[socket.id] = setTimeout(() => {
            // 超时未重连，移除玩家
            const r = rooms[roomId];
            if (r) {
              const p = r.players.find(pp => pp.id === socket.id);
              if (p && p.disconnected) {
                console.log(`玩家 ${p.name} 重连超时，移除`);
                removePlayer(roomId, socket.id);
                io.to(roomId).emit('playerJoined', {
                  players: getPlayerList(r),
                  count: r.players.length,
                  mode: r.mode
                });
              }
            }
          }, 30000); // 30秒重连超时
          
          console.log(`玩家 ${player.name} 断线，等待重连...`);
        }
      }
    } catch (err) {
      console.error(`处理断开连接失败:`, err);
    }
  });

  /**
   * 调试模式事件处理
   */
  
  // 调试暂停/继续
  socket.on('debugPause', ({ roomId, paused }) => {
    try {
      const room = rooms[roomId];
      if (!room) {
        socket.emit('debugResponse', { success: false, message: '房间不存在' });
        return;
      }
      
      // 检查是否是调试模式
      if (room.mode !== 'solo') {
        socket.emit('debugResponse', { success: false, message: '仅单人调试模式支持此功能' });
        return;
      }
      
      // 设置暂停状态
      room.isPaused = paused;
      
      // 通知客户端
      io.to(roomId).emit('debugResponse', { 
        success: true, 
        message: paused ? '游戏已暂停' : '游戏已继续' 
      });
      
      // 如果暂停，停止所有定时器
      if (paused) {
        if (room.phaseTimer) {
          clearTimeout(room.phaseTimer);
          room.phaseTimer = null;
        }
        if (room.aiTimer) {
          clearTimeout(room.aiTimer);
          room.aiTimer = null;
        }
      } else {
        // 如果继续，重新启动当前阶段的定时器
        // 这里需要根据当前阶段重新调度
        console.log('游戏继续，需要重新调度阶段');
      }
      
      console.log(`房间 ${roomId} 调试${paused ? '暂停' : '继续'}`);
    } catch (err) {
      console.error(`处理调试暂停失败:`, err);
      socket.emit('debugResponse', { success: false, message: '处理失败' });
    }
  });
  
  // 调试跳过阶段
  socket.on('debugSkipPhase', ({ roomId }) => {
    try {
      const room = rooms[roomId];
      if (!room) {
        socket.emit('debugResponse', { success: false, message: '房间不存在' });
        return;
      }
      
      // 检查是否是调试模式
      if (room.mode !== 'solo') {
        socket.emit('debugResponse', { success: false, message: '仅单人调试模式支持此功能' });
        return;
      }
      
      // 跳过当前阶段
      // 这里需要调用游戏流程的跳过函数
      // 暂时只是发送响应
      socket.emit('debugResponse', { success: true, message: '阶段跳过请求已发送' });
      
      console.log(`房间 ${roomId} 调试跳过阶段`);
    } catch (err) {
      console.error(`处理调试跳过阶段失败:`, err);
      socket.emit('debugResponse', { success: false, message: '处理失败' });
    }
  });
  
  // 调试跳转到指定阶段
  socket.on('debugJumpToPhase', ({ roomId, targetPhase }) => {
    try {
      const room = rooms[roomId];
      if (!room) {
        socket.emit('debugResponse', { success: false, message: '房间不存在' });
        return;
      }
      
      // 检查是否是调试模式
      if (room.mode !== 'solo') {
        socket.emit('debugResponse', { success: false, message: '仅单人调试模式支持此功能' });
        return;
      }
      
      // 验证目标阶段
      const validPhases = ['propose', 'discuss', 'vote', 'mission', 'reveal', 'roundIntro'];
      if (!validPhases.includes(targetPhase)) {
        socket.emit('debugResponse', { success: false, message: '无效的目标阶段' });
        return;
      }
      
      // 跳转到指定阶段
      // 这里需要调用游戏流程的跳转函数
      // 暂时只是发送响应
      socket.emit('debugResponse', { success: true, message: `跳转到阶段: ${targetPhase}` });
      
      console.log(`房间 ${roomId} 调试跳转到阶段: ${targetPhase}`);
    } catch (err) {
      console.error(`处理调试跳转阶段失败:`, err);
      socket.emit('debugResponse', { success: false, message: '处理失败' });
    }
  });
  
  // 获取游戏状态（调试用）
  socket.on('debugGetGameState', ({ roomId }) => {
    try {
      const room = rooms[roomId];
      if (!room) {
        socket.emit('debugResponse', { success: false, message: '房间不存在' });
        return;
      }
      
      // 检查是否是调试模式
      if (room.mode !== 'solo') {
        socket.emit('debugResponse', { success: false, message: '仅单人调试模式支持此功能' });
        return;
      }
      
      // 发送游戏状态
      const gameState = {
        roomId: room.roomId,
        status: room.status,
        currentRound: room.currentRound,
        maxRounds: room.maxRounds,
        currentPhase: room.currentPhase,
        missionSuccesses: room.missionSuccesses,
        missionFailures: room.missionFailures,
        currentLeader: room.currentLeader,
        proposedTeam: room.proposedTeam,
        isPaused: room.isPaused || false,
        players: room.players.map(p => ({
          id: p.id,
          name: p.name,
          isAI: p.isAI,
          role: p.role,
          voteIntent: p.voteIntent,
          isPossessed: p.isPossessed
        }))
      };
      
      socket.emit('gameStateUpdate', gameState);
      
      console.log(`房间 ${roomId} 发送游戏状态`);
    } catch (err) {
      console.error(`获取游戏状态失败:`, err);
      socket.emit('debugResponse', { success: false, message: '获取游戏状态失败' });
    }
  });
  
  /**
   * 重连请求
   */
  socket.on('reconnect', ({ roomId, playerName }) => {
    try {
      const room = rooms[roomId];
      if (!room) {
        socket.emit('error', '房间不存在');
        return;
      }
      
      // 查找断线的玩家
      const player = room.players.find(p => 
        p.name === playerName && p.disconnected
      );
      
      if (!player) {
        socket.emit('error', '未找到断线的玩家');
        return;
      }
      
      // 清除重连超时
      if (room.reconnectTimers && room.reconnectTimers[player.id]) {
        clearTimeout(room.reconnectTimers[player.id]);
        delete room.reconnectTimers[player.id];
      }
      
      // 更新玩家ID
      const oldId = player.id;
      player.id = socket.id;
      player.disconnected = false;
      delete player.disconnectTime;
      
      // 更新角色映射
      if (room.roles && room.roles[oldId]) {
        room.roles[socket.id] = room.roles[oldId];
        delete room.roles[oldId];
      }
      
      // 更新队长
      if (room.currentLeader === oldId) {
        room.currentLeader = socket.id;
      }
      
      // 更新小队成员
      if (room.proposedTeam) {
        const idx = room.proposedTeam.indexOf(oldId);
        if (idx >= 0) {
          room.proposedTeam[idx] = socket.id;
        }
      }
      
      // 更新投票
      if (room.teamVotes && room.teamVotes[oldId] !== undefined) {
        room.teamVotes[socket.id] = room.teamVotes[oldId];
        delete room.teamVotes[oldId];
      }
      
      if (room.missionVotes && room.missionVotes[oldId] !== undefined) {
        room.missionVotes[socket.id] = room.missionVotes[oldId];
        delete room.missionVotes[oldId];
      }
      
      // 加入房间
      socket.join(roomId);
      
      // 通知所有玩家
      io.to(roomId).emit('playerJoined', {
        players: getPlayerList(room),
        count: room.players.filter(p => !p.disconnected).length,
        mode: room.mode
      });
      
      io.to(roomId).emit('playerReconnected', {
        playerId: socket.id,
        playerName: player.name
      });
      
      // 发送当前游戏状态给重连的玩家
      if (room.status === 'playing') {
        const role = room.roles[socket.id];
        console.log(`重连玩家 ${player.name}，角色: ${role}，房间状态: ${room.status}`);
        
        socket.emit('rolesRevealed', {
          role,
          roleLabel: gameData.ROLE_LABELS[role],
          roleDescription: gameData.ROLE_DESCRIPTIONS[role],
          players: room.players.map(p => ({
            id: p.id,
            name: p.name,
            position: p.position
          }))
        });
        
        socket.emit('phaseChange', {
          phase: room.currentPhase,
          roundNumber: room.currentRound,
          totalRounds: gameData.GAME_CONSTANTS.MAX_ROUNDS,
          proposedTeam: room.proposedTeam,
          missionResults: room.missionResults,
          timeLimit: 30 // 默认时间
        });
      } else {
        console.log(`重连玩家 ${player.name}，房间状态不是playing: ${room.status}`);
      }
      
      console.log(`玩家 ${player.name} 重连成功`);
    } catch (err) {
      console.error(`处理重连失败:`, err);
      socket.emit('error', '重连失败');
    }
  });
}

module.exports = { registerRoomHandlers };
