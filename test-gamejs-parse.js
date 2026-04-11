const fs = require('fs');
const path = require('path');
const vm = require('vm');
const assert = require('assert');

const sourcePath = path.join(__dirname, 'client/js/game.js');
const source = fs.readFileSync(sourcePath, 'utf8');

assert.doesNotThrow(() => new vm.Script(source), 'client/js/game.js should parse without syntax errors');

console.log('game.js parse test passed');
