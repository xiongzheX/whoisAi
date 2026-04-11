# 谁是AI Server Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split the migrated `谁是AI` server into focused modules without changing the current gameplay flow or Socket.IO contract.

**Architecture:** Keep `server/server.js` as the entrypoint, but move shared state, room helpers, and AI/round logic into small server modules. The entrypoint should only bootstrap Express/Socket.IO, register handlers, and connect dependencies. Preserve existing event names and payload shapes so the client keeps working.

**Tech Stack:** Node.js, Express, Socket.IO, CommonJS modules

---

### Task 1: Extract shared state and room helpers

**Files:**
- Create: `server/state.js`
- Modify: `server/server.js`

- [ ] **Step 1: Write the minimal module boundary**

```js
// server/state.js
module.exports = {
  rooms: {},
  TEST_MODE: { enabled: false },
  MAX_ROOM_PLAYERS: 6,
};
```

- [ ] **Step 2: Move room creation/reset helpers out of `server.js`**
- [ ] **Step 3: Wire `server.js` to import the shared state**
- [ ] **Step 4: Start the server and verify room join still works**

### Task 2: Extract AI and round-flow helpers

**Files:**
- Create: `server/aiEngine.js`
- Create: `server/gameFlow.js`
- Modify: `server/server.js`

- [ ] **Step 1: Move pure AI generation helpers into `aiEngine.js`**
- [ ] **Step 2: Move round summary / match resolution helpers into `gameFlow.js`**
- [ ] **Step 3: Inject `io` and shared state into the flow helpers instead of reading globals**
- [ ] **Step 4: Verify `gameStarted`, `phaseChange`, `questionAsked`, and `gameFinished` still emit**

### Task 3: Clean branding and metadata

**Files:**
- Modify: `package.json`
- Modify: `README.md`
- Modify: `client/index.html`
- Modify: `client/test.html`

- [ ] **Step 1: Normalize package name/description to the new product**
- [ ] **Step 2: Replace leftover old-theme text in client and docs**
- [ ] **Step 3: Verify the browser title and login copy show 谁是AI**

### Task 4: Smoke test the migrated project

**Files:**
- Modify: none

- [ ] **Step 1: Run the server**
- [ ] **Step 2: Run the socket smoke test**
- [ ] **Step 3: Fix any runtime regressions caused by the extraction**
- [ ] **Step 4: Re-run the checks until the join/start flow is stable**

