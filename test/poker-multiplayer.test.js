'use strict';
const { test } = require('node:test');
const assert = require('node:assert/strict');
const { Table } = require('../client/games/texas-holdem/engine.js');
const cards = text => text.split(' ').map(s => ({ rank: '23456789TJQKA'.indexOf(s[0]) + 2, suit: 'shcd'.indexOf(s[1]) }));
function seeded(initial) {
  let seed = initial;
  return () => { seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0; return seed / 4294967296; };
}
function playCalls(table) {
  let actions = 0;
  while (table.phase !== 'done') { assert.ok(actions++ < 1000); assert.ok(table.act('call')); }
}
test('reject tables outside the supported 2–9 seat range', () => {
  for (const size of [0, 1, 10, 2.5, NaN, '9']) assert.throws(() => new Table(Math.random, size), RangeError);
});
for (let count = 2; count <= 9; count++) {
  test(`${count} seats: all four streets, action order, fresh hand and button rotation`, () => {
    const t = new Table(seeded(140 + count), count);
    assert.equal(t.start(), true);
    assert.equal(t.players.length, count);
    const preflop = Array.from({ length: count }, (_, n) => ((count === 2 ? 0 : 3) + n) % count);
    for (const seat of preflop) { assert.equal(t.actor, seat); assert.equal(t.phase, 'preflop'); t.act('call'); }
    for (const phase of ['flop', 'turn', 'river']) {
      assert.equal(t.phase, phase);
      for (let n = 1; n <= count; n++) { assert.equal(t.actor, n % count); t.act('call'); }
    }
    assert.equal(t.phase, 'done');
    assert.equal(t.board.length, 5);
    assert.equal(t.players.reduce((sum, p) => sum + p.chips, 0), count * 1000);
    assert.equal(t.start(), true); assert.equal(t.dealer, 1); assert.equal(t.hand, 2);
    assert.equal(t.board.length, 0); assert.equal(t.pot, 30);
    playCalls(t);
  });
  test(`${count} seats: seeded mixed actions and bot decisions conserve chips over repeated games`, () => {
    for (let run = 0; run < 30; run++) {
      const random = seeded(count * 1000 + run);
      const t = new Table(random, count);
      for (let hand = 0; hand < 15 && t.start(); hand++) {
        let actions = 0;
        while (t.phase !== 'done') {
          assert.ok(actions++ < 2000, `stuck: ${count} seats, run ${run}`);
          const acting = t.players[t.actor];
          assert.ok(!acting.folded && acting.chips > 0);
          if (t.actor > 0 && run % 2 === 0) assert.ok(t.bot());
          else {
            const o = t.options(), roll = random();
            assert.ok(o.canRaise && roll < .25 ? t.act('raise', roll < .1 ? o.max : Math.min(o.max, o.min)) : t.act(roll > .82 && o.call ? 'fold' : 'call'));
          }
          assert.equal(t.players.reduce((sum, p) => sum + p.chips, t.pot), count * 1000);
          assert.ok(t.players.every(p => Number.isInteger(p.chips) && p.chips >= 0));
        }
        const dealt = [...t.board, ...t.players.flatMap(p => p.cards)].map(c => `${c.rank}:${c.suit}`);
        assert.equal(new Set(dealt).size, dealt.length);
        assert.equal(t.payouts.length, count);
      }
    }
  });
}
test('nine seats: full-table all-in runs out once and divides nine nested side pots', () => {
  const t = new Table(seeded(84), 9);
  t.players.forEach((p, i) => { p.chips = (i + 1) * 100; });
  t.start();
  let actions = 0;
  while (t.phase !== 'done') {
    assert.ok(actions++ < 30);
    const o = t.options();
    assert.ok(o.canRaise ? t.act('raise', o.max) : t.act('call'));
  }
  assert.equal(t.board.length, 5);
  assert.equal(t.players.reduce((sum, p) => sum + p.chips, 0), 4500);
  // Strongest hand has the smallest contribution: each successive winner covers one extra side pot.
  t.board = cards('Ts Jh Qd Ks 2h');
  const holes = ['As Ah', '9c 9h', 'Kc Kh', 'Qc Qh', 'Jc Jd', 'Tc Th', '8c 8h', '7c 7h', '6c 6h'];
  t.players.forEach((p, i) => Object.assign(p, { chips: 0, total: (i + 1) * 100, folded: false, cards: cards(holes[i]) }));
  t.pot = 4500; t.finish(true);
  assert.deepEqual(t.payouts, [900, 800, 700, 600, 500, 400, 300, 200, 100]);
});
test('nine seats: split pot odd chips follow seats left of dealer, across seat zero', () => {
  const t = new Table(Math.random, 9);
  t.board = cards('As Ks Qs Js Ts'); t.dealer = 4;
  t.players.forEach((p, i) => Object.assign(p, { chips: 0, total: i === 0 ? 0 : 1, folded: ![1, 4, 8].includes(i), cards: cards('2h 3h') }));
  t.pot = 8; t.finish(true);
  assert.deepEqual(t.payouts, [0, 3, 0, 0, 2, 0, 0, 0, 3]);
});
test('nine seats: skip busted seats and rotate to heads-up with sparse seat indices', () => {
  const t = new Table(seeded(59), 9);
  t.players.forEach((p, i) => { p.chips = [0, 8].includes(i) ? 4500 : 0; });
  t.start(); assert.equal(t.dealer, 0); assert.equal(t.small, 0); assert.equal(t.big, 8); assert.equal(t.actor, 0);
  t.act('call'); t.act('call'); assert.equal(t.actor, 8);
  playCalls(t); t.start(); assert.equal(t.dealer, 8); assert.equal(t.small, 8); assert.equal(t.big, 0); assert.equal(t.actor, 8);
  assert.ok(t.players.slice(1, 8).every(p => p.cards.length === 0 && p.folded));
});
test('nine seats: cumulative short all-ins reopen action only once they reach a full raise', () => {
  const t = new Table(Math.random, 9); t.start();
  t.act('raise', 100); // seat 3, full raise = 80
  t.players[4].chips = 130; t.act('raise', 130);
  t.players[5].chips = 180; t.act('raise', 180);
  while (t.actor !== 3) t.act('call');
  assert.equal(t.options().canRaise, true);
  assert.equal(t.options().min, 260);
  assert.equal(t.act('raise', 259), false);
  assert.equal(t.act('raise', 260), true);
});
