/**
 * 谁是AI v3 — 客户端
 *
 * 事件接收：
 *   playerJoined, rolesRevealed, phaseChange, missionProposed,
 *   chat, teamVoteResult, missionResult, possessionAlert, signalCheck,
 *   gameFinished, roomReset, error
 *
 * 事件发送：
 *   joinRoom, startGame, proposeMission, teamVote, missionVote, chat, resetRoom
 */

// 全局错误处理
window.onerror = function(msg, url, line, col, error) {
  console.error('游戏错误:', msg, 'at', url, line, col);
  showError('游戏出现错误，请刷新页面重试');
  return false;
};

// 未处理的 Promise 拒绝
window.onunhandledrejection = function(event) {
  console.error('未处理的 Promise 拒绝:', event.reason);
  showError('操作失败，请重试');
};

// 显示错误提示
function showError(message, duration = 5000) {
  showToast(`❌ ${message}`, duration);
}

// 显示成功提示
function showSuccess(message, duration = 3000) {
  showToast(`✅ ${message}`, duration);
}

// 网络错误处理
function handleNetworkError() {
  showError('网络连接失败，请检查网络后重试');
}

// 带重试的网络请求
function withRetry(fn, maxRetries = 3, delay = 1000) {
  return new Promise((resolve, reject) => {
    let retries = 0;
    
    function attempt() {
      fn()
        .then(resolve)
        .catch(err => {
          retries++;
          if (retries < maxRetries) {
            console.log(`重试 ${retries}/${maxRetries}...`);
            setTimeout(attempt, delay);
          } else {
            reject(err);
          }
        });
    }
    
    attempt();
  });
}

// 显示加载状态
function showLoading(message = '加载中...') {
  const overlay = document.getElementById('loadingOverlay');
  if (overlay) {
    const text = overlay.querySelector('.loading-text');
    if (text) text.textContent = message;
    overlay.classList.remove('hidden');
  }
}

// 隐藏加载状态
function hideLoading() {
  const overlay = document.getElementById('loadingOverlay');
  if (overlay) {
    overlay.classList.add('hidden');
  }
}

// 显示连接状态
function updateConnectionStatus(status) {
  const statusEl = document.getElementById('connectionStatus');
  if (statusEl) {
    statusEl.className = `connection-status ${status}`;
    const statusText = {
      'connected': '已连接',
      'connecting': '连接中...',
      'disconnected': '已断开',
      'reconnecting': '重连中...'
    };
    statusEl.textContent = statusText[status] || status;
  }
  
  // 显示/隐藏断线重连提示
  const overlay = document.getElementById('disconnectOverlay');
  if (overlay) {
    if (status === 'disconnected' || status === 'reconnecting') {
      overlay.classList.remove('hidden');
      updateReconnectProgress(status);
    } else {
      overlay.classList.add('hidden');
    }
  }
}

// 更新重连进度
function updateReconnectProgress(status) {
  const bar = document.getElementById('reconnectBar');
  const statusText = document.getElementById('reconnectStatus');
  
  if (status === 'reconnecting') {
    if (bar) bar.style.width = '50%';
    if (statusText) statusText.textContent = '正在尝试重连...';
  } else if (status === 'disconnected') {
    if (bar) bar.style.width = '0%';
    if (statusText) statusText.textContent = '连接已断开';
  }
}

// 手动重连
function manualReconnect() {
  if (socket) {
    updateConnectionStatus('reconnecting');
    socket.connect();
    
    // 重连成功后重新加入房间
    socket.on('connect', () => {
      updateConnectionStatus('connected');
      if (currentRoomId && myName) {
        socket.emit('joinRoom', {
          roomId: currentRoomId,
          name: myName,
          mode: gameMode
        });
      }
    });
  }
}

// 显示帮助面板
function showHelp() {
  const helpPanel = document.getElementById('helpPanel');
  if (helpPanel) {
    helpPanel.classList.remove('hidden');
  }
}

// 隐藏帮助面板
function hideHelp() {
  const helpPanel = document.getElementById('helpPanel');
  if (helpPanel) {
    helpPanel.classList.add('hidden');
  }
}

// 显示教程
function showTutorial() {
  const tutorialPanel = document.getElementById('tutorialPanel');
  if (tutorialPanel) {
    tutorialPanel.classList.remove('hidden');
    startTutorial();
  }
}

// 隐藏教程
function hideTutorial() {
  const tutorialPanel = document.getElementById('tutorialPanel');
  if (tutorialPanel) {
    tutorialPanel.classList.add('hidden');
  }
}

// 开始教程
function startTutorial() {
  const steps = [
    {
      title: '欢迎来到谁是AI！',
      content: '这是一个社交推理游戏，你需要找出谁是渗透者。',
      target: null
    },
    {
      title: '游戏目标',
      content: '好人阵营需要完成3次任务，坏人阵营需要破坏3次任务。',
      target: null
    },
    {
      title: '角色介绍',
      content: '工程师（好人）、渗透者（坏人）、信号员（好人，能检测AI附身）、观察者（好人，查看投票历史）、保护者（好人，防止附身）、干扰者（坏人，改变投票）。',
      target: null
    },
    {
      title: '游戏流程',
      content: '每轮包括：提名→讨论→投票→执行。队长提名小队，全员投票决定是否执行。',
      target: null
    },
    {
      title: 'AI附身',
      content: '每轮有50%概率有人被AI附身，他们的消息会被改写。信号员能检测到附身。观察发言风格异常的玩家。',
      target: null
    },
    {
      title: '投票历史',
      content: '游戏会记录每轮的投票情况。分析投票模式，找出投票不一致的玩家。',
      target: null
    },
    {
      title: '策略建议',
      content: '工程师：观察发言风格，分析投票历史。渗透者：伪装成好人，投票时投"同意"，执行时投"破坏"。信号员：利用信息引导讨论，但不要过早暴露身份。',
      target: null
    },
    {
      title: '开始游戏',
      content: '选择模式，输入昵称，点击开始游戏即可！',
      target: null
    }
  ];
  
  let currentStep = 0;
  
  function showStep() {
    const step = steps[currentStep];
    const tutorialContent = document.getElementById('tutorialContent');
    if (tutorialContent) {
      tutorialContent.innerHTML = `
        <h3>${step.title}</h3>
        <p>${step.content}</p>
        <div class="tutorial-nav">
          <button onclick="prevTutorialStep()" ${currentStep === 0 ? 'disabled' : ''}>上一步</button>
          <span>${currentStep + 1} / ${steps.length}</span>
          <button onclick="nextTutorialStep()">${currentStep === steps.length - 1 ? '完成' : '下一步'}</button>
        </div>
      `;
    }
  }
  
  window.nextTutorialStep = function() {
    if (currentStep < steps.length - 1) {
      currentStep++;
      showStep();
    } else {
      hideTutorial();
    }
  };
  
  window.prevTutorialStep = function() {
    if (currentStep > 0) {
      currentStep--;
      showStep();
    }
  };
  
  showStep();
}

// ═══════════════════════════════════════
//  状态
// ═══════════════════════════════════════
let socket = null;
let myId = null;
let myName = '';
let myRole = null;
let currentRoomId = '';
let gameMode = 'test';
let currentPhase = null;
let messagesLeft = 4;   // 剩余消息数
let maxMessages = 4;
let proposedTeamIds = []; // 当前提名的小队
let selectedMembers = []; // 队长选中的成员
let teamSizeNeeded = 2;
let isLeader = false;
let timerInterval = null;
let timerValue = 0;
let stage = null; // Canvas 小人舞台
let voteHistory = []; // 投票历史
let signalHistory = []; // 信号员历史

// 谜题状态（方向 C）
let currentPuzzle = null;
let puzzleAnswer = null;
let puzzleSubPhase = null;  // 'discuss' | 'vote' | 'reveal'
let isOnMissionTeam = false; // 当前玩家是否在小队中

// 调试模式状态
let debugMode = false; // 是否启用调试模式
let isPaused = false; // 游戏是否暂停
let debugLogEntries = []; // 调试日志
let aiPlayers = []; // AI玩家列表
let gameRoom = null; // 游戏房间状态

// ═══════════════════════════════════════
//  登录 & 加入
// ═══════════════════════════════════════
function setMode(mode) {
  gameMode = mode;
  document.querySelectorAll('.mode-btn').forEach(btn => {
    btn.classList.toggle('active', btn.dataset.mode === mode);
  });
}

function updateModeDisplay(mode) {
  const modeEl = document.getElementById('currentMode');
  if (modeEl) {
    const modeLabels = {
      'test': '测试模式',
      'online': '联网模式',
      'solo': '单人调试'
    };
    const newText = modeLabels[mode] || mode;
    modeEl.textContent = newText;
  }
}

function backToHome() {
  // 断开 socket 连接
  if (socket) {
    socket.disconnect();
    socket = null;
  }
  // 重置状态
  myId = null;
  myName = '';
  myRole = null;
  currentRoomId = '';
  gameMode = 'test';
  currentPhase = null;
  // 显示登录界面
  showScreen('loginScreen');
}

function joinGame() {
  const nameInput = document.getElementById('playerName');
  const roomInput = document.getElementById('roomId');
  const joinBtn = document.getElementById('joinBtn');
  
  // 验证昵称
  myName = nameInput.value.trim();
  if (!myName) {
    showToast('❌ 请输入昵称');
    nameInput.focus();
    return;
  }
  
  if (myName.length < 2) {
    showToast('❌ 昵称至少需要2个字符');
    nameInput.focus();
    return;
  }
  
  currentRoomId = roomInput.value.trim() || 'room1';

  // 显示按钮加载状态
  const btnText = joinBtn.querySelector('.btn-text');
  const btnLoading = joinBtn.querySelector('.btn-loading');
  if (btnText && btnLoading) {
    btnText.classList.add('hidden');
    btnLoading.classList.remove('hidden');
    joinBtn.disabled = true;
  }

  // 先显示等待界面
  showScreen('waitingScreen');
  const waitingRoomId = document.getElementById('waitingRoomId');
  if (waitingRoomId) waitingRoomId.textContent = currentRoomId;
  
  // 开始等待提示
  startWaitingTips();

  // 连接 Socket
  socket = io();

  // 注册事件（在连接之前）
  registerEvents();

  socket.on('connect', () => {
    myId = socket.id;
    console.log('已连接:', myId);
    socket.emit('joinRoom', {
      roomId: currentRoomId,
      name: myName,
      mode: gameMode
    });
  });
  
  socket.on('connect_error', (error) => {
    console.error('连接错误:', error);
    showToast('❌ 连接失败，请检查网络');
    // 恢复按钮状态
    if (btnText && btnLoading) {
      btnText.classList.remove('hidden');
      btnLoading.classList.add('hidden');
      joinBtn.disabled = false;
    }
  });
}

function copyRoomId() {
  navigator.clipboard.writeText(currentRoomId).then(() => {
    // 按钮反馈
    const btn = document.getElementById('copyBtn');
    if (btn) {
      const originalText = btn.innerHTML;
      btn.innerHTML = '✅ 已复制！';
      btn.disabled = true;
      setTimeout(() => {
        btn.innerHTML = originalText;
        btn.disabled = false;
      }, 2000);
    }
    showToast('房间号已复制');
    // 添加复制成功动画
    copyRoomIdSuccess();
  }).catch(() => {
    showToast('复制失败，请手动复制');
  });
}

// ═══════════════════════════════════════
//  事件注册
// ═══════════════════════════════════════
function registerEvents() {

  // 玩家加入/离开
  socket.on('playerJoined', ({ players, count, mode }) => {
    renderWaitingPlayers(players);
    // 更新模式显示
    if (mode) {
      gameMode = mode;
      updateModeDisplay(mode);
    }
    // 如果在游戏界面，更新玩家列表
    if (currentPhase) {
      renderGamePlayers(players);
    }
  });

  // 角色揭示
  socket.on('rolesRevealed', ({ role, roleLabel, roleDescription, players }) => {
    myRole = role;
    document.getElementById('myRole').textContent = roleLabel;
    
    // 停止等待提示
    stopWaitingTips();
    
    // 显示测试模式水印
    if (gameMode === 'test' || gameMode === 'solo') {
      const watermark = document.getElementById('testModeWatermark');
      if (watermark) {
        watermark.textContent = gameMode === 'test' ? '🧪 测试环境' : '🔧 单人调试';
        watermark.classList.remove('hidden');
      }
    }
    
    // 初始化调试模式
    initDebugMode();
    
    // 显示角色揭示动画
    showRoleReveal(role, roleLabel, roleDescription, () => {
      showScreen('gameScreen');
      renderGamePlayers(players);
    });

    // 初始化舞台小人
    if (window.PlayerStage && window.innerWidth > 768) {
      stage = new PlayerStage('playerStage');
      stage.setPlayers(players.map(p => ({
        id: p.id,
        name: p.name,
        role: 'unknown', // 游戏开始前不显示角色
        eliminated: false,
      })));
      stage.startLoop();
    }
  });

  // 阶段切换
  socket.on('phaseChange', (data) => {
    handlePhaseChange(data);
  });

  // 小队提名
  socket.on('missionProposed', ({ leaderId, leaderName, memberIds, memberNames }) => {
    document.getElementById('leaderName').textContent = leaderName;
    proposedTeamIds = memberIds;
    renderProposedTeam(memberNames);
    showToast(`${leaderName} 提名了小队`, 2000);

    // 舞台：队长指向被提名者
    if (stage && leaderId) {
      stage.setPointing(leaderId, memberIds);
    }
  });

  // 聊天消息
  socket.on('chat', ({ playerId, playerName, message, messagesLeft: left, isPossessed }) => {
    addChatMessage(playerName, message, playerId === myId);
    if (playerId === myId) {
      messagesLeft = left;
      updateMsgCounter();
    }

    // 舞台：发言者弹跳
    if (stage) {
      stage.setAnimation(playerId, 'speaking');
      setTimeout(() => {
        if (stage) stage.setAnimation(playerId, 'idle');
      }, 1000);
    }
  });

  // 小队投票结果
  socket.on('teamVoteResult', ({ approved, approveCount, rejectCount, votes, voteHistory }) => {
    const result = approved ? '小队通过 ✅' : '小队被否决 ❌';
    showToast(`${result} (${approveCount}:${rejectCount})`, 3000);

    // 在聊天区显示投票结果（匿名化）
    addChatMessage('系统', `投票结果: ${approveCount} 票同意, ${rejectCount} 票反对`, false, true);
    
    // 更新投票历史面板
    if (voteHistory) {
      window.voteHistory = voteHistory; // 更新全局变量
      updateVoteHistory(voteHistory);
      
      // 为最新投票记录添加动画
      const voteHistoryList = document.getElementById('voteHistoryList');
      if (voteHistoryList && voteHistoryList.lastElementChild) {
        voteHistoryList.lastElementChild.classList.add('vote-result-animation');
        setTimeout(() => voteHistoryList.lastElementChild.classList.remove('vote-result-animation'), 500);
      }
    }

    // 舞台：显示投票结果牌
    if (stage) {
      stage.showVoteResults({ approveCount, rejectCount });
      setTimeout(() => {
        if (stage) stage.resetAnimations();
      }, 3000);
    }
  });

  // 任务结果
  socket.on('missionResult', ({ roundNumber, success, sabotageCount, missionResults, missionSuccesses, missionFailures }) => {
    const result = success ? '任务成功 ✅' : `任务失败 ❌ (${sabotageCount} 票破坏)`;
    showToast(`第 ${roundNumber} 轮: ${result}`, 3000);
    addChatMessage('系统', `第 ${roundNumber} 轮任务${success ? '成功' : '失败'}`, false, true);
    
    // 更新任务进度显示
    const successCountEl = document.getElementById('successCount');
    const failCountEl = document.getElementById('failCount');
    if (successCountEl) successCountEl.textContent = missionSuccesses;
    if (failCountEl) failCountEl.textContent = missionFailures;
    
    // 渲染任务点并添加动画
    renderMissionDots(missionResults);
    
    // 为任务点添加动画
    const missionDots = document.getElementById('missionDots');
    if (missionDots) {
      const dots = missionDots.querySelectorAll('.dot');
      if (dots.length > 0) {
        const lastDot = dots[dots.length - 1];
        if (success) {
          lastDot.classList.add('mission-success-animation');
          setTimeout(() => lastDot.classList.remove('mission-success-animation'), 800);
        } else {
          lastDot.classList.add('mission-fail-animation');
          setTimeout(() => lastDot.classList.remove('mission-fail-animation'), 800);
        }
      }
    }

    // 舞台：成功欢呼 / 失败沮丧
    if (stage) {
      const animType = success ? 'celebrating' : 'defeated';
      proposedTeamIds.forEach(id => stage.setAnimation(id, animType));
      setTimeout(() => {
        if (stage) stage.resetAnimations();
      }, 2000);
    }
  });

  // 附身警告（只发给被附身者）
  socket.on('possessionAlert', () => {
    document.getElementById('possessionWarning').classList.remove('hidden');
    showToast('⚠️ 你被 AI 信号干扰了！你的消息可能会被改写', 5000);

    // 舞台：被附身者闪烁
    if (stage) {
      stage.setAnimation(myId, 'possessed');
    }
  });

  // 信号员感知
  socket.on('signalCheck', ({ hasPossession, roundNumber, signalHistory }) => {
    const alert = document.getElementById('signalAlert');
    alert.classList.remove('hidden');
    document.getElementById('signalText').textContent = hasPossession
      ? `第 ${roundNumber} 轮：检测到异常信号 ⚠️`
      : `第 ${roundNumber} 轮：信号正常 ✅`;
    
    // 更新信号员历史
    if (signalHistory) {
      updateSignalHistory(signalHistory);
    }
  });

  // 任务投票提示（小队成员）
  socket.on('missionVotePrompt', ({ canSabotage }) => {
    document.getElementById('missionActions').classList.remove('hidden');
    if (canSabotage) {
      document.getElementById('sabotageBtn').classList.remove('hidden');
    }
  });

  // 游戏结束
  socket.on('gameFinished', ({ winner, winnerLabel, roles, missionResults }) => {
    handleGameFinished(winner, winnerLabel, roles, missionResults);
  });

  // 房间重置
  socket.on('roomReset', () => {
    location.reload();
  });

  // 错误
  socket.on('error', (msg) => {
    showError(msg);
  });
  
  // 连接错误
  socket.on('connect_error', (error) => {
    console.error('连接错误:', error);
    handleNetworkError();
  });
  
  // 断开连接
  socket.on('disconnect', (reason) => {
    console.log('断开连接:', reason);
    if (reason === 'io server disconnect') {
      showError('服务器断开连接，请刷新页面重试');
    } else {
      showToast('连接已断开，正在重新连接...', 3000);
    }
  });
  
  // 重新连接
  socket.on('reconnect', () => {
    showSuccess('重新连接成功');
    // 重新加入房间
    if (currentRoomId && myName) {
      socket.emit('joinRoom', {
        roomId: currentRoomId,
        name: myName,
        mode: gameMode
      });
    }
  });
}

// ═══════════════════════════════════════
//  阶段处理
// ═══════════════════════════════════════
function handlePhaseChange(data) {
  const { phase, roundNumber, totalRounds, leader, teamSize,
    proposedTeam, proposedTeamNames, missionResults,
    maxMessages: mm, maxChars: mc, timeLimit } = data;

  currentPhase = phase;
  
  const roundInfo = document.getElementById('roundInfo');
  if (roundInfo) roundInfo.textContent = `第 ${roundNumber}/${totalRounds} 轮`;

  // 重置附身警告
  const possessionWarning = document.getElementById('possessionWarning');
  if (possessionWarning) possessionWarning.classList.add('hidden');

  // 更新任务进度
  if (missionResults) {
    renderMissionDots(missionResults);
  }

  // 清除旧定时器
  if (timerInterval) clearInterval(timerInterval);
  startTimer(timeLimit);

  // 隐藏所有操作按钮
  hideAllActions();

  // 显示新手引导提示
  showTutorialHint(phase);
  
  switch (phase) {
    case 'propose':
      const phaseInfo = document.getElementById('phaseInfo');
      if (phaseInfo) phaseInfo.textContent = '提名阶段';
      
      const leaderName = document.getElementById('leaderName');
      if (leaderName) leaderName.textContent = (leader && leader.name) || '--';
      
      isLeader = leader && leader.id === myId;
      teamSizeNeeded = teamSize;

      // 重新渲染玩家列表（更新可点击状态）
      updatePlayerSelectionUI();

      // 舞台：队长发光
      if (stage && leader) {
        stage.setLeader(leader.id);
        stage.setAnimation(leader.id, 'speaking');
      }

      if (isLeader) {
        const proposeActions = document.getElementById('proposeActions');
        if (proposeActions) proposeActions.classList.remove('hidden');
        
        const proposeHint = document.getElementById('proposeHint');
        if (proposeHint) proposeHint.textContent = `请选择 ${teamSize} 名队友`;
        
        const needCount = document.getElementById('needCount');
        if (needCount) needCount.textContent = teamSize;
        
        selectedMembers = [];
        updateProposeBtn();
        showToast(`你是队长！请选择 ${teamSize} 名队友`, 3000);
      } else {
        showToast(`等待 ${(leader && leader.name) || '队长'} 提名小队...`, 2000);
      }

      // 禁用聊天
      disableChat('提名阶段无法发言，等待队长提名小队');
      break;

    case 'discuss':
      const discussPhaseInfo = document.getElementById('phaseInfo');
      if (discussPhaseInfo) discussPhaseInfo.textContent = '讨论阶段';
      
      // 启用聊天
      maxMessages = mm || 4;
      messagesLeft = maxMessages;
      enableChat(mc || 30);
      updateMsgCounter();

      // 清空聊天区（每轮重新开始）
      // 不清空，保留历史
      addChatMessage('系统', `— 第 ${roundNumber} 轮讨论 —`, false, true);

      // 显示提名小队
      if (proposedTeamNames) {
        renderProposedTeam(proposedTeamNames);
      }

      // 舞台：清除指向，所有人 idle
      if (stage) {
        stage.clearPointing();
        stage.resetAnimations();
      }
      break;

    case 'team_vote':
      const votePhaseInfo = document.getElementById('phaseInfo');
      if (votePhaseInfo) votePhaseInfo.textContent = '投票阶段';
      
      disableChat('投票阶段无法发言，请投票');
      
      const voteActions = document.getElementById('voteActions');
      if (voteActions) voteActions.classList.remove('hidden');

      // 舞台：所有人举手投票
      if (stage) {
        stage.setAnimationAll('voting');
      }
      break;

    case 'mission':
      const missionPhaseInfo = document.getElementById('phaseInfo');
      if (missionPhaseInfo) missionPhaseInfo.textContent = '执行任务';
      
      disableChat('任务执行阶段无法发言');

      // 舞台：小队成员突出
      if (stage) {
        stage.resetAnimations();
        if (proposedTeam) {
          proposedTeam.forEach(id => stage.setAnimation(id, 'voting'));
        }
      }

      // 方向 C：谜题 UI 由 missionPuzzle/missionSpectate 事件处理
      // 这里只做初始化隐藏
      hidePuzzleUI();
      break;
  }
}

// ═══════════════════════════════════════
//  聊天系统（受限）
// ═══════════════════════════════════════
function enableChat(maxChars) {
  const input = document.getElementById('chatInput');
  const btn = document.getElementById('chatSendBtn');
  if (input) {
    input.disabled = false;
    input.maxLength = maxChars || 30;
    input.placeholder = `输入消息（最多${maxChars || 30}字）...`;
    input.focus();
  }
  if (btn) btn.disabled = false;
}

function disableChat(reason = '') {
  const input = document.getElementById('chatInput');
  const btn = document.getElementById('chatSendBtn');
  if (input) {
    input.disabled = true;
    input.placeholder = reason || '当前阶段无法发言';
  }
  if (btn) btn.disabled = true;
}

function updateMsgCounter() {
  const counter = document.getElementById('msgCounter');
  if (counter) {
    counter.textContent = `${maxMessages - messagesLeft}/${maxMessages}`;
  }
}

function sendChat() {
  const input = document.getElementById('chatInput');
  if (!input) return;
  
  const msg = input.value.trim();
  if (!msg) return;

  // 谜题讨论阶段 → 走 puzzleChat
  if (currentPhase === 'mission' && puzzleSubPhase === 'discuss' && isOnMissionTeam) {
    if (puzzleMessagesLeft <= 0) {
      showToast('谜题讨论消息已用完', 2000);
      return;
    }
    socket.emit('puzzleChat', { roomId: currentRoomId, message: msg });
    puzzleMessagesLeft--;
    input.value = '';
    input.placeholder = `谜题讨论（${puzzleMessagesLeft}/${puzzleMaxMessages}）...`;
    if (puzzleMessagesLeft <= 0) {
      input.disabled = true;
      const btn = document.getElementById('chatSendBtn');
      if (btn) btn.disabled = true;
    }
    input.focus();
    return;
  }

  // 普通讨论阶段
  if (messagesLeft <= 0) {
    showToast('本轮消息已用完', 2000);
    return;
  }

  socket.emit('chat', { roomId: currentRoomId, message: msg });
  input.value = '';
  input.focus();
}

// 回车发送
document.addEventListener('keydown', (e) => {
  if (e.key === 'Enter' && document.activeElement === document.getElementById('chatInput')) {
    sendChat();
  }
});

// 键盘导航支持
document.addEventListener('keydown', (e) => {
  // Tab 键导航
  if (e.key === 'Tab') {
    // 添加焦点样式
    document.body.classList.add('keyboard-nav');
  }
  
  // Enter 键激活按钮
  if (e.key === 'Enter') {
    const activeElement = document.activeElement;
    if (activeElement && activeElement.tagName === 'BUTTON') {
      activeElement.click();
    }
  }
  
  // Escape 键关闭弹窗
  if (e.key === 'Escape') {
    const overlay = document.querySelector('.role-reveal-overlay');
    if (overlay) {
      overlay.click();
    }
  }
});

// 鼠标点击时移除键盘导航样式
document.addEventListener('mousedown', () => {
  document.body.classList.remove('keyboard-nav');
});

function addChatMessage(name, text, isMe, isSystem = false) {
  const container = document.getElementById('chatMessages');
  if (!container) return;
  const div = document.createElement('div');
  div.className = 'chat-msg' + (isMe ? ' mine' : '') + (isSystem ? ' system' : '');
  div.innerHTML = `<span class="msg-name">${name}:</span> <span class="msg-text">${escapeHtml(text)}</span>`;
  container.appendChild(div);
  container.scrollTop = container.scrollHeight;
}

// ═══════════════════════════════════════
//  提名系统
// ═══════════════════════════════════════
function toggleMemberSelection(playerId) {
  if (!isLeader || currentPhase !== 'propose') return;

  const idx = selectedMembers.indexOf(playerId);
  if (idx >= 0) {
    selectedMembers.splice(idx, 1);
  } else if (selectedMembers.length < teamSizeNeeded) {
    selectedMembers.push(playerId);
  }

  updatePlayerSelectionUI();
  updateProposeBtn();
}

function updateProposeBtn() {
  const btn = document.getElementById('confirmProposeBtn');
  btn.disabled = selectedMembers.length !== teamSizeNeeded;
}

function confirmPropose() {
  if (selectedMembers.length !== teamSizeNeeded) return;
  socket.emit('proposeMission', { roomId: currentRoomId, memberIds: selectedMembers });
  hideAllActions();
}

// ═══════════════════════════════════════
//  投票系统
// ═══════════════════════════════════════
function submitTeamVote(approve) {
  socket.emit('teamVote', { roomId: currentRoomId, approve });
  hideAllActions();
  showToast(approve ? '已投同意' : '已投反对', 1500);
}

function submitMissionVote(success) {
  socket.emit('missionVote', { roomId: currentRoomId, success });
  hideAllActions();
  showToast(success ? '已投成功' : '已投破坏', 1500);
}

// ═══════════════════════════════════════
//  渲染
// ═══════════════════════════════════════
// 更新信号员历史
function updateSignalHistory(signalHistory) {
  const container = document.getElementById('signalHistoryList');
  if (!container) return;
  
  let html = '';
  signalHistory.forEach((record, index) => {
    const roundNum = index + 1;
    const icon = record.hasPossession ? '⚠️' : '✅';
    const text = record.hasPossession ? '有附身' : '无附身';
    const statusClass = record.hasPossession ? 'possessed' : 'normal';
    
    html += `<div class="signal-record ${statusClass}">`;
    html += `<span class="signal-round">第${roundNum}轮</span>`;
    html += `<span class="signal-status">${icon} ${text}</span>`;
    html += `</div>`;
  });
  
  container.innerHTML = html;
}

// 更新投票历史面板
function updateVoteHistory(voteHistory) {
  const container = document.getElementById('voteHistoryList');
  if (!container) return;
  
  // 使用DocumentFragment减少DOM操作
  const fragment = document.createDocumentFragment();
  
  voteHistory.forEach((record, index) => {
    const roundNum = record.round || (index + 1);
    const statusIcon = record.approved ? '✅' : '❌';
    const statusClass = record.approved ? 'approved' : 'rejected';
    
    const recordElement = document.createElement('div');
    recordElement.className = `vote-record ${statusClass}`;
    
    // 创建头部
    const header = document.createElement('div');
    header.className = 'vote-record-header';
    
    const roundSpan = document.createElement('span');
    roundSpan.className = 'vote-round';
    roundSpan.textContent = `第${roundNum}轮`;
    
    const statusSpan = document.createElement('span');
    statusSpan.className = 'vote-status';
    statusSpan.textContent = statusIcon;
    
    header.appendChild(roundSpan);
    header.appendChild(statusSpan);
    recordElement.appendChild(header);
    
    // 创建团队信息
    const teamDiv = document.createElement('div');
    teamDiv.className = 'vote-team';
    teamDiv.textContent = `提名: ${record.team.join(', ')}`;
    recordElement.appendChild(teamDiv);
    
    // 创建投票详情
    const detailsDiv = document.createElement('div');
    detailsDiv.className = 'vote-details';
    
    const approveCount = record.approveCount || 0;
    const rejectCount = record.rejectCount || 0;
    
    const approveSpan = document.createElement('span');
    approveSpan.className = 'vote-summary';
    approveSpan.textContent = `✅ ${approveCount} 票`;
    
    const rejectSpan = document.createElement('span');
    rejectSpan.className = 'vote-summary';
    rejectSpan.textContent = `❌ ${rejectCount} 票`;
    
    detailsDiv.appendChild(approveSpan);
    detailsDiv.appendChild(rejectSpan);
    recordElement.appendChild(detailsDiv);
    
    fragment.appendChild(recordElement);
  });
  
  // 清空容器并添加新内容
  container.innerHTML = '';
  container.appendChild(fragment);
}

// 角色图标映射
const ROLE_ICONS = {
  'engineer': '🔧',
  'infiltrator': '🦠',
  'signal_keeper': '📡'
};

// 显示新手引导提示
function showTutorialHint(phase) {
  const hints = {
    'propose': '💡 提名阶段：队长需要选择队友执行任务。点击右侧玩家列表选择队友，然后点击"确认提名"。',
    'discuss': '💡 讨论阶段：所有玩家可以发言讨论。每人最多发送6条消息，每条最多50字。注意观察发言风格异常的玩家！',
    'team_vote': '💡 投票阶段：所有玩家投票"同意"或"反对"这个小队。投票历史会记录在左侧面板。',
    'mission': '💡 执行阶段：小队成员投票"成功"或"破坏"。只要有一票"破坏"，任务就失败！'
  };
  
  const hint = hints[phase];
  if (hint) {
    showToast(hint, 5000);
  }
}

// 显示角色揭示动画
function showRoleReveal(role, roleLabel, roleDescription, onComplete) {
  // 创建遮罩层
  const overlay = document.createElement('div');
  overlay.className = 'role-reveal-overlay';
  overlay.innerHTML = `
    <div class="role-reveal-content">
      <div class="role-reveal-icon ${`role-${role}`}">${ROLE_ICONS[role] || '❓'}</div>
      <div class="role-reveal-label">${roleLabel}</div>
      <div class="role-reveal-desc">${roleDescription}</div>
      <div class="role-reveal-hint">点击任意处继续</div>
    </div>
  `;
  
  document.body.appendChild(overlay);
  
  // 点击继续
  overlay.addEventListener('click', () => {
    overlay.style.animation = 'fadeOut 0.3s ease forwards';
    setTimeout(() => {
      overlay.remove();
      if (onComplete) onComplete();
    }, 300);
  });
  
  // 5秒后自动继续
  setTimeout(() => {
    if (document.body.contains(overlay)) {
      overlay.click();
    }
  }, 5000);
}

// 等待提示文案
const WAITING_TIPS = [
  '好游戏值得等待 ✨',
  '正在为你匹配最佳对手...',
  '一场精彩的推理对决即将开始',
  '耐心等待，惊喜即将到来',
  '你的队友正在赶来的路上'
];

let waitingTipIndex = 0;
let waitingTipInterval = null;

function startWaitingTips() {
  if (waitingTipInterval) return;
  waitingTipInterval = setInterval(() => {
    waitingTipIndex = (waitingTipIndex + 1) % WAITING_TIPS.length;
    const tipsEl = document.getElementById('waitingTips');
    if (tipsEl) {
      tipsEl.textContent = WAITING_TIPS[waitingTipIndex];
    }
  }, 5000);
}

function stopWaitingTips() {
  if (waitingTipInterval) {
    clearInterval(waitingTipInterval);
    waitingTipInterval = null;
  }
}

// 缓存 DOM 元素
const domCache = {
  waitingPlayerList: null,
  waitingCount: null,
  waitingText: null,
  slots: []
};

// 初始化 DOM 缓存
function initDOMCache() {
  domCache.waitingPlayerList = document.getElementById('waitingPlayerList');
  domCache.waitingCount = document.getElementById('waitingCount');
  domCache.waitingText = document.getElementById('waitingText');
  
  for (let i = 0; i < 6; i++) {
    domCache.slots[i] = document.getElementById(`slot${i}`);
  }
}

function renderWaitingPlayers(players) {
  // 使用缓存的 DOM 元素
  if (!domCache.waitingPlayerList) {
    initDOMCache();
  }
  
  // 批量更新 DOM
  const fragment = document.createDocumentFragment();
  
  // 更新玩家列表
  if (domCache.waitingPlayerList) {
    const html = players.map(p =>
      `<div class="waiting-player">${p.isAI ? '🤖' : '👤'} ${escapeHtml(p.name)}</div>`
    ).join('');
    domCache.waitingPlayerList.innerHTML = html;
  }
  
  // 更新计数
  if (domCache.waitingCount) {
    domCache.waitingCount.textContent = players.length;
  }
  
  // 更新进度条
  updateWaitingProgress(players.length, 6);
  
  // 更新等待提示文案
  if (domCache.waitingText) {
    const humanCount = players.filter(p => !p.isAI).length;
    const aiCount = players.filter(p => p.isAI).length;
    let text = '正在召唤你的队友...';
    
    if (humanCount >= 2 && aiCount === 0) {
      text = `${humanCount}位真人玩家已加入`;
    } else if (aiCount > 0) {
      text = `${humanCount}位真人 + ${aiCount}位AI已加入`;
    }
    
    domCache.waitingText.textContent = text;
  }
  
  // 更新玩家占位符
  for (let i = 0; i < 6; i++) {
    const slot = domCache.slots[i];
    if (!slot) continue;
    
    if (i < players.length) {
      const player = players[i];
      const isMe = player.id === myId;
      slot.className = `player-slot filled${isMe ? ' you' : ''}`;
      slot.innerHTML = `
        <div class="slot-avatar">${player.isAI ? '🤖' : (isMe ? '⭐' : '👤')}</div>
        <div class="slot-name">${escapeHtml(player.name)}${isMe ? ' (你)' : ''}</div>
        <div class="slot-status">${isMe ? '就是你！' : '已加入'}</div>
      `;
      
      // 添加加入动画
      slot.classList.add('player-join-animation');
      setTimeout(() => slot.classList.remove('player-join-animation'), 500);
    } else {
      slot.className = 'player-slot';
      slot.innerHTML = `
        <div class="slot-avatar">👤</div>
        <div class="slot-name">等待中...</div>
        <div class="slot-status">准备中</div>
      `;
    }
  }
}

// 玩家列表缓存（用于事件委托）
let cachedPlayers = [];

// 事件委托：点击玩家列表
const playerListElement = document.getElementById('playerList');
if (playerListElement) {
  playerListElement.addEventListener('click', (e) => {
    const item = e.target.closest('.player-item');
    if (!item || !isLeader || currentPhase !== 'propose') return;
    const playerId = item.dataset.playerId;
    if (playerId && playerId !== myId) {
      toggleMemberSelection(playerId);
    }
  });
}

function renderGamePlayers(players) {
  cachedPlayers = players;
  const container = document.getElementById('playerList');
  if (!container) return;
  
  // 使用DocumentFragment减少DOM操作
  const fragment = document.createDocumentFragment();
  
  players.forEach(p => {
    const isMe = p.id === myId;
    const isOnTeam = proposedTeamIds.includes(p.id);
    const isSelected = selectedMembers.includes(p.id);
    let cls = 'player-item';
    if (isMe) cls += ' me';
    if (isOnTeam) cls += ' on-team';
    if (isSelected) cls += ' selected';
    if (p.eliminated) cls += ' eliminated';
    if (isLeader && currentPhase === 'propose' && !isMe && !p.eliminated) cls += ' clickable';
    
    const playerElement = document.createElement('div');
    playerElement.className = cls;
    playerElement.dataset.playerId = p.id;
    
    const nameSpan = document.createElement('span');
    nameSpan.className = 'player-name';
    nameSpan.textContent = `${escapeHtml(p.name)}${isMe ? ' (你)' : ''}`;
    playerElement.appendChild(nameSpan);
    
    if (isOnTeam) {
      const teamTag = document.createElement('span');
      teamTag.className = 'team-tag';
      teamTag.textContent = '🎯';
      playerElement.appendChild(teamTag);
    }
    
    if (p.eliminated) {
      const elimTag = document.createElement('span');
      elimTag.className = 'elim-tag';
      elimTag.textContent = '❌';
      playerElement.appendChild(elimTag);
    }
    
    fragment.appendChild(playerElement);
  });
  
  // 清空容器并添加新内容
  container.innerHTML = '';
  container.appendChild(fragment);
}

function renderProposedTeam(names) {
  const container = document.getElementById('proposedTeam');
  if (!container) return;
  
  // 添加提名动画
  container.classList.add('nomination-animation');
  setTimeout(() => container.classList.remove('nomination-animation'), 600);
  
  container.innerHTML = names.map(n =>
    `<span class="team-member">${escapeHtml(n)}</span>`
  ).join('');
}

function renderMissionDots(results) {
  const container = document.getElementById('missionDots');
  if (!container) return;
  let html = '';
  for (let i = 0; i < 4; i++) {
    if (i < results.length) {
      html += `<span class="dot ${results[i] ? 'success' : 'fail'}">${results[i] ? '✅' : '❌'}</span>`;
    } else {
      html += `<span class="dot pending">○</span>`;
    }
  }
  container.innerHTML = html;

  const successCountEl = document.getElementById('successCount');
  const failCountEl = document.getElementById('failCount');
  if (successCountEl) successCountEl.textContent = results.filter(r => r).length;
  if (failCountEl) failCountEl.textContent = results.filter(r => !r).length;
}

function updatePlayerSelectionUI() {
  // 用缓存的玩家数据重新渲染
  if (cachedPlayers.length > 0) {
    renderGamePlayers(cachedPlayers);
  }
}

// ═══════════════════════════════════════
//  游戏结束
// ═══════════════════════════════════════
function handleGameFinished(winner, winnerLabel, roles, missionResults) {
  currentPhase = null;

  // 舞台：胜利方跳舞
  if (stage) {
    stage.updateRoles(
      Object.fromEntries(Object.entries(roles).map(([id, info]) => [id, info.role]))
    );
    for (const [playerId, info] of Object.entries(roles)) {
      stage.setAnimation(playerId, info.isWinner ? 'celebrating' : 'defeated');
    }
    // 3 秒后停止
    setTimeout(() => {
      if (stage) {
        stage.stopLoop();
        stage.destroy();
        stage = null;
      }
    }, 3000);
  }

  showScreen('finishedScreen');

  const resultTitle = document.getElementById('resultTitle');
  if (resultTitle) resultTitle.textContent = winnerLabel;
  
  const resultMessage = document.getElementById('resultMessage');
  if (resultMessage) {
    resultMessage.textContent = winner === 'engineer' ? '好人阵营成功完成了3次任务！' : '渗透者成功破坏了3次任务！';
  }

  // 角色揭示
  const list = document.getElementById('rolesRevealList');
  if (list) {
    let html = '';
    for (const [playerId, info] of Object.entries(roles)) {
      const winTag = info.isWinner ? ' 🏆' : '';
      html += `<div class="role-card ${info.faction}">
        <span class="role-label">${info.roleLabel}${winTag}</span>
        <span class="role-name">${escapeHtml(info.name)}</span>
      </div>`;
    }
    list.innerHTML = html;
  }

  // 任务记录
  const dots = document.getElementById('missionSummaryDots');
  if (dots) {
    dots.innerHTML = missionResults.map(r =>
      `<span class="dot-big ${r ? 'success' : 'fail'}">${r ? '✅' : '❌'}</span>`
    ).join('');
  }
  
  // 填充游戏回顾
  fillGameReview();
  
  // 更新游戏统计
  updateGameStats({
    duration: calculateGameDuration(),
    voteRounds: voteHistory ? voteHistory.length : 0,
    chatMessages: document.querySelectorAll('.chat-msg').length,
    possessionCount: signalHistory ? signalHistory.filter(r => r.hasPossession).length : 0
  });
  
  // 更新MVP评选
  updateMVPs({
    reasoning: findMVPReasoning(roles),
    acting: findMVPActing(roles),
    chatter: findMVPChatter()
  });
  
  // 添加关键事件
  addKeyGameEvents();
}

// 填充游戏回顾（匿名化）
function fillGameReview() {
  // 投票历史（匿名化）
  const voteContainer = document.getElementById('reviewVoteHistory');
  if (voteContainer && voteHistory && voteHistory.length > 0) {
    let html = '';
    voteHistory.forEach((record, index) => {
      const roundNum = record.round || (index + 1);
      const statusIcon = record.approved ? '✅' : '❌';
      const approveCount = record.approveCount || 0;
      const rejectCount = record.rejectCount || 0;
      html += `<div class="review-item">`;
      html += `第${roundNum}轮 ${statusIcon} 提名: ${record.team.join(', ')}`;
      html += `<br><small>投票结果: ${approveCount}票同意, ${rejectCount}票反对</small>`;
      html += `</div>`;
    });
    voteContainer.innerHTML = html;
  } else if (voteContainer) {
    voteContainer.innerHTML = '<div class="review-item">无投票记录</div>';
  }
  
  // 信号员记录
  const signalContainer = document.getElementById('reviewSignalHistory');
  if (signalContainer && signalHistory && signalHistory.length > 0) {
    let html = '';
    signalHistory.forEach((record, index) => {
      const roundNum = index + 1;
      const icon = record.hasPossession ? '⚠️' : '✅';
      const text = record.hasPossession ? '有附身' : '无附身';
      html += `<div class="review-item">`;
      html += `第${roundNum}轮 ${icon} ${text}`;
      html += `</div>`;
    });
    signalContainer.innerHTML = html;
  } else if (signalContainer) {
    signalContainer.innerHTML = '<div class="review-item">无信号记录</div>';
  }
}

// ═══════════════════════════════════════
//  计时器
// ═══════════════════════════════════════
function startTimer(seconds) {
  timerValue = seconds;
  updateTimerDisplay();
  if (timerInterval) clearInterval(timerInterval);
  timerInterval = setInterval(() => {
    timerValue--;
    updateTimerDisplay();
    if (timerValue <= 0) {
      clearInterval(timerInterval);
    }
  }, 1000);
}

function updateTimerDisplay() {
  const min = Math.floor(timerValue / 60);
  const sec = timerValue % 60;
  const timer = document.getElementById('timer');
  if (timer) {
    timer.textContent = `⏱ ${min}:${sec.toString().padStart(2, '0')}`;
  }
}

// ═══════════════════════════════════════
//  工具函数
// ═══════════════════════════════════════
function showScreen(screenId) {
  document.querySelectorAll('.screen').forEach(s => s.classList.add('hidden'));
  const screen = document.getElementById(screenId);
  if (screen) screen.classList.remove('hidden');
}

function hideAllActions() {
  const proposeActions = document.getElementById('proposeActions');
  const voteActions = document.getElementById('voteActions');
  const missionActions = document.getElementById('missionActions');
  const sabotageBtn = document.getElementById('sabotageBtn');
  
  if (proposeActions) proposeActions.classList.add('hidden');
  if (voteActions) voteActions.classList.add('hidden');
  if (missionActions) missionActions.classList.add('hidden');
  if (sabotageBtn) sabotageBtn.classList.add('hidden');
}

function showToast(msg, duration = 3000) {
  const toast = document.getElementById('toast');
  if (toast) {
    toast.textContent = msg;
    toast.classList.remove('hidden');
    setTimeout(() => toast.classList.add('hidden'), duration);
  }
}

function escapeHtml(str) {
  const div = document.createElement('div');
  div.textContent = str;
  return div.innerHTML;
}

// ═══════════════════════════════════════
//  方向 C：谜题 UI 函数
// ═══════════════════════════════════════

/**
 * 隐藏所有谜题相关 UI
 */
function hidePuzzleUI() {
  const puzzleArea = document.getElementById('puzzleArea');
  const revealArea = document.getElementById('revealArea');
  const spectateArea = document.getElementById('spectateArea');
  
  if (puzzleArea) puzzleArea.classList.add('hidden');
  if (revealArea) revealArea.classList.add('hidden');
  if (spectateArea) spectateArea.classList.add('hidden');
}

/**
 * 显示谜题（小队成员）
 */
function showPuzzle(puzzle, canSabotage) {
  hidePuzzleUI();
  const area = document.getElementById('puzzleArea');
  if (area) area.classList.remove('hidden');

  const puzzleTitle = document.getElementById('puzzleTitle');
  const puzzleScenario = document.getElementById('puzzleScenario');
  if (puzzleTitle) puzzleTitle.textContent = puzzle.title;
  if (puzzleScenario) puzzleScenario.textContent = puzzle.scenario;

  // 填充选项文字
  const optA = document.getElementById('puzzleOptA');
  const optB = document.getElementById('puzzleOptB');
  const optC = document.getElementById('puzzleOptC');

  if (optA) optA.textContent = `A: ${puzzle.options.A}`;
  if (optB) optB.textContent = `B: ${puzzle.options.B}`;
  if (optC) optC.textContent = `C: ${puzzle.options.C}`;

  // 重置状态
  [optA, optB, optC].forEach(btn => {
    if (btn) {
      btn.classList.remove('selected', 'correct', 'wrong');
      btn.disabled = true; // 讨论阶段不可点击
    }
  });

  puzzleAnswer = null;
}

/**
 * 启用谜题投票
 */
function enablePuzzleVote() {
  const opts = document.querySelectorAll('.puzzle-opt');
  opts.forEach(btn => {
    btn.disabled = false;
    btn.addEventListener('click', handlePuzzleOptionClick);
  });
}

function handlePuzzleOptionClick(e) {
  if (puzzleSubPhase !== 'vote') return;
  const btn = e.target.closest('.puzzle-opt');
  if (!btn) return;

  // 高亮选中
  document.querySelectorAll('.puzzle-opt').forEach(b => b.classList.remove('selected'));
  btn.classList.add('selected');
  puzzleAnswer = btn.dataset.answer;

  // 自动提交
  socket.emit('missionVote', { roomId: currentRoomId, answer: puzzleAnswer });
  showToast(`已选择 ${puzzleAnswer}`, 1500);

  // 禁用所有选项
  document.querySelectorAll('.puzzle-opt').forEach(b => b.disabled = true);
}

/**
 * 启用谜题讨论聊天
 */
let puzzleMessagesLeft = 0;
let puzzleMaxMessages = 2;

function enablePuzzleChat(maxMessages, maxChars) {
  puzzleMaxMessages = maxMessages || 2;
  puzzleMessagesLeft = puzzleMaxMessages;

  const input = document.getElementById('chatInput');
  const btn = document.getElementById('chatSendBtn');
  if (input) {
    input.disabled = false;
    input.maxLength = maxChars || 30;
    input.placeholder = `谜题讨论（${puzzleMessagesLeft}/${puzzleMaxMessages}）...`;
    input.focus();
  }
  if (btn) btn.disabled = false;
}

/**
 * 显示旁观界面（非小队成员）
 */
function showSpectate(teamNames, puzzleTitle) {
  hidePuzzleUI();
  const area = document.getElementById('spectateArea');
  if (area) area.classList.remove('hidden');

  const spectateTeam = document.getElementById('spectateTeam');
  if (spectateTeam) spectateTeam.textContent = `小队: ${teamNames.join('、')}`;
}

/**
 * 显示揭晓结果
 */
function showReveal(correctAnswer, correctLabel, explanation, votes, justifications, success, hadPossession) {
  hidePuzzleUI();
  const area = document.getElementById('revealArea');
  if (area) area.classList.remove('hidden');

  // 结果标题
  const resultDiv = document.getElementById('revealResult');
  if (resultDiv) {
    const statusClass = success ? 'reveal-success' : 'reveal-fail';
    const statusIcon = success ? '✅' : '❌';
    resultDiv.innerHTML = `
      <h3 class="${statusClass}">${statusIcon} 任务${success ? '成功' : '失败'}</h3>
      <p class="explanation">正确答案: ${correctAnswer} (${escapeHtml(correctLabel)}) — ${escapeHtml(explanation)}</p>
    `;
  }

  // 每人投票详情
  const detailDiv = document.getElementById('revealDetail');
  if (detailDiv) {
    let html = '';
    for (const [playerId, vote] of Object.entries(votes)) {
      const just = justifications[playerId];
      const isCorrect = vote.answer === correctAnswer;
      const ansClass = isCorrect ? 'correct-ans' : 'wrong-ans';
      const ansIcon = isCorrect ? '✓' : '✗';
      html += `
        <div class="reveal-vote-item">
          <span class="vote-name">${escapeHtml(vote.name)}</span>
          <span class="vote-answer ${ansClass}">${vote.answer} ${ansIcon}</span>
          <span class="vote-justification">${just ? escapeHtml(just.lastMessage) : ''}</span>
        </div>
      `;
    }
    detailDiv.innerHTML = html;
  }

  // 高亮谜题区选项（如果还显示的话）
  document.querySelectorAll('.puzzle-opt').forEach(btn => {
    btn.disabled = true;
    const ans = btn.dataset.answer;
    if (ans === correctAnswer) btn.classList.add('correct');
    else if (votes[myId] && votes[myId].answer === ans) btn.classList.add('wrong');
  });
}

// ═══════════════════════════════════════
//  调试模式功能
// ═══════════════════════════════════════

/**
 * 初始化调试模式
 */
function initDebugMode() {
  // 检查是否是单人调试模式
  if (gameMode === 'solo') {
    debugMode = true;
    showDebugPanel();
    addDebugLog('调试模式已启用', 'success');
    
    // 监听服务器事件以获取AI状态
    setupDebugSocketListeners();
  }
}

/**
 * 显示调试面板
 */
function showDebugPanel() {
  const panel = document.getElementById('debugPanel');
  if (panel) {
    panel.classList.remove('hidden');
    addDebugLog('调试面板已显示', 'info');
  }
}

/**
 * 隐藏调试面板
 */
function hideDebugPanel() {
  const panel = document.getElementById('debugPanel');
  if (panel) {
    panel.classList.add('hidden');
  }
}

/**
 * 切换调试面板展开/折叠
 */
function toggleDebugPanel() {
  const panel = document.getElementById('debugPanel');
  const toggle = document.querySelector('.debug-toggle');
  if (panel && toggle) {
    panel.classList.toggle('collapsed');
    toggle.textContent = panel.classList.contains('collapsed') ? '▲' : '▼';
  }
}

/**
 * 添加调试日志
 */
function addDebugLog(message, type = 'info') {
  const timestamp = new Date().toLocaleTimeString();
  const entry = { timestamp, message, type };
  debugLogEntries.push(entry);
  
  // 限制日志条目数量
  if (debugLogEntries.length > 50) {
    debugLogEntries.shift();
  }
  
  // 更新日志显示
  updateDebugLogDisplay();
}

/**
 * 更新调试日志显示
 */
function updateDebugLogDisplay() {
  const logDiv = document.getElementById('debugLog');
  if (!logDiv) return;
  
  let html = '';
  debugLogEntries.slice(-20).forEach(entry => {
    html += `<div class="debug-log-entry ${entry.type}">
      <span class="time">[${entry.timestamp}]</span> ${escapeHtml(entry.message)}
    </div>`;
  });
  
  logDiv.innerHTML = html;
  logDiv.scrollTop = logDiv.scrollHeight;
}

/**
 * 切换暂停/继续
 */
function togglePause() {
  isPaused = !isPaused;
  const btn = document.getElementById('pauseBtn');
  
  if (isPaused) {
    btn.textContent = '▶️ 继续';
    btn.classList.add('paused');
    addDebugLog('游戏已暂停', 'warning');
    
    // 发送暂停事件到服务器
    if (socket) {
      socket.emit('debugPause', { roomId: currentRoomId, paused: true });
    }
  } else {
    btn.textContent = '⏸️ 暂停';
    btn.classList.remove('paused');
    addDebugLog('游戏已继续', 'success');
    
    // 发送继续事件到服务器
    if (socket) {
      socket.emit('debugPause', { roomId: currentRoomId, paused: false });
    }
  }
}

/**
 * 跳过当前阶段
 */
function skipPhase() {
  addDebugLog('跳过当前阶段', 'info');
  
  if (socket) {
    socket.emit('debugSkipPhase', { roomId: currentRoomId });
  }
}

/**
 * 跳转到指定阶段
 */
function jumpToPhase(phase) {
  addDebugLog(`跳转到阶段: ${phase}`, 'info');
  
  if (socket) {
    socket.emit('debugJumpToPhase', { roomId: currentRoomId, targetPhase: phase });
  }
}

/**
 * 更新AI状态显示
 */
function updateAIStatusDisplay() {
  const container = document.getElementById('aiStatusList');
  if (!container) return;
  
  let html = '';
  
  // 从游戏状态中获取AI玩家信息
  if (gameRoom && gameRoom.players) {
    const aiPlayers = gameRoom.players.filter(p => p.isAI);
    
    aiPlayers.forEach(ai => {
      const role = ai.role || '未知';
      const voteIntent = ai.voteIntent || '未知';
      const isPossessed = ai.isPossessed ? '⚠️ 附身' : '';
      
      html += `
        <div class="ai-status-item">
          <span class="ai-name">${escapeHtml(ai.name)}</span>
          <span class="ai-role">${role}</span>
          <span class="ai-vote">投票: ${voteIntent}</span>
          ${isPossessed ? `<span class="ai-possessed">${isPossessed}</span>` : ''}
        </div>
      `;
    });
  }
  
  if (!html) {
    html = '<div class="ai-status-item">暂无AI玩家信息</div>';
  }
  
  container.innerHTML = html;
}

/**
 * 更新游戏状态显示
 */
function updateGameStatusDisplay() {
  const container = document.getElementById('gameStatusInfo');
  if (!container) return;
  
  let html = '';
  
  if (gameRoom) {
    html = `
      <p><span class="label">房间ID:</span> <span class="value">${escapeHtml(gameRoom.roomId || '未知')}</span></p>
      <p><span class="label">游戏状态:</span> <span class="value">${gameRoom.status || '未知'}</span></p>
      <p><span class="label">当前阶段:</span> <span class="value">${currentPhase || '未知'}</span></p>
      <p><span class="label">当前轮次:</span> <span class="value">${gameRoom.currentRound || 0}/${gameRoom.maxRounds || 5}</span></p>
      <p><span class="label">任务成功:</span> <span class="value">${gameRoom.missionSuccesses || 0}</span></p>
      <p><span class="label">任务失败:</span> <span class="value">${gameRoom.missionFailures || 0}</span></p>
      <p><span class="label">当前队长:</span> <span class="value">${gameRoom.currentLeaderName || '未知'}</span></p>
      <p><span class="label">已提名:</span> <span class="value">${gameRoom.proposedTeam ? gameRoom.proposedTeam.length : 0}人</span></p>
      <p><span class="label">暂停状态:</span> <span class="value">${isPaused ? '已暂停' : '运行中'}</span></p>
    `;
  } else {
    html = '<p>游戏状态加载中...</p>';
  }
  
  container.innerHTML = html;
}

/**
 * 设置调试模式Socket监听器
 */
function setupDebugSocketListeners() {
  if (!socket) return;
  
  // 监听游戏状态更新
  socket.on('gameStateUpdate', (state) => {
    gameRoom = state;
    updateAIStatusDisplay();
    updateGameStatusDisplay();
    addDebugLog('游戏状态已更新', 'info');
  });
  
  // 监听AI状态更新
  socket.on('aiStatusUpdate', (aiData) => {
    if (gameRoom && gameRoom.players) {
      // 更新AI玩家信息
      aiData.forEach(aiInfo => {
        const aiPlayer = gameRoom.players.find(p => p.id === aiInfo.id);
        if (aiPlayer) {
          aiPlayer.role = aiInfo.role;
          aiPlayer.voteIntent = aiInfo.voteIntent;
          aiPlayer.isPossessed = aiInfo.isPossessed;
        }
      });
      updateAIStatusDisplay();
    }
  });
  
  // 监听调试响应
  socket.on('debugResponse', (data) => {
    addDebugLog(`调试响应: ${data.message}`, data.success ? 'success' : 'error');
  });
}

/**
 * 更新调试面板
 */
function updateDebugPanel() {
  if (!debugMode) return;
  
  updateAIStatusDisplay();
  updateGameStatusDisplay();
}

/**
 * 定期更新调试面板
 */
setInterval(() => {
  if (debugMode && !isPaused) {
    updateDebugPanel();
  }
}, 2000);

// ═══════════════════════════════════════
//  UI优化相关函数
// ═══════════════════════════════════════

/**
 * 分享房间功能
 */
function shareRoom() {
  const roomId = document.getElementById('waitingRoomId').textContent;
  const shareText = `来玩「谁是AI」吧！房间号：${roomId}`;
  
  if (navigator.share) {
    navigator.share({
      title: '谁是AI - 阿瓦隆式社交推理游戏',
      text: shareText,
      url: window.location.href
    }).catch(console.error);
  } else {
    // 复制到剪贴板
    navigator.clipboard.writeText(shareText).then(() => {
      showToast('✅ 分享内容已复制到剪贴板');
    }).catch(() => {
      showToast('❌ 复制失败，请手动分享');
    });
  }
}

/**
 * 分享游戏结果
 */
function shareResult() {
  const resultTitle = document.getElementById('resultTitle').textContent;
  const shareText = `我在「谁是AI」中${resultTitle}！来挑战我吧！`;
  
  if (navigator.share) {
    navigator.share({
      title: '谁是AI - 游戏结果',
      text: shareText,
      url: window.location.href
    }).catch(console.error);
  } else {
    navigator.clipboard.writeText(shareText).then(() => {
      showToast('✅ 游戏结果已复制到剪贴板');
    }).catch(() => {
      showToast('❌ 复制失败，请手动分享');
    });
  }
}

/**
 * 提交游戏反馈
 */
function submitFeedback(type) {
  const feedbackMessages = {
    'great': '🎉 感谢你的积极反馈！我们会继续努力！',
    'good': '😊 感谢你的反馈！我们会继续改进！',
    'bad': '😔 感谢你的反馈，我们会努力改进！'
  };
  
  showToast(feedbackMessages[type] || '感谢你的反馈！');
  
  // 这里可以添加向服务器发送反馈的逻辑
  console.log('用户反馈:', type);
}

/**
 * 更新等待界面进度条
 */
function updateWaitingProgress(count, total) {
  const progressFill = document.getElementById('progressFill');
  if (progressFill) {
    const percentage = (count / total) * 100;
    progressFill.style.width = `${percentage}%`;
  }
}

/**
 * 更新玩家槽位状态
 */
function updatePlayerSlot(index, name, isYou = false) {
  const slot = document.getElementById(`slot${index}`);
  if (!slot) return;
  
  const slotName = slot.querySelector('.slot-name');
  const slotStatus = slot.querySelector('.slot-status');
  
  if (name) {
    slot.classList.add('filled');
    slotName.textContent = name;
    slotStatus.textContent = '已加入';
    
    if (isYou) {
      slot.classList.add('you');
      slotStatus.textContent = '就是你！';
    }
    
    // 添加加入动画
    slot.classList.add('player-join-animation');
    setTimeout(() => slot.classList.remove('player-join-animation'), 500);
  } else {
    slot.classList.remove('filled', 'you');
    slotName.textContent = '等待中...';
    slotStatus.textContent = '准备中';
  }
}

/**
 * 轮播等待提示
 */
function startTipCarousel() {
  const tips = document.querySelectorAll('.tip-carousel .tip');
  let currentTip = 0;
  
  setInterval(() => {
    tips.forEach(tip => tip.classList.remove('active'));
    tips[currentTip].classList.add('active');
    currentTip = (currentTip + 1) % tips.length;
  }, 5000);
}

/**
 * 更新游戏结束界面统计
 */
function updateGameStats(stats) {
  if (!stats) return;
  
  const elements = {
    'gameDuration': stats.duration || '--',
    'voteRounds': stats.voteRounds || '--',
    'chatMessages': stats.chatMessages || '--',
    'possessionCount': stats.possessionCount || '--'
  };
  
  Object.entries(elements).forEach(([id, value]) => {
    const element = document.getElementById(id);
    if (element) element.textContent = value;
  });
}

/**
 * 更新MVP评选
 */
function updateMVPs(mvps) {
  if (!mvps) return;
  
  const elements = {
    'mvpReasoning': mvps.reasoning || '--',
    'mvpActing': mvps.acting || '--',
    'mvpChatter': mvps.chatter || '--'
  };
  
  Object.entries(elements).forEach(([id, value]) => {
    const element = document.getElementById(id);
    if (element) element.textContent = value;
  });
}

/**
 * 添加关键事件到回顾
 */
function addKeyEvent(event) {
  const reviewKeyEvents = document.getElementById('reviewKeyEvents');
  if (!reviewKeyEvents) return;
  
  const eventElement = document.createElement('div');
  eventElement.className = 'key-event';
  eventElement.innerHTML = `
    <span class="event-time">${event.time}</span>
    <span class="event-desc">${event.description}</span>
  `;
  
  reviewKeyEvents.appendChild(eventElement);
}

/**
 * 播放成功动画
 */
function playSuccessAnimation(element) {
  if (!element) return;
  element.classList.add('success-animation');
  setTimeout(() => element.classList.remove('success-animation'), 500);
}

/**
 * 播放失败动画
 */
function playFailAnimation(element) {
  if (!element) return;
  element.classList.add('fail-animation');
  setTimeout(() => element.classList.remove('fail-animation'), 500);
}

/**
 * 播放阶段转换动画
 */
function playPhaseTransition() {
  const topBar = document.querySelector('.top-bar');
  if (topBar) {
    topBar.classList.add('phase-transition');
    setTimeout(() => topBar.classList.remove('phase-transition'), 500);
  }
}

/**
 * 增强计时器显示
 */
function enhanceTimer(seconds) {
  const timerElement = document.getElementById('timer');
  if (!timerElement) return;
  
  if (seconds <= 10) {
    timerElement.classList.add('timer-urgent');
  } else {
    timerElement.classList.remove('timer-urgent');
  }
}

/**
 * 添加聊天消息动画
 */
function addChatMessageWithAnimation(message, isMine = false) {
  const chatMessages = document.getElementById('chatMessages');
  if (!chatMessages) return;
  
  const messageElement = document.createElement('div');
  messageElement.className = `chat-msg ${isMine ? 'mine' : ''}`;
  messageElement.innerHTML = `
    <span class="msg-name">${message.name}</span>
    <span class="msg-text">${message.text}</span>
  `;
  
  messageElement.classList.add('chat-message-animation');
  chatMessages.appendChild(messageElement);
  chatMessages.scrollTop = chatMessages.scrollHeight;
}

/**
 * 复制房间号成功反馈
 */
function copyRoomIdSuccess() {
  const copyBtn = document.getElementById('copyBtn');
  if (copyBtn) {
    copyBtn.classList.add('copy-success');
    setTimeout(() => copyBtn.classList.remove('copy-success'), 500);
  }
}

/**
 * 更新投票历史显示
 */
function updateVoteHistoryWithAnimation(voteRecord) {
  const voteHistoryList = document.getElementById('voteHistoryList');
  if (!voteHistoryList) return;
  
  const recordElement = document.createElement('div');
  recordElement.className = `vote-record ${voteRecord.approved ? 'approved' : 'rejected'}`;
  recordElement.innerHTML = `
    <div class="vote-record-header">
      <span class="vote-round">第${voteRecord.round}轮</span>
      <span class="vote-status">${voteRecord.approved ? '✅' : '❌'}</span>
    </div>
    <div class="vote-team">队伍: ${voteRecord.team}</div>
    <div class="vote-details">
      ${voteRecord.votes.map(v => `<span class="vote-player">${v.player}: ${v.vote ? '✅' : '❌'}</span>`).join('')}
    </div>
  `;
  
  recordElement.classList.add('vote-result-animation');
  voteHistoryList.appendChild(recordElement);
}

/**
 * 更新信号员历史显示
 */
function updateSignalHistoryWithAnimation(signalRecord) {
  const signalHistoryList = document.getElementById('signalHistoryList');
  if (!signalHistoryList) return;
  
  const recordElement = document.createElement('div');
  recordElement.className = `signal-record ${signalRecord.possessed ? 'possessed' : 'normal'}`;
  recordElement.innerHTML = `
    <span class="signal-round">第${signalRecord.round}轮</span>
    <span class="signal-status">${signalRecord.possessed ? '⚠️ 检测到AI附身' : '✅ 未检测到附身'}</span>
  `;
  
  signalHistoryList.appendChild(recordElement);
}

/**
 * 显示有趣事实
 */
function showFunFact() {
  const funFacts = [
    '在这个游戏中，AI玩家会模仿人类行为，让你难以分辨真假！',
    '信号员的能力可以检测AI附身，但附身者可能不会被检测到！',
    '好人阵营需要完成3次任务才能获胜，而坏人需要破坏3次！',
    '每轮游戏有50%的概率出现AI附身干扰！',
    '游戏中的AI玩家会根据你的行为调整策略！'
  ];
  
  const funFactElement = document.getElementById('funFact');
  if (funFactElement) {
    funFactElement.textContent = funFacts[Math.floor(Math.random() * funFacts.length)];
  }
}

/**
 * 初始化UI优化
 */
function initUIOptimizations() {
  // 启动提示轮播
  startTipCarousel();
  
  // 定期显示有趣事实
  setInterval(showFunFact, 30000);
  
  // 添加键盘快捷键支持
  document.addEventListener('keydown', (e) => {
    // Ctrl+C 复制房间号
    if (e.ctrlKey && e.key === 'c' && document.activeElement.tagName !== 'INPUT') {
      copyRoomId();
    }
    
    // Enter 开始游戏
    if (e.key === 'Enter' && document.getElementById('loginScreen').classList.contains('hidden') === false) {
      joinGame();
    }
  });
  
  // 添加按钮点击反馈
  document.querySelectorAll('button').forEach(btn => {
    btn.addEventListener('click', function() {
      this.classList.add('btn-click-feedback');
      setTimeout(() => this.classList.remove('btn-click-feedback'), 200);
    });
  });
}

// 在页面加载完成后初始化UI优化
document.addEventListener('DOMContentLoaded', initUIOptimizations);

/**
 * 计算游戏时长
 */
function calculateGameDuration() {
  // 这里应该记录游戏开始时间，然后计算时长
  // 暂时返回一个模拟值
  return '10分30秒';
}

/**
 * 找到最佳推理MVP
 */
function findMVPReasoning(roles) {
  // 这里应该根据投票历史和推理准确性来判断
  // 暂时返回第一个玩家
  const players = Object.values(roles);
  return players.length > 0 ? players[0].name : '--';
}

/**
 * 找到最佳演技MVP
 */
function findMVPActing(roles) {
  // 这里应该根据AI附身和伪装能力来判断
  // 暂时返回随机玩家
  const players = Object.values(roles);
  return players.length > 1 ? players[1].name : '--';
}

/**
 * 找到话痨王MVP
 */
function findMVPChatter() {
  // 这里应该根据聊天消息数量来判断
  // 暂时返回当前玩家
  return myName || '--';
}

/**
 * 添加关键游戏事件
 */
function addKeyGameEvents() {
  // 清空现有事件
  const reviewKeyEvents = document.getElementById('reviewKeyEvents');
  if (reviewKeyEvents) {
    reviewKeyEvents.innerHTML = '';
  }
  
  // 添加一些关键事件示例
  addKeyEvent({
    time: '00:05',
    description: '游戏开始，角色分配完成'
  });
  
  addKeyEvent({
    time: '02:30',
    description: '第一次投票通过'
  });
  
  addKeyEvent({
    time: '05:15',
    description: 'AI附身被检测到'
  });
  
  addKeyEvent({
    time: '08:45',
    description: '关键任务执行'
  });
}
