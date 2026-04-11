const fs = require('fs');
const path = require('path');
const assert = require('assert');

const sourcePath = path.join(__dirname, 'client/js/game.js');
const source = fs.readFileSync(sourcePath, 'utf8');
const match = source.match(/function drawPixelTable\(\) \{[\s\S]*?\n\}/);

assert(match, 'drawPixelTable() not found');
const body = match[0];

assert(!body.includes('baseRadius + 6'), 'drawPixelTable() should not draw extra outer circle rings');
assert(!body.includes('baseRadius + 12'), 'drawPixelTable() should not draw extra outer circle rings');
assert(!body.includes('baseRadius + 18'), 'drawPixelTable() should not draw extra outer circle rings');

console.log('drawPixelTable ring regression test passed');
