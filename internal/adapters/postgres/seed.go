package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/Kaue-123/rtc-hub/internal/adapters/postgres/db"
	"github.com/Kaue-123/rtc-hub/internal/domain"
)

// SemearAliquotasPadrao cria as linhas com tenant_id nulo, se não houver.
// Devolve quantas criou.
//
// Os valores de 2027 vêm dos exemplos da RFB e NÃO são alíquota fixada em lei —
// a observação diz isso e viaja até o relatório de conferência.
func SemearAliquotasPadrao(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	consulta := db.New(pool)

	existentes, err := consulta.ContarAliquotasPadrao(ctx)
	if err != nil {
		return 0, err
	}
	if existentes > 0 {
		return 0, nil
	}

	dia := func(ano int, mes time.Month, d int) time.Time {
		return time.Date(ano, mes, d, 0, 0, 0, 0, time.UTC)
	}
	fim2026 := dia(2026, time.December, 31)

	linhas := []db.CriarAliquotaParams{
		{
			VigenciaInicio: dia(2026, time.January, 1),
			VigenciaFim:    &fim2026,
			Cbs:            decimal.RequireFromString("0.9"),
			IbsEstadual:    decimal.RequireFromString("0.05"),
			IbsMunicipal:   decimal.RequireFromString("0.05"),
			Fonte:          domain.FonteAliquotaPadrao,
			Observacao: ptr("Ano-teste: CBS 0,9% e IBS 0,1%. Em 2026 o motor usa as " +
				"alíquotas de referência dele e estas não são enviadas."),
		},
		{
			VigenciaInicio: dia(2027, time.January, 1),
			Cbs:            decimal.RequireFromString("8.8"),
			IbsEstadual:    decimal.RequireFromString("0.05"),
			IbsMunicipal:   decimal.RequireFromString("0.05"),
			Fonte:          domain.FonteAliquotaPadrao,
			Observacao: ptr("ESTIMATIVA. Valores dos exemplos oficiais da RFB, não " +
				"alíquota fixada em lei. Revisar quando o Senado publicar a de referência."),
		},
	}

	for _, linha := range linhas {
		if _, err := consulta.CriarAliquota(ctx, linha); err != nil {
			return 0, err
		}
	}
	return len(linhas), nil
}

// GarantirTenantDeDesenvolvimento cria o tenant "dev" se o banco estiver vazio,
// para a API subir utilizável. Sai de cena quando houver cadastro de verdade.
func GarantirTenantDeDesenvolvimento(ctx context.Context, pool *pgxpool.Pool) error {
	consulta := db.New(pool)

	tenants, err := consulta.ListarTenants(ctx)
	if err != nil {
		return err
	}
	if len(tenants) > 0 {
		return nil
	}

	_, err = consulta.CriarTenant(ctx, db.CriarTenantParams{
		Nome: "Tenant de desenvolvimento", Slug: "dev",
	})
	return err
}

func ptr[T any](valor T) *T { return &valor }
