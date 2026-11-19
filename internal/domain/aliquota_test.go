package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/Kaue-123/rtc-hub/internal/domain"
)

func dia(ano int, mes time.Month, d int) time.Time {
	return time.Date(ano, mes, d, 0, 0, 0, 0, time.UTC)
}

func ptr[T any](v T) *T { return &v }

type opcao func(*domain.AliquotaVigencia)

func comTenant(id int64) opcao   { return func(a *domain.AliquotaVigencia) { a.TenantID = &id } }
func comUF(uf string) opcao      { return func(a *domain.AliquotaVigencia) { a.UF = &uf } }
func comMunicipio(m int32) opcao { return func(a *domain.AliquotaVigencia) { a.Municipio = &m } }
func comCBS(v string) opcao {
	return func(a *domain.AliquotaVigencia) { a.CBS = decimal.RequireFromString(v) }
}
func ateDia(t time.Time) opcao { return func(a *domain.AliquotaVigencia) { a.VigenciaFim = &t } }

func linha(opcoes ...opcao) domain.AliquotaVigencia {
	a := domain.AliquotaVigencia{
		VigenciaInicio: dia(2027, time.January, 1),
		CBS:            decimal.RequireFromString("8.8"),
		IBSEstadual:    decimal.RequireFromString("0.05"),
		IBSMunicipal:   decimal.RequireFromString("0.05"),
		Fonte:          domain.FonteAliquotaPadrao,
	}
	for _, o := range opcoes {
		o(&a)
	}
	return a
}

func resolve(t *testing.T, candidatas []domain.AliquotaVigencia, uf string, municipio *int32) domain.AliquotaVigencia {
	t.Helper()
	escolhida, err := domain.ResolverAliquota(candidatas, dia(2027, time.June, 1), uf, municipio)
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	return escolhida
}

func TestLinhaDoTenantVenceAPadrao(t *testing.T) {
	candidatas := []domain.AliquotaVigencia{
		linha(),
		linha(comTenant(1), comCBS("7.5")),
	}
	if got := resolve(t, candidatas, "RS", nil).CBS; !got.Equal(decimal.RequireFromString("7.5")) {
		t.Errorf("cbs = %s", got)
	}
}

func TestMunicipioVenceUFQueVenceOGeral(t *testing.T) {
	candidatas := []domain.AliquotaVigencia{
		linha(comCBS("1")),
		linha(comUF("RS"), comCBS("2")),
		linha(comUF("RS"), comMunicipio(4314902), comCBS("3")),
	}

	casos := []struct {
		uf        string
		municipio *int32
		esperado  string
	}{
		{"MA", nil, "1"},
		{"RS", nil, "2"},
		{"RS", ptr(int32(4314902)), "3"},
	}
	for _, caso := range casos {
		got := resolve(t, candidatas, caso.uf, caso.municipio).CBS
		if !got.Equal(decimal.RequireFromString(caso.esperado)) {
			t.Errorf("uf=%s municipio=%v → cbs %s, esperava %s",
				caso.uf, caso.municipio, got, caso.esperado)
		}
	}
}

// Linha de SP não pode valer para nota do RS só por ter recorte geográfico.
func TestRegraDeOutraUFNaoEhAplicadaPorSerMaisEspecifica(t *testing.T) {
	candidatas := []domain.AliquotaVigencia{
		linha(comCBS("1")),
		linha(comUF("SP"), comCBS("9")),
	}
	if got := resolve(t, candidatas, "RS", nil).CBS; !got.Equal(decimal.RequireFromString("1")) {
		t.Errorf("cbs = %s", got)
	}
}

func TestDataForaDeQualquerVigenciaRecusaEmVezDeChutar(t *testing.T) {
	candidatas := []domain.AliquotaVigencia{linha(ateDia(dia(2027, time.December, 31)))}

	_, err := domain.ResolverAliquota(candidatas, dia(2028, time.January, 1), "RS", nil)
	if !errors.Is(err, domain.ErrSemAliquotaVigente) {
		t.Fatalf("erro = %v", err)
	}
}

func TestVigenciaFechadaNaoValeNoDiaSeguinteAoFim(t *testing.T) {
	candidatas := []domain.AliquotaVigencia{linha(ateDia(dia(2027, time.June, 30)))}

	if _, err := domain.ResolverAliquota(candidatas, dia(2027, time.June, 30), "RS", nil); err != nil {
		t.Errorf("último dia da vigência deveria valer: %v", err)
	}
	if _, err := domain.ResolverAliquota(candidatas, dia(2027, time.July, 1), "RS", nil); err == nil {
		t.Error("dia seguinte ao fim não deveria valer")
	}
}

func TestNenhumaCandidataRecusa(t *testing.T) {
	if _, err := domain.ResolverAliquota(nil, dia(2027, time.June, 1), "RS", nil); err == nil {
		t.Error("lista vazia deveria recusar")
	}
}

// A nota carrega o instante da emissão com fuso; a vigência é dia de calendário.
// Reduzir um ao outro com time.Truncate(24h) trunca em UTC, não no fuso da nota,
// e joga tudo que foi emitido depois das 21:00 de Brasília para o dia seguinte —
// bem em cima da virada de 2026 para 2027, que é a fronteira do produto.
func TestNotaEmitidaNoFimDoUltimoDiaDaVigenciaNaoPulaParaOAnoSeguinte(t *testing.T) {
	brasilia := time.FixedZone("-03", -3*60*60)
	fim2026 := dia(2026, time.December, 31)
	candidatas := []domain.AliquotaVigencia{
		linha(comCBS("0.9"), ateDia(fim2026)), // vigência de 2026, fecha em 31/12
		linha(comCBS("8.8")),                  // vigência de 2027, abre em 01/01
	}
	// A de 2027 nasce em 01/01/2027; a de 2026 é a que vale na véspera.
	candidatas[0].VigenciaInicio = dia(2026, time.January, 1)

	casos := []struct {
		nome     string
		emissao  time.Time
		esperado string
	}{
		{"meio do dia", time.Date(2026, time.December, 31, 14, 0, 0, 0, brasilia), "0.9"},
		{"22h, ainda 2026", time.Date(2026, time.December, 31, 22, 0, 0, 0, brasilia), "0.9"},
		{"23h59 da virada", time.Date(2026, time.December, 31, 23, 59, 0, 0, brasilia), "0.9"},
		{"primeira hora de 2027", time.Date(2027, time.January, 1, 0, 30, 0, 0, brasilia), "8.8"},
	}
	for _, caso := range casos {
		escolhida, err := domain.ResolverAliquota(candidatas, caso.emissao, "RS", nil)
		if err != nil {
			t.Errorf("%s: %v", caso.nome, err)
			continue
		}
		if !escolhida.CBS.Equal(decimal.RequireFromString(caso.esperado)) {
			t.Errorf("%s (%s): cbs = %s, esperava %s",
				caso.nome, caso.emissao.Format(time.RFC3339), escolhida.CBS, caso.esperado)
		}
	}
}

func TestDiaDaCompetenciaUsaOFusoDaNotaEEhIdempotente(t *testing.T) {
	brasilia := time.FixedZone("-03", -3*60*60)
	emissao := time.Date(2026, time.December, 31, 22, 0, 0, 0, brasilia)

	got := domain.DiaDaCompetencia(emissao)
	if !got.Equal(dia(2026, time.December, 31)) {
		t.Errorf("DiaDaCompetencia = %s, esperava 2026-12-31", got.Format(time.RFC3339))
	}
	if segunda := domain.DiaDaCompetencia(got); !segunda.Equal(got) {
		t.Errorf("não é idempotente: %s → %s", got.Format(time.RFC3339), segunda.Format(time.RFC3339))
	}
}
