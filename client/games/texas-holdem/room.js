'use strict';
(() => {
  const newToken = () => Array.from(crypto.getRandomValues(new Uint8Array(24)), value => value.toString(16).padStart(2, '0')).join('');
  const $ = id => document.getElementById(id);
  window.createPokerRoom = (onState, onConnection) => {
    let code = new URLSearchParams(location.search).get('room') || '';
    let state = null, connected = false, busy = false, polling = false;
    let token = sessionStorage.getItem(`poker-token.${code}`) || localStorage.getItem('poker-player-token') || newToken();
    if (!localStorage.getItem('poker-player-token')) localStorage.setItem('poker-player-token', token);
    let recordedSession = '', historyLoading = false;
    let pollTimer, generation = 0, displayedTarget;
    $('pokerCode').value = code;
    $('pokerName').value = localStorage.getItem('poker-name') || sessionStorage.getItem('poker-name') || '';
    function message(text) { $('roomMessage').textContent = text; }
    function connection(value) { if (connected === value) return; connected = value; onConnection(value); }
    async function api(operation, data = {}) {
      const controller = new AbortController();
      const timeout = setTimeout(() => controller.abort(), 7000);
      try {
        const response = await fetch(`/api/poker/${operation}${operation === 'state' ? `?code=${encodeURIComponent(code)}` : ''}`, {
          method: ['state', 'history'].includes(operation) ? 'GET' : 'POST', cache: 'no-store', signal: controller.signal,
          headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
          ...(['state', 'history'].includes(operation) ? {} : { body: JSON.stringify({ code, ...data }) })
        });
        const result = await response.json();
        if (!response.ok) { const error = new Error(result.error || '房间请求失败'); error.status = response.status; throw error; }
        return result;
      } finally { clearTimeout(timeout); }
    }
    function storageLabel(persistent) {
      $('pokerPersistence').textContent = persistent ? '已结算战绩长期保存 · 进行中的牌局重启会结束' : '试玩模式 · 战绩仅本次服务运行期间保留';
    }
    async function loadHistory() {
      if (historyLoading) return;
      historyLoading = true;
      const identity = token;
      $('historyStatus').textContent = '正在翻阅你的战绩…';
      try {
        const result = await api('history');
        if (identity !== token) return;
        storageLabel(result.persistent);
        const h = result.history;
        const signed = n => n > 0 ? `+${n.toLocaleString()}` : n.toLocaleString();
        $('historySummary').textContent = `${h.hands} 局 · 净赢 ${signed(h.net)} 豆 · 获利 ${h.wins} 局${h.lastChips == null ? '' : ` · 上局剩余 ${h.lastChips.toLocaleString()} 豆`}`;
        $('historyStatus').textContent = result.persistent ? '已保存的好友局战绩。身份保存在当前浏览器，清除网站数据或换设备会成为新玩家。' : '当前为内存试玩，服务器重启后战绩会清空。';
        $('historyList').replaceChildren(...h.recent.map(entry => {
          const item = document.createElement('li');
          const title = document.createElement('strong'); title.textContent = `第 ${entry.hand} 局 · ${signed(entry.net)} 豆`;
          const detail = document.createElement('span'); detail.textContent = `${new Date(entry.endedAt).toLocaleString()} · ${entry.startChips} → ${entry.endChips} 豆`;
          const resultText = document.createElement('span'); resultText.textContent = entry.result;
          item.append(title, detail, resultText); return item;
        }));
        if (!h.recent.length) { const item = document.createElement('li'); item.textContent = '还没有已结算的好友局，坐下来玩一局吧。'; $('historyList').append(item); }
      } catch (error) { $('historyStatus').textContent = '战绩暂时无法读取，稍后点击刷新重试。'; }
      finally { historyLoading = false; }
    }
    function connectedMessage() {
      message(state?.settlementPending ? '本局战绩待保存，正在重试。保存成功后可继续下一局。' : '已连接 · 牌局由服务端同步');
    }
    function accept(next) {
      if (state && next.code === state.code && next.version < state.version) return;
      const fresh = !state || state.sessionId !== next.sessionId;
      state = next; code = next.code; connection(true);
      sessionStorage.setItem(`poker-token.${code}`, token);
      const url = new URL(location.href); url.searchParams.set('room', code); url.searchParams.delete('intent'); history.replaceState(null, '', url);
      $('roomEntry').hidden = true; $('roomJoined').hidden = false;
      $('roomTitle').textContent = '朋友到齐，快乐开局。';
      $('roomCodeText').textContent = `邀请码 ${code}`;
      const host = next.hostId === next.playerId, playing = Boolean(next.table);
      $('pokerRoom').classList.toggle('is-playing', playing);
      storageLabel(next.persistent);
      const humans = next.members.length;
      const signature = next.members.map(m => `${m.id}:${m.displayName}:${m.connectionStatus}`).join('|');
      if ($('pokerMembers').dataset.signature !== signature) {
        $('pokerMembers').replaceChildren(...next.members.map(m => {
          const chip = document.createElement('span');
          chip.textContent = `${m.displayName}${m.id === next.hostId ? ' · 房主' : ''}${m.id === next.playerId ? ' · 你' : ''}${m.connectionStatus === 'offline' ? ' · 断线' : ''}`;
          return chip;
        }));
        $('pokerMembers').dataset.signature = signature;
      }
      if (displayedTarget !== next.target || playing) $('roomTarget').value = next.target;
      displayedTarget = next.target;
      $('roomTarget').disabled = !host || playing;
      $('savePokerTarget').disabled = !host || playing;
      $('startPoker').hidden = playing;
      $('startPoker').disabled = !host || busy;
      $('startPoker').textContent = host ? '开始游戏' : '等待房主开始';
      $('fillPreview').textContent = `${humans} 位真人 + ${Math.max(0, next.target - humans)} 位电脑 = ${next.target} 人桌`;
      $('pokerLobby').hidden = !host || !playing;
      $('pokerLobby').disabled = next.settlementPending || (playing && next.table.phase !== 'done');
      $('leavePoker').disabled = next.settlementPending || (playing && next.table.phase !== 'done');
      onState(next, fresh);
      if (next.table?.phase === 'done' && !next.settlementPending && recordedSession !== next.sessionId) { recordedSession = next.sessionId; loadHistory(); }
    }
    async function poll() {
      clearTimeout(pollTimer);
      if (!code || polling) return;
      const current = generation;
      polling = true;
      try { const next = await api('state'); if (current === generation) { accept(next); if (!busy) connectedMessage(); } }
      catch (error) { if (current === generation) {
        connection(false);
        if (error.status === 404 || error.status === 403) {
          code = ''; state = null; recordedSession = '';
          $('roomEntry').hidden = false; $('roomJoined').hidden = true;
          $('pokerRoom').classList.remove('is-playing');
          onState({ table: null });
          token = localStorage.getItem('poker-player-token');
          const url = new URL(location.href); url.searchParams.delete('room'); history.replaceState(null, '', url);
          message('原房间已结束或身份失效，可以重新开房；已保存的战绩仍可查看。'); loadHistory();
        } else message(`${error.message}。正在尝试恢复连接…`);
      } }
      finally { polling = false; if (code) pollTimer = setTimeout(poll, 700); }
    }
    async function send(operation, data = {}) {
      if (busy) return;
      busy = true;
      const current = ++generation;
      try {
        const result = await api(operation, { ...(['action', 'start', 'next'].includes(operation) ? { version: state?.version || 0, sessionId: state?.sessionId || '' } : {}), ...data });
        if (operation === 'leave') { code = ''; state = null; connection(false); location.href = '/games/texas-holdem/'; return; }
        if (current === generation) { accept(result); connectedMessage(); }
      } catch (error) { connection(false); message(error.message); }
      finally { busy = false; if (code) { clearTimeout(pollTimer); pollTimer = setTimeout(poll, 300); } }
    }
    async function join(create) {
      const name = $('pokerName').value.trim(); if (!name) { message('先填一个昵称吧'); $('pokerName').focus(); return; }
      code = create ? '' : $('pokerCode').value.trim();
      if (!create && !code) { message('请输入邀请码'); return; }
      token = (!create && sessionStorage.getItem(`poker-token.${code}`)) || localStorage.getItem('poker-player-token');
      sessionStorage.setItem('poker-name', name);
      localStorage.setItem('poker-name', name);
      await send(create ? 'create' : 'join', { name, ...(create ? { target: Number($('roomTarget').value) } : {}) });
    }
    $('createPoker').addEventListener('click', () => join(true));
    $('joinPoker').addEventListener('click', () => join(false));
    $('savePokerTarget').addEventListener('click', () => send('configure', { target: Number($('roomTarget').value) }));
    $('startPoker').addEventListener('click', () => send('start'));
    $('pokerLobby').addEventListener('click', () => send('lobby'));
    $('leavePoker').addEventListener('click', () => send('leave'));
    $('copyPokerInvite').addEventListener('click', async () => {
      const url = new URL('/games/texas-holdem/', location.origin); url.searchParams.set('room', code);
      try { await navigator.clipboard.writeText(url.href); message('邀请链接已复制，发给朋友就能加入。'); }
      catch { message(`邀请链接：${url.href}`); }
    });
    $('refreshHistory').addEventListener('click', loadHistory);
    $('pokerHistory').addEventListener('toggle', () => { if ($('pokerHistory').open) loadHistory(); });
    loadHistory();
    if (code && sessionStorage.getItem(`poker-token.${code}`)) poll();
    return { send, isConnected: () => connected, isHost: () => state?.hostId === state?.playerId, isBusy: () => busy };
  };
})();
