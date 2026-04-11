const { rooms } = require('../state');

function registerSkillHandlers({
  socket,
  io,
  buildRoundSummary,
  aiAnswerToSkillQuestion,
  advanceRound
}) {
  socket.on('useSkillObserve', ({ roomId, targetId }) => {
    const room = rooms[roomId];
    if (!room || room.status !== 'action') return;

    const skills = room.detectiveSkills[socket.id];
    if (!skills || skills.observe <= 0) {
      socket.emit('error', '观察技能次数已用完');
      return;
    }

    skills.observe--;
    const target = room.players.find(p => p.id === targetId);
    if (target) {
      io.to(roomId).emit('skillUsed', {
        playerId: socket.id,
        skillType: 'observe',
        targetId,
        description: target.description
      });
      room.roundActions[socket.id] = buildRoundSummary(room, socket.id, 'observe', targetId, `观察了 ${target.name}`);
    }
  });

  socket.on('useSkillQuestion', ({ roomId, targetId, question }) => {
    const room = rooms[roomId];
    if (!room || room.status !== 'action') return;

    const skills = room.detectiveSkills[socket.id];
    if (!skills || skills.question <= 0) {
      socket.emit('error', '质问技能次数已用完');
      return;
    }

    skills.question--;

    io.to(roomId).emit('skillUsed', {
      playerId: socket.id,
      skillType: 'question',
      targetId,
      question
    });

    room.roundActions[socket.id] = buildRoundSummary(room, socket.id, 'question', targetId, question);

    const target = room.players.find(p => p.id === targetId);
    if (target && target.isAI) {
      setTimeout(() => {
        const answer = aiAnswerToSkillQuestion(target.aiLevel, question, target.persona, target.id, room.roundEvent);
        io.to(roomId).emit('skillAnswered', {
          answererId: targetId,
          answer
        });
      }, 800);
    }
  });

  socket.on('useSkillListen', ({ roomId, targetId }) => {
    const room = rooms[roomId];
    if (!room || room.status !== 'action') return;

    const skills = room.detectiveSkills[socket.id];
    if (!skills || skills.listen <= 0) {
      socket.emit('error', '偷听技能次数已用完');
      return;
    }

    skills.listen--;

    const target = room.players.find(p => p.id === targetId);
    if (target) {
      io.to(roomId).emit('skillUsed', {
        playerId: socket.id,
        skillType: 'listen',
        targetId,
        targetName: target.name,
        isAI: target.isAI,
        hint: target.isAI ? '这个玩家的行为有点可疑...' : '这个玩家表现很自然'
      });
      room.roundActions[socket.id] = buildRoundSummary(room, socket.id, 'listen', targetId, `偷听了 ${target.name}`);
    }
  });

  socket.on('skipClueCollecting', ({ roomId }) => {
    const room = rooms[roomId];
    if (!room || room.status !== 'action') return;

    if (room.currentPhaseTimer) {
      clearTimeout(room.currentPhaseTimer);
      room.currentPhaseTimer = null;
    }

    room.roundActions[socket.id] = buildRoundSummary(room, socket.id, 'skip', null, '跳过本轮行动');
    const allActed = room.players.every(p => room.roundActions[p.id]);
    if (allActed) {
      room.currentPhaseTimer = setTimeout(() => advanceRound(roomId), 300);
    }
  });

  socket.on('askQuestion', ({ roomId, targetId, question }) => {
    const room = rooms[roomId];
    if (!room || room.status !== 'action') return;

    if (room.questionCount[socket.id] <= 0) {
      socket.emit('error', '提问次数已用完');
      return;
    }

    room.questionCount[socket.id]--;

    io.to(roomId).emit('questionAsked', {
      questionerId: socket.id,
      targetId,
      question,
      remainingQuestions: room.questionCount
    });

    room.roundActions[socket.id] = buildRoundSummary(room, socket.id, 'ask', targetId, question);

    const targetPlayer = room.players.find(p => p.id === targetId);
    if (targetPlayer && targetPlayer.isAI) {
      setTimeout(() => {
        const answer = aiAnswerToSkillQuestion(targetPlayer.aiLevel, question, targetPlayer.persona, targetPlayer.id, room.roundEvent);
        io.to(roomId).emit('questionAnswered', {
          answererId: targetPlayer.id,
          answer
        });
      }, 800);
    }
  });

  socket.on('rejectAnswer', ({ roomId }) => {
    const room = rooms[roomId];
    if (!room) return;

    io.to(roomId).emit('answerRejected', {
      playerId: socket.id
    });
  });

  socket.on('answerTimeout', ({ roomId, targetId }) => {
    const room = rooms[roomId];
    if (!room) return;

    io.to(roomId).emit('answerTimeout', {
      targetId
    });
  });
}

module.exports = {
  registerSkillHandlers
};
