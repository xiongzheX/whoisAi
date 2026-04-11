const { rooms } = require('./state');

function createRoundResolution({ io }) {
  function buildAiProfile(room) {
    const personas = ['analytical', 'cautious', 'confrontational', 'quirky'];
    const personaCounts = personas.reduce((acc, persona) => {
      acc[persona] = room.players.filter((player) => player.isAI && player.persona === persona).length;
      return acc;
    }, {});

    const dominantPersona = personas
      .map((persona) => ({ persona, count: personaCounts[persona] }))
      .sort((a, b) => b.count - a.count)[0];

    const aiActions = room.players
      .filter((player) => player.isAI)
      .map((player) => ({
        id: player.id,
        name: player.name,
        persona: player.persona,
        actionCount: Object.values(room.roundActions).filter((action) => action.actorId === player.id).length,
        pressure: room.aiPressure[player.id] || 0
      }))
      .sort((a, b) => {
        if (b.actionCount !== a.actionCount) return b.actionCount - a.actionCount;
        return b.pressure - a.pressure;
      });

    const dominantAI = aiActions[0] || null;

    const eventLabels = (room.roundEventHistory || []).map((event) => event.label);
    const eventTrail = eventLabels.length ? [...new Set(eventLabels)] : [];

    return {
      personaCounts,
      dominantPersona: dominantPersona?.persona || 'cautious',
      dominantPersonaLabel: dominantPersona?.persona || '谨慎型',
      dominantAI,
      eventTrail,
      eventCount: room.roundEventHistory?.length || 0,
      pressureTop: aiActions.slice(0, 3)
    };
  }

  function buildMatchScores(room) {
    const contributionScore = {};
    const accuracyScore = {};
    const influenceScore = {};

    room.players.forEach((player) => {
      contributionScore[player.id] = room.roundActions[player.id] ? 1 : 0;
      accuracyScore[player.id] = room.playerTasks[player.id]?.completed ? 1 : 0;
      influenceScore[player.id] = room.roundActions[player.id]?.targetId ? 1 : 0;
    });

    return {
      contributionScore,
      accuracyScore,
      influenceScore,
      aiPressure: Object.keys(room.roundActions).length,
      humanSignal: room.players.length
    };
  }

  function resolveMatch(roomId) {
    const room = rooms[roomId];
    if (!room) return;

    const scores = buildMatchScores(room);
    const winner = scores.aiPressure >= scores.humanSignal ? 'humans' : 'ai';
    io.to(roomId).emit('gameFinished', {
      winner,
      voteResults: room.voteCount,
      scores,
      summary: room.roundSummary,
      tasks: room.playerTasks,
      aiProfile: buildAiProfile(room)
    });

    setTimeout(() => {
      if (rooms[roomId]) {
        rooms[roomId].players = [];
        rooms[roomId].status = 'waiting';
      }
    }, 10000);
  }

  return {
    buildMatchScores,
    buildAiProfile,
    resolveMatch
  };
}

module.exports = {
  createRoundResolution
};
