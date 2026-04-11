const assert = require('assert');
const fs = require('fs');
const vm = require('vm');
const path = require('path');

const gameJs = fs.readFileSync(path.join(__dirname, 'client/js/game.js'), 'utf8');

const calls = [];
function createMockClassList() {
  const classes = new Set();
  return {
    add: (...tokens) => tokens.forEach(token => classes.add(token)),
    remove: (...tokens) => tokens.forEach(token => classes.delete(token)),
    toggle: (token, force) => {
      if (force === true) {
        classes.add(token);
        return true;
      }
      if (force === false) {
        classes.delete(token);
        return false;
      }
      if (classes.has(token)) {
        classes.delete(token);
        return false;
      }
      classes.add(token);
      return true;
    },
    contains: (token) => classes.has(token),
    toString: () => Array.from(classes).join(' '),
  };
}

function createMockElement(tagName = 'div') {
  return {
    tagName: tagName.toUpperCase(),
    children: [],
    innerHTML: '',
    textContent: '',
    className: '',
    dataset: {},
    style: {},
    disabled: false,
    classList: createMockClassList(),
    appendChild(child) {
      this.children.push(child);
      return child;
    },
    insertBefore(child, before) {
      const index = this.children.indexOf(before);
      if (index === -1) {
        this.children.push(child);
      } else {
        this.children.splice(index, 0, child);
      }
      return child;
    },
    remove() {
      this._removed = true;
    },
    setAttribute(name, value) {
      this[name] = value;
    },
    querySelector: () => null,
    querySelectorAll: () => [],
  };
}

const modeStatus = createMockElement();
const skillPanel = createMockElement();
const messageList = createMockElement();
const elements = {
  modeStatus,
  skillPanel,
  messageList,
};
const socket = {
  on: () => {},
  emit: () => {},
  connected: true,
};

const context = {
  console,
  window: {},
  document: {
    getElementById: (id) => elements[id] || null,
    querySelectorAll: () => [],
    querySelector: () => null,
    createElement: (tagName) => createMockElement(tagName),
    createElementNS: (ns, tagName) => createMockElement(tagName),
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

let error = null;
try {
  vm.runInContext(gameJs, context, { filename: 'client/js/game.js' });
} catch (e) {
  error = e;
}

assert.strictEqual(error, null, `expected client script to load without throwing, got: ${error && error.message}`);
assert.strictEqual(typeof context.window.onload, 'function', 'expected window.onload to be assigned');
assert.strictEqual(typeof context.getLobbyHint, 'function', 'expected getLobbyHint to be assigned');
assert.strictEqual(typeof context.getPhaseCopy, 'function', 'expected getPhaseCopy to be assigned');
assert.ok(typeof context.document.querySelector === 'function', 'expected document querySelector to exist');

context.setGameMode('solo');
assert.strictEqual(modeStatus.textContent, 'SOLO DEBUG', 'expected mode status text to show SOLO DEBUG');
assert.strictEqual(context.getLobbyHint('solo'), '1 PLAYER TO START', 'expected solo lobby hint to be short');
assert.strictEqual(context.getLobbyHint('test'), '6 PLAYERS TO START', 'expected test lobby hint to be short');
assert.strictEqual(context.getPhaseCopy('action', 3), 'ROUND 3: ACT', 'expected action phase copy to be concise');
assert.strictEqual(context.getPhaseCopy('roundIntro', 2, { message: 'ROUND 2 START!' }), 'ROUND 2 START!', 'expected round intro to preserve summary text');

context.renderSkillPanel();
assert.ok(skillPanel.innerHTML.includes('我的技能') || skillPanel.children.length > 0, 'expected skill panel to render Chinese content');
