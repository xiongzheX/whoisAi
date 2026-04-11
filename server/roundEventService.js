const { rooms } = require('./state');
const { getRandomRoundEvent } = require('./gameData');

function createRoundEventService({ io }) {
  function triggerRoundEvent(roomId) {
    const room = rooms[roomId];
    if (!room || room.matchResolved) return null;

    const eventChance = room.mode === 'solo' ? 0.55 : 0.45;
    if (Math.random() > eventChance) {
      room.roundEvent = null;
      room.roundTempoMultiplier = 1;
      return null;
    }

    const event = getRandomRoundEvent();
    const aiPlayers = room.players.filter((player) => player.isAI);
    const eventTarget = aiPlayers.length ? aiPlayers[Math.floor(Math.random() * aiPlayers.length)] : null;

    room.roundEvent = {
      ...event,
      targetId: event.id === 'tempo_shift' ? null : eventTarget?.id || null,
      targetName: eventTarget?.name || null
    };
    room.roundTempoMultiplier = event.tempoMultiplier || 1;
    room.roundEventHistory = room.roundEventHistory || [];
    room.roundEventHistory.push({
      roundNumber: room.roundNumber,
      type: event.id,
      label: event.label,
      targetId: room.roundEvent.targetId,
      targetName: room.roundEvent.targetName
    });
    room.roundEventHistory = room.roundEventHistory.slice(-8);

    if (eventTarget && event.pressureBoost) {
      room.aiPressure[eventTarget.id] = (room.aiPressure[eventTarget.id] || 0) + event.pressureBoost;
    }

    io.to(roomId).emit('roundEvent', {
      type: event.id,
      label: event.label,
      message: eventTarget && event.id !== 'tempo_shift'
        ? `${event.message} 目标：${eventTarget.name}`
        : event.message,
      targetId: room.roundEvent.targetId,
      targetName: room.roundEvent.targetName
    });

    return room.roundEvent;
  }

  return {
    triggerRoundEvent
  };
}

module.exports = {
  createRoundEventService
};
