/* global CozyPoker, PokerTurnClock, createPokerRoom */
'use strict';
(() => {
  const multiplayer = new URLSearchParams(location.search).get('mode') !== 'solo';
  let roomClient, remoteState, pendingRemote;
  let appliedVersion = -1, lastRemoteAction = 0;
  let table = new CozyPoker.Table(Math.random, 9);
  let timer;
  let transitionTimer;
  let transitioning = false;
  const turnClock = new PokerTurnClock();
  const TURN_SECONDS = 20;
  const bubbleTimers = new Map();
  const $ = id => document.getElementById(id);
  const escapeHTML = value => String(value).replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const suits = ['♠', '♥', '♣', '♦'];
  const suitNames = ['黑桃', '红桃', '梅花', '方块'];
  const animals = ['you', 'peach', 'chestnut', 'turtle', 'panda', 'orange', 'sheep', 'owl', 'bean'];
  const tips = ['用你的 2 张手牌和 5 张公共牌，组合出最好的 5 张牌。', '不想冒险就弃牌。保住豆豆，下一手又是新的机会。', '顺子里的 A 可以最大，也可以最小：A、2、3、4、5 也算哦。', '所有人都过牌，就能免费看到下一张公共牌。', '“加注至”是本轮累计下注，不是额外增加的数量。', '牌型相同时会比较踢脚牌；完全相同就平分底池。', '桃桃爱看翻牌，小橘大胆出击，绵绵偏爱跟注；每位牌友都有自己的风格。', '牌友会参考最近的公开操作；一直加注或经常弃牌，都会影响它们的判断。'];
  let tip = 0;
  function avatar(i) {
    const colors = ['#f7e7a9', '#fff5e9', '#c79b71', '#96b782', '#f5f2e6', '#e9b279', '#f3e8d6', '#bcabab', '#bdd4a3'];
    const ears = i === 1 ? '<ellipse cx="17" cy="13" rx="6" ry="13" fill="#fff5e9"/><ellipse cx="33" cy="13" rx="6" ry="13" fill="#fff5e9"/><path d="M17 4v14M33 4v14" stroke="#e5bab0" stroke-width="3" stroke-linecap="round"/>' : i === 2 ? '<circle cx="12" cy="14" r="8" fill="#a97c59"/><circle cx="38" cy="14" r="8" fill="#a97c59"/>' : i === 0 ? '<path d="M8 24V7l13 9M29 16 42 7v18" fill="#f7e7a9" stroke="#c7a158" stroke-width="1.5"/>' : '<ellipse cx="25" cy="38" rx="22" ry="10" fill="#799c64"/>';
    return `<svg viewBox="0 0 50 50" aria-hidden="true">${ears}<ellipse cx="25" cy="29" rx="19" ry="17" fill="${colors[i]}"/><ellipse cx="14" cy="33" rx="4" ry="2.5" fill="#e6a38d" opacity=".65"/><ellipse cx="36" cy="33" rx="4" ry="2.5" fill="#e6a38d" opacity=".65"/><circle cx="19" cy="27" r="1.6" fill="#494e38"/><circle cx="31" cy="27" r="1.6" fill="#494e38"/><path d="M22 33q3 4 6 0" stroke="#65533f" stroke-width="1.5" fill="none" stroke-linecap="round"/>${i === 0 ? '<path d="m21 11 4-7 4 7" stroke="#b3964a" fill="#eac969"/>' : ''}</svg>`;
  }
  function card(c, hidden = false) {
    if (hidden) return '<div class="card back" aria-label="未公开的手牌"></div>';
    if (!c) return '<div class="card empty" aria-label="尚未发出的公共牌">✧</div>';
    const rank = ({ 11: 'J', 12: 'Q', 13: 'K', 14: 'A' })[c.rank] || c.rank;
    return `<div class="card ${c.suit % 2 ? 'red' : ''}" data-rank="${rank}" aria-label="${suitNames[c.suit]}${rank}"><span class="card-rank">${rank}</span><span class="card-suit">${suits[c.suit]}</span></div>`;
  }
  // Keep seats, avatars and unchanged cards mounted across actions.
  function initializeSeats() {
    const count = table.players.length;
    const slots = {
      2: ['top'], 3: ['left', 'right'], 4: ['left', 'top', 'right'],
      5: ['left-middle', 'top-left', 'top-right', 'right-middle'],
      6: ['left-bottom', 'left-top', 'top-center', 'right-top', 'right-bottom'],
      7: ['left-bottom', 'left-middle', 'top-left', 'top-right', 'right-middle', 'right-bottom'],
      8: ['left-bottom', 'left-middle', 'left-top', 'top-center', 'right-top', 'right-middle', 'right-bottom'],
      9: ['left-bottom', 'left-middle', 'left-top', 'top-left', 'top-right', 'right-top', 'right-middle', 'right-bottom']
    }[count];
    document.querySelector('.scene').classList.toggle('dense-table', count >= 5);
    $('tableCapacity').textContent = `${count} 人桌`;
    $('playerCount').value = count;
    $('seats').replaceChildren();
    table.players.forEach((p, i) => {
      const seat = document.createElement('div');
      seat.id = `seat${i}`;
      seat.className = `seat seat-${i === 0 ? 'you' : slots[i - 1]}`;
      seat.setAttribute('aria-label', `${i + 1} 号座位 · ${p.name}`);
      if (i > 0) seat.title = `${p.name} · ${multiplayer && !p.bot ? '真人玩家' : CozyPoker.styles[p.sourceSeat ?? i].label}`;
      $('seats').appendChild(seat);
      seat.innerHTML = `<div class="seat-details"><div class="avatar ${animals[i]}">${avatar(i)}<span class="dealer-button" title="庄家" hidden>D</span><span class="seat-clock" hidden></span></div><div class="seat-name">${escapeHTML(p.name)}${i === 0 ? ' · 你' : multiplayer ? (p.bot ? ' · 电脑' : ' · 真人') : ''}</div><span class="seat-stack"></span></div><div class="hole-cards"></div><div class="seat-action"></div><div class="action-bubble" hidden></div>`;
    });
  }
  function syncCards(element, cards, hidden = false) {
    cards.forEach((c, index) => {
      const key = c ? (hidden ? 'back' : `${c.rank}:${c.suit}`) : 'empty';
      const old = element.children[index];
      if (old?.dataset.cardKey === key) return;
      const template = document.createElement('template');
      template.innerHTML = card(c, hidden);
      const next = template.content.firstElementChild;
      next.dataset.cardKey = key;
      if (c && !hidden) next.style.animationDelay = `${index * 70}ms`;
      if (old) old.replaceWith(next); else element.appendChild(next);
    });
    while (element.children.length > cards.length) element.lastElementChild.remove();
  }
  function resetHandEffects() {
    bubbleTimers.forEach(clearTimeout);
    bubbleTimers.clear();
    document.querySelectorAll('.action-bubble').forEach(b => { b.hidden = true; });
    document.querySelectorAll('.seat').forEach(seat => {
      delete seat.dataset.motion;
      seat.classList.remove('is-acting');
      seat.querySelector('.hole-cards').replaceChildren();
    });
    $('board').replaceChildren();
  }
  function render() {
    const done = table.phase === 'done';
    const myTurn = table.actor === 0 && !done && (!multiplayer || (roomClient?.isConnected() && !transitioning && turnClock.remaining() > 0));
    table.players.forEach((p, i) => {
      const seat = $(`seat${i}`);
      seat.classList.toggle('is-turn', table.actor === i);
      seat.classList.toggle('is-folded', p.folded);
      seat.querySelector('.dealer-button').hidden = table.dealer !== i;
      seat.querySelector('.seat-stack').textContent = `● ${p.chips.toLocaleString()}`;
      seat.querySelector('.seat-action').textContent = table.actor === i ? (i === 0 ? '到你啦 ✦' : '想一想…') : p.action || '已入座';
      syncCards(seat.querySelector('.hole-cards'), p.cards, i !== 0 && !(table.revealed && !p.folded));
    });
    syncCards($('board'), Array.from({ length: 5 }, (_, i) => table.board[i]));
    $('potValue').textContent = (done ? table.lastPot : table.pot).toLocaleString();
    $('handLabel').textContent = `第 ${String(table.hand).padStart(2, '0')} 局 · 盲注 10 / 20`;
    $('streetLabel').textContent = { preflop: '翻牌前 · 好戏才刚开始', flop: '翻牌 · 和好运碰个面', turn: '转牌 · 故事有了新进展', river: '河牌 · 最后一点小心跳', done: '本局结束 · 好牌值得等待' }[table.phase];
    const score = table.board.length >= 3 ? CozyPoker.evaluate([...table.players[0].cards, ...table.board]) : null;
    $('handStrength').textContent = score ? CozyPoker.names[score[0]] : table.players[0].cards.length === 2 && table.players[0].cards[0]?.rank === table.players[0].cards[1]?.rank ? '起手一对' : '两张小期待';
    $('turnTitle').textContent = done ? (table.payouts[0] > 0 ? '这把有收获，漂亮！' : '快乐不散场，下把再来！') : myTurn ? '轮到你啦，20 秒内做决定。' : table.players[0].folded ? '歇一歇，看看牌友的表演。' : `${table.players[table.actor]?.name || '牌友'}正在想一想…`;
    const o = table.options();
    $('turnHint').textContent = done ? table.result : myTurn ? (o.call ? `补上 ${o.call} 颗豆豆就能跟注，也可以加注或弃牌。` : '这一轮可以免费过牌，也可以主动下注。') : '每次操作限时 20 秒，超时自动过牌或弃牌。';
    $('turnIcon').textContent = done ? '✿' : '☀';
    $('actions').hidden = done;
    $('betControls').hidden = done;
    $('nextButton').hidden = !done;
    $('nextButton').disabled = multiplayer && !roomClient?.isHost();
    $('nextButton').textContent = table.players[0].chips === 0 || table.players.filter(p => p.chips > 0).length < 2 ? '带上新豆豆，重新开桌 →' : '再来一局 →';
    if (multiplayer) { $('nextButton').textContent = roomClient?.isHost() ? (table.players.filter(p => p.chips > 0).length < 2 ? '本桌结束，请回到等待房' : '下一局 →') : '等待房主开始下一局'; if (table.players.filter(p => p.chips > 0).length < 2) $('nextButton').disabled = true; }
    $('foldButton').disabled = !myTurn;
    $('callButton').disabled = !myTurn;
    $('raiseButton').disabled = !myTurn || !o?.canRaise;
    $('raiseRange').disabled = !myTurn || !o?.canRaise;
    document.querySelectorAll('[data-bet]').forEach(b => { b.disabled = !myTurn || !o?.canRaise; });
    $('callButton').innerHTML = myTurn && o.call ? `跟注 ${o.call}<span>${o.call === table.players[0].chips ? '跟注全下' : '再看一眼'}</span>` : '过牌<span>免费再看看</span>';
    if (myTurn && o.canRaise) {
      $('raiseRange').min = Math.min(o.min, o.max);
      $('raiseRange').max = o.max;
      $('raiseRange').value = Math.min(o.min, o.max);
    }
    updateRaise();
    const recent = table.log.slice(0, 8);
    const log = $('gameLog');
    if (log.dataset.latest !== recent.join('|')) {
      log.replaceChildren(...recent.map(text => { const li = document.createElement('li'); li.textContent = text; return li; }));
      log.dataset.latest = recent.join('|');
    }
    $('announcement').textContent = `${$('turnTitle').textContent} ${$('turnHint').textContent}`;
  }
  function updateRaise() {
    $('raiseValue').value = $('raiseRange').value;
    $('raiseButton').innerHTML = `加注至 ${$('raiseRange').value}<span>加一点勇气 ↗</span>`;
  }
  function lockControls() {
    document.querySelectorAll('#actions button, #betControls button, #raiseRange').forEach(control => { control.disabled = true; });
  }
  function updateClock() {
    const running = turnClock.deadline !== null && !transitioning;
    $('turnClock').hidden = false;
    document.querySelectorAll('.seat-clock').forEach(clock => {
      const seat = clock.closest('.seat');
      const active = running && seat.id === `seat${table.actor}`;
      clock.hidden = !active;
      if (active) clock.textContent = `${Math.min(TURN_SECONDS, Math.ceil(turnClock.remaining() / 1000))}s`;
    });
    if (!running) {
      $('clockLabel').textContent = transitioning ? '正在展示本次行动' : '本局已结束';
      $('clockSeconds').textContent = '—';
      $('clockFill').style.transform = 'scaleX(0)';
      $('turnClock').classList.remove('is-urgent');
      return;
    }
    const remaining = Math.min(TURN_SECONDS * 1000, turnClock.remaining());
    const seconds = Math.ceil(remaining / 1000);
    $('clockSeconds').textContent = `${seconds}s`;
    $('clockLabel').textContent = table.actor === 0 ? '你的操作时间 · 超时自动过牌 / 弃牌' : `${table.players[table.actor].name}正在考虑`;
    $('clockFill').style.transform = `scaleX(${remaining / (TURN_SECONDS * 1000)})`;
    $('turnClock').classList.toggle('is-urgent', seconds <= 5);
    $(`seat${table.actor}`).classList.toggle('is-urgent', seconds <= 5);
    if (multiplayer) { if (seconds === 0) lockControls(); return; }
    if (turnClock.consumeExpiry()) performAction(() => table.act(table.options().call ? 'fold' : 'call'), true);
  }
  function animateAction(action, timedOut) {
    bubbleTimers.forEach(clearTimeout);
    bubbleTimers.clear();
    document.querySelectorAll('.action-bubble').forEach(bubble => { bubble.hidden = true; });
    const seat = $(`seat${action.seat}`);
    const bubble = seat.querySelector('.action-bubble');
    clearTimeout(bubbleTimers.get(action.seat));
    const motion = action.kind === 'fold' ? 'fold' : action.paid ? (action.chips === 0 ? 'all-in' : 'bet') : 'check';
    seat.dataset.motion = motion;
    seat.classList.add('is-acting');
    bubble.hidden = false;
    bubble.dataset.kind = motion;
    bubble.textContent = `${timedOut ? '时间到 · ' : ''}${action.text}${motion === 'check' ? ' 👋' : motion === 'all-in' ? '！' : ''}`;
    if (!matchMedia('(prefers-reduced-motion: reduce)').matches) bubble.animate([
      { opacity: 0, transform: 'translateY(8px) scale(.85)' },
      { opacity: 1, transform: 'translateY(-3px) scale(1.05)', offset: .7 },
      { opacity: 1, transform: 'translateY(0) scale(1)' }
    ], { duration: 320, easing: 'ease-out' });
    seat.querySelector('.seat-action').textContent = timedOut ? '超时自动操作' : '刚刚行动';
    seat.querySelector('.seat-stack').textContent = `● ${action.chips.toLocaleString()}`;
    $('potValue').textContent = action.pot.toLocaleString();
    $('turnTitle').textContent = `${table.players[action.seat].name} · ${action.text}`;
    $('turnHint').textContent = timedOut ? '操作时间已到，已替你执行安全操作。' : '看清这一手，再轮到下一位。';
    $('announcement').textContent = `${timedOut ? '超时，' : ''}${$('turnTitle').textContent}`;
    if (action.paid) flyChips(seat, action.paid);
    bubbleTimers.set(action.seat, setTimeout(() => { bubble.hidden = true; bubbleTimers.delete(action.seat); }, 3500));
  }
  function flyChips(seat, amount) {
    if (matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    const scene = document.querySelector('.scene');
    const origin = seat.querySelector('.avatar').getBoundingClientRect();
    const target = $('potValue').getBoundingClientRect();
    const bounds = scene.getBoundingClientRect();
    const chip = document.createElement('span');
    chip.className = 'flying-chips';
    chip.setAttribute('aria-hidden', 'true');
    chip.textContent = `● ${amount}`;
    const x = origin.left + origin.width / 2, y = origin.top + origin.height / 2;
    chip.style.left = `${x - bounds.left}px`;
    chip.style.top = `${y - bounds.top}px`;
    scene.appendChild(chip);
    const dx = target.left + target.width / 2 - x, dy = target.top + target.height / 2 - y;
    const animation = chip.animate([
      { opacity: 0, transform: 'translate(-50%, -50%) scale(.6)' },
      { opacity: 1, transform: `translate(calc(-50% + ${dx * .2}px), calc(-50% + ${dy * .2 - 20}px)) scale(1.1)`, offset: .3 },
      { opacity: 1, transform: `translate(calc(-50% + ${dx}px), calc(-50% + ${dy}px)) scale(.75)`, offset: .85 },
      { opacity: 0, transform: `translate(calc(-50% + ${dx}px), calc(-50% + ${dy}px)) scale(.5)` }
    ], { duration: 950, easing: 'ease-in-out' });
    animation.onfinish = () => chip.remove();
    animation.oncancel = () => chip.remove();
  }
  function schedule() {
    clearTimeout(timer);
    document.querySelectorAll('.seat').forEach(seat => { seat.classList.remove('is-urgent', 'is-acting'); delete seat.dataset.motion; });
    render();
    if (table.phase === 'done') { turnClock.stop(); updateClock(); return; }
    turnClock.start(TURN_SECONDS);
    updateClock();
    if (table.actor > 0) timer = setTimeout(() => {
      if (turnClock.expired()) { updateClock(); return; }
      performAction(() => table.bot());
    }, table.botDelay());
  }
  function performAction(action, timedOut = false) {
    if (transitioning || table.phase === 'done') return;
    if (!action()) return;
    clearTimeout(timer);
    turnClock.stop();
    transitioning = true;
    lockControls();
    updateClock();
    if (timedOut) table.record(`${table.players[table.lastAction.seat].name} · 超时自动${table.lastAction.text}`);
    animateAction(table.lastAction, timedOut);
    // Let the actor finish their gesture before moving the highlight or dealing.
    transitionTimer = setTimeout(() => {
      transitioning = false;
      schedule();
    }, 1200);
  }
  function act(kind, amount) {
    if (multiplayer) { if (table.actor !== 0 || transitioning || !roomClient?.isConnected() || turnClock.remaining() <= 0) return; lockControls(); roomClient.send('action', { kind, ...(amount === undefined ? {} : { amount }) }); return; }
    if (table.actor !== 0 || transitioning) return;
    if (turnClock.expired()) { updateClock(); return; }
    performAction(() => table.act(kind, amount));
  }
  $('foldButton').addEventListener('click', () => act('fold'));
  $('callButton').addEventListener('click', () => act('call'));
  $('raiseButton').addEventListener('click', () => act('raise', Number($('raiseRange').value)));
  $('raiseRange').addEventListener('input', updateRaise);
  document.querySelectorAll('[data-bet]').forEach(button => button.addEventListener('click', () => {
    const o = table.options();
    if (table.actor !== 0 || !o?.canRaise) return;
    const amount = { min: o.min, half: table.currentBet + Math.floor(table.pot / 2), pot: table.currentBet + table.pot, all: o.max }[button.dataset.bet];
    $('raiseRange').value = Math.min(o.max, Math.max(o.min, amount));
    updateRaise();
  }));
  $('nextButton').addEventListener('click', () => {
    if (multiplayer) { roomClient.send('next'); return; }
    if (table.phase !== 'done' || transitioning) return;
    if (table.players[0].chips === 0 || table.players.filter(p => p.chips > 0).length < 2) table = new CozyPoker.Table(Math.random, table.players.length);
    resetHandEffects(); table.start(); schedule();
  });
  $('newTableButton').addEventListener('click', () => {
    const count = Number($('playerCount').value);
    clearTimeout(timer);
    clearTimeout(transitionTimer);
    turnClock.stop();
    transitioning = false;
    resetHandEffects();
    document.querySelectorAll('.flying-chips').forEach(chip => chip.remove());
    table = new CozyPoker.Table(Math.random, count);
    initializeSeats();
    table.start();
    schedule();
  });
  $('rulesButton').addEventListener('click', () => $('rulesDialog').showModal());
  $('gotIt').addEventListener('click', () => $('rulesDialog').close());
  $('rulesDialog').addEventListener('click', event => { if (event.target === $('rulesDialog')) { const rect = event.target.getBoundingClientRect(); if (event.clientX < rect.left || event.clientX > rect.right || event.clientY < rect.top || event.clientY > rect.bottom) event.target.close(); } });
  $('moreTips').addEventListener('click', () => { tip = (tip + 1) % tips.length; $('tipText').textContent = tips[tip]; });
  setInterval(updateClock, 100);
  document.addEventListener('visibilitychange', updateClock);
  function rotatedTable(state) {
    const source = state.table, count = source.players.length, own = state.selfSeat;
    const rotate = seat => seat < 0 ? -1 : (seat - own + count) % count;
    const result = { ...source,
      players: [...source.players.slice(own), ...source.players.slice(0, own)].map((p, i) => ({ ...p, sourceSeat: (i + own) % count })),
      dealer: rotate(source.dealer), actor: rotate(source.actor),
      payouts: source.payouts ? [...source.payouts.slice(own), ...source.payouts.slice(0, own)] : [],
      log: source.log || [],
      lastAction: source.lastAction ? { ...source.lastAction, seat: rotate(source.lastAction.seat) } : null,
      options: () => source.options
    };
    return result;
  }
  function applyRemote(state) {
    remoteState = state;
    appliedVersion = state.version;
    const nextTable = rotatedTable(state);
    const rebuild = table.players.map(p => p.id).join('|') !== nextTable.players.map(p => p.id).join('|');
    const nextHand = table.hand !== nextTable.hand || rebuild;
    table = nextTable;
    if (rebuild) initializeSeats();
    if (nextHand) resetHandEffects();
    document.querySelectorAll('.seat').forEach(seat => { seat.classList.remove('is-acting', 'is-urgent'); delete seat.dataset.motion; });
    turnClock.deadline = state.deadline ? Date.now() + Math.max(0, state.deadline - state.serverTime) : null;
    render(); updateClock();
  }
  function receiveRemote(state, fresh) {
    document.querySelector('.game-layout').hidden = !state.table;
    if (!state.table) {
      clearTimeout(transitionTimer); transitioning = false; turnClock.stop(); remoteState = state; appliedVersion = -1; lastRemoteAction = 0; pendingRemote = null; return;
    }
    if (state.selfSeat < 0) return;
    if (transitioning && !fresh) { pendingRemote = state; return; }
    if (!fresh && state.version === appliedVersion) return;
    const action = state.table.lastAction;
    if (!fresh && action && action.sequence > lastRemoteAction && remoteState?.table) {
      lastRemoteAction = action.sequence;
      const rotated = rotatedTable(state);
      transitioning = true; lockControls(); turnClock.stop(); updateClock();
      animateAction(rotated.lastAction, action.timedOut);
      pendingRemote = state;
      transitionTimer = setTimeout(() => {
        transitioning = false;
        const latest = pendingRemote; pendingRemote = null;
        if (latest) applyRemote(latest);
      }, Math.max(150, Math.min(1200, state.moveAfter - state.serverTime)));
    } else {
      clearTimeout(transitionTimer); transitioning = false;
      lastRemoteAction = action?.sequence || 0;
      applyRemote(state);
    }
  }
  if (multiplayer) {
    $('pokerRoom').hidden = false;
    document.querySelector('.table-settings').hidden = true;
    document.querySelector('.game-layout').hidden = true;
    roomClient = createPokerRoom(receiveRemote, connected => {
      if (!connected) { lockControls(); $('turnTitle').textContent = '连接中断，正在恢复…'; }
      else if (remoteState?.table && !transitioning) render();
    });
  } else {
    $('pokerModeLabel').textContent = '单人休闲 · 电脑牌友';
    $('pokerPersistence').textContent = '本地练习局 · 刷新页面会重新开始';
    initializeSeats(); table.start(); schedule();
  }
})();
