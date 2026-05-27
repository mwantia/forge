package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const schema = `
CREATE TABLE IF NOT EXISTS forge_kv (
	key  TEXT  PRIMARY KEY,
	data BYTEA NOT NULL DEFAULT ''::bytea
);
CREATE INDEX IF NOT EXISTS forge_kv_prefix ON forge_kv (key text_pattern_ops)`

type PostgresBackend struct {
	dsn      string
	maxConns int32
	pool     *pgxpool.Pool
	logger   hclog.Logger
}

func NewPostgresBackend(logger hclog.Logger, dsn string, maxConns int32) *PostgresBackend {
	return &PostgresBackend{
		dsn:      dsn,
		maxConns: maxConns,
		logger:   logger,
	}
}

func (b *PostgresBackend) PostInit(ctx context.Context) error {
	cfg, err := pgxpool.ParseConfig(b.dsn)
	if err != nil {
		return fmt.Errorf("postgres: invalid DSN: %w", err)
	}
	if b.maxConns > 0 {
		cfg.MaxConns = b.maxConns
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("postgres: connect: %w", err)
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		pool.Close()
		return fmt.Errorf("postgres: schema init: %w", err)
	}
	b.pool = pool
	b.logger.Info("connected", "dsn_prefix", safeDSN(b.dsn))
	return nil
}

func (b *PostgresBackend) Cleanup(_ context.Context) error {
	if b.pool != nil {
		b.pool.Close()
	}
	return nil
}

func (b *PostgresBackend) Serve(_ context.Context) error { return nil }

func (b *PostgresBackend) ReadRaw(ctx context.Context, key string) ([]byte, error) {
	var data []byte
	err := b.pool.QueryRow(ctx, `SELECT data FROM forge_kv WHERE key = $1`, key).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: read %q: %w", key, err)
	}
	if len(data) == 0 {
		return nil, nil
	}
	return data, nil
}

func (b *PostgresBackend) ReadJson(ctx context.Context, key string, v any) error {
	raw, err := b.ReadRaw(ctx, key)
	if err != nil {
		return err
	}
	if raw == nil {
		return nil
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("postgres: unmarshal %q: %w", key, err)
	}
	return nil
}

func (b *PostgresBackend) WriteRaw(ctx context.Context, key string, value []byte) error {
	_, err := b.pool.Exec(ctx,
		`INSERT INTO forge_kv (key, data) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET data = EXCLUDED.data`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("postgres: write %q: %w", key, err)
	}
	return nil
}

func (b *PostgresBackend) WriteJson(ctx context.Context, key string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("postgres: marshal %q: %w", key, err)
	}
	return b.WriteRaw(ctx, key, data)
}

// CreateEntry stores an empty-data sentinel for the key. Idempotent.
func (b *PostgresBackend) CreateEntry(ctx context.Context, key string) error {
	_, err := b.pool.Exec(ctx,
		`INSERT INTO forge_kv (key, data) VALUES ($1, ''::bytea) ON CONFLICT DO NOTHING`,
		key,
	)
	if err != nil {
		return fmt.Errorf("postgres: create entry %q: %w", key, err)
	}
	return nil
}

// ListEntry returns the immediate children of prefix. Entries that have
// deeper descendants are returned as "segment/" (with trailing slash);
// leaf entries are returned without a slash. Matches the file backend contract.
func (b *PostgresBackend) ListEntry(ctx context.Context, prefix string) ([]string, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if prefix == "" {
		rows, err = b.pool.Query(ctx, `SELECT key FROM forge_kv ORDER BY key`)
	} else {
		rows, err = b.pool.Query(ctx,
			`SELECT key FROM forge_kv WHERE key LIKE $1 ESCAPE '!' ORDER BY key`,
			escapeLike(prefix)+"%",
		)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: list %q: %w", prefix, err)
	}
	defer rows.Close()

	seen := make(map[string]struct{})
	var out []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("postgres: list scan: %w", err)
		}
		rest := strings.TrimPrefix(key, prefix)
		if rest == "" {
			continue
		}
		idx := strings.IndexByte(rest, '/')
		var entry string
		if idx < 0 {
			entry = rest
		} else {
			entry = rest[:idx+1]
		}
		if _, ok := seen[entry]; !ok {
			seen[entry] = struct{}{}
			out = append(out, entry)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list rows: %w", err)
	}
	return out, nil
}

func (b *PostgresBackend) DeleteEntry(ctx context.Context, key string) error {
	_, err := b.pool.Exec(ctx, `DELETE FROM forge_kv WHERE key = $1`, key)
	if err != nil {
		return fmt.Errorf("postgres: delete %q: %w", key, err)
	}
	return nil
}

func (b *PostgresBackend) DeletePrefix(ctx context.Context, prefix string) error {
	if prefix == "" {
		if _, err := b.pool.Exec(ctx, `DELETE FROM forge_kv`); err != nil {
			return fmt.Errorf("postgres: delete all: %w", err)
		}
		return nil
	}
	_, err := b.pool.Exec(ctx,
		`DELETE FROM forge_kv WHERE key LIKE $1 ESCAPE '!'`,
		escapeLike(prefix)+"%",
	)
	if err != nil {
		return fmt.Errorf("postgres: delete prefix %q: %w", prefix, err)
	}
	return nil
}

// escapeLike escapes LIKE special chars using '!' as the escape character.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "!", "!!")
	s = strings.ReplaceAll(s, "%", "!%")
	s = strings.ReplaceAll(s, "_", "!_")
	return s
}

// safeDSN strips the password from a DSN for logging.
func safeDSN(dsn string) string {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return "<invalid>"
	}
	cfg.ConnConfig.Password = "***"
	return cfg.ConnString()
}
