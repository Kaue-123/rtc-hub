package rfb

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/Kaue-123/rtc-hub/internal/domain"
)

func requisicaoDe(t *testing.T, doc domain.DocumentoFiscal, aliquota domain.AliquotaVigencia) map[string]any {
	t.Helper()
	motor := NovoRegimeRegular("http://exemplo")
	bruto, err := json.Marshal(motor.montaRequisicao(doc, aliquota))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var corpo map[string]any
	if err := json.Unmarshal(bruto, &corpo); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return corpo
}

func documentoEm(ano int, uf string) domain.DocumentoFiscal {
	emissao := time.Date(ano, time.September, 1, 0, 0, 0, 0, time.UTC)
	base := decimal.RequireFromString("100.00")
	return domain.DocumentoFiscal{
		Tipo: "nfe", UF: uf, DhEmissao: &emissao,
		Itens: []domain.ItemDeclarado{{
			Numero: 1, NCM: "76071120", CST: "200", CClassTrib: "200022",
			BaseCalculo: &base,
		}},
	}
}

func aliquota() domain.AliquotaVigencia {
	return domain.AliquotaVigencia{
		CBS:          decimal.RequireFromString("8.8"),
		IBSEstadual:  decimal.RequireFromString("0.05"),
		IBSMunicipal: decimal.RequireFromString("0.05"),
	}
}

// Campo vazio e campo ausente não são a mesma coisa para o motor: "uf": ""
// volta VAL-004, enquanto a ausência passa. A NFS-e não traz UF, e o zero value
// de string em Go serializa como "" — é o jeito fácil de reintroduzir o defeito.
func TestUFVaziaEhOmitidaDaRequisicao(t *testing.T) {
	corpo := requisicaoDe(t, documentoEm(2026, ""), aliquota())
	if _, presente := corpo["uf"]; presente {
		t.Errorf("uf não deveria estar na requisição: %v", corpo["uf"])
	}

	corpo = requisicaoDe(t, documentoEm(2026, "MA"), aliquota())
	if corpo["uf"] != "MA" {
		t.Errorf("uf = %v", corpo["uf"])
	}
}

// Antes de 2027 informar alíquota é PROIBIDO; de 2027 em diante, obrigatório.
func TestAliquotaNominalSoVaiDe2027EmDiante(t *testing.T) {
	item := func(ano int) map[string]any {
		corpo := requisicaoDe(t, documentoEm(ano, "MA"), aliquota())
		return corpo["itens"].([]any)[0].(map[string]any)
	}

	if _, presente := item(2026)["aliquotasNominais"]; presente {
		t.Error("2026 não pode mandar aliquotasNominais")
	}
	nominais, presente := item(2027)["aliquotasNominais"]
	if !presente {
		t.Fatal("2027 tem de mandar aliquotasNominais")
	}
	if nominais.(map[string]any)["cbs"] != "8.8" {
		t.Errorf("cbs = %v", nominais.(map[string]any)["cbs"])
	}
}

// O motor recusa NCM e NBS juntos.
func TestServicoMandaNBSEBemMandaNCM(t *testing.T) {
	doc := documentoEm(2026, "MA")
	doc.Itens[0].NBS = "101011100"

	item := requisicaoDe(t, doc, aliquota())["itens"].([]any)[0].(map[string]any)
	if _, presente := item["ncm"]; presente {
		t.Error("com NBS, ncm não deve ir")
	}
	if item["nbs"] != "101011100" {
		t.Errorf("nbs = %v", item["nbs"])
	}
}

// Omitir gTribRegular faz o motor recusar documento correto.
func TestTributacaoRegularEhRepassadaQuandoOXMLTraz(t *testing.T) {
	doc := documentoEm(2026, "MA")
	item := requisicaoDe(t, doc, aliquota())["itens"].([]any)[0].(map[string]any)
	if _, presente := item["tributacaoRegular"]; presente {
		t.Error("sem gTribRegular no XML, não se inventa o campo")
	}

	doc.Itens[0].CSTRegular = "000"
	doc.Itens[0].CClassTribRegular = "000001"
	item = requisicaoDe(t, doc, aliquota())["itens"].([]any)[0].(map[string]any)
	regular, presente := item["tributacaoRegular"].(map[string]any)
	if !presente {
		t.Fatal("tributacaoRegular não foi repassada")
	}
	if regular["cst"] != "000" || regular["cClassTrib"] != "000001" {
		t.Errorf("tributacaoRegular = %v", regular)
	}
}
