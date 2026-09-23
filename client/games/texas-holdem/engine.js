/* Local, play-money Texas Hold'em. No network or real-money balances. */
(function (root) {
  'use strict';
  const names = ['高牌', '一对', '两对', '三条', '顺子', '同花', '葫芦', '四条', '同花顺'];
  function compare(a, b) {
    for (let i = 0; i < Math.max(a.length, b.length); i++) {
      const d = (a[i] || 0) - (b[i] || 0);
      if (d) return d;
    }
    return 0;
  }
  function five(cards) {
    const ranks = cards.map(c => c.rank).sort((a, b) => b - a);
    const counts = new Map();
    ranks.forEach(r => counts.set(r, (counts.get(r) || 0) + 1));
    const groups = [...counts].sort((a, b) => b[1] - a[1] || b[0] - a[0]);
    const flush = cards.every(c => c.suit === cards[0].suit);
    const unique = [...counts.keys()];
    const straight = unique.length === 5 ? (unique[0] - unique[4] === 4 ? unique[0] : unique.join(',') === '14,5,4,3,2' ? 5 : 0) : 0;
    if (flush && straight) return [8, straight];
    if (groups[0][1] === 4) return [7, groups[0][0], groups[1][0]];
    if (groups[0][1] === 3 && groups[1][1] === 2) return [6, groups[0][0], groups[1][0]];
    if (flush) return [5, ...ranks];
    if (straight) return [4, straight];
    if (groups[0][1] === 3) return [3, ...groups.map(g => g[0])];
    if (groups[0][1] === 2 && groups[1][1] === 2) return [2, ...groups.map(g => g[0])];
    if (groups[0][1] === 2) return [1, ...groups.map(g => g[0])];
    return [0, ...ranks];
  }
  function evaluate(cards) {
    let best = null;
    for (let a = 0; a < cards.length - 4; a++)
      for (let b = a + 1; b < cards.length - 3; b++)
        for (let c = b + 1; c < cards.length - 2; c++)
          for (let d = c + 1; d < cards.length - 1; d++)
            for (let e = d + 1; e < cards.length; e++) {
              const score = five([cards[a], cards[b], cards[c], cards[d], cards[e]]);
              if (!best || compare(score, best) > 0) best = score;
            }
    return best;
  }
  const styles = [
    { label: '稳稳观察', looseness: -.07, aggression: .28, bluff: .04, sizing: .6, pace: 1.2 },
    { label: '爱看翻牌', looseness: .10, aggression: .32, bluff: .06, sizing: .45, pace: .9 },
    { label: '耐心猎手', looseness: -.09, aggression: .62, bluff: .05, sizing: .85, pace: 1.25 },
    { label: '慢热思考', looseness: -.04, aggression: .23, bluff: .03, sizing: .55, pace: 1.5 },
    { label: '均衡派', looseness: 0, aggression: .48, bluff: .10, sizing: .65, pace: 1 },
    { label: '大胆出击', looseness: .06, aggression: .78, bluff: .19, sizing: .9, pace: .8 },
    { label: '温柔跟注', looseness: .13, aggression: .18, bluff: .03, sizing: .45, pace: 1.1 },
    { label: '灵活试探', looseness: .02, aggression: .59, bluff: .16, sizing: .55, pace: 1.05 },
    { label: '随性玩家', looseness: .07, aggression: .51, bluff: .12, sizing: .75, pace: .85 }
  ];
  const clamp = (value, min, max) => Math.max(min, Math.min(max, value));
  function assessHand(cards, board, opponents) {
    const high = Math.max(...cards.map(c => c.rank)), low = Math.min(...cards.map(c => c.rank));
    if (!board.length) {
      const pair = high === low;
      const suited = cards[0].suit === cards[1].suit;
      const connected = high - low <= 2;
      const headsUp = pair ? .51 + high * .023 : .17 + high * .023 + low * .013 + (suited ? .045 : 0) + (connected ? .035 : 0);
      return { equity: Math.pow(clamp(headsUp, .18, .86), 1 + (opponents - 1) * .28), draw: 0, danger: 0 };
    }
    const all = [...cards, ...board], score = evaluate(all);
    const suits = [0, 1, 2, 3].map(s => all.filter(c => c.suit === s).length);
    const boardSuits = [0, 1, 2, 3].map(s => board.filter(c => c.suit === s).length);
    const ranks = new Set(all.map(c => c.rank));
    if (ranks.has(14)) ranks.add(1);
    let straightDraw = false;
    for (let start = 1; start <= 10; start++) {
      if (Array.from({ length: 5 }, (_, i) => start + i).filter(r => ranks.has(r)).length === 4) straightDraw = true;
    }
    const flushDraw = suits.some((count, suit) => count === 4 && cards.some(c => c.suit === suit));
    const draw = board.length < 5 ? (flushDraw ? .13 : 0) + (straightDraw && score[0] < 4 ? .08 : 0) : 0;
    const sortedBoard = [...new Set(board.map(c => c.rank))].sort((a, b) => a - b);
    const connectedBoard = sortedBoard.some((rank, i) => sortedBoard[i + 2] - rank <= 4);
    const danger = (Math.max(...boardSuits) >= 3 ? .10 : 0) + (connectedBoard ? .07 : 0);
    let equity = [.15, .46, .64, .76, .84, .89, .94, .98, .995][score[0]];
    if (score[0] === 0) equity += (high - 8) * .016;
    if (score[0] === 1) {
      const personalPair = cards.some(c => c.rank === score[1]);
      equity += personalPair ? (score[1] >= Math.max(...board.map(c => c.rank)) ? .17 : .02) : -.21;
    }
    // A strong public board is shared; it must not be valued like private strength.
    if (board.length === 5 && compare(score, evaluate(board)) === 0) equity = .45 / (opponents + 1);
    else equity = Math.pow(clamp(equity - (score[0] < 5 ? danger : 0), .05, .995), 1 + (opponents - 1) * .22);
    return { equity: clamp(equity + draw, .03, .995), draw, danger };
  }
  // Receives a deliberately limited view: no deck, opponent hole cards or future board.
  function decideBot(view, random = Math.random) {
    const style = styles[view.seat % styles.length];
    const hand = assessHand(view.cards, view.board, view.opponents.length);
    const o = view.options;
    const odds = o.call / Math.max(1, view.pot + o.call);
    const pressure = o.call / Math.max(1, view.chips);
    const aggressor = view.opponents.find(p => p.seat === view.lastAggressor);
    const recent = aggressor?.recent || [];
    const raises = recent.filter(a => a === 'raise').length;
    // Only adapt once enough observed actions exist; recent history naturally expires.
    const read = recent.length >= 6 ? clamp((raises / recent.length - .3) * .15, -.035, .06) : 0;
    const position = view.latePosition ? .035 : -.025;
    const edge = hand.equity + style.looseness + position + read - odds - pressure * .055;
    const foldChance = clamp(.42 - edge * 2.1, .015, .96);
    if (o.call && random() < foldChance) return { kind: 'fold' };
    const value = hand.equity > .56 && edge > .14;
    const steal = !o.call && view.latePosition ? .06 : 0;
    const observed = view.opponents.flatMap(p => p.recent || []);
    const foldRead = observed.length >= 12 ? clamp((observed.filter(a => a === 'fold').length / observed.length - .2) * .16, -.025, .06) : 0;
    const bluffChance = Math.max(0, style.bluff + steal + hand.draw * .5 + foldRead) / Math.max(1, view.opponents.length * .9);
    const raiseChance = value ? style.aggression * (.55 + hand.equity * .45) : bluffChance * (hand.danger ? .7 : 1);
    if (o.canRaise && random() < raiseChance) {
      const fraction = clamp(style.sizing + (random() - .5) * .5 + (value ? hand.danger : 0), .3, 1.2);
      let target = view.currentBet + Math.round((view.pot + o.call) * fraction / 10) * 10;
      if (!view.board.length) target = Math.max(target, 40 + Math.floor(random() * 3) * 20);
      // Commit a short stack with value, but avoid routinely shoving speculative hands.
      if (value && view.chips <= (view.pot + o.call) * 1.25 && random() < .65) target = o.max;
      return { kind: 'raise', amount: Math.min(o.max, Math.max(o.min, target)) };
    }
    return { kind: 'call' };
  }
  class Table {
    constructor(random = Math.random, playerCount = 4) {
      if (!Number.isInteger(playerCount) || playerCount < 2 || playerCount > 9) throw new RangeError('牌桌人数必须为 2–9 人');
      this.random = random;
      this.players = ['你', '桃桃', '阿栗', '慢慢', '团团', '小橘', '绵绵', '咕咕', '豆包'].slice(0, playerCount).map(name => ({ name, chips: 1000, cards: [], recent: [] }));
      this.dealer = -1;
      this.hand = 0;
      this.phase = 'idle';
      this.board = [];
      this.log = [];
      this.actor = -1;
      this.pot = 0;
      this.payouts = [];
    }
    nextSeat(from, predicate) {
      for (let n = 1; n <= this.players.length; n++) {
        const i = (from + n) % this.players.length;
        if (predicate(this.players[i], i)) return i;
      }
      return -1;
    }
    record(text) { this.log.unshift(text); this.log = this.log.slice(0, 35); }
    start() {
      if (!['idle', 'done'].includes(this.phase)) return false;
      if (this.players[0].chips === 0 || this.players.filter(p => p.chips > 0).length < 2) return false;
      this.hand++;
      this.deck = Array.from({ length: 52 }, (_, i) => ({ rank: i % 13 + 2, suit: Math.floor(i / 13) }));
      for (let i = 51; i > 0; i--) {
        const j = Math.floor(this.random() * (i + 1));
        [this.deck[i], this.deck[j]] = [this.deck[j], this.deck[i]];
      }
      this.board = []; this.pot = 0; this.payouts = []; this.revealed = false; this.log = [];
      this.players.forEach(p => Object.assign(p, { cards: [], bet: 0, total: 0, folded: p.chips === 0, lastActionBet: null, action: p.chips === 0 ? '休息中' : '等待行动' }));
      this.dealer = this.nextSeat(this.dealer, p => !p.folded);
      for (let round = 0; round < 2; round++) this.players.forEach(p => { if (!p.folded) p.cards.push(this.deck.pop()); });
      const headsUp = this.players.filter(p => !p.folded).length === 2;
      this.small = headsUp ? this.dealer : this.nextSeat(this.dealer, p => !p.folded);
      this.big = this.nextSeat(this.small, p => !p.folded);
      this.pay(this.small, 10); this.players[this.small].action = `小盲 ${this.players[this.small].bet}`;
      this.pay(this.big, 20); this.players[this.big].action = `大盲 ${this.players[this.big].bet}`;
      this.lastAggressor = null;
      this.currentBet = 20; this.lastRaise = 20; this.phase = 'preflop';
      this.pending = new Set(this.players.map((p, i) => !p.folded && p.chips > 0 ? i : -1).filter(i => i >= 0));
      this.record(`第 ${this.hand} 局开始 · 小盲 10 / 大盲 20`);
      this.advance(this.big);
      return true;
    }
    pay(i, amount) {
      const p = this.players[i];
      const paid = Math.min(p.chips, amount);
      p.chips -= paid; p.bet += paid; p.total += paid; this.pot += paid;
    }
    options() {
      if (this.actor < 0 || this.phase === 'done') return null;
      const p = this.players[this.actor];
      const call = Math.max(0, this.currentBet - p.bet);
      const max = p.bet + p.chips;
      const reopened = p.lastActionBet === null || this.currentBet - p.lastActionBet >= this.lastRaise;
      const opponent = this.players.some((q, i) => i !== this.actor && !q.folded && q.chips > 0);
      return { call: Math.min(call, p.chips), min: this.currentBet + this.lastRaise, max, canRaise: reopened && opponent && max > this.currentBet };
    }
    act(kind, amount) {
      const o = this.options();
      if (!o) return false;
      const i = this.actor, p = this.players[i];
      const chipsBefore = p.chips;
      if (kind === 'raise') {
        if (!o.canRaise || !Number.isInteger(amount) || amount > o.max || amount <= this.currentBet || (amount < o.min && amount !== o.max)) return false;
        const increase = amount - this.currentBet;
        this.pay(i, amount - p.bet);
        if (increase >= this.lastRaise) this.lastRaise = increase;
        this.currentBet = amount;
        this.players.forEach((q, j) => { if (j !== i && !q.folded && q.chips && q.bet < amount) this.pending.add(j); });
        p.action = p.chips ? `加注至 ${amount}` : `全下 ${amount}`;
      } else if (kind === 'call') {
        this.pay(i, o.call);
        p.action = o.call ? (p.chips ? `跟注 ${o.call}` : `全下 ${o.call}`) : '过牌';
      } else if (kind === 'fold') {
        p.folded = true; p.action = '弃牌';
      } else return false;
      p.recent.push(kind === 'call' && chipsBefore === p.chips ? 'check' : kind);
      if (p.recent.length > 24) p.recent.shift();
      if (kind === 'raise') this.lastAggressor = i;
      this.lastAction = { seat: i, kind, text: p.action, paid: chipsBefore - p.chips, chips: p.chips, pot: this.pot };
      p.lastActionBet = this.currentBet;
      this.record(`${p.name} · ${p.action}`);
      this.pending.delete(i);
      this.advance(i);
      return true;
    }
    advance(from) {
      const active = this.players.filter(p => !p.folded);
      if (active.length === 1) { this.finish(false); return; }
      const canAct = this.players.map((p, i) => !p.folded && p.chips > 0 ? i : -1).filter(i => i >= 0);
      if (canAct.length < 2 && (canAct.length === 0 || this.players[canAct[0]].bet >= this.currentBet)) this.pending.clear();
      for (const i of this.pending) if (this.players[i].folded || !this.players[i].chips) this.pending.delete(i);
      if (this.pending.size) {
        this.actor = this.nextSeat(from, (_, i) => this.pending.has(i)); return;
      }
      if (this.phase === 'river') { this.finish(true); return; }
      this.deck.pop(); // Burn a card before each community-card street.
      const count = this.phase === 'preflop' ? 3 : 1;
      for (let n = 0; n < count; n++) this.board.push(this.deck.pop());
      this.phase = { preflop: 'flop', flop: 'turn', turn: 'river' }[this.phase];
      this.lastAggressor = null;
      this.currentBet = 0; this.lastRaise = 20;
      this.players.forEach(p => { p.bet = 0; p.lastActionBet = null; if (!p.folded && p.chips) p.action = '等待行动'; });
      this.record({ flop: '翻牌 · 三张公共牌登场', turn: '转牌 · 再看一张', river: '河牌 · 最后一张牌' }[this.phase]);
      this.pending = new Set(canAct);
      this.advance(this.dealer);
    }
    finish(showdown) {
      this.revealed = showdown;
      this.payouts = Array(this.players.length).fill(0);
      const active = this.players.map((p, i) => !p.folded ? i : -1).filter(i => i >= 0);
      const scores = this.players.map(p => !p.folded && showdown ? evaluate([...p.cards, ...this.board]) : null);
      const levels = [...new Set(this.players.map(p => p.total).filter(Boolean))].sort((a, b) => a - b);
      let previous = 0;
      for (const level of levels) {
        const contributors = this.players.map((p, i) => p.total >= level ? i : -1).filter(i => i >= 0);
        const amount = (level - previous) * contributors.length;
        previous = level;
        const eligible = active.filter(i => this.players[i].total >= level);
        // An unmatched excess is returned, even if its contributor subsequently folded.
        if (!eligible.length) { contributors.forEach(i => { this.payouts[i] += amount / contributors.length; }); continue; }
        let winners = [eligible[0]];
        for (const i of eligible.slice(1)) {
          const diff = showdown ? compare(scores[i], scores[winners[0]]) : 0;
          if (diff > 0) winners = [i]; else if (diff === 0) winners.push(i);
        }
        winners.sort((a, b) => ((a - this.dealer - 1 + this.players.length) % this.players.length) - ((b - this.dealer - 1 + this.players.length) % this.players.length));
        winners.forEach((i, n) => { this.payouts[i] += Math.floor(amount / winners.length) + (n < amount % winners.length ? 1 : 0); });
      }
      this.lastPot = this.pot;
      this.players.forEach((p, i) => { p.chips += this.payouts[i]; if (!p.folded) p.action = showdown ? names[scores[i][0]] : '大家都弃牌啦'; });
      this.result = this.players.map((p, i) => this.payouts[i] ? `${p.name}收获 ${this.payouts[i]}` : '').filter(Boolean).join(' · ');
      this.record(this.result);
      this.pot = 0; this.actor = -1; this.phase = 'done';
    }
    botView() {
      if (this.actor <= 0 || this.phase === 'done') return null;
      const p = this.players[this.actor];
      const opponents = this.players.flatMap((q, seat) => seat !== this.actor && !q.folded ? [{ seat, chips: q.chips, recent: [...q.recent] }] : []);
      const order = [];
      for (let n = 1; n <= this.players.length; n++) {
        const seat = (this.dealer + n) % this.players.length;
        if (!this.players[seat].folded) order.push(seat);
      }
      return { seat: this.actor, cards: p.cards.map(c => ({ ...c })), board: this.board.map(c => ({ ...c })), chips: p.chips,
        pot: this.pot, currentBet: this.currentBet, options: this.options(), opponents,
        lastAggressor: this.lastAggressor, latePosition: order.indexOf(this.actor) >= order.length - 2 };
    }
    botDelay() {
      const view = this.botView();
      if (!view) return 0;
      const pressure = view.options.call / Math.max(1, view.chips);
      return Math.round(clamp((800 + this.random() * 1400 + Math.min(1, pressure) * 1200) * styles[view.seat].pace, 700, 4800));
    }
    bot() {
      const view = this.botView();
      if (!view) return false;
      const action = decideBot(view, this.random);
      return this.act(action.kind, action.amount);
    }
  }
  const api = { Table, evaluate, compare, names, styles, assessHand, decideBot };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else root.CozyPoker = api;
})(typeof window !== 'undefined' ? window : globalThis);
