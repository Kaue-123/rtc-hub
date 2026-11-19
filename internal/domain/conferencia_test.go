package domain_test

import (
	"testing"

	"github.com/shopspring/decimal"

	"github.com/Kaue-123/rtc-hub/internal/domain"
)

func d(valor string) *decimal.Decimal {
	v := decimal.RequireFromString(valor)
	return &v
}

func itemConforme() domain.ItemDeclarado {
	return domain.ItemDeclarado{
		Numero:      1,
		BaseCalculo: d("100.00"),
		IBSUF:       domain.TributoDeclarado{AliquotaEfetiva: d("1.00"), Valor: d("1.00")},
		IBSMun:      domain.TributoDeclarado{AliquotaEfetiva: d("1.00"), Valor: d("1.00")},
		CBS:         domain.TributoDeclarado{AliquotaEfetiva: d("10.00"), Valor: d("10.00")},
	}
}

func apuradoConforme() domain.ItemApurado {
	return domain.ItemApurado{
		Estado: "CALCULADO", BaseCalculo: d("100.00"),
		IBSUF: d("1.00"), IBSMun: d("1.00"), CBS: d("10.00"),
		AliqEfetIBSUF: d("1.00"), AliqEfetIBSMun: d("1.00"), AliqEfetCBS: d("10.00"),
		MemoriaCBS: "porque sim",
	}
}

func doc(item domain.ItemDeclarado) domain.DocumentoFiscal {
	return domain.DocumentoFiscal{Arquivo: "x.xml", Tipo: "nfe", CRT: "3",
		Itens: []domain.ItemDeclarado{item}}
}

func TestDocumentoQueBateNaoGeraDivergencia(t *testing.T) {
	got := domain.Conferir(doc(itemConforme()),
		map[int]domain.ItemApurado{1: apuradoConforme()})
	if len(got) != 0 {
		t.Fatalf("esperava nenhuma divergência, veio %d: %+v", len(got), got)
	}
}

func TestCentavoDeDiferencaViraDivergenciaAltaComMemoria(t *testing.T) {
	item := itemConforme()
	item.CBS = domain.TributoDeclarado{AliquotaEfetiva: d("10.00"), Valor: d("9.99")}

	got := domain.Conferir(doc(item), map[int]domain.ItemApurado{1: apuradoConforme()})

	if len(got) != 1 {
		t.Fatalf("divergências = %d: %+v", len(got), got)
	}
	div := got[0]
	if div.Campo != "v_cbs" || div.Categoria != domain.CatValor || div.Severidade != domain.Alta {
		t.Errorf("campo/categoria/severidade = %s/%s/%s", div.Campo, div.Categoria, div.Severidade)
	}
	// As casas decimais do original são preservadas: relatório fiscal não
	// mostra "10" onde o documento diz "10.00".
	if *div.Declarado != "9.99" || *div.Apurado != "10.00" {
		t.Errorf("declarado/apurado = %s/%s", *div.Declarado, *div.Apurado)
	}
	if !div.Diferenca.Equal(decimal.RequireFromString("0.01")) {
		t.Errorf("diferenca = %s", div.Diferenca)
	}
	// A memória do motor viaja junto com a acusação.
	if div.MemoriaCalculo != "porque sim" {
		t.Errorf("memoria = %q", div.MemoriaCalculo)
	}
}

// 0.00 e 0 são o mesmo valor; comparar texto daria falso positivo.
func TestFormatacaoDiferenteDoMesmoNumeroNaoEhDivergencia(t *testing.T) {
	item := itemConforme()
	item.CBS = domain.TributoDeclarado{AliquotaEfetiva: d("10.00"), Valor: d("10")}
	apurado := apuradoConforme()
	apurado.CBS = d("10.0000")

	if got := domain.Conferir(doc(item), map[int]domain.ItemApurado{1: apurado}); len(got) != 0 {
		t.Fatalf("divergências = %+v", got)
	}
}

func TestItemRecusadoViraDivergenciaDeClassificacao(t *testing.T) {
	apurado := domain.ItemApurado{
		Estado: "INCONSISTENCIA_ENTRADA",
		Erros: []domain.ErroDoMotor{{
			Codigo: "REG-011", Titulo: "NCM não vinculada", Ponteiro: "#/itens/0/ncm",
			Detalhe: "NCM 02013000 não vinculada à cClassTrib 200035",
		}},
	}

	got := domain.Conferir(doc(itemConforme()), map[int]domain.ItemApurado{1: apurado})

	if len(got) != 1 {
		t.Fatalf("divergências = %d", len(got))
	}
	if got[0].Categoria != domain.CatClassificacao || got[0].Codigo != "REG-011" {
		t.Errorf("categoria/codigo = %s/%s", got[0].Categoria, got[0].Codigo)
	}
	if got[0].Severidade != domain.Alta {
		t.Errorf("severidade = %s", got[0].Severidade)
	}
}

// Totais somam só itens calculados; item sumido não pode virar silêncio.
func TestItemOmitidoPeloMotorNaoPassaSilenciosamente(t *testing.T) {
	got := domain.Conferir(doc(itemConforme()), map[int]domain.ItemApurado{})
	if len(got) != 1 || got[0].Categoria != domain.CatNaoCalculado {
		t.Fatalf("divergências = %+v", got)
	}
}

func TestCampoAusenteNoDocumentoNaoEhComparadoComZero(t *testing.T) {
	item := itemConforme()
	item.CBS = domain.TributoDeclarado{AliquotaEfetiva: d("10.00"), Valor: nil}

	got := domain.Conferir(doc(item), map[int]domain.ItemApurado{1: apuradoConforme()})

	var achou bool
	for _, div := range got {
		if div.Campo != "v_cbs" {
			continue
		}
		achou = true
		if div.Categoria != domain.CatAusente || div.Severidade != domain.Baixa {
			t.Errorf("categoria/severidade = %s/%s", div.Categoria, div.Severidade)
		}
		if div.Declarado != nil || div.Diferenca != nil {
			t.Errorf("declarado/diferenca deveriam ser nulos")
		}
	}
	if !achou {
		t.Error("nenhuma divergência para v_cbs")
	}
}

// O Simples não separa IBS municipal; ausência no motor não é erro do emitente.
func TestCampoQueOMotorNaoApuraNaoViraDivergencia(t *testing.T) {
	apurado := apuradoConforme()
	apurado.IBSMun = nil
	apurado.AliqEfetIBSMun = nil

	for _, div := range domain.Conferir(doc(itemConforme()), map[int]domain.ItemApurado{1: apurado}) {
		if div.Campo == "v_ibs_mun" || div.Campo == "p_aliq_efet_ibs_mun" {
			t.Errorf("divergência indevida em %s", div.Campo)
		}
	}
}

func TestAliquotaEfetivaDivergenteEhMediaNaoAlta(t *testing.T) {
	item := itemConforme()
	item.CBS = domain.TributoDeclarado{AliquotaEfetiva: d("9.00"), Valor: d("10.00")}

	for _, div := range domain.Conferir(doc(item), map[int]domain.ItemApurado{1: apuradoConforme()}) {
		if div.Campo == "p_aliq_efet_cbs" && div.Severidade != domain.Media {
			t.Errorf("severidade = %s", div.Severidade)
		}
	}
}
