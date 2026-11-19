package fiscalxml_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/Kaue-123/rtc-hub/internal/adapters/fiscalxml"
	"github.com/Kaue-123/rtc-hub/internal/domain"
)

// exemplo lê um XML do repositório da RFB clonado em upstream/.
func exemplo(t *testing.T, relativo string) []byte {
	t.Helper()
	caminho := filepath.Join(
		"..", "..", "..", "upstream", "calculadora-rtc-regime-regular",
		"src", "test", "resources", "xml", relativo)
	conteudo, err := os.ReadFile(caminho)
	if err != nil {
		t.Skipf("exemplo ausente: %s", caminho)
	}
	return conteudo
}

func exigeDecimal(t *testing.T, campo string, obtido *decimal.Decimal, esperado string) {
	t.Helper()
	if obtido == nil {
		t.Fatalf("%s: nulo, esperava %s", campo, esperado)
	}
	if !obtido.Equal(decimal.RequireFromString(esperado)) {
		t.Errorf("%s = %s, esperava %s", campo, obtido, esperado)
	}
}

func TestNFeSobEnviNFeComTributosDeclarados(t *testing.T) {
	doc, err := fiscalxml.Parse(exemplo(t, "nfe/valido/nota/nfe_valido.xml"), "nfe_valido.xml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if doc.Tipo != "nfe" || doc.Modelo != "55" {
		t.Errorf("tipo/modelo = %s/%s", doc.Tipo, doc.Modelo)
	}
	if doc.UF != "MA" {
		t.Errorf("uf = %s", doc.UF)
	}
	if doc.Municipio == nil || *doc.Municipio != 2111300 {
		t.Errorf("municipio = %v", doc.Municipio)
	}
	if doc.CRT != "3" || doc.EmitenteCNPJ != "11111111000111" {
		t.Errorf("crt/cnpj = %s/%s", doc.CRT, doc.EmitenteCNPJ)
	}
	if doc.Chave != "21250911111111000111558890000000051003360290" {
		t.Errorf("chave = %s", doc.Chave)
	}
	if doc.AnoFatoGerador() != 2025 {
		t.Errorf("ano = %d", doc.AnoFatoGerador())
	}

	if len(doc.Itens) != 1 {
		t.Fatalf("itens = %d", len(doc.Itens))
	}
	item := doc.Itens[0]
	if item.NCM != "76071120" || item.CST != "200" || item.CClassTrib != "200022" {
		t.Errorf("ncm/cst/cClassTrib = %s/%s/%s", item.NCM, item.CST, item.CClassTrib)
	}
	exigeDecimal(t, "baseCalculo", item.BaseCalculo, "100.00")
	exigeDecimal(t, "cbs.aliquota", item.CBS.Aliquota, "2.00")
	exigeDecimal(t, "cbs.reducao", item.CBS.Reducao, "100.00")
	exigeDecimal(t, "cbs.valor", item.CBS.Valor, "0.00")
	exigeDecimal(t, "ibsUF.aliquota", item.IBSUF.Aliquota, "10.00")
	exigeDecimal(t, "ibsMun.aliquota", item.IBSMun.Aliquota, "5.00")
}

func TestGTribRegularEhLidoPorqueOMotorExige(t *testing.T) {
	doc, err := fiscalxml.Parse(exemplo(t, "nfe/valido/nota/nfe_valido.xml"), "x.xml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	item := doc.Itens[0]
	if item.CSTRegular != "000" || item.CClassTribRegular != "000001" {
		t.Errorf("tributação regular = %s/%s", item.CSTRegular, item.CClassTribRegular)
	}
}

// O mesmo layout vem embrulhado em nfeProc; a raiz não pode ser exigida.
func TestNFCeSobNFeProc(t *testing.T) {
	doc, err := fiscalxml.Parse(exemplo(t, "nfce/valido/nota/nfce_valido.xml"), "nfce.xml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Tipo != "nfce" || doc.Modelo != "65" {
		t.Errorf("tipo/modelo = %s/%s", doc.Tipo, doc.Modelo)
	}
	if len(doc.Itens) == 0 {
		t.Error("sem itens")
	}
}

func TestNFSeViraItemSintetico(t *testing.T) {
	doc, err := fiscalxml.Parse(exemplo(t, "nfse/valido/nota/nfse_valido_com_IBSCBS.xml"), "nfse.xml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Tipo != "nfse" {
		t.Errorf("tipo = %s", doc.Tipo)
	}
	if doc.Municipio == nil || *doc.Municipio != 4113700 {
		t.Errorf("municipio = %v", doc.Municipio)
	}
	if len(doc.Itens) != 1 {
		t.Fatalf("itens = %d", len(doc.Itens))
	}

	item := doc.Itens[0]
	// Classificação vem do grupo IBSCBS de dentro da DPS, não do de fora.
	if item.CST != "000" || item.CClassTrib != "000001" {
		t.Errorf("cst/cClassTrib = %s/%s", item.CST, item.CClassTrib)
	}
	if item.NBS != "101011100" {
		t.Errorf("nbs = %s", item.NBS)
	}
	exigeDecimal(t, "baseCalculo", item.BaseCalculo, "950.00")
	exigeDecimal(t, "cbs.aliquota", item.CBS.Aliquota, "0.90")
	exigeDecimal(t, "cbs.valor", item.CBS.Valor, "8.55")
	exigeDecimal(t, "ibsUF.valor", item.IBSUF.Valor, "0.95")
	exigeDecimal(t, "ibsMun.valor", item.IBSMun.Valor, "0.00")
}

func TestOpSimpNacViraCRTERoteiaOMotor(t *testing.T) {
	doc, err := fiscalxml.Parse(exemplo(t, "nfse/valido/nota/nfse_valido_com_IBSCBS.xml"), "nfse.xml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// opSimpNac 1 (não optante) corresponde a CRT 3.
	if doc.CRT != "3" || doc.Motor() != domain.MotorRegimeRegular {
		t.Errorf("crt/motor = %s/%s", doc.CRT, doc.Motor())
	}
}

func TestXMLQueNaoEhDFeEhRecusado(t *testing.T) {
	_, err := fiscalxml.Parse([]byte("<qualquerCoisa><a/></qualquerCoisa>"), "x.xml")
	if !errors.Is(err, fiscalxml.ErrNaoReconhecido) {
		t.Fatalf("erro = %v", err)
	}
}

func TestXMLMalformadoNaoDerrubaOLote(t *testing.T) {
	_, err := fiscalxml.Parse([]byte("<sem fechar"), "x.xml")
	if !errors.Is(err, fiscalxml.ErrNaoReconhecido) {
		t.Fatalf("erro = %v", err)
	}
}

// A NT 2026.005 põe CRT=4 fora do cálculo.
func TestMEIECRTDesconhecidoFicamForaDeEscopo(t *testing.T) {
	casos := map[string]string{
		"1": domain.MotorSimples,
		"2": domain.MotorSimples,
		"3": domain.MotorRegimeRegular,
		"4": domain.MotorForaDeEscopo,
		"":  domain.MotorForaDeEscopo,
	}
	for crt, esperado := range casos {
		doc := domain.DocumentoFiscal{Arquivo: "x", Tipo: "nfe", CRT: crt}
		if obtido := doc.Motor(); obtido != esperado {
			t.Errorf("crt %q → %s, esperava %s", crt, obtido, esperado)
		}
	}
}
