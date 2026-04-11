const fs = require('fs');
const path = require('path');
const assert = require('assert');

const sourcePath = path.join(__dirname, 'client/js/game.js');
const source = fs.readFileSync(sourcePath, 'utf8');
const match = source.match(/function drawPixelTable\(\) \{[\s\S]*?\n\}/);

assert(match, 'drawPixelTable() not found');
const body = match[0];

assert(
  body.includes("document.querySelector('.discussion-room')"),
  'drawPixelTable() should use .discussion-room as the table container'
);

assert(
  !body.includes("document.querySelector('.round-table')"),
  'drawPixelTable() should not query the missing .round-table element'
);

console.log('drawPixelTable regression test passed');
