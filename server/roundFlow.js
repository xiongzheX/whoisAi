const { rooms } = require('./state');
const { getModeConfig, getRandomWord } = require('./gameData');
const { buildRoundSummary } = require('./roomService');
const { createRoundEventService } = require('./roundEventService');
const { createRoundResolution } = require('./roundResolution');

function createRoundFlow({ io, assignDetectiveTasks, scheduleAIActions }) {
  const roundEventFlow = createRoundEventService({ io });
  const roundResolution = createRoundResolution({ io });

  function initializeMatch(roomId) {
    const room = rooms[roomId];
    if (!room) return;

    const modeConfig = getModeConfig(room);
    const difficulties = ['low', 'medium', 'high'];
    room.wordDifficulty = difficulties[Math.floor(Math.random() * difficulties.length)];
    const wordPair = getRandomWord(room.wordDifficulty);
    room.currentWordPair = wordPair;
    room.currentWord = wordPair[0];
    room.roundNumber = 1;
    room.status = 'roundIntro';
    room.matchResolved = false;
    room.roundActions = {};
    room.roundSummary = null;
    room.voteCount = {};
    room.defendingPlayers = [];
    room.debatingPlayers = [];
    room.roundEvent = null;
    room.roundTempoMultiplier = 1;
    room.players = room.players.sort(() => Math.random() - 0.5);
    room.players.forEach((player, index) => {
      player.position = index;
      player.description = '';
      player.roundAction = null;
      player.aiWord = player.isAI ? wordPair[1] : wordPair[0];
    });
    assignDetectiveTasks(roomId);

    io.to(roomId).emit('gameStarted', {
      word: room.currentWord,
      players: room.players,
      wordDifficulty: room.wordDifficulty,
      roundNumber: room.roundNumber
    });
    io.to(roomId).emit('phaseChange', {
      phase: 'roundIntro',
      word: room.currentWord,
      roundNumber: room.roundNumber,
      summary: null
    });

    return modeConfig;
  }

  function recordRoundAction(roomId, playerId, actionType, targetId, detail) {
    const room = rooms[roomId];
    if (!room || room.matchResolved) return false;
    if (room.roundActions[playerId]) return false;

    room.roundActions[playerId] = buildRoundSummary(room, playerId, actionType, targetId, detail);
    const allActed = room.players.every((player) => room.roundActions[player.id]);
    if (allActed) {
      if (room.currentPhaseTimer) {
        clearTimeout(room.currentPhaseTimer);
      }
      room.currentPhaseTimer = setTimeout(() => advanceRound(roomId), 250);
    }
    return true;
  }

  function startRoundActionPhase(roomId) {
    const room = rooms[roomId];
    if (!room || room.matchResolved) return;
    room.status = 'action';
    roundEventFlow.triggerRoundEvent(roomId);
    io.to(roomId).emit('phaseChange', {
      phase: 'action',
      roundNumber: room.roundNumber,
      word: room.currentWord
    });
    scheduleAIActions(roomId, recordRoundAction);
  }

  function advanceRound(roomId) {
    const room = rooms[roomId];
    if (!room || room.matchResolved) return;

    const actions = Object.values(room.roundActions);
    const mostSuspicious = actions[0] || null;
    const targetId = mostSuspicious ? mostSuspicious.targetId : null;
    room.roundSummary = {
      roundNumber: room.roundNumber,
      actions,
      mostSuspiciousId: targetId,
      mostSuspiciousName: targetId ? (room.players.find((player) => player.id === targetId)?.name || 'Unknown') : null,
      message: actions.length
        ? `第 ${room.roundNumber} 轮已结算，${actions.length} 个动作产生了线索。`
        : `第 ${room.roundNumber} 轮没有有效动作。`
    };
    io.to(roomId).emit('roundSummary', room.roundSummary);

    room.roundNumber += 1;
    room.roundActions = {};

    if (room.roundNumber > 3) {
      room.matchResolved = true;
      room.status = 'finished';
      roundResolution.resolveMatch(roomId);
      return;
    }

    room.status = 'roundIntro';
    room.roundEvent = null;
    room.roundTempoMultiplier = 1;
    io.to(roomId).emit('phaseChange', {
      phase: 'roundIntro',
      roundNumber: room.roundNumber,
      summary: room.roundSummary
    });

    if (room.currentPhaseTimer) {
      clearTimeout(room.currentPhaseTimer);
    }
    room.currentPhaseTimer = setTimeout(() => startRoundActionPhase(roomId), 1200);
  }

  function startGame(roomId) {
    const room = rooms[roomId];
    if (!room) return;

    initializeMatch(roomId);

    if (room.currentPhaseTimer) {
      clearTimeout(room.currentPhaseTimer);
    }
    room.currentPhaseTimer = setTimeout(() => {
      startRoundActionPhase(roomId);
    }, 1200);
  }

  return {
    initializeMatch,
    recordRoundAction,
    startRoundActionPhase,
    advanceRound,
    startGame,
    ...roundEventFlow,
    ...roundResolution
  };
}

module.exports = {
  createRoundFlow
};
