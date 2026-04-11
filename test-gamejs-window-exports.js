const fs = require('fs');
const path = require('path');
const assert = require('assert');

const sourcePath = path.join(__dirname, 'client/js/game.js');
const source = fs.readFileSync(sourcePath, 'utf8');

for (const name of ['init', 'joinGame', 'setGameMode', 'toggleTestMode', 'toggleSoloDebug']) {
  assert(
    source.includes(`window.${name} = ${name};`),
    `${name} should be exported on window`
  );
}

console.log('game.js window export test passed');
