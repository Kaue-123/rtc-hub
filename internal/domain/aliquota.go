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

// IBSSimplesNacional é a alíquota de IBS que o motor do Simples espera em
// pIBSSN.
//
// No Simples não existe separação entre IBS estadual e municipal: o motor aplica
// pIBSSN uma única vez sobre a base e o vIBSSN que sai é o IBS inteiro. Mandar
// só a parcela estadual faz o imposto sair pela metade, e uma nota subdeclarada
// passaria como conforme — o oposto do que o produto existe para fazer.
func (a AliquotaVigencia) IBSSimplesNacional() decimal.Decimal {
	return a.IBSEstadual.Add(a.IBSMunicipal)
}

// Aplicavel diz se a linha pode ser usada para esta competência e recorte
// geográfico. Recorte é aplicabilidade, não preferência: linha de SP não vale
// para nota do RS nem na falta de outra.
func (a AliquotaVigencia) Aplicavel(competencia time.Time, uf string, municipio *int32) bool {
	dia := competencia.Truncate(24 * time.Hour)
	if a.VigenciaInicio.After(dia) {
		return false
	}
	if a.VigenciaFim != nil && a.VigenciaFim.Before(dia) {
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
