package poker

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/001_records.sql
var recordsSchema string

var ErrRecordsUnavailable = errors.New("战绩存储暂不可用，请稍后重试")

// PostgresRecords uses a small, idle-friendly pool for Render and Neon free plans.
type PostgresRecords struct{ pool *pgxpool.Pool }

func OpenPostgresRecords(ctx context.Context, url string) (*PostgresRecords, error) {
	config, err := pgxpool.ParseConfig(url)
	// Driver errors may contain a connection string. Never return them to HTTP or logs.
	if err != nil {
		return nil, errors.New("DATABASE_URL 格式无效")
	}
	config.MaxConns = 4
	config.MinConns = 0
	config.MaxConnIdleTime = time.Minute
	config.MaxConnLifetime = 30 * time.Minute
	config.ConnConfig.ConnectTimeout = 5 * time.Second
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, ErrRecordsUnavailable
	}
	db := &PostgresRecords{pool: pool}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, errors.New("无法连接 PostgreSQL，请检查 DATABASE_URL、网络与数据库状态")
	}
	if err = db.migrate(ctx); err != nil {
		pool.Close()
		return nil, errors.New("PostgreSQL 初始化失败，请检查建表权限与数据库版本")
	}
	return db, nil
}
func (db *PostgresRecords) Close()     { db.pool.Close() }
func (*PostgresRecords) Durable() bool { return true }
func (db *PostgresRecords) migrate(ctx context.Context) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(734829163)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS poker_schema_migrations (version integer PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM poker_schema_migrations WHERE version=1)`).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		if _, err = tx.Exec(ctx, recordsSchema, pgx.QueryExecModeSimpleProtocol); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO poker_schema_migrations(version) VALUES(1)`); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (db *PostgresRecords) Player(ctx context.Context, id, name string) error {
	_, err := db.pool.Exec(ctx, `INSERT INTO poker_players(id,display_name) VALUES($1,$2) ON CONFLICT(id) DO UPDATE SET display_name=EXCLUDED.display_name,updated_at=now()`, id, name)
	if err != nil {
		return ErrRecordsUnavailable
	}
	return nil
}
func (db *PostgresRecords) SaveHand(ctx context.Context, h HandRecord) error {
	if err := validRecord(h); err != nil {
		return err
	}
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return ErrRecordsUnavailable
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `INSERT INTO poker_hands(id,room_code,hand_number,ended_at,result) VALUES($1,$2,$3,$4,$5) ON CONFLICT(id) DO NOTHING`, h.ID, h.RoomCode, h.Hand, h.EndedAt, h.Result)
	if err != nil {
		return ErrRecordsUnavailable
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	for _, p := range h.Players {
		var id any
		if !p.Bot {
			id = p.ProfileID
		}
		_, err = tx.Exec(ctx, `INSERT INTO poker_hand_players(hand_id,seat,player_id,display_name,is_bot,start_chips,end_chips,payout) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, h.ID, p.Seat, id, p.Name, p.Bot, p.StartChips, p.EndChips, p.Payout)
		if err != nil {
			return ErrRecordsUnavailable
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return ErrRecordsUnavailable
	}
	return nil
}
func (db *PostgresRecords) History(ctx context.Context, id string) (History, error) {
	h := History{Recent: []HistoryEntry{}}
	// One snapshot keeps the aggregate and recent rows consistent during settlement.
	tx, err := db.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return h, ErrRecordsUnavailable
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `SELECT display_name FROM poker_players WHERE id=$1`, id).Scan(&h.Name)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return h, ErrRecordsUnavailable
	}
	err = tx.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE end_chips>start_chips),COALESCE(sum(end_chips-start_chips),0) FROM poker_hand_players WHERE player_id=$1`, id).Scan(&h.Hands, &h.Wins, &h.Net)
	if err != nil {
		return h, ErrRecordsUnavailable
	}
	rows, err := tx.Query(ctx, `SELECT h.id,h.room_code,h.hand_number,h.ended_at,h.result,p.start_chips,p.end_chips FROM poker_hand_players p JOIN poker_hands h ON h.id=p.hand_id WHERE p.player_id=$1 ORDER BY h.ended_at DESC,h.id DESC LIMIT 20`, id)
	if err != nil {
		return h, ErrRecordsUnavailable
	}
	for rows.Next() {
		var e HistoryEntry
		if err = rows.Scan(&e.ID, &e.RoomCode, &e.Hand, &e.EndedAt, &e.Result, &e.StartChips, &e.EndChips); err != nil {
			rows.Close()
			return h, ErrRecordsUnavailable
		}
		e.Net = e.EndChips - e.StartChips
		h.Recent = append(h.Recent, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return h, ErrRecordsUnavailable
	}
	if len(h.Recent) > 0 {
		chips := h.Recent[0].EndChips
		h.LastChips = &chips
	}
	if err = tx.Commit(ctx); err != nil {
		return h, fmt.Errorf("读取战绩: %w", ErrRecordsUnavailable)
	}
	return h, nil
}
