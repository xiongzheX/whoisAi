const { io } = require('socket.io-client');

const gameId = process.argv[2];
const roomId = process.argv[3];
const name = process.argv[4] || '自动玩家';

const configs = {
  'bean-sprint': {
    name: '小橙',
    stats: { burst: 6, speed: 7, stamina: 6, stability: 6, reaction: 6, grit: 5 },
    strategy: { start: 'normal', middle: 'steady', sprint: 'late' }
  },
  'dumpling-sumo': {
    name: '圆圆',
    stats: { power: 6, weight: 7, balance: 7, footwork: 5, stamina: 6, spirit: 5 },
    style: 'defensive'
  }
};

if (!configs[gameId] || !roomId) {
  throw new Error('usage: node test/duel-smoke-client.js <game-id> <room-id> [name]');
}

const socket = io('http://127.0.0.1:3014', { transports: ['websocket'] });
const timeout = setTimeout(() => {
  console.error('duel smoke test timed out');
  process.exit(1);
}, 10000);

socket.on('connect', () => {
  socket.emit('joinDuelRoom', {
    gameId,
    roomId,
    name,
    playerToken: `smoke_${gameId.replaceAll('-', '_')}_${Date.now()}`
  });
});

socket.on('roomJoined', () => {
  socket.emit('duelReady', { gameId, roomId, config: configs[gameId] });
});

socket.on('duelState', (state) => {
  if (state.gameId !== gameId || state.roomCode !== roomId || state.phase !== 'finished') return;
  clearTimeout(timeout);
  console.log(JSON.stringify({
    gameId,
    sessionId: state.sessionId,
    round: state.round,
    playerCount: state.players.length,
    winner: state.result?.match?.winner
  }));
  socket.close();
  process.exit(0);
});

socket.on('error', (message) => {
  clearTimeout(timeout);
  console.error(message);
  process.exit(1);
});
