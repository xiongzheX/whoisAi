const { rooms, TEST_MODE, MAX_ROOM_PLAYERS } = require('../state');
const { getModeConfig } = require('../gameData');

function registerRoomHandlers({
  socket,
  io,
  createRoom,
  resetRoom,
  addAIToRoom,
  startGame,
  buildRoundSummary,
  advanceRound
}) {
  socket.on('toggleTestMode', ({ enabled }) => {
    TEST_MODE.enabled = enabled;
    console.log('测试模式切换:', enabled ? 'ON' : 'OFF');
  });

  socket.on('resetRoom', ({ roomId }) => {
    resetRoom(roomId);
    io.to(roomId).emit('roomReset', { roomId });
    console.log(`房间 ${roomId} 已被重置`);
  });

  socket.on('joinRoom', ({ roomId, name, mode, testMode: isTestMode }) => {
    const selectedMode = mode === 'solo' ? 'solo' : (isTestMode || TEST_MODE.enabled ? 'test' : 'normal');
    if (rooms[roomId] && rooms[roomId].status === 'finished') {
      createRoom(roomId);
    }

    if (!rooms[roomId]) {
      createRoom(roomId);
    }

    const room = rooms[roomId];
    room.mode = selectedMode;

    if (room.players.length >= MAX_ROOM_PLAYERS) {
      socket.emit('error', '房间人数已满');
      return;
    }

    const isAI = name.startsWith('AI_');
    const player = {
      id: socket.id,
      name,
      isAI,
      description: '',
      position: room.players.length
    };

    room.players.push(player);
    room.questionCount[socket.id] = 3;

    socket.join(roomId);
    io.to(roomId).emit('playerJoined', {
      players: room.players
    });

    console.log(`房间 ${roomId} 当前玩家数: ${room.players.length}, 玩家:`, room.players.map(p => p.name));

    const modeConfig = getModeConfig(room);
    if (modeConfig.autoFillAI) {
      while (room.players.length < Math.min(modeConfig.minPlayers, MAX_ROOM_PLAYERS)) {
        console.log(`${selectedMode} mode：添加AI`);
        addAIToRoom(roomId);
      }
    }

    if (selectedMode === 'solo' && room.players.length < MAX_ROOM_PLAYERS) {
      while (room.players.length < MAX_ROOM_PLAYERS) {
        console.log('solo mode：补足AI到6人');
        addAIToRoom(roomId);
      }
    }

    const shouldStartNow = selectedMode === 'solo'
      || selectedMode === 'test'
      || (room.players.length >= MAX_ROOM_PLAYERS && selectedMode === 'normal');

    if (shouldStartNow) {
      console.log(`${selectedMode} mode：游戏开始！`);
      startGame(roomId);
    }
  });

  socket.on('submitDescription', ({ roomId, description }) => {
    const room = rooms[roomId];
    if (!room || room.status !== 'action') return;

    const player = room.players.find(p => p.id === socket.id);
    if (!player) return;

    player.description = description;
    room.roundActions[player.id] = buildRoundSummary(room, player.id, 'describe', null, description);
    io.to(roomId).emit('descriptionSubmitted', {
      playerId: socket.id,
      description
    });

    const allActed = room.players.every(p => room.roundActions[p.id]);
    if (allActed) {
      if (room.currentPhaseTimer) {
        clearTimeout(room.currentPhaseTimer);
      }
      room.currentPhaseTimer = setTimeout(() => advanceRound(roomId), 500);
    }
  });

  socket.on('disconnect', () => {
    console.log('玩家断开:', socket.id);
    for (const [roomId, room] of Object.entries(rooms)) {
      const index = room.players.findIndex(p => p.id === socket.id);
      if (index !== -1) {
        room.players.splice(index, 1);
        delete room.questionCount[socket.id];
        delete room.voteCount[socket.id];
        delete room.detectiveSkills[socket.id];
        delete room.playerTasks[socket.id];
        delete room.aiPressure[socket.id];
        delete room.roundActions[socket.id];
        io.to(roomId).emit('playerJoined', { players: room.players });
        break;
      }
    }
  });
}

module.exports = {
  registerRoomHandlers
};
