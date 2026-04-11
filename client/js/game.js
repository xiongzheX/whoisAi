// 游戏状态
let socket;
let currentRoom;
let currentPlayer;
let currentWord;
let gamePhase = 'waiting';
let remainingQuestions = 3;
let selectedTarget = null;
const storedGameMode = typeof localStorage !== 'undefined' ? localStorage.getItem('gameMode') : null;
let gameMode = storedGameMode || 'test';
let detectiveSkills = { observe: 3, question: 3, listen: 1 }; // 侦探技能次数
let currentTask = null; // 当前侦探任务

// 像素小人身体颜色列表
const bodyColors = [
    '#4080ff', // 蓝
    '#ff4080', // 粉
    '#40c040', // 绿
    '#ffa040', // 橙
    '#a040ff', // 紫
    '#40c0c0', // 青
];

// 像素小人头部颜色（肤色）
const skinColors = [
    '#ffcc88',
    '#ffaa66',
    '#ffe0b0',
    '#cc8844',
    '#ffddaa',
    '#ddaa77',
];

let playerPositions = [];

const personaStyles = {
    analytical: { className: 'persona-analytical', chip: '分析型' },
    cautious: { className: 'persona-cautious', chip: '谨慎型' },
    confrontational: { className: 'persona-confrontational', chip: '对抗型' },
    quirky: { className: 'persona-quirky', chip: '跳脱型' }
};

function getPersonaStyle(persona) {
    return personaStyles[persona] || personaStyles.cautious;
}

function updateModeToggleUI() {
    document.querySelectorAll('.mode-toggle-btn').forEach(btn => {
        const isActive = btn.dataset.mode === gameMode;
        btn.classList.toggle('active', isActive);
    });
    document.querySelectorAll('.mode-status').forEach(status => {
        status.textContent = gameMode === 'solo' ? '单人调试' : '测试模式';
    });
}

function setGameMode(mode) {
    gameMode = mode === 'solo' ? 'solo' : 'test';
    if (typeof localStorage !== 'undefined') {
        localStorage.setItem('gameMode', gameMode);
    }
    updateModeToggleUI();
}

function getLobbyHint(mode) {
    return mode === 'solo' ? '1 人即可开始' : '需要 6 名玩家开始';
}

function getPhaseCopy(phase, roundNumber, summary) {
    if (phase === 'roundIntro') {
        return summary?.message || `第 ${roundNumber || ''} 轮开始`;
    }
    if (phase === 'action') {
        return `第 ${roundNumber || ''} 轮：行动`;
    }
    if (phase === 'questioning1' || phase === 'questioning2') {
        return `第 ${roundNumber || ''} 轮：提问`;
    }
    if (phase === 'defending') {
        return '辩解阶段';
    }
    if (phase === 'summary') {
        return summary?.message || '本轮总结';
    }
    if (phase === 'finished') {
        return '对局结束';
    }
    if (phase === 'clueCollecting') {
        return '线索收集';
    }
    return phase ? phase.toUpperCase() : '';
}

function getPhaseHint(phase, roundNumber, summary) {
    const hints = {
        waiting: '先加入房间，再开始新一局。',
        roundIntro: summary?.message || `第 ${roundNumber || ''} 轮准备中，先记住线索。`,
        describing: '先描述本轮词语，让其他人观察你的措辞。',
        action: '先点头像，再使用侦探技能或提交描述。',
        clueCollecting: '这是本轮最关键的观察窗口，别浪费技能。',
        questioning1: '抓住可疑发言，直接发起提问。',
        questioning2: '第二轮提问，继续逼近真正的 AI。',
        voting1: '先观察票型，再投给最可疑的人。',
        voting2: '最终投票阶段，错误一票可能直接改变胜负。',
        defending: 'PK 阶段，留意辩解内容和票型变化。',
        summary: '看完本轮总结，准备下一轮判断。',
        finished: '查看胜负和关键线索，准备复盘。'
    };
    return hints[phase] || '跟着阶段提示操作，局内信息会逐步展开。';
}

function getMessageTone(type, content) {
    if (type !== 'system') return type;
    const text = (content || '').toLowerCase();
    if (text.includes('投票结果') || text.includes('本轮总结') || text.includes('最可疑玩家') || text.includes('对局结束') || text.includes('胜利') || text.includes('辩论') || text.includes('票型已锁定')) {
        return 'summary';
    }
    if (text.includes('click a player') || text.includes('please click') || text.includes('use your detective skills') || text.includes('choose one') || text.includes('select') || text.includes('点击') || text.includes('侦探技能')) {
        return 'action';
    }
    if (text.includes('not found') || text.includes('timeout') || text.includes('error') || text.includes('refused') || text.includes('未找到') || text.includes('超时') || text.includes('错误') || text.includes('拒绝')) {
        return 'warning';
    }
    if (text.includes('game start') || text.includes('submitted') || text.includes('ready') || text.includes('success') || text.includes('开始') || text.includes('提交') || text.includes('成功')) {
        return 'success';
    }
    return 'system';
}


function getSkillAvailability(skillKey) {
    if (!currentPlayer) return false;
    if (gamePhase !== 'clueCollecting') return false;
    return (detectiveSkills[skillKey] || 0) > 0;
}

function getSkillLabel(skillKey) {
    const labels = {
        observe: '观察',
        question: '质问',
        listen: '偷听',
    };
    return labels[skillKey] || skillKey;
}

function getSkillHelpText(skillKey) {
    const help = {
        observe: '点击头像后查看该玩家的发言内容。',
        question: '点击头像后向该玩家提问。',
        listen: '点击头像后偷听该玩家的状态。',
    };
    return help[skillKey] || '';
}

function renderSkillPanel() {
    const panel = document.getElementById('skillPanel');
    if (!panel) return;

    const skills = [
        { key: 'observe' },
        { key: 'question' },
        { key: 'listen' },
    ];

    panel.innerHTML = [
        '<div class="skill-panel-title">我的技能</div>',
        '<div class="skill-panel-subtitle">先点击头像，再使用技能</div>',
        '<div class="skill-panel-tip"><span class="tip-arrow">☞</span><span>点击圆桌头像后，再点右侧技能按钮</span></div>',
        '<div class="skill-list">',
        skills.map(({ key }) => {
            const count = detectiveSkills[key] ?? 0;
            const available = getSkillAvailability(key);
            const classes = ['skill-item'];
            if (available) classes.push('is-ready');
            else classes.push('is-disabled');
            return `
                <button type="button" class="${classes.join(' ')}" data-skill="${key}" ${available ? '' : 'disabled'} onclick="useSkill${key.charAt(0).toUpperCase() + key.slice(1)}()">
                    <span class="skill-name">${getSkillLabel(key)}</span>
                    <span class="skill-count">剩余 ${count} 次</span>
                    <span class="skill-help">${getSkillHelpText(key)}</span>
                </button>
            `;
        }).join(''),
        '</div>',
    ].join('');
}

function updateSkillPanel() {
    renderSkillPanel();
}

function setupModeToggle() {
    updateModeToggleUI();
    renderSkillPanel();
}

function init() {
    console.log('=== Initializing game ===');

    // 先渲染一次侦探小人（在socket连接之前）
    renderDetective();
    setupModeToggle();

    const locationInfo = window.location || {};
    const protocol = locationInfo.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = locationInfo.hostname || 'localhost';
    const port = locationInfo.port || (protocol === 'wss:' ? 443 : 3000);

    socket = io(`${protocol}//${host}:${port}`);
    bindLateSocketEvents();

    socket.on('connect', () => {
        console.log('已连接服务器');
        // 在登录页面也显示侦探动画
        renderDetective();
    });

    socket.on('playerJoined', ({ players }) => {
        console.log('=== playerJoined event received ===');
        console.log('Players:', players);
        updatePlayerList(players);
        const myPlayer = players.find(p => p.name === document.getElementById('playerName')?.value);
        if (myPlayer) {
            console.log('Found my player:', myPlayer);
            updateUserInfo(myPlayer);
        } else {
            console.log('My player not found in list');
        }
    });

    socket.on('gameStarted', ({ word, players, wordDifficulty }) => {
        console.log('=== gameStarted event received ===');
        console.log('Word:', word);
        console.log('Players:', players);
        console.log('Word difficulty:', wordDifficulty);

        currentWord = word;
        gamePhase = 'describing';
        playerPositions = players;
        document.getElementById('wordDisplay').textContent = word;
        updatePhaseDisplay('describing', 1, null);

        console.log('Switching to gameScreen...');
        showScreen('gameScreen');
        console.log('Showing input panel...');
        showInputPanel();
        console.log('Drawing pixel table...');
        drawPixelTable();
        console.log('Rendering detective...');
        renderDetective();
        console.log('Rendering pixel players...');
        renderPixelPlayers(players);

        const myPlayer = players.find(p => p.name === currentPlayer?.name);
        if (myPlayer) {
            console.log('Updating current player:', myPlayer);
            currentPlayer = myPlayer;
            updateUserInfo(myPlayer);
        }

        addMessage('system', `对局开始！词语难度：${wordDifficulty?.toUpperCase() || 'MEDIUM'}`);
        addMessage('system', '请用一句话描述这个词。');
    });

    socket.on('phaseChange', ({ phase, word, roundNumber, summary }) => {
        gamePhase = phase;
        if (word) currentWord = word;
        updatePhaseDisplay(phase, roundNumber, summary);

        hideAllPanels();

        if (phase === 'roundIntro') {
            addMessage('system', summary?.message || `第 ${roundNumber || ''} 轮开始`);
            addMessage('system', '行动阶段开始后，请尽快做出选择。');
        } else if (phase === 'action') {
            addMessage('system', `【第 ${roundNumber || ''} 轮行动】请选择一个动作。`);
        } else if (phase === 'summary') {
            addMessage('summary', summary?.message || '【本轮总结】');
        } else if (phase === 'finished') {
            addMessage('summary', '【对局结束】');
        } else if (phase === 'clueCollecting') {
            showCluePanel();
            addMessage('system', '【线索收集阶段】可以使用侦探技能。');
            if (duration) {
                startTimer(duration);
            }
        } else if (phase === 'questioning1' || phase === 'questioning2') {
            document.getElementById('questionButtons').classList.remove('hidden');
            addMessage('system', '【提问阶段】可以自由提问。');
        } else if (phase === 'defending') {
            document.getElementById('questionButtons').classList.add('hidden');
            document.getElementById('defendPanel').classList.remove('hidden');
            addMessage('system', '【辩解阶段】');
        }
    });

    socket.on('roundEvent', ({ type, label, message, targetName }) => {
        const eventLabel = label ? `【随机事件：${label}】` : '【随机事件】';
        showRoundEventBanner(type, label, message, targetName);
        addMessage('summary', `${eventLabel}${message}`);
    });

    socket.on('roundSummary', ({ roundNumber, message, mostSuspiciousName, actions }) => {
        addMessage('summary', `【第 ${roundNumber} 轮总结】${message}`);
        if (mostSuspiciousName) {
            addMessage('summary', `最可疑玩家：${mostSuspiciousName}`);
        }
        if (actions && actions.length) {
            actions.slice(0, 3).forEach(action => {
                addMessage('system', `- ${action.actorName}：${action.note}`);
            });
        }
    });

    socket.on('descriptionSubmitted', ({ playerId, description }) => {
        const player = playerPositions.find(p => p.id === playerId);
        if (player) {
            player.description = description;
            updatePlayerDescription(playerId, description);
            // 发言闪烁效果
            triggerAvatarAnim(playerId, 'speaking', 1500);
            addMessage('system', `${player.name} 已提交描述`);
        }
    });

    socket.on('questionAsked', ({ questionerId, targetId, question, remainingQuestions: rq }) => {
        const questioner = playerPositions.find(p => p.id === questionerId);
        const target = playerPositions.find(p => p.id === targetId);

        if (questioner && target) {
            addMessage('question', `${questioner.name} → ${target.name}：${question}`);
            // 提问者闪烁，被提问者闪烁
            triggerAvatarAnim(questionerId, 'speaking', 800);
            triggerAvatarAnim(targetId, 'questioned', 2000);
            if (currentPlayer && currentPlayer.id === targetId) {
                showAnswerOptions();
            }
        }

        if (rq) {
            document.getElementById('remainingQuestions').textContent = rq[currentPlayer?.id] ?? 0;
        }
    });

    socket.on('questionAnswered', ({ answererId, answer }) => {
        const answerer = playerPositions.find(p => p.id === answererId);
        if (answerer) {
            addMessage('answer', `${answerer.name}：${answer}`);
            // 回答闪烁
            triggerAvatarAnim(answererId, 'speaking', 1000);
        }
    });

    socket.on('answerRejected', ({ playerId }) => {
        const player = playerPositions.find(p => p.id === playerId);
        if (player) addMessage('system', `${player.name} 拒绝回答`);
    });

    socket.on('answerTimeout', ({ targetId }) => {
        const player = playerPositions.find(p => p.id === targetId);
        if (player) addMessage('system', `${player.name} 超时未回答`);
    });

    socket.on('voteResult', ({ voteResults, defendingPlayers }) => {
        displayVoteResults(voteResults);
        showDefendingPlayers(defendingPlayers);
        addMessage('summary', '票型已锁定，进入下一阶段。');
    });

    socket.on('playerDefended', ({ playerId, statement }) => {
        const player = playerPositions.find(p => p.id === playerId);
        if (player) addMessage('system', `${player.name} 辩解：${statement}`);
    });

    socket.on('gameFinished', ({ winner, voteResults, scores, summary, aiProfile }) => {
        showScreen('finishedScreen');
        const title = document.getElementById('resultTitle');
        const message = document.getElementById('resultMessage');
        const emoji = document.getElementById('resultEmoji');
        const rewardSummary = document.getElementById('rewardSummary');
        const aiProfileSummary = document.getElementById('aiProfileSummary');

        if (winner === 'humans') {
            title.textContent = '真人胜利！';
            title.style.color = '#40c040';
            message.textContent = '你们找到了 AI！';
            emoji.textContent = '▓▓▓';
        } else {
            title.textContent = 'AI 胜利！';
            title.style.color = '#ff4040';
            message.textContent = 'AI 成功存活到最后！';
            emoji.textContent = '░░░';
        }

        addMessage('summary', winner === 'humans' ? '真人方赢下了这一局。' : 'AI 方完成了伪装与生存。');

        if (rewardSummary && scores) {
            const me = currentPlayer?.id;
            rewardSummary.innerHTML = me ? [
                `<div><strong>Contribution</strong>: ${scores.contributionScore?.[me] ?? 0}</div>`,
                `<div><strong>Accuracy</strong>: ${scores.accuracyScore?.[me] ?? 0}</div>`,
                `<div><strong>Influence</strong>: ${scores.influenceScore?.[me] ?? 0}</div>`,
                summary?.message ? `<div>${summary.message}</div>` : ''
            ].filter(Boolean).join('') : '';
        }

        if (aiProfileSummary) {
            renderAiProfileSummary(aiProfile);
        }
    });

    socket.on('error', (message) => {
        addMessage('system', `错误：${message}`);
    });
}

// 切换测试模式
function toggleTestMode() {
    setGameMode('test');
}

function toggleSoloDebug() {
    setGameMode('solo');
}

// 加入游戏
function joinGame() {
    console.log('=== joinGame called ===');
    console.log('Socket status:', socket ? (socket.connected ? 'connected' : 'disconnected') : 'not initialized');

    const name = document.getElementById('playerName').value.trim();
    const roomId = document.getElementById('roomId').value.trim() || 'room1';

    console.log('Name:', name);
    console.log('Room ID:', roomId);

    if (!name) {
        alert('请输入昵称');
        return;
    }

    currentPlayer = { name, mode: gameMode };
    console.log('Current player:', currentPlayer);
    resetGameState();
    currentRoom = roomId;

    socket.emit('joinRoom', { roomId, name, mode: gameMode, testMode: gameMode === 'test' });
    if (gameMode === 'normal') {
        showScreen('waitingScreen');
        updateWaitingScreen('normal');
        updateUserInfo({ name, role: '等待中...' });
    } else {
        showScreen('gameScreen');
        document.getElementById('phaseText').textContent = '正在开始...';
        updateUserInfo({ name, role: '正在开始...' });
    }
}

// 更新左上角用户信息
function updateUserInfo(player) {
    const userInfo = document.getElementById('userInfo');
    const userName = document.getElementById('userName');
    const userRole = document.getElementById('userRole');
    const userAvatarEl = document.getElementById('userAvatar');

    if (userInfo) userInfo.classList.remove('hidden');
    if (userName) userName.textContent = player.name.toUpperCase();
    if (userRole) userRole.textContent = player.isAI ? '🤖 AI 玩家' : '👤 真人玩家';

    // 画像素小人头像（左上角）
    if (userAvatarEl) {
        const idx = Math.abs((player.name.charCodeAt(0) || 0)) % bodyColors.length;
        userAvatarEl.innerHTML = buildPixelPersonSVG(bodyColors[idx], skinColors[idx], 48, 48);
    }
}

// 隐藏所有操作面板
function hideAllPanels() {
    ['descriptionPanel', 'questionButtons', 'votePanel', 'defendPanel'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.classList.add('hidden');
    });
}

// 重置游戏状态
function resetGameState() {
    playerPositions = [];
    gamePhase = 'waiting';
    remainingQuestions = 3;
    currentWord = '';
    detectiveSkills = { observe: 3, question: 3, listen: 1 };
    selectedTarget = null;

    const list = document.getElementById('playerList');
    if (list) list.innerHTML = '';

    const messageList = document.getElementById('messageList');
    if (messageList) messageList.innerHTML = '';

    const avatarsEl = document.getElementById('playerAvatars');
    if (avatarsEl) avatarsEl.innerHTML = '';

    const roundEventBanner = document.getElementById('roundEventBanner');
    if (roundEventBanner) {
        roundEventBanner.textContent = '';
        roundEventBanner.classList.add('hidden');
    }

    const aiProfileSummary = document.getElementById('aiProfileSummary');
    if (aiProfileSummary) {
        aiProfileSummary.innerHTML = '';
    }

    hideAllPanels();
    renderSkillPanel();

    const remainingQuestionsEl = document.getElementById('remainingQuestions');
    if (remainingQuestionsEl) {
        remainingQuestionsEl.textContent = '3';
    }
}

// 更新玩家列表（等待界面）
function updatePlayerList(players) {
    playerPositions = players;
    const list = document.getElementById('playerList');
    list.innerHTML = players.map(p =>
        `<div class="player-badge">${p.name.toUpperCase()}</div>`
    ).join('');
}

// 显示界面
function showScreen(screenId) {
    document.querySelectorAll('.screen').forEach(s => s.classList.add('hidden'));
    document.getElementById(screenId).classList.remove('hidden');
}

// 提交描述
function submitDescription() {
    const description = document.getElementById('descriptionInput').value.trim();
    if (!description) { alert('请输入描述'); return; }
    socket.emit('submitDescription', { roomId: currentRoom, description });
    hideInputPanel();
}

function showInputPanel() {
    document.getElementById('descriptionPanel').classList.remove('hidden');
}

function hideInputPanel() {
    document.getElementById('descriptionPanel').classList.add('hidden');
}

// 更新阶段显示
function updatePhaseDisplay(phase, roundNumber, summary) {
    const phaseText = {
        'roundIntro':   '新一轮',
        'action':       '行动阶段',
        'summary':      '本轮总结',
        'finished':     '对局结束',
        'waiting':      '等待中',
        'describing':   '描述阶段',
        'questioning1': '提问阶段 1',
        'questioning2': '提问阶段 2',
        'voting1':      '投票阶段 1',
        'voting2':      '最终投票',
        'defending':    '辩解阶段'
    };
    const phaseTextEl = document.getElementById('phaseText');
    const phaseHintEl = document.getElementById('phaseHint');
    if (phaseTextEl) {
        phaseTextEl.textContent = phaseText[phase] || phase.toUpperCase();
    }
    if (phaseHintEl) {
        phaseHintEl.textContent = getPhaseHint(phase, roundNumber, summary);
    }
}

function showRoundEventBanner(type, label, message, targetName) {
    const banner = document.getElementById('roundEventBanner');
    if (!banner) return;

    const labelText = label ? `随机事件 · ${label}` : '随机事件';
    const targetText = targetName && !String(message || '').includes('目标：') ? `目标：${targetName}` : '';
    banner.textContent = [labelText, message, targetText].filter(Boolean).join('  ·  ');
    banner.classList.remove('hidden');
    banner.classList.remove('pulse-in');
    banner.classList.remove('is-glitch', 'is-tempo_shift', 'is-echo');
    if (type) {
        banner.classList.add(`is-${type}`);
    }
    void banner.offsetWidth;
    banner.classList.add('pulse-in');

    if (banner._hideTimer) {
        clearTimeout(banner._hideTimer);
    }
    banner._hideTimer = setTimeout(() => {
        banner.classList.add('hidden');
    }, 6000);
}

// ============================
// 渲染侦探动画
// ============================
function renderDetective() {
    console.log('=== renderDetective called ===');
    // 获取当前显示的页面
    const currentScreen = document.querySelector('.screen:not(.hidden)');
    console.log('Current screen:', currentScreen?.id);
    if (!currentScreen) {
        console.log('No visible screen found');
        return;
    }

    // 获取对应页面的侦探容器
    let detectiveContainer = null;
    if (currentScreen.id === 'loginScreen') {
        detectiveContainer = document.getElementById('loginDetective');
        console.log('Login detective container:', detectiveContainer);
    } else if (currentScreen.id === 'waitingScreen') {
        detectiveContainer = document.getElementById('waitingDetective');
    } else if (currentScreen.id === 'gameScreen') {
        detectiveContainer = document.querySelector('.discussion-room');
        // 游戏界面需要创建容器
        if (!detectiveContainer.querySelector('.detective-area')) {
            const area = document.createElement('div');
            area.className = 'detective-area';
            detectiveContainer.insertBefore(area, detectiveContainer.firstChild);
        }
        detectiveContainer = detectiveContainer.querySelector('.detective-area');
    }

    if (!detectiveContainer) {
        console.log('No detective container found');
        return;
    }

    console.log('Detective container found:', detectiveContainer);
    console.log('Container classes:', detectiveContainer.className);
    console.log('Container dimensions:', {
        width: detectiveContainer.offsetWidth,
        height: detectiveContainer.offsetHeight,
        display: getComputedStyle(detectiveContainer).display
    });

    // 如果已经有侦探内容就不再重复渲染
    if (detectiveContainer.querySelector('.detective')) {
        console.log('Detective already exists');
        return;
    }

    console.log('Rendering detective...');

    // 创建侦探元素
    const detectiveHint = document.createElement('div');
    detectiveHint.className = 'detective-hint';
    detectiveHint.textContent = '找出 AI！';

    const detectiveDiv = document.createElement('div');
    detectiveDiv.className = 'detective';

    // 侦探SVG（带放大镜的8-bit侦探）
    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('viewBox', '0 0 40 40');
    svg.setAttribute('xmlns', 'http://www.w3.org/2000/svg');

    // 身体
    const body = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
    body.setAttribute('x', '14');
    body.setAttribute('y', '20');
    body.setAttribute('width', '12');
    body.setAttribute('height', '16');
    body.setAttribute('fill', '#4a3728');

    // 头部
    const head = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
    head.setAttribute('x', '16');
    head.setAttribute('y', '10');
    head.setAttribute('width', '8');
    head.setAttribute('height', '10');
    head.setAttribute('fill', '#deb887');

    // 侦探帽
    const hat1 = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
    hat1.setAttribute('x', '12');
    hat1.setAttribute('y', '6');
    hat1.setAttribute('width', '16');
    hat1.setAttribute('height', '6');
    hat1.setAttribute('fill', '#2a2a2a');

    const hat2 = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
    hat2.setAttribute('x', '18');
    hat2.setAttribute('y', '2');
    hat2.setAttribute('width', '4');
    hat2.setAttribute('height', '4');
    hat2.setAttribute('fill', '#2a2a2a');

    // 放大镜手柄
    const handle1 = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
    handle1.setAttribute('x', '28');
    handle1.setAttribute('y', '22');
    handle1.setAttribute('width', '6');
    handle1.setAttribute('height', '3');
    handle1.setAttribute('fill', '#8b4513');

    const handle2 = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
    handle2.setAttribute('x', '32');
    handle2.setAttribute('y', '20');
    handle2.setAttribute('width', '4');
    handle2.setAttribute('height', '7');
    handle2.setAttribute('fill', '#a0522d');

    // 放大镜镜框
    const frame = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
    frame.setAttribute('x', '24');
    frame.setAttribute('y', '16');
    frame.setAttribute('width', '10');
    frame.setAttribute('height', '10');
    frame.setAttribute('fill', '#c0c0c0');
    frame.setAttribute('stroke', '#808080');
    frame.setAttribute('stroke-width', '2');

    // 放大镜镜片
    const lens = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
    lens.setAttribute('x', '26');
    lens.setAttribute('y', '18');
    lens.setAttribute('width', '6');
    lens.setAttribute('height', '6');
    lens.setAttribute('fill', '#87ceeb');
    lens.setAttribute('opacity', '0.6');

    // 眼睛
    const eye1 = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
    eye1.setAttribute('x', '18');
    eye1.setAttribute('y', '14');
    eye1.setAttribute('width', '3');
    eye1.setAttribute('height', '3');
    eye1.setAttribute('fill', '#000000');

    const eye2 = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
    eye2.setAttribute('x', '22');
    eye2.setAttribute('y', '14');
    eye2.setAttribute('width', '3');
    eye2.setAttribute('height', '3');
    eye2.setAttribute('fill', '#000000');

    // 组装SVG
    svg.appendChild(body);
    svg.appendChild(head);
    svg.appendChild(hat1);
    svg.appendChild(hat2);
    svg.appendChild(handle1);
    svg.appendChild(handle2);
    svg.appendChild(frame);
    svg.appendChild(lens);
    svg.appendChild(eye1);
    svg.appendChild(eye2);

    detectiveDiv.appendChild(svg);
    detectiveContainer.appendChild(detectiveHint);
    detectiveContainer.appendChild(detectiveDiv);

    console.log('Detective rendered successfully');
    console.log('Detective container HTML:', detectiveContainer.innerHTML);
    console.log('Detective element:', detectiveDiv);
    console.log('Detective element styles:', getComputedStyle(detectiveDiv));
}

// ============================
// 像素圆桌绘制
// ============================
function drawPixelTable() {
    const canvas = document.getElementById('tableCanvas');
    const ctx = canvas.getContext('2d');

    // 动态获取容器实际尺寸
    const room = document.querySelector('.discussion-room');
    const displayW = room.offsetWidth;
    const displayH = room.offsetHeight;

    // 设置 canvas 内部分辨率匹配显示尺寸
    canvas.width = displayW;
    canvas.height = displayH;

    const W = canvas.width;
    const H = canvas.height;
    const cx = W / 2, cy = H / 2;
    const baseRadius = Math.min(W, H) / 2 - 22;

    ctx.clearRect(0, 0, W, H);

    // 关闭抗锯齿，保持像素感
    ctx.imageSmoothingEnabled = false;

    // 桌子阴影（偏移矩形模拟像素投影）
    ctx.fillStyle = 'rgba(0,0,0,0.5)';
    ctx.beginPath();
    ctx.arc(cx + 6, cy + 6, baseRadius, 0, Math.PI * 2);
    ctx.fill();

    // 桌面主体（8-bit像素木纹）
    ctx.fillStyle = '#5a3010';
    ctx.beginPath();
    ctx.arc(cx, cy, baseRadius, 0, Math.PI * 2);
    ctx.fill();

    // 木纹条纹（8-bit感，更粗的像素块）
    ctx.save();
    ctx.beginPath();
    ctx.arc(cx, cy, baseRadius, 0, Math.PI * 2);
    ctx.clip();
    for (let i = -baseRadius; i <= baseRadius; i += 24) {
        ctx.fillStyle = i % 48 === 0 ? '#6a3818' : '#5a3010';
        ctx.fillRect(cx + i, cy - baseRadius, 12, baseRadius * 2);
    }
    ctx.restore();

    // 桌面内圈（较亮）
    const innerRadius = baseRadius * 0.86;
    ctx.fillStyle = '#7a4820';
    ctx.beginPath();
    ctx.arc(cx, cy, innerRadius, 0, Math.PI * 2);
    ctx.fill();

    // 木纹内圈（8-bit像素）
    ctx.save();
    ctx.beginPath();
    ctx.arc(cx, cy, innerRadius, 0, Math.PI * 2);
    ctx.clip();
    for (let i = -innerRadius; i <= innerRadius; i += 24) {
        ctx.fillStyle = i % 48 === 0 ? '#8a5030' : '#7a4820';
        ctx.fillRect(cx + i, cy - innerRadius, 12, innerRadius * 2);
    }
    ctx.restore();

    // 桌面高光（左上角光源）
    const grd = ctx.createRadialGradient(cx - baseRadius * 0.36, cy - baseRadius * 0.36, baseRadius * 0.05, cx, cy, baseRadius);
    grd.addColorStop(0, 'rgba(255,200,120,0.2)');
    grd.addColorStop(0.5, 'rgba(255,150,60,0.05)');
    grd.addColorStop(1, 'rgba(0,0,0,0.3)');
    ctx.fillStyle = grd;
    ctx.beginPath();
    ctx.arc(cx, cy, baseRadius, 0, Math.PI * 2);
    ctx.fill();

    // 8-bit边框（外圈像素虚线）
    ctx.strokeStyle = '#ffa040';
    ctx.lineWidth = 4;
    ctx.setLineDash([]);

    // 桌面中央装饰——8-bit像素符文
    ctx.fillStyle = 'rgba(255,180,80,0.24)';
    const fontSize = Math.round(baseRadius * 0.25);
    ctx.font = `bold ${fontSize}px "Press Start 2P", monospace`;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText('?', cx, cy);

}

// ============================
// ============================
// 构建复古8-bit风格像素小人 SVG
// ============================
function buildPixelPersonSVG(bodyColor, skinColor, w, h, blinkDelay = '0s', mouthDelay = '0s') {
    const scale = w / 120;

    return `<svg width="${w}" height="${h}" xmlns="http://www.w3.org/2000/svg">
        <!-- 8-bit方正造型 -->
        <!-- 身体 -->
        <rect x="${42*scale}" y="${68*scale}" width="${36*scale}" height="${42*scale}" fill="${bodyColor}"/>

        <!-- 身体阴影像素块 -->
        <rect x="${60*scale}" y="${68*scale}" width="${8*scale}" height="${42*scale}" fill="rgba(0,0,0,0.2)"/>
        <rect x="${42*scale}" y="${100*scale}" width="${36*scale}" height="${10*scale}" fill="rgba(0,0,0,0.3)"/>

        <!-- 头部（矩形） -->
        <rect x="${35*scale}" y="${20*scale}" width="${50*scale}" height="${45*scale}" fill="${skinColor}"/>

        <!-- 头发（方正块状） -->
        <rect x="${33*scale}" y="${15*scale}" width="${54*scale}" height="${15*scale}" fill="#4A3728"/>

        <!-- 眼睛（像素方块，眨眼动画） -->
        <g class="eye" style="--blink-delay: ${blinkDelay};">
            <rect x="${45*scale}" y="${38*scale}" width="${10*scale}" height="${12*scale}" fill="#000000"/>
            <rect x="${65*scale}" y="${38*scale}" width="${10*scale}" height="${12*scale}" fill="#000000"/>
            <!-- 像素高光 -->
            <rect x="${48*scale}" y="${40*scale}" width="${3*scale}" height="${4*scale}" fill="#ffffff"/>
            <rect x="${68*scale}" y="${40*scale}" width="${3*scale}" height="${4*scale}" fill="#ffffff"/>
        </g>

        <!-- 嘴巴（像素块状，微动动画） -->
        <g class="mouth" style="--mouth-delay: ${mouthDelay};">
            <rect x="${52*scale}" y="${55*scale}" width="${16*scale}" height="${5*scale}" fill="#000000"/>
        </g>

        <!-- 手臂（矩形） -->
        <rect x="${28*scale}" y="${72*scale}" width="${12*scale}" height="${14*scale}" fill="${skinColor}"/>
        <rect x="${80*scale}" y="${72*scale}" width="${12*scale}" height="${14*scale}" fill="${skinColor}"/>

        <!-- 腿部（方块状） -->
        <rect x="${40*scale}" y="${112*scale}" width="${16*scale}" height="${8*scale}" fill="#000000"/>
        <rect x="${64*scale}" y="${112*scale}" width="${16*scale}" height="${8*scale}" fill="#000000"/>
    </svg>`;
}

// ============================
// 渲染像素玩家围圆桌
// ============================
function renderPixelPlayers(players) {
    const container = document.getElementById('playerAvatars');
    const roundTable = document.querySelector('.round-table');
    container.innerHTML = '';

    // 动态获取容器实际尺寸（兼容响应式）
    const room = document.querySelector('.discussion-room') || roundTable;
    const w = room.offsetWidth;
    const h = room.offsetHeight;
    const cx = w / 2;
    const cy = h / 2 + 20;
    const avatarWidth = 92;
    const avatarHeight = 74;
    const roomRadius = Math.min(w, h) / 2 - 60;
    const radius = roomRadius - Math.hypot(avatarWidth / 2, avatarHeight / 2) - 10;

    players.forEach((player, index) => {
        const angle = (index / players.length) * Math.PI * 2 - Math.PI / 2;
        const x = cx + radius * Math.cos(angle);
        const y = cy + radius * Math.sin(angle);

        const bodyColor = bodyColors[index % bodyColors.length];
        const skinColor = skinColors[index % skinColors.length];
        const personaStyle = player.isAI ? getPersonaStyle(player.persona) : null;

        // 随机动画延迟，让每个小人的眨眼和嘴巴动作错开
        const blinkDelay = `${Math.random() * 3}s`;
        const mouthDelay = `${Math.random() * 2}s`;

        const avatarEl = document.createElement('div');
        avatarEl.className = `player-avatar${player.isAI ? ' isAI' : ''}${personaStyle ? ` ${personaStyle.className}` : ''}`;
        avatarEl.style.left = `${x}px`;
        avatarEl.style.top = `${y}px`;
        avatarEl.dataset.playerId = player.id;
        if (player.persona) {
            avatarEl.dataset.persona = player.persona;
        }
        avatarEl.onclick = () => handlePlayerClick(player);

        // 像素小人用SVG绘制
        const personDiv = document.createElement('div');
        personDiv.className = 'avatar';
        personDiv.style.cssText = `--body-color:${bodyColor}; width:56px; height:56px; position:relative;`;
        personDiv.innerHTML = buildPixelPersonSVG(bodyColor, skinColor, 56, 56, blinkDelay, mouthDelay);

        const nameDiv = document.createElement('div');
        nameDiv.className = 'name';
        nameDiv.textContent = player.name.toUpperCase();

        if (player.isAI && personaStyle) {
            const chipDiv = document.createElement('div');
            chipDiv.className = 'persona-chip';
            chipDiv.textContent = personaStyle.chip;
            avatarEl.appendChild(chipDiv);
        }

        avatarEl.appendChild(personDiv);
        avatarEl.appendChild(nameDiv);

        // 添加座位
        const seatDiv = document.createElement('div');
        seatDiv.className = 'player-seat';
        avatarEl.appendChild(seatDiv);

        container.appendChild(avatarEl);
    });
}

// 更新玩家描述气泡
function updatePlayerDescription(playerId, description) {
    const avatar = document.querySelector(`[data-player-id="${playerId}"]`);
    if (avatar) {
        // 移除旧气泡
        const old = avatar.querySelector('.description');
        if (old) old.remove();

        const descDiv = document.createElement('div');
        descDiv.className = 'description';
        descDiv.textContent = description;
        avatar.insertBefore(descDiv, avatar.firstChild);
    }
}

// 处理玩家点击
function handlePlayerClick(player) {
    if (gamePhase === 'action') {
        selectPlayer(player.id);
    } else if (gamePhase === 'voting1' || gamePhase === 'voting2') {
        vote(player.id);
    }
}

// 提问
function showQuestionPanel() {
    if (gamePhase !== 'action') return;
    const targetId = prompt('输入目标玩家名字（留空随机选择）：');
    const target = targetId
        ? playerPositions.find(p => p.name.toLowerCase() === targetId.toLowerCase())
        : playerPositions.filter(p => p.id !== currentPlayer?.id)[Math.floor(Math.random() * (playerPositions.length - 1))];

    if (!target) { addMessage('system', '未找到玩家'); return; }

    const question = prompt(`向 ${target.name.toUpperCase()} 提问：`);
    if (question) {
        socket.emit('askQuestion', { roomId: currentRoom, targetId: target.id, question });
    }
}

// 显示回答选项
function showAnswerOptions() {
    const answer = prompt('请输入回答（输入“拒绝”可跳过）：');
    if (answer) {
        if (answer.toUpperCase() === 'REFUSE' || answer === '拒绝') {
            socket.emit('rejectAnswer', { roomId: currentRoom });
        }
    }
}

// 处理玩家点击（投票或侦探技能）
function handlePlayerClick(player) {
    if (gamePhase === 'action') {
        selectPlayer(player.id);
    } else if (gamePhase === 'voting1' || gamePhase === 'voting2') {
        vote(player.id);
    }
}

function selectPlayer(playerId) {
    // 清除之前的选择
    clearPlayerSelection();

    selectedTarget = playerId;
    const avatar = document.querySelector(`[data-player-id="${playerId}"]`);
    if (avatar) {
        avatar.style.boxShadow = '0 0 10px 4px var(--pixel-cyan)';
        avatar.style.transform = 'translate(-50%, -50%) scale(1.1)';
    }

    // 显示选中提示
    addMessage('system', '已选择玩家，请点击侦探技能按钮使用。');
}

function clearPlayerSelection() {
    const avatars = document.querySelectorAll('.player-avatar');
    avatars.forEach(avatar => {
        avatar.style.boxShadow = '';
        avatar.style.transform = 'translate(-50%, -50%)';
    });
    selectedTarget = null;
}

// 投票
function startVoting() {
    if (gamePhase !== 'action') return;
    if (!selectedTarget) {
        addMessage('warning', '请先点击一个玩家，再提交投票。');
        return;
    }

    vote(selectedTarget);
    selectedTarget = null;
    clearPlayerSelection();
}

function vote(targetId) {
    socket.emit('vote', { roomId: currentRoom, targetId });
    document.querySelectorAll('.player-avatar').forEach(a => a.classList.remove('voting'));
    // 投票后闪烁
    triggerAvatarAnim(targetId, 'voting', 1500);
    addMessage('success', '投票已提交！');
}

// 显示投票结果
function displayVoteResults(voteResults) {
    let result = '=== 投票结果 ===\n';
    playerPositions.forEach(p => {
        const votes = voteResults[p.id] || 0;
        const bar = '█'.repeat(votes) + '░'.repeat(Math.max(0, 3 - votes));
        result += `${p.name}: ${bar} ${votes}`;
    });
    addMessage('summary', result);
}

function getPersonaLabel(persona) {
    return getPersonaStyle(persona)?.chip || '谨慎型';
}

function renderAiProfileSummary(aiProfile) {
    const container = document.getElementById('aiProfileSummary');
    if (!container) return;

    if (!aiProfile) {
        container.innerHTML = '';
        return;
    }

    const personaCounts = aiProfile.personaCounts || {};
    const dominantAI = aiProfile.dominantAI;
    const eventTrail = aiProfile.eventTrail || [];
    const pressureTop = aiProfile.pressureTop || [];

    container.innerHTML = `
        <div class="ai-profile-title">本局 AI 行为画像</div>
        <div class="ai-profile-grid">
            <div class="profile-card">
                <div class="label">人格分布</div>
                <div class="value">
                    分析型 ${personaCounts.analytical || 0} · 谨慎型 ${personaCounts.cautious || 0}<br>
                    对抗型 ${personaCounts.confrontational || 0} · 跳脱型 ${personaCounts.quirky || 0}
                </div>
            </div>
            <div class="profile-card">
                <div class="label">主导角色</div>
                <div class="value">
                    ${dominantAI ? `${dominantAI.name} / ${getPersonaLabel(dominantAI.persona)}<br>动作 ${dominantAI.actionCount} 次 · 压力 ${dominantAI.pressure}` : '本局未形成明显主导 AI'}
                </div>
            </div>
            <div class="profile-card">
                <div class="label">事件轨迹</div>
                <div class="value">${aiProfile.eventCount || 0} 次随机事件</div>
            </div>
            <div class="profile-card">
                <div class="label">高压 AI</div>
                <div class="value">
                    ${pressureTop.length ? pressureTop.map(ai => `${ai.name}(${ai.pressure})`).join(' · ') : '没有明显压力峰值'}
                </div>
            </div>
        </div>
        <div class="profile-card">
            <div class="label">事件碎片</div>
            <div class="profile-trail">
                ${eventTrail.length ? eventTrail.map(label => `<span class="trail-chip">${label}</span>`).join('') : '<span class="trail-chip">无随机事件</span>'}
            </div>
        </div>
        <div class="profile-note">
            这份画像会帮助你回看：AI 这局更偏向哪种人格、被哪些事件推动、哪几个 AI 更值得怀疑。
        </div>
    `;
}

// 显示辩解玩家
function showDefendingPlayers(players) {
    const container = document.getElementById('defendingPlayers');
    container.innerHTML = players.map(id => {
        const player = playerPositions.find(p => p.id === id);
        return player ? `<div class="player-badge">${player.name.toUpperCase()}</div>` : '';
    }).join('');
}

// 添加消息
function addMessage(type, content) {
    const list = document.getElementById('messageList');
    const message = document.createElement('div');
    const tone = getMessageTone(type, content);
    message.className = `message ${tone}`;
    message.textContent = content;
    list.appendChild(message);
    list.scrollTop = list.scrollHeight;
}

// 触发小人头像动画
function triggerAvatarAnim(playerId, animClass, duration) {
    const avatar = document.querySelector(`[data-player-id="${playerId}"]`);
    if (!avatar) return;

    // 先移除可能存在的同类动画
    avatar.classList.remove(animClass);

    // 强制重排，让浏览器重新应用动画
    void avatar.offsetWidth;

    // 添加动画 class
    avatar.classList.add(animClass);

    // 指定时间后移除
    setTimeout(() => {
        avatar.classList.remove(animClass);
    }, duration);
}

// 发送消息
function sendMessage() {
    const input = document.getElementById('messageInput');
    const message = input.value.trim();
    if (message) {
        socket.emit('chat', { roomId: currentRoom, message });
        addMessage('user', `> ${message}`);
        input.value = '';
    }
}

// ============================
// 新增功能：线索搜集和侦探技能
// ============================

function hideAllPanels() {
    ['descriptionPanel', 'cluePanel', 'debatePanel', 'questionButtons', 'votePanel', 'defendPanel'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.classList.add('hidden');
    });
}

function showCluePanel() {
    document.getElementById('cluePanel').classList.remove('hidden');
    updateSkillCounts();
    addMessage('system', '使用侦探技能收集线索。');
}

function updateSkillCounts() {
    document.getElementById('observeCount').textContent = detectiveSkills.observe;
    document.getElementById('questionCount').textContent = detectiveSkills.question;
    document.getElementById('listenCount').textContent = detectiveSkills.listen;

    renderSkillPanel();

    // 更新按钮状态
    document.getElementById('btnObserve').disabled = detectiveSkills.observe <= 0;
    document.getElementById('btnQuestion').disabled = detectiveSkills.question <= 0;
    document.getElementById('btnListen').disabled = detectiveSkills.listen <= 0;
}

function useSkillObserve() {
    if (!selectedTarget) {
        alert('请先点击要观察的玩家！');
        return;
    }

    if (detectiveSkills.observe <= 0) return;

    socket.emit('useSkillObserve', {
        roomId: currentRoom,
        targetId: selectedTarget
    });
}

function useSkillQuestion() {
    if (!selectedTarget) {
        alert('请先点击要质问的玩家！');
        return;
    }

    if (detectiveSkills.question <= 0) return;

    const question = prompt('输入你要质问的问题：');
    if (question && question.trim()) {
        socket.emit('useSkillQuestion', {
            roomId: currentRoom,
            targetId: selectedTarget,
            question: question.trim()
        });
    }
}

function useSkillListen() {
    if (!selectedTarget) {
        alert('请先点击要偷听的玩家！');
        return;
    }

    if (detectiveSkills.listen <= 0) return;

    if (!confirm('确定要使用偷听技能吗？(仅限1次)')) return;

    socket.emit('useSkillListen', {
        roomId: currentRoom,
        targetId: selectedTarget
    });
}

function skipClueCollecting() {
    if (!confirm('确定要跳过线索搜集阶段吗？')) return;
    socket.emit('skipClueCollecting', { roomId: currentRoom });
}

function bindLateSocketEvents() {
    // 处理服务器发送的侦探技能结果
    socket.on('skillUsed', ({ playerId, skillType, targetId, description, question, targetName, isAI, hint }) => {
        const player = playerPositions.find(p => p.id === playerId);
        const target = playerPositions.find(p => p.id === targetId);

        if (socket && playerId === socket.id) {
            if (skillType === 'observe' && detectiveSkills.observe > 0) {
                detectiveSkills.observe--;
                updateSkillCounts();
            } else if (skillType === 'question' && detectiveSkills.question > 0) {
                detectiveSkills.question--;
                updateSkillCounts();
            } else if (skillType === 'listen' && detectiveSkills.listen > 0) {
                detectiveSkills.listen--;
                updateSkillCounts();
            }

            selectedTarget = null;
            clearPlayerSelection();
        }

        if (skillType === 'observe') {
            addMessage('system', `${player.name} 观察 ${target.name}`);
            if (description) {
                addMessage('system', `描述内容：“${description}”`);
            }
        } else if (skillType === 'question') {
            addMessage('system', `${player.name} 质问 ${target.name}：“${question}”`);
        } else if (skillType === 'listen') {
            addMessage('system', `${player.name} 偷听 ${targetName}`);
            addMessage('system', `提示：${hint}`);
        }
    });

    socket.on('skillAnswered', ({ answererId, answer }) => {
        const answerer = playerPositions.find(p => p.id === answererId);
        if (answerer) {
            addMessage('answer', `${answerer.name}：“${answer}”`);
            triggerAvatarAnim(answererId, 'speaking', 1500);
        }
    });

    socket.on('debating', ({ debatingPlayers, voteResults }) => {
        const panel = document.getElementById('debatePanel');
        const playersDiv = document.getElementById('debatingPlayers');
        playersDiv.innerHTML = '';

        debatingPlayers.forEach(playerId => {
            const player = playerPositions.find(p => p.id === playerId);
            if (player) {
                const playerDiv = document.createElement('div');
                playerDiv.className = 'message';
                playerDiv.style.borderColor = 'var(--pixel-red)';
                playerDiv.textContent = `🔴 ${player.name}`;
                playersDiv.appendChild(playerDiv);
            }
        });

        panel.classList.remove('hidden');
        addMessage('system', '【紧急辩论】平票，进入 PK 模式！');
    });

    socket.on('debateStatement', ({ playerId, statement }) => {
        const player = playerPositions.find(p => p.id === playerId);
        if (player) {
            addMessage('system', `${player.name} 辩言：“${statement}”`);
            triggerAvatarAnim(playerId, 'speaking', 2000);
        }
    });
}


// ============================
// 临时功能：重新开始下一把
// ============================
function restartGame() {
    if (socket && currentRoom) {
        // 通知服务器重置房间状态
        socket.emit('resetRoom', { roomId: currentRoom });
    }
    resetGameState();
    currentPlayer = null;
    currentRoom = null;
    currentWord = null;
    gamePhase = 'waiting';

    // 隐藏用户信息栏
    const userInfo = document.getElementById('userInfo');
    if (userInfo) userInfo.classList.add('hidden');

    // 回到登录界面
    showScreen('loginScreen');
}

window.getLobbyHint = getLobbyHint;
window.getPhaseCopy = getPhaseCopy;
window.renderSkillPanel = renderSkillPanel;
window.updateSkillPanel = updateSkillPanel;
window.getMessageTone = getMessageTone;
window.init = init;
window.joinGame = joinGame;
window.setGameMode = setGameMode;
window.toggleTestMode = toggleTestMode;
window.toggleSoloDebug = toggleSoloDebug;
window.onload = init;
