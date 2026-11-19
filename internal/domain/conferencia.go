package domain

import "github.com/shopspring/decimal"

// Severidades e categorias de divergência.
const (
	Alta  = "alta"
	Media = "media"
	Baixa = "baixa"

	CatValor         = "valor_tributo"
	CatBase          = "base_calculo"
	CatAliquota      = "aliquota_efetiva"
	CatClassificacao = "classificacao"
	CatNaoCalculado  = "nao_calculado"
	CatAusente       = "ausente_no_documento"
)

// Estados em que o motor entregou número.
var estadosComNumero = map[string]bool{"CALCULADO": true, "CALCULO_SIMULADO": true}

// CarimboVersao identifica o motor que produziu um resultado.
type CarimboVersao struct {
	Motor        string
	VersaoApp    string
	VersaoDB     string
	DataVersaoDB string
	Ambiente     string
}

// ErroDoMotor é um item que o motor recusou.
type ErroDoMotor struct {
	Codigo   string
	Titulo   string
	Ponteiro string
	Detalhe  string
}

// ItemApurado é a resposta do motor para um item, já normalizada: os dois
// motores têm formatos diferentes e o comparador não deve saber disso.
type ItemApurado struct {
	Estado         string
	Erros          []ErroDoMotor
	BaseCalculo    *decimal.Decimal
	IBSUF          *decimal.Decimal
	IBSMun         *decimal.Decimal
	CBS            *decimal.Decimal
	AliqEfetIBSUF  *decimal.Decimal
	AliqEfetIBSMun *decimal.Decimal
	AliqEfetCBS    *decimal.Decimal
	MemoriaIBSUF   string
	MemoriaIBSMun  string
	MemoriaCBS     string
}

type Divergencia struct {
	Item           int
	Campo          string
	Categoria      string
	Severidade     string
	Declarado      *string
	Apurado        *string
	Diferenca      *decimal.Decimal
	Codigo         string
	Detalhe        string
	MemoriaCalculo string
}

// Conferir compara o declarado com o apurado, item a item. É função pura: a
// chamada ao motor e a persistência ficam no caso de uso.
//
// HTTP 200 não quer dizer calculado — os motores devolvem 200 com itens em
// erro, e os totais somam só os calculados. Item recusado ou omitido vira
// divergência, nunca silêncio.
func Conferir(doc DocumentoFiscal, apurados map[int]ItemApurado) []Divergencia {
	var divergencias []Divergencia
	for _, item := range doc.Itens {
		apurado, existe := apurados[item.Numero]
		if !existe {
			divergencias = append(divergencias, Divergencia{
				Item: item.Numero, Campo: "item", Categoria: CatNaoCalculado,
				Severidade: Media, Detalhe: "o motor não devolveu resultado para este item",
			})
			continue
		}
		divergencias = append(divergencias, confereItem(item, apurado)...)
	}
	return divergencias
}

func confereItem(item ItemDeclarado, apurado ItemApurado) []Divergencia {
	if !estadosComNumero[apurado.Estado] {
		if len(apurado.Erros) == 0 {
			return []Divergencia{{
				Item: item.Numero, Campo: "item", Categoria: CatNaoCalculado,
				Severidade: Media,
				Detalhe:    "o motor devolveu estado " + apurado.Estado + " sem erro associado",
			}}
		}
		recusas := make([]Divergencia, 0, len(apurado.Erros))
		for _, erro := range apurado.Erros {
			campo := erro.Ponteiro
			if campo == "" {
				campo = "item"
			}
			detalhe := erro.Detalhe
			if detalhe == "" {
				detalhe = erro.Titulo
			}
			recusas = append(recusas, Divergencia{
				Item: item.Numero, Campo: campo, Categoria: CatClassificacao,
				Severidade: Alta, Codigo: erro.Codigo, Detalhe: detalhe,
			})
		}
		return recusas
	}

	comparacoes := []struct {
		campo      string
		declarado  *decimal.Decimal
		apurado    *decimal.Decimal
		categoria  string
		severidade string
		memoria    string
	}{
		{"base_calculo", item.BaseCalculo, apurado.BaseCalculo, CatBase, Alta, ""},
		{"v_ibs_uf", item.IBSUF.Valor, apurado.IBSUF, CatValor, Alta, apurado.MemoriaIBSUF},
		{"v_ibs_mun", item.IBSMun.Valor, apurado.IBSMun, CatValor, Alta, apurado.MemoriaIBSMun},
		{"v_cbs", item.CBS.Valor, apurado.CBS, CatValor, Alta, apurado.MemoriaCBS},
		{"p_aliq_efet_ibs_uf", item.IBSUF.AliquotaEfetiva, apurado.AliqEfetIBSUF, CatAliquota, Media, ""},
		{"p_aliq_efet_ibs_mun", item.IBSMun.AliquotaEfetiva, apurado.AliqEfetIBSMun, CatAliquota, Media, ""},
		{"p_aliq_efet_cbs", item.CBS.AliquotaEfetiva, apurado.AliqEfetCBS, CatAliquota, Media, ""},
	}

	var divergencias []Divergencia
	for _, c := range comparacoes {
		if d := compara(item.Numero, c.campo, c.declarado, c.apurado, c.categoria, c.severidade, c.memoria); d != nil {
			divergencias = append(divergencias, *d)
		}
	}
	return divergencias
}

func compara(
	numeroItem int, campo string, declarado, apurado *decimal.Decimal,
	categoria, severidade, memoria string,
) *Divergencia {
	// O motor não apurou o campo (o Simples não separa IBS municipal).
	if apurado == nil {
		return nil
	}
	textoApurado := comEscala(*apurado)

	if declarado == nil {
		return &Divergencia{
			Item: numeroItem, Campo: campo, Categoria: CatAusente, Severidade: Baixa,
			Apurado:        &textoApurado,
			Detalhe:        "o documento não declara " + campo + "; o motor apurou " + textoApurado,
			MemoriaCalculo: memoria,
		}
	}
	// Equal compara valor, não formatação: 0.00 e 0 são o mesmo número.
	if declarado.Equal(*apurado) {
		return nil
	}

	textoDeclarado := comEscala(*declarado)
	diferenca := apurado.Sub(*declarado)
	return &Divergencia{
		Item: numeroItem, Campo: campo, Categoria: categoria, Severidade: severidade,
		Declarado: &textoDeclarado, Apurado: &textoApurado, Diferenca: &diferenca,
		MemoriaCalculo: memoria,
	}
}

// comEscala preserva as casas decimais do valor original: String() corta zeros
// à direita e transformaria "5.00" em "5" no relatório fiscal.
func comEscala(valor decimal.Decimal) string {
	if casas := -valor.Exponent(); casas > 0 {
		return valor.StringFixed(casas)
	}
	return valor.String()
}
