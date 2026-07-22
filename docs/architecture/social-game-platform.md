# 社交小游戏平台底层设计

设计日期：2026-07-20
状态：第一阶段实施基线

## 1. 目标与边界

平台承载持续存在的好友房间，一个房间可以连续开启多局、复玩或切换游戏。《谁是 AI》是首个正式游戏；第二个游戏接入后不应修改房间、成员、邀请和重连的公共逻辑。

第一阶段不开放任意第三方代码上传。游戏由平台注册并由 Go 服务端权威运行，避免在产品模型尚未稳定时引入代码沙箱、版本兼容和内容审核问题。

## 2. 领域边界

```text
GameDefinition 游戏目录
        |
        v
PartyRoom 社交房间 1 ─────── n GameSession 单局游戏
        |                              |
        n                              n
   RoomMember                   SessionParticipant
                                       |
                         Snapshot / PrivateSnapshot / Event
```

### PartyRoom

负责跨游戏持续存在的关系：

- 房间码、房主、当前选择的游戏。
- 真人成员、座位、准备状态、在线状态。
- 当前活动单局和房间版本。
- 换游戏、原班复玩和房间聊天。

AI 不属于长期房间成员。AI 是某个 `GameSession` 的参与者，复玩时可以重新生成。

### GameSession

负责一局游戏的生命周期：

- 唯一 `sessionId`，同一房间内递增的 `sequence`。
- 游戏类型、模式和开局设置。
- 参与者快照，包括当局 AI。
- 状态版本、开始/结束时间和胜负摘要。

每次复玩都创建新 Session，不复用上一局的数据容器。

### Snapshot 与 Event

- `public_state`：所有参与者都能看到且刷新后必须恢复的事实。
- `private_state`：按参与者裁剪的身份、私密线索和本人动作状态。
- `event`：审计与回放账本；界面临时动画不作为恢复依据。
- `version`：单局内单调递增。写入必须使用乐观并发检查。

## 3. 标识设计

| 标识 | 用途 | 生命周期 | 是否公开 |
| --- | --- | --- | --- |
| `room_id` | 数据库内部房间主键 | 房间生命周期 | 否 |
| `room_code` | 邀请码 | 可轮换 | 是 |
| `member_id` | 房间成员身份 | 房间生命周期 | 是 |
| `resume_token_hash` | 恢复成员身份 | 可撤销/轮换 | 否 |
| `session_id` | 一局游戏 | 单局 | 是 |
| `game_id` | 游戏类型，如 `who-is-ai` | 长期稳定 | 是 |
| `version` | 单局状态顺序 | 单局递增 | 是 |

当前客户端继续持有原有 `playerToken` 以兼容本地恢复，但服务端会按“房间 + 令牌”单向派生公开 `member_id/player_id`；成员列表和游戏事件不再广播原始恢复令牌。引入数据库后服务端只保存令牌哈希，并支持撤销与轮换。

## 4. 关系表

### `games`

平台游戏目录。`id` 是代码和协议使用的稳定键；展示名称允许修改。

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `id` | text | PK |
| `slug` | text | UNIQUE |
| `name` | text | NOT NULL |
| `status` | text | active / coming_soon / disabled |
| `min_players` | smallint | > 0 |
| `max_players` | smallint | >= min |
| `supports_ai` | boolean | NOT NULL |
| `manifest` | jsonb | 游戏展示与能力声明 |

### `party_rooms`

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `id` | uuid | PK |
| `code` | varchar(16) | UNIQUE，大小写归一化 |
| `host_member_id` | uuid | FK room_members |
| `selected_game_id` | text | FK games |
| `active_session_id` | uuid | FK game_sessions，可空 |
| `status` | text | open / in_game / closed |
| `version` | bigint | 乐观锁，>= 1 |
| `created_at/updated_at` | timestamptz | NOT NULL |

### `party_room_members`

只保存真人社交席位；断线不会删除记录。

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `id` | uuid | PK |
| `room_id` | uuid | FK party_rooms |
| `user_id` | uuid | 未来账号体系，可空 |
| `display_name` | varchar(32) | NOT NULL |
| `seat` | smallint | 同房间的未离开成员唯一，left 记录不占席 |
| `role` | text | host / player |
| `connection_status` | text | online / offline / left |
| `ready` | boolean | NOT NULL |
| `resume_token_hash` | bytea | 同房间的未离开成员唯一 |
| `joined_at/last_seen_at` | timestamptz | NOT NULL |

### `game_sessions`

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `id` | uuid | PK |
| `room_id` | uuid | FK party_rooms |
| `game_id` | text | FK games |
| `sequence` | bigint | 同房间唯一递增 |
| `status` | text | created / confirming / running / finished / abandoned |
| `mode` | text | normal / test / solo 等游戏模式 |
| `settings` | jsonb | 开局后冻结 |
| `state_version` | bigint | >= 0 |
| `started_at/ended_at` | timestamptz | 可空 |
| `result_summary` | jsonb | 结算摘要，可空 |

同一房间最多只有一条 `created/confirming/running` Session，由部分唯一索引保证。

### `game_session_participants`

冻结当局阵容，不因房间成员改名或离开而改变历史。

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `id` | uuid | PK |
| `room_id` | uuid | 与 session/member 组成复合外键，防止跨房引用 |
| `session_id` | uuid | FK game_sessions |
| `room_member_id` | uuid | 真人成员，可空 |
| `participant_key` | text | 当局稳定键，同局唯一 |
| `display_name` | varchar(32) | 当局快照 |
| `kind` | text | human / bot |
| `seat` | smallint | 同局唯一 |

`human` 必须绑定同房间成员，`bot` 必须不绑定成员；该约束同时存在于数据库和领域层。

### `game_snapshots` 与 `game_private_snapshots`

公共快照主键为 `(session_id, version)`；私密快照主键为 `(session_id, version, participant_id)`。公开和私密状态必须在同一事务中写入，提交后再广播。

快照按版本保留可支持回放；成本需要控制时，可保留每轮关键帧和最新完整快照，详细过程由事件账本承担。

### `game_events`

追加写入的事实账本，包含 `session_id`、`version`、`round`、`actor_participant_id`、`event_type`、`scope`、`target_participant_id`、`payload` 和服务端时间。

所有客户端动作必须携带 `action_id`，数据库用 `(session_id, actor_participant_id, action_id)` 唯一约束实现幂等。

### `party_room_messages`

房间聊天与游戏证据分开：房间聊天可以跨局持续；游戏内发言必须绑定 `session_id`，新局不会误读上一局发言。

## 5. 状态迁移规则

### 开局

1. 校验房主、游戏状态和人数。
2. 创建 GameSession 与参与者快照。
3. 设置 `party_rooms.active_session_id`，房间进入 `in_game`。
4. 初始化第 0 版公开/私密快照。
5. 提交事务后广播。

### 原班复玩

1. 结束旧 Session，不删除其证据。
2. 新建更高 `sequence` 的 Session。
3. 复制仍在房间里的真人成员，不复制 AI 和游戏内状态。
4. 客户端发现 `sessionId` 变化，先清空派生状态，再应用新快照。

### 换游戏

只有房主能在不存在活动 Session 时更换 `selected_game_id`。目标游戏的 `max_players` 必须容纳当前未离场成员，否则数据库前的领域校验直接拒绝。房间成员和房间聊天保留；游戏设置、AI、身份、证据和计分全部重新创建。

### 双人轻竞技

《豆豆百米赛》和《团子相扑》共享 `internal/duel` 生命周期，但各自由独立模拟引擎结算：

1. 两名真人加入 PartyRoom，第三人由游戏目录的 `max_players = 2` 拒绝。
2. 每名玩家只提交自己的选手配置；公开状态只显示是否准备，不提前公开配点。
3. 双方在线且准备后冻结配置到 `game_sessions.settings.playerConfigs`。
4. 服务端创建新 GameSession，分配随机天赋、环境和种子并生成唯一结果。
5. 完整回放进入第 1 版公共快照，胜者和原因进入 `result_summary`，随后 Session 封存。
6. 任意一方提交新配置即清理上一局准备态；双方再次准备会创建更高 `sequence` 的复赛 Session。

### 重连

重连令牌恢复 `member_id`，然后发送：

1. PartyRoom 快照。
2. 当前 GameSession 的最新公共快照。
3. 当前参与者的同版本私密快照。

不得通过重放零散 Socket 事件拼装权威页面。

## 6. 代码分层

```text
internal/platform     房间、目录、单局、快照和仓储接口
internal/duel         双人游戏共用的配置、准备、复赛和结果状态机
runner_race/duel      百米赛/相扑模拟引擎的公开适配边界
internal/games        游戏驱动注册与通用动作边界（后续阶段）
internal/game         当前《谁是 AI》规则，逐步迁为首个驱动
internal/realtime     Socket/HTTP 适配，不保存领域事实
client                平台壳与各游戏视图
```

第一阶段使用 `platform.MemoryStore`，其对象关系与数据库表一致。生产数据库接入时实现同一仓储接口，领域服务和 Socket 协议不改。

## 7. 实施顺序

1. 建立平台领域类型、游戏目录、内存仓储和数据库迁移。
2. 让现有加入、断线、开局和结算同步到 PartyRoom/GameSession。
3. 为《谁是 AI》增加 Session ID、快照版本和统一恢复包。
4. 把复玩改为新建 Session，旧 Session 只读保留。
5. 增加游戏平台入口和换游戏协议。
6. 接入第二款游戏验证抽象，不为《谁是 AI》继续特判平台层。

## 8. 实施状态

已完成第一阶段平台基线：

- 游戏目录、PartyRoom、GameSession、参与者、快照和事件领域模型。
- 与关系表一致的内存仓储、PostgreSQL 首版迁移和平台 HTTP API。
- 现有进房、断线、开局、结算和复玩已同步到平台 Session。
- 《谁是 AI》公共/私密快照、版本门禁、过期 Session 动作拦截和统一重连包。
- 低饱和轻松风格游戏平台大厅，以及三款正式游戏入口。
- 《豆豆百米赛》《团子相扑》双人房间、双方准备、服务端统一回放、冻结配置与复赛 Session。
- 房主空闲时跨游戏切换、目标游戏人数上限校验，以及跨游戏共享的房间恢复身份。

下一阶段按顺序是：客户端动作 `action_id` 全链路幂等 → 《谁是 AI》身份确认/准备门禁 → 把现有两类运行时收敛到正式 `GameDriver` 接口 → 房间级游戏选择与邀请页。
