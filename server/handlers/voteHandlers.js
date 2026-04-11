function registerVoteHandlers({
  socket,
  recordVote,
  recordDefend
}) {
  socket.on('vote', ({ roomId, targetId }) => {
    recordVote(roomId, socket.id, targetId);
  });

  socket.on('defend', ({ roomId, statement }) => {
    recordDefend(roomId, socket.id, statement);
  });
}

module.exports = {
  registerVoteHandlers
};
