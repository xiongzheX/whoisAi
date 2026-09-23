'use strict';
const { test } = require('node:test');
const assert = require('node:assert/strict');
const { Table, evaluate, compare } = require('../client/games/texas-holdem/engine.js');
const cards = text => text.split(' ').map(s => ({ rank: '23456789TJQKA'.indexOf(s[0]) + 2, suit: 'shcd'.indexOf(s[1]) }));
test('all hand categories, wheel, seven-card selection and kickers', () => {
  const hands = ['As Jh 9c 7d 2s', 'As Ah 9c 7d 2s', 'As Ah 9c 9d 2s', 'As Ah Ac 7d 2s', 'As 2h 3c 4d 5s', 'As Js 9s 7s 2s', 'As Ah Ac 7d 7s', 'As Ah Ac Ad 2s', 'As Ks Qs Js Ts'];
  hands.forEach((hand, index) => assert.equal(evaluate(cards(hand))[0], index));
  assert.deepEqual(evaluate(cards('As 2h 3c 4d 5s')), [4, 5]);
  assert.deepEqual(evaluate(cards('As Ah Ac Ks Kh Kc 2s')), [6, 14, 13]);
  assert.ok(compare(evaluate(cards('As Ah Kc 7d 2s')), evaluate(cards('Ac Ad Qc 7h 2c'))) > 0);
  assert.equal(compare(evaluate(cards('As Ks Qs Js Ts 2c 3c')), evaluate(cards('As Ks Qs Js Ts 4h 5h'))), 0);
});
test('legal actions, big blind option and all four betting rounds', () => {
  const t = new Table(); t.start();
  assert.equal(t.actor, 3); assert.equal(t.pot, 30);
  assert.equal(t.act('raise', 39), false);
  assert.equal(t.act('raise', NaN), false);
  for (let i = 0; i < 3; i++) t.act('call');
  assert.equal(t.actor, t.big); assert.equal(t.phase, 'preflop');
  t.act('call'); assert.equal(t.phase, 'flop'); assert.equal(t.board.length, 3);
  for (let n = 0; n < 12; n++) assert.ok(t.act('call'));
  assert.equal(t.phase, 'done'); assert.equal(t.board.length, 5);
  assert.equal(t.players.reduce((sum, p) => sum + p.chips, 0), 4000);
});
test('side pots award only amounts covered, tied pots split in seat order', () => {
  const t = new Table();
  t.dealer = 0; t.board = cards('2s 3h 7c 8d 9s');
  [100, 200, 300, 300].forEach((total, i) => Object.assign(t.players[i], { chips: 0, total, folded: false, cards: cards(['As Ah', 'Ks Kh', 'Qs Qh', 'Js Jh'][i]) }));
  t.pot = 900; t.finish(true);
  assert.deepEqual(t.payouts, [400, 300, 200, 0]);
  t.board = cards('As Ks Qs Js Ts'); t.pot = 900;
  t.players.forEach(p => { p.chips = 0; }); t.finish(true);
  assert.deepEqual(t.payouts, [100, 200, 300, 300]);
});
test('short all-in does not reopen raising to players who already acted', () => {
  const t = new Table(); t.start();
  t.act('raise', 100); // seat 3
  t.act('call'); // seat 0
  t.players[1].chips = 110; // small blind can reach 120 only
  t.act('raise', 120);
  t.act('call'); // seat 2
  assert.equal(t.actor, 3);
  assert.equal(t.options().canRaise, false);
  assert.equal(t.act('raise', 200), false);
  assert.ok(t.act('call'));
});
test('heads-up dealer is small blind, acts first preflop and last postflop', () => {
  const t = new Table(); t.players[2].chips = 0; t.players[3].chips = 0;
  t.start(); assert.equal(t.small, t.dealer); assert.equal(t.actor, t.dealer);
  t.act('call'); t.act('call'); assert.equal(t.actor, t.big);
});
test('random action simulations terminate, preserve chips and never duplicate cards', () => {
  let seed = 823;
  const random = () => { seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0; return seed / 4294967296; };
  for (let trial = 0; trial < 100; trial++) {
    const t = new Table(random);
    for (let hand = 0; hand < 10 && t.start(); hand++) {
      let actions = 0;
      while (t.phase !== 'done') {
        assert.ok(actions++ < 300, 'hand must terminate');
        const o = t.options(), roll = random();
        if (o.canRaise && roll < .2) t.act('raise', random() < .4 ? o.max : Math.min(o.max, o.min));
        else t.act(roll > .85 && o.call ? 'fold' : 'call');
        assert.equal(t.players.reduce((sum, p) => sum + p.chips, t.pot), 4000);
        assert.ok(t.players.every(p => Number.isInteger(p.chips) && p.chips >= 0));
      }
      const dealt = [...t.board, ...t.players.flatMap(p => p.cards)].map(c => `${c.rank}:${c.suit}`);
      assert.equal(new Set(dealt).size, dealt.length);
    }
  }
});
