package rfb

import (
	"encoding/json"
	"testing"

	"github.com/Kaue-123/rtc-hub/internal/domain"
)

func itemSimplesDe(t *testing.T, aliquota domain.AliquotaVigencia) map[string]any {
	t.Helper()
	motor := NovoSimplesNacional("http://exemplo")
	bruto, err := json.Marshal(motor.montaRequisicao(documentoEm(2027, "MA"), aliquota))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var corpo map[string]any
	if err := json.Unmarshal(bruto, &corpo); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return corpo["itens"].([]any)[0].(map[string]any)
}

// No Simples não há IBS estadual e municipal separados: o motor aplica pIBSSN
// uma única vez sobre a base e o vIBSSN que sai é o IBS inteiro. Medido no motor
// 1.6.1, base 1000,00: pIBSSN 0.05 → vIBSSN 0.50; pIBSSN 0.10 → vIBSSN 1.00.
// Mandar só a parcela estadual fazia o imposto sair pela metade.
func TestPIBSSNLevaOIBSInteiroENaoSoAParcelaEstadual(t *testing.T) {
	item := itemSimplesDe(t, aliquota()) // 0.05 estadual + 0.05 municipal

	if item["pIBSSN"] != "0.1" {
		t.Errorf("pIBSSN = %v, esperava 0.1 (estadual + municipal)", item["pIBSSN"])
	}
	if item["pCBSSN"] != "8.8" {
		t.Errorf("pCBSSN = %v", item["pCBSSN"])
	}
}
