#!/usr/bin/env node

const assert = require('assert');
const fs = require('fs');
const vm = require('vm');
const path = require('path');

const gameJs = fs.readFileSync(path.join(__dirname, 'client/js/game.js'), 'utf8');

const modeStatus = { textContent: '' };
const socket = {
  on: () => {},
  emit: () => {},
  connected: true,
};

const context = {
  console,
  window: {
    location: {
      protocol: 'http:',
      hostname: 'localhost',
      port: '3000',
    },
  },
  document: {
    getElementById: (id) => {
      if (id === 'modeStatus') return modeStatus;
      return null;
    },
    querySelectorAll: () => [],
    querySelector: () => null,
  },
  io: () => socket,
  alert: () => {},
  confirm: () => true,
  prompt: () => '',
  setTimeout,
  clearTimeout,
  Math,
  Date,
  JSON,
  Array,
  Object,
  String,
  Number,
  Boolean,
  RegExp,
  Promise,
  performance: { now: () => 0 },
  localStorage: {
    getItem: () => 'test',
    setItem: () => {},
  },
};
context.global = context;
context.window = context;

vm.createContext(context);
vm.runInContext(gameJs, context, { filename: 'client/js/game.js' });

assert.strictEqual(typeof context.window.onload, 'function', 'expected window.onload to be assigned');
assert.doesNotThrow(() => context.window.onload(), 'page init should not throw when it runs');
assert.strictEqual(modeStatus.textContent, 'TEST MODE', 'expected mode status text to stay in test mode');
