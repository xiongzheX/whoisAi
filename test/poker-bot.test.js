'use strict';
const { test } = require('node:test');
const assert = require('node:assert/strict');
const { Table, decideBot, assessHand, styles } = require('../client/games/texas-holdem/engine.js');
const cards = text => text.split(' ').map(s => ({ rank: '23456789TJQKA'.indexOf(s[0]) + 2, suit: 'shcd'.indexOf(s[1]) }));
function seeded(seed) { return () => { seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0; return seed / 4294967296; }; }
function view(overrides = {}) {
  return { seat: 5, cards: cards('As Kh'), board: [], chips: 1000, pot: 100, currentBet: 20,
    options: { call: 20, min: 40, max: 1000, canRaise: true }, opponents: [{ seat: 0, chips: 1000, recent: [] }], latePosition: true, lastAggressor: 0, ...overrides };
}
function sample(v, n = 2000) {
  const random = seeded(123), result = { fold: 0, call: 0, raise: 0, amounts: new Set() };
  for (let i = 0; i < n; i++) { const a = decideBot(v, random); result[a.kind]++; if (a.amount) result.amounts.add(a.amount); }
  return result;
}
test('weak hands can fold to small bets; stronger hands continue more often', () => {
  const weak = sample(view({ cards: cards('7s 2h') }));
  const strong = sample(view({ cards: cards('As Ah') }));
  assert.ok(weak.fold > 0); assert.ok(strong.fold < weak.fold); assert.ok(strong.raise > weak.raise);
});
test('multiway pots and expensive calls tighten weak-hand decisions', () => {
  const base = view({ cards: cards('8s 6h') });
  const headsUp = sample(base);
  const crowded = sample({ ...base, opponents: Array.from({ length: 8 }, (_, seat) => ({ seat, recent: [] })) });
  const expensive = sample({ ...base, options: { ...base.options, call: 700 } });
  assert.ok(crowded.fold > headsUp.fold); assert.ok(expensive.fold > headsUp.fold);
});
test('personalities change action frequencies and bets scale with pot', () => {
  const strong = { cards: cards('As Ah') };
  const aggressive = sample(view({ ...strong, seat: 5 }));
  const passive = sample(view({ ...strong, seat: 6 }));
  assert.ok(aggressive.raise > passive.raise * 2);
  assert.ok(aggressive.amounts.size > 2);
  const bigPot = sample(view({ ...strong, pot: 500 }));
  assert.ok(Math.max(...bigPot.amounts) > Math.max(...aggressive.amounts));
  assert.equal(new Set(styles.slice(1).map(s => s.label)).size, 8);
});
test('draws count before river; a shared royal flush is not a private monster', () => {
  const draw = assessHand(cards('Ah Kh'), cards('Qh 7h 2c'), 1);
  assert.ok(draw.draw > 0);
  assert.equal(assessHand(cards('Ah Kh'), cards('Qh 7h 2c 3s 9d'), 1).draw, 0);
  const shared = assessHand(cards('2c 3d'), cards('As Ks Qs Js Ts'), 4);
  assert.ok(shared.equity < .2);
});
test('recent public aggression and folds influence decisions', () => {
  const base = view({ cards: cards('8s 6h'), pot: 20 });
  const tight = sample({ ...base, opponents: [{ seat: 0, recent: Array(12).fill('check') }] });
  const loose = sample({ ...base, opponents: [{ seat: 0, recent: Array(12).fill('raise') }] });
  assert.ok(loose.fold < tight.fold);
  const steal = { ...base, options: { ...base.options, call: 0 } };
  assert.ok(sample({ ...steal, opponents: [{ seat: 0, recent: Array(12).fill('fold') }] }).raise > sample({ ...steal, opponents: [{ seat: 0, recent: Array(12).fill('call') }] }).raise);
});
test('bot view excludes opponent cards and deck; swapping secrets cannot change decision', () => {
  const table = new Table(seeded(6), 9); table.start();
  const before = table.botView();
  const decision = decideBot(before, seeded(10));
  table.players[0].cards = cards('As Ah'); table.deck.reverse();
  assert.deepEqual(table.botView(), before);
  assert.deepEqual(decideBot(table.botView(), seeded(10)), decision);
  assert.equal(before.deck, undefined); assert.ok(before.opponents.every(p => !('cards' in p)));
});
test('decisions respect short all-ins and closed raising rights; delays remain within turn limit', () => {
  const random = seeded(12);
  const v = view({ cards: cards('As Ah'), options: { call: 20, min: 200, max: 80, canRaise: true } });
  for (let i = 0; i < 200; i++) {
    const action = decideBot(v, random);
    if (action.kind === 'raise') assert.equal(action.amount, 80);
    assert.notEqual(decideBot({ ...v, options: { ...v.options, canRaise: false } }, random).kind, 'raise');
  }
  const table = new Table(random, 9); table.start();
  const delays = new Set(Array.from({ length: 30 }, () => table.botDelay()));
  assert.ok(delays.size > 10); assert.ok([...delays].every(d => d >= 700 && d <= 4800));
});
test('public memory is bounded and persists between hands', () => {
  const t = new Table(seeded(42), 2);
  for (let n = 0; n < 10; n++) { assert.ok(t.start()); while (t.phase !== 'done') t.act('call'); }
  assert.ok(t.players.every(p => p.recent.length === 24));
  t.start(); assert.ok(t.players.every(p => p.recent.length === 24));
});
