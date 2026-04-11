const { rooms } = require('./state');

function createVoteFlow({ io, buildRoundSummary, advanceRound }) {
  function recordVote(roomId, playerId, targetId) {
    const room = rooms[roomId];
    if (!room || room.status !== 'action') return false;

    if (room.voteCount[playerId]) {
      return false;
    }

    room.voteCount[playerId] = targetId;
    room.roundActions[playerId] = buildRoundSummary(room, playerId, 'vote', targetId, `投给了 ${targetId}`);

    const allVoted = room.players.every((player) => room.voteCount[player.id]);
    if (allVoted) {
      advanceRound(roomId);
    }

    return true;
  }

  function recordDefend(roomId, playerId, statement) {
    const room = rooms[roomId];
    if (!room || room.status !== 'action') return false;

    io.to(roomId).emit('playerDefended', {
      playerId,
      statement
    });

    room.roundActions[playerId] = buildRoundSummary(room, playerId, 'defend', null, statement);
    const allActed = room.players.every((player) => room.roundActions[player.id]);
    if (allActed) {
      advanceRound(roomId);
    }

    return true;
  }

  return {
    recordVote,
    recordDefend
  };
}

module.exports = {
  createVoteFlow
};
