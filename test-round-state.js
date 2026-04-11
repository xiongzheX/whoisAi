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

  let aiTargets = [];

  await new Promise((resolve, reject) => {
    const timeout = setTimeout(() => reject(new Error('timed out waiting for round intro')), 7000);

    socket.on('connect', () => {
      socket.emit('joinRoom', {
        roomId: ROOM_ID,
        name: PLAYER_NAME,
        mode: 'solo',
      });
    });

    socket.on('questionAsked', ({ questionerId, targetId }) => {
      if (questionerId !== targetId) {
        aiTargets.push({ questionerId, targetId });
      }
      if (aiTargets.length >= 1) {
        clearTimeout(timeout);
        socket.disconnect();
        resolve();
      }
    });

    socket.on('skillUsed', ({ playerId, targetId }) => {
      if (playerId !== targetId) {
        aiTargets.push({ playerId, targetId });
      }
      if (aiTargets.length >= 1) {
        clearTimeout(timeout);
        socket.disconnect();
        resolve();
      }
    });

    socket.on('connect_error', reject);
    socket.on('error', reject);
  });

  assert.ok(aiTargets.length >= 1, 'expected at least one AI action targeting another player');
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
