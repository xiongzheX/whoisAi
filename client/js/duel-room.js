(function () {
  'use strict';

  function newToken() {
    const random = window.crypto?.randomUUID
      ? window.crypto.randomUUID().replaceAll('-', '')
      : `${Date.now().toString(36)}${Math.random().toString(36).slice(2)}`;
    return `duel_${random}`.slice(0, 80);
  }

  function tokenFor(gameId, roomId) {
    const key = `party-tab.${gameId}.${roomId}.token`;
    const saved = sessionStorage.getItem(key);
    if (saved && /^[A-Za-z0-9_-]{16,80}$/.test(saved)) return saved;
    const whoIsAIToken = sessionStorage.getItem(`whoisai.playerToken.${roomId}`);
    if (whoIsAIToken && /^[A-Za-z0-9_-]{16,80}$/.test(whoIsAIToken)) {
      sessionStorage.setItem(key, whoIsAIToken);
      return whoIsAIToken;
    }
    const token = newToken();
    sessionStorage.setItem(key, token);
    return token;
  }

  function suggestedPlayerName() {
    const saved = sessionStorage.getItem('party-tab.player-name');
    if (saved) return saved;
    const colors = ['薄荷', '杏桃', '柠檬', '蓝莓', '奶油', '海盐'];
    const friends = ['豆', '兔', '团', '狐', '鹅', '熊'];
    const name = `${colors[Math.floor(Math.random() * colors.length)]}${friends[Math.floor(Math.random() * friends.length)]}`;
    sessionStorage.setItem('party-tab.player-name', name);
    return name;
  }

  function createDuelRoomClient(options) {
    const socket = io();
    let roomId = '';
    let playerId = '';
    let joined = false;
    let pendingToken = '';

    socket.on('connect', () => options.onConnection?.(true));
    socket.on('disconnect', () => {
      joined = false;
      options.onConnection?.(false);
    });
    socket.on('roomJoined', (payload = {}) => {
      if (payload.gameId !== options.gameId) return;
      roomId = payload.roomId || roomId;
      playerId = payload.playerId || '';
      joined = true;
      if (pendingToken && roomId) {
        sessionStorage.setItem(`party-tab.${options.gameId}.${roomId}.token`, pendingToken);
      }
      options.onJoined?.({ roomId, playerId });
    });
    socket.on('duelState', (state) => {
      if (!state || state.gameId !== options.gameId || state.roomCode !== roomId) return;
      options.onState?.(state, playerId);
    });
    socket.on('error', (message) => options.onError?.(String(message || '操作失败')));
    socket.on('roomLeft', () => {
      const previousRoomId = roomId;
      joined = false;
      roomId = '';
      playerId = '';
      pendingToken = '';
      options.onLeft?.({ roomId: previousRoomId });
    });

    return {
      join(nextRoomId, name) {
        roomId = String(nextRoomId || '').trim();
        joined = false;
        pendingToken = tokenFor(options.gameId, roomId);
        socket.emit('joinDuelRoom', {
          roomId,
          name: String(name || '').trim(),
          gameId: options.gameId,
          playerToken: pendingToken,
          createNew: false
        });
      },
      create(name) {
        roomId = '';
        joined = false;
        pendingToken = newToken();
        socket.emit('joinDuelRoom', {
          roomId: '',
          name: String(name || '').trim(),
          gameId: options.gameId,
          playerToken: pendingToken,
          createNew: true
        });
      },
      ready(config) {
        if (!joined) throw new Error('请先加入房间');
        socket.emit('duelReady', { roomId, gameId: options.gameId, config });
      },
      cancelReady() {
        if (!joined) throw new Error('请先加入房间');
        socket.emit('duelCancelReady', { roomId, gameId: options.gameId });
      },
      leave() {
        if (!joined) return;
        socket.emit('leaveRoom');
      },
      isJoined() {
        return joined;
      },
      playerId() {
        return playerId;
      }
    };
  }

  function duelReadinessPrompt(state, playerId) {
    const players = Array.isArray(state?.players) ? state.players : [];
    const onlinePlayers = players.filter(player => player.connectionStatus === 'online');
    const me = players.find(player => player.id === playerId);
    const opponent = onlinePlayers.find(player => player.id !== playerId);
    const bothPresent = onlinePlayers.length === 2 && Boolean(me) && Boolean(opponent);
    return {
      me,
      opponent,
      bothPresent,
      opponentReady: Boolean(opponent?.ready),
      needsReady: bothPresent && !me.ready && state.phase !== 'running'
    };
  }

  function prefillDuelRoomFromQuery(input) {
    const room = new URLSearchParams(window.location.search).get('room')?.trim();
    if (input && room && /^[A-Za-z0-9_-]{1,20}$/.test(room)) input.value = room;
  }

  window.createDuelRoomClient = createDuelRoomClient;
  window.duelReadinessPrompt = duelReadinessPrompt;
  window.prefillDuelRoomFromQuery = prefillDuelRoomFromQuery;
  window.suggestedDuelPlayerName = suggestedPlayerName;
})();
