# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository overview

`谁是AI/` is a small Node.js + Socket.IO game app for “谁是AI”. The server is the source of truth for room/game state, and the browser client is a thin UI that renders the current phase and sends player actions over Socket.IO.

### High-level architecture

- `server/server.js` contains the entire game backend: Express setup, Socket.IO handlers, in-memory room state, phase progression, AI/player generation, voting, and timers.
- `client/js/game.js` contains the browser client logic: it opens the socket connection, reacts to server events, updates the UI, and sends player actions.
- `client/index.html` and `client/css/style.css` are the static UI shell served directly by Express.
- `test-socket.js` is a standalone smoke test that connects to the server with `socket.io-client` and verifies the join/game-start flow.

The game uses in-memory state only (`rooms` in `server/server.js`), so restarting the server resets all rooms.

## Common commands

From `谁是AI/`:

```bash
npm install
npm run server
node test-socket.js
```

- `npm install` installs runtime dependencies.
- `npm run server` starts the app on `http://localhost:3000`.
- `node test-socket.js` runs the socket smoke test against a local server.

## Working notes

- There is no build step or lint script in `package.json`.
- There is no formal unit test suite; `test-socket.js` is the main automated check available in the repo.
- Most gameplay behavior lives in one large server file, so changes to game flow usually require checking both server events and client event handlers together.
- When editing gameplay logic, keep the emitted socket event names and payload shapes aligned between `server/server.js` and `client/js/game.js`.
