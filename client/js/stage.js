/**
 * 谁是AI v3 — 舞台区 Canvas 小人渲染引擎
 *
 * 职责：
 *   - 在 Canvas 上绘制 32x32 像素小人
 *   - 弧形排列（围成半圆）
 *   - 7 种动画状态
 *   - 提名指向虚线
 */

// ═══════════════════════════════════════
//  颜色常量（与 DESIGN.md 一致）
// ═══════════════════════════════════════
const COLORS = {
  bg:         '#0d0d1a',
  panel:      '#1e1e2e',
  blue:       '#4040ff',   // 工程师
  red:        '#ff4040',   // 渗透者
  green:      '#40c040',   // 信号员
  gray:       '#41436a',   // 未知
  white:      '#f0f0f0',   // 名字、高光
  yellow:     '#ffc040',   // 队长标记
  accent:     '#ff80c0',   // 指向线
  skin:       '#deb887',   // 皮肤
  skinDark:   '#c8a06e',   // 皮肤暗部
  text:       '#e0e0ff',   // 名字标签
  textDim:    '#6a6c9a',   // 次要文字
};

// 角色 → 身体颜色映射
const ROLE_COLORS = {
  engineer:      COLORS.blue,
  infiltrator:   COLORS.red,
  signal_keeper: COLORS.green,
  unknown:       COLORS.gray,
};

// ═══════════════════════════════════════
//  PlayerStage 类
// ═══════════════════════════════════════
class PlayerStage {
  constructor(canvasId) {
    this.canvas = document.getElementById(canvasId);
    if (!this.canvas) {
      console.warn('Stage: canvas not found:', canvasId);
      return;
    }
    this.ctx = this.canvas.getContext('2d');
    this.players = [];
    this.pointingFrom = null;
    this.pointingTo = [];
    this.animFrame = 0;
    this.animationId = null;
    this.dpr = window.devicePixelRatio || 1;

    this._resizeBound = this.resize.bind(this);
    window.addEventListener('resize', this._resizeBound);
    this.resize();
  }

  // ─────────────────────────────────────
  //  设置玩家
  // ─────────────────────────────────────
  setPlayers(players) {
    this.players = players.map(p => ({
      id: p.id,
      name: p.name,
      role: p.role || 'unknown',
      isLeader: false,
      x: 0,
      y: 0,
      state: 'idle',
      stateTimer: 0,
      floatOffset: Math.random() * Math.PI * 2, // 随机相位
      eliminated: p.eliminated || false,
    }));
    this.arrangeArc();
  }

  updateRoles(roleMap) {
    this.players.forEach(p => {
      if (roleMap[p.id]) {
        p.role = roleMap[p.id];
      }
    });
  }

  setLeader(playerId) {
    this.players.forEach(p => { p.isLeader = p.id === playerId; });
  }

  // ─────────────────────────────────────
  //  坐标计算
  // ─────────────────────────────────────
  arrangeArc() {
    const n = this.players.length;
    if (n === 0) return;

    const w = this.canvas.width / this.dpr;
    const h = 160;
    const padding = 30;  // 减小边距
    const usableW = w - padding * 2;

    // 弧形参数：椭圆中心在 canvas 下方
    const cx = w / 2;
    const cy = h + 60;           // 椭圆中心在画面下方
    const rx = usableW / 2;      // 水平半径
    const ry = 100;              // 垂直半径

    for (let i = 0; i < n; i++) {
      const t = n > 1 ? (i / (n - 1)) * Math.PI : Math.PI / 2;
      // 从左到右，角度从 π 到 0
      const angle = Math.PI - t;
      this.players[i].x = cx + rx * Math.cos(angle);
      this.players[i].y = cy - ry * Math.sin(angle);
    }
  }

  // ─────────────────────────────────────
  //  动画状态
  // ─────────────────────────────────────
  setAnimation(playerId, type) {
    const p = this.players.find(pl => pl.id === playerId);
    if (p) {
      p.state = type;
      p.stateTimer = 0;
    }
  }

  setAnimationAll(type) {
    this.players.forEach(p => {
      p.state = type;
      p.stateTimer = 0;
    });
  }

  resetAnimations() {
    this.players.forEach(p => {
      p.state = p.eliminated ? 'defeated' : 'idle';
      p.stateTimer = 0;
    });
  }

  // ─────────────────────────────────────
  //  指向
  // ─────────────────────────────────────
  setPointing(fromId, toIds) {
    this.pointingFrom = fromId;
    this.pointingTo = toIds || [];
    // 队长做 pointing 动画
    this.setAnimation(fromId, 'pointing');
  }

  clearPointing() {
    this.pointingFrom = null;
    this.pointingTo = [];
    this.resetAnimations();
  }

  // ─────────────────────────────────────
  //  尺寸自适应
  // ─────────────────────────────────────
  resize() {
    if (!this.canvas) return;
    const container = this.canvas.parentElement;
    if (!container) return;

    const w = container.clientWidth;
    const h = 160;

    this.canvas.width = w * this.dpr;
    this.canvas.height = h * this.dpr;
    this.canvas.style.width = w + 'px';
    this.canvas.style.height = h + 'px';
    this.ctx.setTransform(this.dpr, 0, 0, this.dpr, 0, 0);

    this.arrangeArc();
  }

  // ─────────────────────────────────────
  //  渲染循环
  // ─────────────────────────────────────
  startLoop() {
    if (this.animationId) return;
    const loop = () => {
      this.animFrame++;
      this.render();
      this.animationId = requestAnimationFrame(loop);
    };
    this.animationId = requestAnimationFrame(loop);
  }

  stopLoop() {
    if (this.animationId) {
      cancelAnimationFrame(this.animationId);
      this.animationId = null;
    }
  }

  destroy() {
    this.stopLoop();
    window.removeEventListener('resize', this._resizeBound);
  }

  // ─────────────────────────────────────
  //  主渲染
  // ─────────────────────────────────────
  render() {
    if (!this.ctx) return;
    const ctx = this.ctx;
    const w = this.canvas.width / this.dpr;
    const h = 160;

    // 清空
    ctx.fillStyle = COLORS.bg;
    ctx.fillRect(0, 0, w, h);

    // 底部弧形桌面线
    this._drawTableArc(ctx, w, h);

    // 先画指向线（在小人下面）
    this._drawPointers(ctx);

    // 画小人
    this.players.forEach(p => this._drawPlayer(ctx, p));
  }

  // ─────────────────────────────────────
  //  桌面弧线（装饰）
  // ─────────────────────────────────────
  _drawTableArc(ctx, w, h) {
    ctx.save();
    ctx.strokeStyle = COLORS.gray;
    ctx.globalAlpha = 0.3;
    ctx.lineWidth = 1;
    ctx.setLineDash([4, 4]);
    ctx.beginPath();
    const radiusX = Math.max(10, w / 2 - 40); // 确保半径为正数
    ctx.ellipse(w / 2, h + 60, radiusX, 80, 0, Math.PI, 0);
    ctx.stroke();
    ctx.setLineDash([]);
    ctx.restore();
  }

  // ─────────────────────────────────────
  //  指向虚线
  // ─────────────────────────────────────
  _drawPointers(ctx) {
    if (!this.pointingFrom || this.pointingTo.length === 0) return;

    const from = this.players.find(p => p.id === this.pointingFrom);
    if (!from) return;

    ctx.save();
    ctx.strokeStyle = COLORS.yellow;
    ctx.lineWidth = 2;
    ctx.setLineDash([6, 4]);
    ctx.globalAlpha = 0.8 + 0.2 * Math.sin(this.animFrame * 0.1);

    this.pointingTo.forEach(toId => {
      const to = this.players.find(p => p.id === toId);
      if (!to) return;

      ctx.beginPath();
      ctx.moveTo(from.x, from.y - 16);
      // 贝塞尔曲线，向上弯曲
      const midX = (from.x + to.x) / 2;
      const midY = Math.min(from.y, to.y) - 40;
      ctx.quadraticCurveTo(midX, midY, to.x, to.y - 16);
      ctx.stroke();

      // 箭头（小三角）
      const angle = Math.atan2(to.y - 16 - midY, to.x - midX);
      const arrowSize = 6;
      ctx.setLineDash([]);
      ctx.fillStyle = COLORS.yellow;
      ctx.beginPath();
      ctx.moveTo(to.x, to.y - 16);
      ctx.lineTo(
        to.x - arrowSize * Math.cos(angle - 0.4),
        to.y - 16 - arrowSize * Math.sin(angle - 0.4)
      );
      ctx.lineTo(
        to.x - arrowSize * Math.cos(angle + 0.4),
        to.y - 16 - arrowSize * Math.sin(angle + 0.4)
      );
      ctx.closePath();
      ctx.fill();
    });

    ctx.restore();
  }

  // ─────────────────────────────────────
  //  绘制单个小人（32x32 像素）
  // ─────────────────────────────────────
  _drawPlayer(ctx, player) {
    const { x, y, role, state, isLeader, eliminated, name, floatOffset } = player;
    const bodyColor = ROLE_COLORS[role] || COLORS.gray;
    const t = this.animFrame;

    ctx.save();
    ctx.translate(x, y);

    // 动画偏移
    let offsetY = 0;
    let rotation = 0;
    let alpha = 1;
    let scaleX = 1;

    switch (state) {
      case 'idle':
        // 呼吸浮动
        offsetY = Math.sin(t * 0.05 + floatOffset) * 2;
        break;
      case 'speaking':
        // 弹跳
        offsetY = -Math.abs(Math.sin(t * 0.15)) * 6;
        break;
      case 'pointing':
        // 轻微倾斜
        offsetY = Math.sin(t * 0.05) * 1;
        rotation = -0.15;
        break;
      case 'voting':
        // 举手（上移）
        offsetY = -3;
        break;
      case 'celebrating':
        // 快速跳
        offsetY = -Math.abs(Math.sin(t * 0.2)) * 8;
        break;
      case 'defeated':
        // 倒下
        rotation = Math.PI / 6; // 30度倾斜
        offsetY = 4;
        break;
      case 'possessed':
        // 闪烁
        offsetY = Math.sin(t * 0.05 + floatOffset) * 2;
        alpha = 0.5 + 0.5 * Math.sin(t * 0.1);
        break;
    }

    ctx.translate(0, offsetY);
    ctx.rotate(rotation);
    ctx.globalAlpha = alpha;

    // 身体（16x20，居中）
    const bx = -8;
    const by = -10;
    ctx.fillStyle = bodyColor;
    ctx.fillRect(bx, by, 16, 20);

    // 身体高光（左上亮条）
    ctx.fillStyle = 'rgba(255,255,255,0.15)';
    ctx.fillRect(bx, by, 4, 20);

    // 头（12x12，在身体上方）
    const hx = -6;
    const hy = -24;
    ctx.fillStyle = COLORS.skin;
    ctx.fillRect(hx, hy, 12, 12);
    // 头发
    ctx.fillStyle = '#5a3a1a';
    ctx.fillRect(hx, hy, 12, 5);
    // 眼睛
    ctx.fillStyle = '#000';
    ctx.fillRect(hx + 3, hy + 7, 2, 2);
    ctx.fillRect(hx + 7, hy + 7, 2, 2);

    // 腿（2x 6x8）
    ctx.fillStyle = '#2a2a3a';
    ctx.fillRect(bx + 2, by + 20, 5, 8);
    ctx.fillRect(bx + 9, by + 20, 5, 8);

    // 手臂
    if (state === 'voting' || state === 'celebrating') {
      // 举手状态：手臂上移
      ctx.fillStyle = COLORS.skin;
      ctx.fillRect(bx - 4, by - 4, 4, 12);
      ctx.fillRect(bx + 16, by - 4, 4, 12);
    } else {
      ctx.fillStyle = COLORS.skin;
      ctx.fillRect(bx - 3, by + 4, 3, 10);
      ctx.fillRect(bx + 16, by + 4, 3, 10);
    }

    // 配件（头顶）
    this._drawAccessory(ctx, role, hx, hy);

    // 队长标记
    if (isLeader) {
      ctx.fillStyle = COLORS.yellow;
      // 小皇冠在头上方
      ctx.fillRect(hx + 2, hy - 6, 8, 4);
      ctx.fillRect(hx + 1, hy - 4, 10, 2);
      // 皇冠尖
      ctx.fillRect(hx + 2, hy - 8, 2, 2);
      ctx.fillRect(hx + 6, hy - 8, 2, 2);
    }

    // 名字标签
    ctx.globalAlpha = eliminated ? 0.3 : 1;
    ctx.fillStyle = eliminated ? COLORS.textDim : COLORS.text;
    ctx.font = '8px "Press Start 2P", monospace';
    ctx.textAlign = 'center';
    const displayName = name.length > 4 ? name.slice(0, 4) : name;
    ctx.fillText(displayName, 0, 28);

    // 淘汰标记
    if (eliminated) {
      ctx.strokeStyle = COLORS.red;
      ctx.lineWidth = 2;
      ctx.beginPath();
      ctx.moveTo(-12, -16);
      ctx.lineTo(12, 12);
      ctx.moveTo(12, -16);
      ctx.lineTo(-12, 12);
      ctx.stroke();
    }

    // 投票结果牌（voting 状态结束后的短暂显示）
    if (player.voteResult !== undefined) {
      const vy = -32;
      ctx.fillStyle = player.voteResult ? COLORS.green : COLORS.red;
      ctx.fillRect(-8, vy, 16, 10);
      ctx.fillStyle = COLORS.white;
      ctx.font = '6px "Press Start 2P", monospace';
      ctx.textAlign = 'center';
      ctx.fillText(player.voteResult ? '✓' : '✗', 0, vy + 8);
    }

    ctx.restore();
  }

  // ─────────────────────────────────────
  //  配件绘制
  // ─────────────────────────────────────
  _drawAccessory(ctx, role, hx, hy) {
    switch (role) {
      case 'engineer':
        // 扳手：横向 L 形
        ctx.fillStyle = '#c0c0c0';
        ctx.fillRect(hx + 12, hy + 2, 6, 2);  // 横杆
        ctx.fillRect(hx + 16, hy, 2, 6);       // 竖杆
        ctx.fillStyle = '#909090';
        ctx.fillRect(hx + 12, hy, 4, 2);       // 扳手头
        break;

      case 'infiltrator':
        // 面具：小圆块 + 眼洞
        ctx.fillStyle = '#800020';
        ctx.fillRect(hx - 2, hy + 2, 4, 6);
        ctx.fillStyle = '#000';
        ctx.fillRect(hx - 1, hy + 3, 1, 2);    // 左眼
        ctx.fillRect(hx + 1, hy + 3, 1, 2);    // 右眼
        break;

      case 'signal_keeper':
        // 天线：竖线 + 顶部圆点
        ctx.fillStyle = '#60ff60';
        ctx.fillRect(hx + 5, hy - 6, 2, 6);    // 天线杆
        ctx.fillRect(hx + 4, hy - 8, 4, 3);    // 天线头
        // 闪烁效果
        if (this.animFrame % 30 < 15) {
          ctx.fillStyle = 'rgba(96,255,96,0.5)';
          ctx.fillRect(hx + 3, hy - 10, 6, 2);
        }
        break;
    }
  }

  // ─────────────────────────────────────
  //  辅助：显示投票结果（匿名化）
  // ─────────────────────────────────────
  showVoteResults(votes) {
    // votes: { approveCount, rejectCount }
    // 匿名化显示，不显示个人投票
    this.players.forEach(p => {
      p.voteResult = null; // 清除之前的投票结果
    });
    // 3 秒后清除
    setTimeout(() => {
      this.players.forEach(p => { delete p.voteResult; });
    }, 3000);
  }
}

// 确保全局可访问
if (typeof window !== 'undefined') {
  window.PlayerStage = PlayerStage;
}
