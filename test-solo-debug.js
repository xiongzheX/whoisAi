#!/usr/bin/env node

const assert = require('assert');
const io = require('socket.io-client');

const SERVER_URL = 'http://localhost:3000';
const ROOM_ID = `solo-debug-${Date.now()}`;
const PLAYER_NAME = `SoloTester-${Date.now().toString().slice(-4)}`;

async function main() {
  const socket = io(SERVER_URL, {
    transports: ['websocket'],
    timeout: 5000,
  });

  let roundIntroSeen = false;
  let gameStartedSeen = false;
  let autoFilledAI = 0;

  await new Promise((resolve, reject) => {
    const timeout = setTimeout(() => reject(new Error('timed out waiting for solo debug start')), 7000);

    socket.on('connect', () => {
      socket.emit('toggleSoloDebug', { enabled: true });
      socket.emit('joinRoom', {
        roomId: ROOM_ID,
        name: PLAYER_NAME,
        soloDebug: true,
      });
    });

    socket.on('playerJoined', ({ players }) => {
      autoFilledAI = players.filter(p => p.isAI).length;
    });

    socket.on('gameStarted', ({ roundNumber }) => {
      gameStartedSeen = true;
      assert.strictEqual(roundNumber, 1, 'solo debug should start at round 1');
    });

    socket.on('phaseChange', ({ phase, roundNumber }) => {
      if (phase === 'roundIntro') {
        roundIntroSeen = true;
        assert.strictEqual(roundNumber, 1, 'solo debug should enter roundIntro immediately');
        clearTimeout(timeout);
        socket.disconnect();
        resolve();
      }
    });

    socket.on('connect_error', reject);
    socket.on('error', reject);
  });

  assert.ok(gameStartedSeen, 'expected gameStarted event');
  assert.ok(roundIntroSeen, 'expected roundIntro event');
  assert.ok(autoFilledAI >= 1, 'expected AI to be auto-filled in solo debug');
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
