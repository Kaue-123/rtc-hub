package rfb

import (
	"context"
	"encoding/json"

	"github.com/Kaue-123/rtc-hub/internal/app"
	"github.com/Kaue-123/rtc-hub/internal/domain"
)

// SimplesNacional fala com calculadora-rtc-simples-nacional.
type SimplesNacional struct{ cliente }

func NovoSimplesNacional(baseURL string) *SimplesNacional {
	return &SimplesNacional{novoCliente(baseURL)}
}

func (m *SimplesNacional) Nome() string { return domain.MotorSimples }

func (m *SimplesNacional) Versao(ctx context.Context) (domain.CarimboVersao, error) {
	var resposta respostaVersao
	if err := m.obter(ctx, "/api/versao", &resposta); err != nil {
		return domain.CarimboVersao{}, err
	}
	return resposta.carimbo(m.Nome()), nil
}

// No Simples a alíquota efetiva sempre vem de quem chama, em qualquer ano.
func (m *SimplesNacional) ExigeAliquotaInformada(domain.DocumentoFiscal) bool { return true }

type itemSimples struct {
	Numero               int    `json:"numero"`
	CAtivSN              string `json:"cAtivSN"`
	CClassTrib           string `json:"cClassTrib"`
	BaseCalculo          string `json:"baseCalculo"`
	ValorImpostoSeletivo string `json:"valorImpostoSeletivo"`
	PIBSSN               string `json:"pIBSSN"`
	PCBSSN               string `json:"pCBSSN"`
}

// Resposta do Simples: plana em tribCalc, sem o aninhamento do regime regular.
type respostaSimples struct {
	Objetos []struct {
		NObj       int    `json:"nObj"`
		EstadoItem string `json:"estadoItem"`
		Erros      []struct {
			Code    string `json:"code"`
			Title   string `json:"title"`
			Pointer string `json:"pointer"`
			Detail  string `json:"detail"`
		} `json:"erros"`
		TribCalc struct {
			VBC    string `json:"vBC"`
			PIBSSN string `json:"pIBSSN"`
			VIBSSN string `json:"vIBSSN"`
			PCBSSN string `json:"pCBSSN"`
			VCBSSN string `json:"vCBSSN"`
		} `json:"tribCalc"`
	} `json:"objetos"`
}

func (m *SimplesNacional) montaRequisicao(
	doc domain.DocumentoFiscal, aliquota domain.AliquotaVigencia,
) map[string]any {
	itens := make([]itemSimples, 0, len(doc.Itens))
	for _, item := range doc.Itens {
		valorIS := "0.00"
		if item.ValorIS != nil {
			valorIS = item.ValorIS.String()
		}
		baseCalculo := "0.00"
		if item.BaseCalculo != nil {
			baseCalculo = item.BaseCalculo.String()
		}
		itens = append(itens, itemSimples{
			Numero: item.Numero, CAtivSN: item.CAtivSN, CClassTrib: item.CClassTrib,
			BaseCalculo: baseCalculo, ValorImpostoSeletivo: valorIS,
			// pIBSSN é o IBS INTEIRO, não a parcela estadual: o Simples não
			// separa estadual de municipal. Ver IBSSimplesNacional.
			PIBSSN: aliquota.IBSSimplesNacional().String(), PCBSSN: aliquota.CBS.String(),
		})
	}

	requisicao := map[string]any{
		"id": identificador(), "versao": "1.0.0", "itens": itens,
	}
	// Vazio é recusado onde ausente passa — ver regime_regular.go.
	if doc.Modelo != "" {
		requisicao["tpDoc"] = doc.Modelo
	}
	if doc.CRT != "" {
		requisicao["CRT"] = doc.CRT
	}
	if doc.DhEmissao != nil {
		requisicao["data"] = doc.DhEmissao.Format("2006-01-02")
	}
	return requisicao
}

func (m *SimplesNacional) Calcular(
	ctx context.Context, doc domain.DocumentoFiscal, aliquota domain.AliquotaVigencia,
) (app.ResultadoMotor, error) {
	requisicao := m.montaRequisicao(doc, aliquota)

	enviado, recebido, _, err := m.postar(
		ctx, "/api/calculadora/observabilidade/simples-nacional", requisicao)
	if err != nil {
		return app.ResultadoMotor{}, err
	}

	var resposta respostaSimples
	if err := json.Unmarshal(recebido, &resposta); err != nil {
		return app.ResultadoMotor{}, err
	}

	apurados := make(map[int]domain.ItemApurado, len(resposta.Objetos))
	for _, objeto := range resposta.Objetos {
		apurado := domain.ItemApurado{
			Estado:      objeto.EstadoItem,
			BaseCalculo: parseDecimal(objeto.TribCalc.VBC),
			IBSUF:       parseDecimal(objeto.TribCalc.VIBSSN),
			// O Simples não separa IBS estadual de municipal: deixar nulo é o
			// que impede o comparador de acusar divergência inexistente.
			IBSMun:        nil,
			CBS:           parseDecimal(objeto.TribCalc.VCBSSN),
			AliqEfetIBSUF: parseDecimal(objeto.TribCalc.PIBSSN),
			AliqEfetCBS:   parseDecimal(objeto.TribCalc.PCBSSN),
		}
		for _, erro := range objeto.Erros {
			apurado.Erros = append(apurado.Erros, domain.ErroDoMotor{
				Codigo: erro.Code, Titulo: erro.Title,
				Ponteiro: erro.Pointer, Detalhe: erro.Detail,
			})
		}
		apurados[objeto.NObj] = apurado
	}

	carimbo, err := m.Versao(ctx)
	if err != nil {
		return app.ResultadoMotor{}, err
	}

	return app.ResultadoMotor{
		Carimbo: carimbo, Itens: apurados,
		Requisicao: enviado, Resposta: recebido,
	}, nil
}
