// Testes da fronteira entre o instante da nota e o dia de calendário da
// vigência. Ficam em arquivo próprio porque é outro assunto: aliquota_test.go
// trata de qual linha vence, este trata de qual DIA está sendo julgado.
package domain_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/Kaue-123/rtc-hub/internal/domain"
)

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
