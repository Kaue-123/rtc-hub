// Package postgres é o adaptador de saída de persistência.
package postgres

import (
	"context"
	"embed"
	"fmt"
	"sort"

	pgxdecimal "github.com/jackc/pgx-shopspring-decimal"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migracoes embed.FS

// Conectar abre o pool e registra o tipo decimal.
//
// Sem o registro, pgx lê numeric como pgtype.Numeric e o scan em
// decimal.Decimal falha em tempo de execução — não de compilação.
func Conectar(ctx context.Context, url string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("url do banco: %w", err)
	}
	config.AfterConnect = func(_ context.Context, conexao *pgx.Conn) error {
		pgxdecimal.Register(conexao.TypeMap())
		return nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

// Migrar aplica as migrações pendentes, em ordem de nome, uma vez cada.
// Substitui o create_all: esquema de produção precisa de histórico.
func Migrar(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migracao (
			nome        TEXT PRIMARY KEY,
			aplicada_em TIMESTAMPTZ NOT NULL DEFAULT now()
		)`)
	if err != nil {
		return nil, fmt.Errorf("criar controle de migração: %w", err)
	}

	entradas, err := migracoes.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	nomes := make([]string, 0, len(entradas))
	for _, entrada := range entradas {
		nomes = append(nomes, entrada.Name())
	}
	sort.Strings(nomes)

	var aplicadas []string
	for _, nome := range nomes {
		var existe bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM schema_migracao WHERE nome = $1)`, nome).Scan(&existe)
		if err != nil {
			return aplicadas, err
		}
		if existe {
			continue
		}

		sql, err := migracoes.ReadFile("migrations/" + nome)
		if err != nil {
			return aplicadas, err
		}

		// Cada migração numa transação: esquema pela metade é pior que falha.
		tx, err := pool.Begin(ctx)
		if err != nil {
			return aplicadas, err
		}
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			_ = tx.Rollback(ctx)
			return aplicadas, fmt.Errorf("migração %s: %w", nome, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migracao (nome) VALUES ($1)`, nome); err != nil {
			_ = tx.Rollback(ctx)
			return aplicadas, err
		}
		if err := tx.Commit(ctx); err != nil {
			return aplicadas, err
		}
		aplicadas = append(aplicadas, nome)
	}
	return aplicadas, nil
}
