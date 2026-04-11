const { rooms } = require('./state');

function createRoom(roomId) {
  rooms[roomId] = {
    players: [],
    status: 'waiting',
    mode: 'normal',
    roundNumber: 0,
    currentWord: '',
    currentWordPair: [],
    wordDifficulty: 'medium',
    questionCount: {},
    voteCount: {},
    unansweredQuestions: [],
    defendingPlayers: [],
    debatingPlayers: [],
    questionRound: 1,
    detectiveSkills: {},
    playerTasks: {},
    aiPressure: {},
    roundActions: {},
    roundSummary: null,
    roundEventHistory: [],
    contributionScore: {},
    accuracyScore: {},
    influenceScore: {},
    currentPhaseTimer: null,
    roundEvent: null,
    roundTempoMultiplier: 1,
    matchResolved: false
  };
  console.log(`房间 ${roomId} 已创建`);
}

function resetRoom(roomId) {
  if (rooms[roomId]) {
    const oldStatus = rooms[roomId].status;
    if (rooms[roomId].currentPhaseTimer) {
      clearTimeout(rooms[roomId].currentPhaseTimer);
    }
    createRoom(roomId);
    console.log(`房间 ${roomId} 已重置 (原状态: ${oldStatus})`);
  }
}

function clearRoom(roomId) {
  if (rooms[roomId]) {
    if (rooms[roomId].currentPhaseTimer) {
      clearTimeout(rooms[roomId].currentPhaseTimer);
    }
    createRoom(roomId);
    console.log(`房间 ${roomId} 已清理`);
  }
}

function getRandomPlayer(roomId, excludeId) {
  const room = rooms[roomId];
  if (!room) return null;
  const candidates = room.players.filter(p => p.id !== excludeId);
  return candidates[Math.floor(Math.random() * candidates.length)] || null;
}

function buildRoundSummary(room, actorId, actionType, targetId, detail) {
  const actor = room.players.find(p => p.id === actorId);
  const target = room.players.find(p => p.id === targetId);
  const actorName = actor ? actor.name : 'Unknown';
  const targetName = target ? target.name : 'Unknown';
  return {
    roundNumber: room.roundNumber,
    actorId,
    actorName,
    actionType,
    targetId,
    targetName,
    detail,
    mostSuspiciousId: targetId,
    mostSuspiciousName: targetName,
    note: detail || `${actorName} 对 ${targetName} 采取了 ${actionType}`
  };
}

function getUniqueAIName(room) {
  const aiNames = ['AI_小明', 'AI_小红', 'AI_小李', 'AI_小王'];
  const usedNames = new Set(room.players.filter(p => p.isAI).map(p => p.name));
  const availableNames = aiNames.filter(name => !usedNames.has(name));
  if (availableNames.length > 0) {
    return availableNames[Math.floor(Math.random() * availableNames.length)];
  }

  let index = 1;
  let fallbackName = `AI_${index}`;
  while (usedNames.has(fallbackName)) {
    index += 1;
    fallbackName = `AI_${index}`;
  }
  return fallbackName;
}

module.exports = {
  createRoom,
  resetRoom,
  clearRoom,
  getRandomPlayer,
  buildRoundSummary,
  getUniqueAIName
};
