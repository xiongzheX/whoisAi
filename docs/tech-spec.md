# 谁是AI 技术设计文档

## 1. 技术栈

- Node.js
- Express
- Socket.IO
- 静态 HTML / CSS / JS

## 2. 架构原则

- 服务端是唯一状态源
- 前端只负责展示和发操作
- 所有关键流程由 Socket 事件驱动
- 先保证可运行，再做模块化拆分

## 3. 目标目录结构

```text
/Users/xiongzhe/myIdea/谁是AI/
  package.json
  server/
    server.js
    roomManager.js
    phaseEngine.js
    aiEngine.js
    eventMap.js
  client/
    index.html
    css/
      style.css
    js/
      game.js
  assets/
    images/
    sprites/
    backgrounds/
  test/
    simulate.js
  docs/
    product-spec.md
    tech-spec.md
    migration-checklist.md
```

## 4. 模块职责

### server.js
- 启动 Express 和 Socket.IO
- 注册静态资源
- 连接业务模块

### roomManager.js
- 创建和清理房间
- 管理玩家加入、离开和重置
- 维护房间状态

### phaseEngine.js
- 管理阶段推进
- 控制定时器
- 处理回合流转

### aiEngine.js
- 分配 AI 身份
- 控制 AI 行为风格
- 生成 AI 相关动作或伪装反馈
- 生成 `侦探 / AI / 间谍` 对应的不同语气与行为差异

### eventMap.js
- 统一事件名
- 避免前后端字符串分散

## 5. 状态模型

建议房间状态至少包含：

```text
room = {
  players: [],
  roles: {
    detectives: [],
    spies: [],
    ai: []
  },
  status: 'waiting' | 'roundIntro' | 'action' | 'clueCollecting' | 'defending' | 'summary' | 'finished',
  mode: 'normal' | 'test' | 'solo',
  roundNumber: number,
  currentWord: string,
  currentWordPair: [string, string],
  currentPhaseTimer: timer,
  voteCount: {},
  questionCount: {},
  roundActions: {},
  roundSummary: {},
  roundEventHistory: [],
  aiProfile: {},
  spyTasks: {},
  roleFeedback: {},
  matchScores: {},
  matchResolved: boolean
}
```

## 6. Socket 事件建议

### 基础事件
- `joinRoom`
- `playerJoined`
- `gameStarted`
- `phaseChange`
- `submitDescription`
- `questionAsked`
- `voteResult`
- `gameFinished`
- `restartGame`

### 建议新增
- `aiAssigned`
- `spyAssigned`
- `clueGenerated`
- `suspicionUpdated`
- `roundScored`
- `identityRevealed`

## 6.1 角色与任务事件

### `aiAssigned`
- 服务端内部生成 AI 身份列表
- 用于 AI 行为模块和房间状态初始化
- 前端只接收与自己有关的提示，不接收完整名单

### `spyAssigned`
- 只下发给被选中的间谍本人
- 可包含本局间谍任务摘要
- 同步给 AI 行为模块，用于生成协同策略

### `identityRevealed`
- 结算阶段统一公开本局角色结果
- 前端据此展示本局侦探、间谍和 AI 的最终归属
- 仅在回合结束后发出，避免中途泄露

### `roundScored`
- 下发本轮角色表现分
- 建议包含：
  - `playerId`
  - `role`
  - `baseScore`
  - `performanceScore`
  - `achievementScore`
  - `rankDelta`
  - `reasonTags`
- 前端用于展示“这轮为什么加分或扣分”

### `suspicionUpdated`
- 用于更新玩家怀疑度
- 建议只广播聚合结果，不广播推导过程
- 前端可展示当前最可疑目标或风险提示

## 6.2 间谍任务回传

间谍任务不应该只停留在规则文档里，服务端需要在状态里保存完成情况，方便结算时给出清晰反馈。

建议 `spyTasks` 结构包含：
- `personaMatch`
- `voteMisleadCount`
- `aiCoverCount`
- `survivalRounds`
- `exposed`

结算时建议回传给前端：
- `taskName`
- `taskStatus`
- `taskScore`
- `taskReason`

## 6.3 结算展示字段

结算页建议同时拿到以下字段：
- `matchScores`
- `roleFeedback`
- `aiProfile`
- `roundEventHistory`
- `identityRevealed`

其中：
- `matchScores` 用于显示积分变化
- `roleFeedback` 用于显示每个角色的高光和失误
- `aiProfile` 用于显示 AI 行为画像
- `roundEventHistory` 用于显示本局事件轨迹
- `identityRevealed` 用于显示最终身份公开结果

## 6.4 推荐 payload 结构

### `spyAssigned`
```text
{
  playerId: string,
  role: 'spy',
  tasks: {
    personaMatch: boolean,
    voteMisleadCount: number,
    aiCoverCount: number,
    survivalRounds: number
  },
  aiAware: true
}
```

### `roundScored`
```text
{
  playerId: string,
  role: 'detective' | 'spy' | 'ai',
  baseScore: number,
  performanceScore: number,
  achievementScore: number,
  rankDelta: number,
  reasonTags: string[]
}
```

### `identityRevealed`
```text
{
  detectives: string[],
  spies: string[],
  ai: string[],
  revealedAt: number
}
```

### `gameFinished`
```text
{
  winner: 'detective' | 'ai' | 'spy' | 'mixed',
  matchScores: Record<string, number>,
  roleFeedback: Record<string, string[]>,
  aiProfile: Record<string, unknown>,
  roundEventHistory: Array<Record<string, unknown>>
}
```

## 6.5 前端展示映射

前端页面建议把服务端字段映射到以下区域：

- `spyAssigned` -> 间谍本人提示区
- `roundScored` -> 回合结算提示条
- `suspicionUpdated` -> 当前怀疑目标提示
- `identityRevealed` -> 结算页身份公开卡片
- `matchScores` -> 结算页积分变化区
- `roleFeedback` -> 结算页高光 / 失误总结
- `aiProfile` -> 结算页 AI 行为画像
- `roundEventHistory` -> 结算页事件回放

## 7. 前后端契约原则

- 服务端只发送事实，不发送 UI 逻辑
- 前端只负责渲染，不自行判定胜负
- 每个事件的 payload 字段固定
- 阶段切换必须由服务端统一完成
- 间谍身份只对自身和 AI 可见，对其他侦探隐藏

## 8. 资源管理

建议统一将可复用资源收拢到：

```text
assets/images
assets/sprites
assets/backgrounds
```

这样后续迁移目录时不容易出现资源断链。

## 9. 测试策略

### 基础测试
- 能启动服务
- 能加载首页
- 能加入房间
- 能开局
- 能切换阶段
- 能完成结算

### 联机测试
- 多客户端同步正常
- 断线处理正常
- 房间状态可恢复
- 间谍身份在客户端展示和结算中的可见性正确

### 规则测试
- 侦探、AI、间谍三种角色命名一致
- AI 能读取间谍身份并做出协同行为
- 结算页能正确展示角色表现分和 AI 行为画像

### 资源测试
- 图片路径无 404
- 像素风样式正常
- 迁移目录后界面不乱
