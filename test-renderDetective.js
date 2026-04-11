const fs = require('fs');
const path = require('path');
const assert = require('assert');

const sourcePath = path.join(__dirname, 'client/js/game.js');
const source = fs.readFileSync(sourcePath, 'utf8');
const match = source.match(/function renderDetective\(\) \{[\s\S]*?\n\}/);

assert(match, 'renderDetective() not found');
const body = match[0];

assert(
  body.includes("document.querySelector('.discussion-room')"),
  'renderDetective() should use .discussion-room on the game screen'
);

assert(
  !body.includes("document.querySelector('.round-table')"),
  'renderDetective() should not query the missing .round-table element'
);

console.log('renderDetective regression test passed');
