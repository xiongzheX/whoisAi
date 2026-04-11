const express = require('express');
const http = require('http');
const socketIO = require('socket.io');
const cors = require('cors');
const path = require('path');

const { createGameFlow } = require('./gameFlow');
const { createVoteFlow } = require('./voteFlow');
const { registerRoomHandlers } = require('./handlers/roomHandlers');
const { registerSkillHandlers } = require('./handlers/skillHandlers');
const { registerVoteHandlers } = require('./handlers/voteHandlers');
const { createRoom, resetRoom, buildRoundSummary } = require('./roomService');
const { aiAnswerToSkillQuestion } = require('./aiEngine');

const app = express();
const server = http.createServer(app);
const io = socketIO(server, {
  cors: {
    origin: '*',
    methods: ['GET', 'POST']
  }
});

const gameFlow = createGameFlow({ io });
const voteFlow = createVoteFlow({
  io,
  buildRoundSummary,
  advanceRound: gameFlow.advanceRound
});
const {
  addAIToRoom,
  startGame,
  advanceRound
} = gameFlow;

app.use(cors());
app.use(express.static(path.join(__dirname, '../client')));
app.get('/favicon.ico', (_req, res) => res.status(204).end());

io.on('connection', (socket) => {
  console.log('玩家连接:', socket.id);

  registerRoomHandlers({
    socket,
    io,
    createRoom,
    resetRoom,
    addAIToRoom,
    startGame,
    buildRoundSummary,
    advanceRound
  });

  registerSkillHandlers({
    socket,
    io,
    buildRoundSummary,
    aiAnswerToSkillQuestion,
    advanceRound
  });

  registerVoteHandlers({
    socket,
    recordVote: voteFlow.recordVote,
    recordDefend: voteFlow.recordDefend
  });
});

const PORT = process.env.PORT || 3000;
server.listen(PORT, () => {
  console.log(`服务器运行在端口 ${PORT}`);
});
