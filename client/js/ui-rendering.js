const TABLE_EDGE_INSET = 22;

const bodyColors = [
    '#4080ff',
    '#ff4080',
    '#40c040',
    '#ffa040',
    '#a040ff',
    '#40c0c0',
];

const skinColors = [
    '#ffcc88',
    '#ffaa66',
    '#ffe0b0',
    '#cc8844',
    '#ffddaa',
    '#ddaa77',
];

function getTableRadius(width, height) {
    return Math.max(0, Math.min(width, height) / 2 - TABLE_EDGE_INSET);
}

function shouldShowPlayerName() {
    return gameMode === 'test' || gameMode === 'solo';
}

function getPlayerNumber(player, fallbackIndex = null) {
    const rawPosition = typeof player?.position === 'number' ? player.position : fallbackIndex;
    if (rawPosition === null || rawPosition === undefined || Number.isNaN(rawPosition)) {
        return '--';
    }
    return String(rawPosition + 1).padStart(2, '0');
}

function getPlayerLabel(player, fallbackIndex = null) {
    const number = getPlayerNumber(player, fallbackIndex);
    const name = player?.name || '未知玩家';
    return shouldShowPlayerName() ? `#${number} ${name}` : `#${number}`;
}

function getPlayerAvatarTitle(player, fallbackIndex = null) {
    return getPlayerLabel(player, fallbackIndex);
}

function buildPixelPersonSVG(bodyColor, skinColor, w, h, blinkDelay = '0s', mouthDelay = '0s') {
    const scale = w / 120;

    return `<svg width="${w}" height="${h}" xmlns="http://www.w3.org/2000/svg">
        <rect x="${42 * scale}" y="${68 * scale}" width="${36 * scale}" height="${42 * scale}" fill="${bodyColor}"/>
        <rect x="${60 * scale}" y="${68 * scale}" width="${8 * scale}" height="${42 * scale}" fill="rgba(0,0,0,0.2)"/>
        <rect x="${42 * scale}" y="${100 * scale}" width="${36 * scale}" height="${10 * scale}" fill="rgba(0,0,0,0.3)"/>
        <rect x="${35 * scale}" y="${20 * scale}" width="${50 * scale}" height="${45 * scale}" fill="${skinColor}"/>
        <rect x="${33 * scale}" y="${15 * scale}" width="${54 * scale}" height="${15 * scale}" fill="#4A3728"/>
        <g class="eye" style="--blink-delay: ${blinkDelay};">
            <rect x="${45 * scale}" y="${38 * scale}" width="${10 * scale}" height="${12 * scale}" fill="#000000"/>
            <rect x="${65 * scale}" y="${38 * scale}" width="${10 * scale}" height="${12 * scale}" fill="#000000"/>
            <rect x="${48 * scale}" y="${40 * scale}" width="${3 * scale}" height="${4 * scale}" fill="#ffffff"/>
            <rect x="${68 * scale}" y="${40 * scale}" width="${3 * scale}" height="${4 * scale}" fill="#ffffff"/>
        </g>
        <g class="mouth" style="--mouth-delay: ${mouthDelay};">
            <rect x="${52 * scale}" y="${55 * scale}" width="${16 * scale}" height="${5 * scale}" fill="#000000"/>
        </g>
        <rect x="${28 * scale}" y="${72 * scale}" width="${12 * scale}" height="${14 * scale}" fill="${skinColor}"/>
        <rect x="${80 * scale}" y="${72 * scale}" width="${12 * scale}" height="${14 * scale}" fill="${skinColor}"/>
        <rect x="${40 * scale}" y="${112 * scale}" width="${16 * scale}" height="${8 * scale}" fill="#000000"/>
        <rect x="${64 * scale}" y="${112 * scale}" width="${16 * scale}" height="${8 * scale}" fill="#000000"/>
    </svg>`;
}

function computePlayerRingLayout(players, containerRect) {
    const orderedPlayers = [...players].sort((a, b) => {
        const aPos = typeof a.position === 'number' ? a.position : Number.MAX_SAFE_INTEGER;
        const bPos = typeof b.position === 'number' ? b.position : Number.MAX_SAFE_INTEGER;
        if (aPos !== bPos) return aPos - bPos;
        return String(a.name || '').localeCompare(String(b.name || ''));
    });

    const width = containerRect?.width || 0;
    const height = containerRect?.height || 0;
    const cx = width / 2;
    const cy = height / 2;
    const tableRadius = getTableRadius(width, height);
    const ringRatio = orderedPlayers.length <= 4 ? 0.66 : orderedPlayers.length <= 6 ? 0.71 : 0.75;
    const radius = Math.max(180, Math.round(tableRadius * ringRatio));
    const anchorOffsetY = 0;

    return orderedPlayers.map((player, index) => {
        const angle = (index / orderedPlayers.length) * Math.PI * 2 - Math.PI / 2;
        const x = cx + radius * Math.cos(angle);
        const y = cy + radius * Math.sin(angle);
        return {
            player,
            index,
            angle,
            x,
            y,
            zIndex: 10 + Math.round(Math.sin(angle) * 3),
            anchorOffsetY
        };
    });
}

function drawPixelTable() {
    const canvas = document.getElementById('tableCanvas');
    const ctx = canvas.getContext('2d');

    const room = document.querySelector('.discussion-room');
    const displayW = room.offsetWidth;
    const displayH = room.offsetHeight;

    canvas.width = displayW;
    canvas.height = displayH;

    const W = canvas.width;
    const H = canvas.height;
    const cx = W / 2;
    const cy = H / 2;
    const baseRadius = getTableRadius(W, H);

    ctx.clearRect(0, 0, W, H);
    ctx.imageSmoothingEnabled = false;

    ctx.fillStyle = 'rgba(0,0,0,0.32)';
    ctx.beginPath();
    ctx.arc(cx, cy, baseRadius + 5, 0, Math.PI * 2);
    ctx.fill();

    ctx.fillStyle = '#5a3010';
    ctx.beginPath();
    ctx.arc(cx, cy, baseRadius, 0, Math.PI * 2);
    ctx.fill();

    ctx.save();
    ctx.beginPath();
    ctx.arc(cx, cy, baseRadius, 0, Math.PI * 2);
    ctx.clip();
    for (let i = -baseRadius; i <= baseRadius; i += 24) {
        ctx.fillStyle = i % 48 === 0 ? '#6a3818' : '#5a3010';
        ctx.fillRect(cx + i, cy - baseRadius, 12, baseRadius * 2);
    }
    ctx.restore();

    const innerRadius = baseRadius * 0.66;
    ctx.fillStyle = '#7a4820';
    ctx.beginPath();
    ctx.arc(cx, cy, innerRadius, 0, Math.PI * 2);
    ctx.fill();

    ctx.save();
    ctx.beginPath();
    ctx.arc(cx, cy, innerRadius, 0, Math.PI * 2);
    ctx.clip();
    for (let i = -innerRadius; i <= innerRadius; i += 24) {
        ctx.fillStyle = i % 48 === 0 ? '#8a5030' : '#7a4820';
        ctx.fillRect(cx + i, cy - innerRadius, 12, innerRadius * 2);
    }
    ctx.restore();

    const grd = ctx.createRadialGradient(cx - baseRadius * 0.36, cy - baseRadius * 0.36, baseRadius * 0.05, cx, cy, baseRadius);
    grd.addColorStop(0, 'rgba(255,200,120,0.2)');
    grd.addColorStop(0.5, 'rgba(255,150,60,0.05)');
    grd.addColorStop(1, 'rgba(0,0,0,0.3)');
    ctx.fillStyle = grd;
    ctx.beginPath();
    ctx.arc(cx, cy, baseRadius, 0, Math.PI * 2);
    ctx.fill();

    ctx.strokeStyle = '#ffa040';
    ctx.lineWidth = 4;
    ctx.setLineDash([]);

    ctx.fillStyle = 'rgba(255,180,80,0.24)';
    const fontSize = Math.round(baseRadius * 0.25);
    ctx.font = `bold ${fontSize}px "Press Start 2P", monospace`;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText('?', cx, cy);
}

function updateAvatarActivityState() {
    const avatars = document.querySelectorAll('.player-avatar');
    avatars.forEach(avatar => {
        avatar.classList.remove('is-actionable', 'is-dim', 'is-self', 'is-selected-target', 'is-disabled-target');
    });

    if (!playerPositions.length) return;

    const actionablePhases = ['clueCollecting', 'questioning1', 'questioning2', 'voting1', 'voting2'];
    const selectable = new Set();
    if (actionablePhases.includes(gamePhase)) {
        playerPositions.forEach(player => {
            if (currentPlayer?.id !== player.id) {
                selectable.add(player.id);
            }
        });
    }

    playerPositions.forEach(player => {
        const avatar = document.querySelector(`[data-player-id="${player.id}"]`);
        if (!avatar) return;
        const isMe = currentPlayer?.id === player.id;
        const isSelected = selectedTarget === player.id;

        if (isMe) {
            avatar.classList.add('is-self');
            avatar.title = `${getPlayerAvatarTitle(player)}（你）`;
        } else if (selectable.has(player.id)) {
            avatar.classList.add('is-actionable');
            avatar.title = `${getPlayerAvatarTitle(player)}（可点）`;
        } else {
            avatar.classList.add('is-dim');
            avatar.classList.add('is-disabled-target');
            avatar.title = `${getPlayerAvatarTitle(player)}（暂不可点）`;
        }

        if (isSelected) {
            avatar.classList.add('is-selected-target');
            avatar.title = `${getPlayerAvatarTitle(player)}（已选中）`;
        }
    });
}

function renderPixelPlayers(players) {
    const container = document.getElementById('playerAvatars');
    const roundTable = document.querySelector('.round-table');
    container.innerHTML = '';

    const room = document.querySelector('.discussion-room') || roundTable;
    const layout = computePlayerRingLayout(players, room.getBoundingClientRect());

    layout.forEach(({ player, index, x, y, zIndex, anchorOffsetY }) => {
        const bodyColor = bodyColors[index % bodyColors.length];
        const skinColor = skinColors[index % skinColors.length];
        const blinkDelay = `${Math.random() * 3}s`;
        const mouthDelay = `${Math.random() * 2}s`;
        const showPlayerName = shouldShowPlayerName();

        const avatarEl = document.createElement('div');
        avatarEl.className = `player-avatar${player.isAI ? ' isAI' : ''}${showPlayerName ? '' : ' player-avatar--compact'}`;
        avatarEl.style.left = `${x}px`;
        avatarEl.style.top = `${y + anchorOffsetY}px`;
        avatarEl.style.zIndex = String(zIndex);
        avatarEl.dataset.playerId = player.id;
        avatarEl.title = getPlayerAvatarTitle(player, index);
        avatarEl.onclick = () => handlePlayerClick(player);

        const personDiv = document.createElement('div');
        personDiv.className = 'avatar';
        personDiv.style.cssText = `--body-color:${bodyColor}; width:56px; height:56px; position:relative;`;
        personDiv.innerHTML = buildPixelPersonSVG(bodyColor, skinColor, 56, 56, blinkDelay, mouthDelay);

        const nameDiv = document.createElement('div');
        nameDiv.className = 'name';
        nameDiv.textContent = player.name.toUpperCase();

        const actionTag = document.createElement('div');
        actionTag.className = 'player-action-tag hidden';

        avatarEl.appendChild(personDiv);
        const numberDiv = document.createElement('div');
        numberDiv.className = 'player-number';
        numberDiv.textContent = `#${getPlayerNumber(player, index)}`;
        avatarEl.appendChild(numberDiv);
        if (showPlayerName) {
            avatarEl.appendChild(nameDiv);
        }
        avatarEl.appendChild(actionTag);

        const seatDiv = document.createElement('div');
        seatDiv.className = 'player-seat';
        avatarEl.appendChild(seatDiv);

        container.appendChild(avatarEl);
    });
    updateAvatarActivityState();
}

function updatePlayerDescription(playerId, description) {
    const avatar = document.querySelector(`[data-player-id="${playerId}"]`);
    if (avatar) {
        const old = avatar.querySelector('.description');
        if (old) old.remove();

        const descDiv = document.createElement('div');
        descDiv.className = 'description';
        descDiv.textContent = description;
        avatar.insertBefore(descDiv, avatar.firstChild);
    }
}
