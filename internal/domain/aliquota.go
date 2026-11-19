package domain

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

const (
	FonteAliquotaPadrao = "padrao_rtc_hub"
	FonteAliquotaTenant = "tenant"
)

// A partir desta competência o motor da RFB não tem alíquota de referência:
// aliquotasNominais passa de proibida a obrigatória na requisição.
const PrimeiroAnoSemAliquotaDeReferencia = 2027

var ErrSemAliquotaVigente = errors.New("nenhuma alíquota vigente cobre a competência")

// DiaDaCompetencia reduz o instante da emissão ao dia de calendário DELA, no
// fuso do próprio documento, representado como meia-noite UTC — que é como as
// colunas DATE de vigência chegam do banco.
//
// NÃO troque por time.Truncate: Truncate opera sobre a duração absoluta desde o
// zero time, ancorado em UTC, e não sobre a forma de apresentação. Toda nota
// emitida depois das 21:00 em Brasília seria truncada para o DIA SEGUINTE,
// enquanto Year(), Month() e Day() continuariam dizendo o dia certo.
func DiaDaCompetencia(instante time.Time) time.Time {
	ano, mes, dia := instante.Date()
	return time.Date(ano, mes, dia, 0, 0, 0, 0, time.UTC)
}

type AliquotaVigencia struct {
	ID             int64
	TenantID       *int64
	VigenciaInicio time.Time
	VigenciaFim    *time.Time
	UF             *string
	Municipio      *int32
	CBS            decimal.Decimal
	IBSEstadual    decimal.Decimal
	IBSMunicipal   decimal.Decimal
	Fonte          string
	Observacao     *string
}

// Aplicavel diz se a linha pode ser usada para esta competência e recorte
// geográfico. Recorte é aplicabilidade, não preferência: linha de SP não vale
// para nota do RS nem na falta de outra.
func (a AliquotaVigencia) Aplicavel(competencia time.Time, uf string, municipio *int32) bool {
	// Normalizar os dois lados é idempotente e impede que a comparação dependa
	// de quem chamou ter normalizado antes.
	dia := DiaDaCompetencia(competencia)
	if DiaDaCompetencia(a.VigenciaInicio).After(dia) {
		return false
	}
	if a.VigenciaFim != nil && DiaDaCompetencia(*a.VigenciaFim).Before(dia) {
		return false
	}
	if a.UF != nil && *a.UF != uf {
		return false
	}
	if a.Municipio != nil && (municipio == nil || *a.Municipio != *municipio) {
		return false
	}
	return true
}

// especificidade ordena do mais geral para o mais específico. Linha do tenant
// pesa mais que a padrão; município pesa mais que UF.
func (a AliquotaVigencia) especificidade(uf string, municipio *int32) int {
	peso := 0
	if a.TenantID != nil {
		peso += 4
	}
	if a.Municipio != nil && municipio != nil && *a.Municipio == *municipio {
		peso += 2
	}
	if a.UF != nil && *a.UF == uf {
		peso++
	}
	return peso
}

// ResolverAliquota escolhe, entre as candidatas, a que vale para o documento.
// Recusa em vez de chutar quando nenhuma se aplica.
func ResolverAliquota(
	candidatas []AliquotaVigencia, competencia time.Time, uf string, municipio *int32,
) (AliquotaVigencia, error) {
	var escolhida AliquotaVigencia
	melhor := -1

	for _, linha := range candidatas {
		if !linha.Aplicavel(competencia, uf, municipio) {
			continue
		}
		if peso := linha.especificidade(uf, municipio); peso > melhor {
			melhor, escolhida = peso, linha
		}
	}

	if melhor < 0 {
		return AliquotaVigencia{}, ErrSemAliquotaVigente
	}
	return escolhida, nil
}
