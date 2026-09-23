# Render 免费服务接入 Neon PostgreSQL

本次只为《摸鱼德州》好友模式接入持久化。其他三款游戏、等待房间及进行中的牌局仍在内存中。

## 保存哪些数据

- 浏览器匿名玩家身份（服务端只存凭证的 SHA-256 摘要）、昵称。
- 每局结果、结算时间、真人/电脑席位、开局与结算豆豆余额、底池分配。
- 累计局数、获利局数、累计净赢、最近 20 局战绩。
- 不保存未公开手牌、剩余牌堆或完整过程回放。

每次新开桌仍从 1,000 虚拟豆豆开始。保存的余额是每局结算记录，不是充值钱包，也不改变重新开桌规则。进入牌桌的“我的战绩”可查看已保存记录。

新身份保存在当前浏览器的 localStorage。刷新、关闭后重开、在不同房间游玩会沿用身份；清理网站数据、换浏览器/设备会变成新玩家，目前没有账号登录或跨设备找回。同一浏览器多个标签页属于同一玩家，模拟多个真人请用不同浏览器或独立隐私会话。旧版正在进行的房间仍兼容原标签页凭证，之后新房使用持久身份。

## 1. 创建 Neon 免费数据库

1. 打开 https://console.neon.tech ，注册/登录，选择 Free 套餐。
2. 创建项目（如 `whoisai`），使用 PostgreSQL 16 或更新版本。区域尽量靠近 Render 服务区域；本仓库 Render 默认 Singapore，优先选择可用的新加坡区域。
3. 在项目 **Connect** 面板选择数据库与角色，开启 **Connection pooling**，复制 PostgreSQL 连接 URL。应包含用户名、密码、数据库名与 TLS 参数，池化主机名通常包含 `-pooler`。
4. 保留 Neon 给出的 TLS 参数（例如 `sslmode=require&channel_binding=require`）；不要在远端设置 `sslmode=disable`。连接池驱动不使用持久 prepared statement，可用于池化连接。

不要把真实连接串贴到聊天、提交 Git 或写入前端代码。

## 2. 配置现有 Render 服务

1. 打开 Render Dashboard → 当前网站服务 → **Environment**。
2. 添加 `DATABASE_URL`，值为 Neon Connect 面板复制的完整连接 URL（不是 `psql '…'` 命令，不含外层引号）。
3. 保存并重新部署包含此代码的最新版本。Web Service 保持 **Free**，无需额外创建 Render Postgres，也无需添加持久磁盘。
4. 启动时自动建立 `poker_schema_migrations`、`poker_players`、`poker_hands`、`poker_hand_players`，无需手动执行 SQL。账号需拥有目标 schema 的建表和读写权限。
5. 日志看到 `Poker history storage: PostgreSQL ready` 表示连接与迁移成功。进入德州扑克后，页脚显示“已结算战绩长期保存”。

新建 Blueprint 部署时，`render.yaml` 会提示手动填写 `DATABASE_URL`。已有服务请在 Environment 中设置。未配置或留空时仍能试玩，日志与页面会明确标注内存模式；配置非空但连接或建表失败时停止启动，不会静默退回内存。

本次只运行 `internal/poker/migrations/001_records.sql`。不要执行 `internal/platform/migrations/001_platform.sql`：该文件是平台整体设计稿，不是本功能的运行时迁移。

## 3. 验证落库

1. 创建 2 人好友桌（1 真人 + 1 电脑），完成至少一局。
2. 打开“我的战绩”，确认局数、净赢和上局余额。
3. 在 Neon SQL Editor 查询：

```sql
SELECT id, room_code, hand_number, ended_at, result
FROM poker_hands ORDER BY ended_at DESC LIMIT 10;
```

4. 手动重启 Render 服务，用同一浏览器打开 `/games/texas-holdem/`，查看“我的战绩”。记录应保留，原房间不恢复。

结算使用单个事务，按随机手局 ID 幂等保存。遇到暂时断库会显示“待保存”并每 5 秒重试，在保存成功前禁止下一局、回等待房或离开；不让失败的结算被覆盖。若此时服务进程也被终止，未提交的结果仍会丢失；已提交的事务不受影响。数据库故障不会被伪装为保存成功。

每 700ms 的牌桌状态轮询只读内存，不查询数据库。数据库只在入房登记、结算及查看战绩时使用；连接池最多 4 条连接，空闲连接会释放，减少免费额度消耗。

## 本地运行与测试

复制 `.env.example` 到 `.env`，填写本地或专门测试数据库的连接 URL，然后 `npm start`。`.env` 已被 Git 忽略。

普通回归：

```sh
npm test
```

PostgreSQL 集成测试需要专用测试库。通过环境变量 `POKER_TEST_DATABASE_URL` 提供 PostgreSQL URL 后执行：

```sh
go test -race ./internal/poker
```

集成测试会创建独立随机 schema，完成后只清理该 schema，覆盖重复/并发结算、事务回滚、迁移重跑、重新连接读取、历史隔离及查询上限。未设置测试 URL 时明确跳过数据库集成测试，不代表已验证真实 PostgreSQL。

## 免费套餐边界

Render 的休眠、冷启动与网络额度限制仍然存在；Neon 的存储、计算与网络流量也受免费配额限制。请以各平台控制台当前套餐为准。

参考：[Render Free](https://render.com/docs/free)、[Neon Go 连接](https://neon.com/docs/guides/go)、[Neon 免费套餐](https://neon.com/docs/introduction/free-tier)。
