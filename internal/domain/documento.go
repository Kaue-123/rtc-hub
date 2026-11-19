// Package domain tem o modelo interno de um documento fiscal, independente do
// layout de origem.
package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

const (
	MotorRegimeRegular = "regime_regular"
	MotorSimples       = "simples_nacional"
	MotorForaDeEscopo  = "fora_de_escopo"
)

var ufPorCodigo = map[string]string{
	"11": "RO", "12": "AC", "13": "AM", "14": "RR", "15": "PA", "16": "AP",
	"17": "TO", "21": "MA", "22": "PI", "23": "CE", "24": "RN", "25": "PB",
	"26": "PE", "27": "AL", "28": "SE", "29": "BA", "31": "MG", "32": "ES",
	"33": "RJ", "35": "SP", "41": "PR", "42": "SC", "43": "RS", "50": "MS",
	"51": "MT", "52": "GO", "53": "DF",
}

// UFPorCodigo traduz o cUF numérico da NF-e para a sigla.
func UFPorCodigo(codigo string) string { return ufPorCodigo[codigo] }

// TributoDeclarado é o que o documento afirma ter apurado para um tributo.
// Ponteiro nulo significa campo ausente no XML, que não é o mesmo que zero.
type TributoDeclarado struct {
	Aliquota        *decimal.Decimal
	AliquotaEfetiva *decimal.Decimal
	Reducao         *decimal.Decimal
	Valor           *decimal.Decimal
}

type ItemDeclarado struct {
	Numero      int
	BaseCalculo *decimal.Decimal
	NCM         string
	NBS         string
	CST         string
	CClassTrib  string
	Quantidade  *decimal.Decimal
	Unidade     string
	Descricao   string

	IBSUF   TributoDeclarado
	IBSMun  TributoDeclarado
	CBS     TributoDeclarado
	ValorIS *decimal.Decimal

	CAtivSN string

	// Grupo gTribRegular do XML. O motor exige para algumas cClassTrib; sem
	// repassar, o item é recusado mesmo com o XML correto.
	CSTRegular        string
	CClassTribRegular string
}

type DocumentoFiscal struct {
	Arquivo      string
	Tipo         string // nfe | nfce | nfse
	Modelo       string
	Chave        string
	Numero       string
	DhEmissao    *time.Time
	UF           string
	Municipio    *int32
	CRT          string
	EmitenteCNPJ string
	Itens        []ItemDeclarado
}

func (d DocumentoFiscal) AnoFatoGerador() int {
	if d.DhEmissao == nil {
		return 0
	}
	return d.DhEmissao.Year()
}

// Motor diz qual calculadora atende o documento.
//
// MEI (CRT=4) e CRT desconhecido ficam de fora: a NT 2026.005 os põe fora do
// escopo, e chutar regime daria número plausível e errado.
func (d DocumentoFiscal) Motor() string {
	switch d.CRT {
	case "1", "2":
		return MotorSimples
	case "3":
		return MotorRegimeRegular
	default:
		return MotorForaDeEscopo
	}
}
