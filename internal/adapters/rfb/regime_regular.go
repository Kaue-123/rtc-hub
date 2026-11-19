package rfb

import (
	"context"
	"encoding/json"

	"github.com/shopspring/decimal"

	"github.com/Kaue-123/rtc-hub/internal/app"
	"github.com/Kaue-123/rtc-hub/internal/domain"
)

// RegimeRegular fala com calculadora-rtc-regime-regular: CBS, IBS e Imposto
// Seletivo do regime geral.
type RegimeRegular struct{ cliente }

func NovoRegimeRegular(baseURL string) *RegimeRegular {
	return &RegimeRegular{novoCliente(baseURL)}
}

func (m *RegimeRegular) Nome() string { return domain.MotorRegimeRegular }

func (m *RegimeRegular) Versao(ctx context.Context) (domain.CarimboVersao, error) {
	var resposta respostaVersao
	if err := m.obter(ctx, "/api/calculadora/dados-abertos/versao", &resposta); err != nil {
		return domain.CarimboVersao{}, err
	}
	return resposta.carimbo(m.Nome()), nil
}

// ExigeAliquotaInformada: até 2026 o motor usa as alíquotas de referência dele
// e informá-las é PROIBIDO; de 2027 em diante passa a ser obrigatório.
func (m *RegimeRegular) ExigeAliquotaInformada(doc domain.DocumentoFiscal) bool {
	return doc.AnoFatoGerador() >= domain.PrimeiroAnoSemAliquotaDeReferencia
}

type itemRegimeRegular struct {
	Numero            int                `json:"numero"`
	NCM               string             `json:"ncm,omitempty"`
	NBS               string             `json:"nbs,omitempty"`
	CST               string             `json:"cst"`
	CClassTrib        string             `json:"cClassTrib"`
	BaseCalculo       string             `json:"baseCalculo"`
	Quantidade        string             `json:"quantidade,omitempty"`
	Unidade           string             `json:"unidade,omitempty"`
	TributacaoRegular *tributacaoRegular `json:"tributacaoRegular,omitempty"`
	AliquotasNominais *aliquotasNominais `json:"aliquotasNominais,omitempty"`
}

type tributacaoRegular struct {
	CST        string `json:"cst"`
	CClassTrib string `json:"cClassTrib"`
}

type aliquotasNominais struct {
	CBS          string `json:"cbs"`
	IBSEstadual  string `json:"ibsEstadual"`
	IBSMunicipal string `json:"ibsMunicipal"`
}

func (m *RegimeRegular) montaRequisicao(
	doc domain.DocumentoFiscal, aliquota domain.AliquotaVigencia,
) map[string]any {
	informaAliquota := m.ExigeAliquotaInformada(doc)

	itens := make([]itemRegimeRegular, 0, len(doc.Itens))
	for _, item := range doc.Itens {
		corpo := itemRegimeRegular{
			Numero: item.Numero, CST: item.CST, CClassTrib: item.CClassTrib,
			BaseCalculo: texto(item.BaseCalculo), Unidade: item.Unidade,
			Quantidade: texto(item.Quantidade),
		}
		// O motor recusa NCM e NBS juntos: serviço manda NBS, bem manda NCM.
		if item.NBS != "" {
			corpo.NBS = item.NBS
		} else {
			corpo.NCM = item.NCM
		}
		// Omitir faz o motor recusar item correto em algumas cClassTrib.
		if item.CSTRegular != "" && item.CClassTribRegular != "" {
			corpo.TributacaoRegular = &tributacaoRegular{
				CST: item.CSTRegular, CClassTrib: item.CClassTribRegular,
			}
		}
		if informaAliquota {
			corpo.AliquotasNominais = &aliquotasNominais{
				CBS:          aliquota.CBS.String(),
				IBSEstadual:  aliquota.IBSEstadual.String(),
				IBSMunicipal: aliquota.IBSMunicipal.String(),
			}
		}
		itens = append(itens, corpo)
	}

	requisicao := map[string]any{
		"id": identificador(), "versao": "1.0.0", "itens": itens,
	}
	// Campo ausente e campo vazio NÃO são a mesma coisa para o motor: "uf": ""
	// é recusado com VAL-004, enquanto a ausência passa. A NFS-e não traz UF.
	if doc.UF != "" {
		requisicao["uf"] = doc.UF
	}
	if doc.DhEmissao != nil {
		requisicao["dhFatoGerador"] = doc.DhEmissao.Format("2006-01-02T15:04:05-07:00")
	}
	if doc.Municipio != nil {
		requisicao["municipio"] = *doc.Municipio
	}
	return requisicao
}

// Formato da resposta do regime regular: tudo aninhado em tribCalc.IBSCBS.
type respostaRegimeRegular struct {
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
			IBSCBS struct {
				GIBSCBS struct {
					VBC    string `json:"vBC"`
					GIBSUF struct {
						VIBSUF         string `json:"vIBSUF"`
						MemoriaCalculo string `json:"memoriaCalculo"`
						GRed           struct {
							PAliqEfet string `json:"pAliqEfet"`
						} `json:"gRed"`
					} `json:"gIBSUF"`
					GIBSMun struct {
						VIBSMun        string `json:"vIBSMun"`
						MemoriaCalculo string `json:"memoriaCalculo"`
						GRed           struct {
							PAliqEfet string `json:"pAliqEfet"`
						} `json:"gRed"`
					} `json:"gIBSMun"`
					GCBS struct {
						VCBS           string `json:"vCBS"`
						MemoriaCalculo string `json:"memoriaCalculo"`
						GRed           struct {
							PAliqEfet string `json:"pAliqEfet"`
						} `json:"gRed"`
					} `json:"gCBS"`
				} `json:"gIBSCBS"`
			} `json:"IBSCBS"`
		} `json:"tribCalc"`
	} `json:"objetos"`
}

func (m *RegimeRegular) Calcular(
	ctx context.Context, doc domain.DocumentoFiscal, aliquota domain.AliquotaVigencia,
) (app.ResultadoMotor, error) {
	requisicao := m.montaRequisicao(doc, aliquota)
	enviado, recebido, cabecalhos, err := m.postar(
		ctx, "/api/calculadora/observabilidade/regime-geral", requisicao)
	if err != nil {
		return app.ResultadoMotor{}, err
	}

	var resposta respostaRegimeRegular
	if err := json.Unmarshal(recebido, &resposta); err != nil {
		return app.ResultadoMotor{}, err
	}

	itens := make(map[int]domain.ItemApurado, len(resposta.Objetos))
	for _, objeto := range resposta.Objetos {
		grupo := objeto.TribCalc.IBSCBS.GIBSCBS
		apurado := domain.ItemApurado{
			Estado:         objeto.EstadoItem,
			BaseCalculo:    parseDecimal(grupo.VBC),
			IBSUF:          parseDecimal(grupo.GIBSUF.VIBSUF),
			IBSMun:         parseDecimal(grupo.GIBSMun.VIBSMun),
			CBS:            parseDecimal(grupo.GCBS.VCBS),
			AliqEfetIBSUF:  parseDecimal(grupo.GIBSUF.GRed.PAliqEfet),
			AliqEfetIBSMun: parseDecimal(grupo.GIBSMun.GRed.PAliqEfet),
			AliqEfetCBS:    parseDecimal(grupo.GCBS.GRed.PAliqEfet),
			MemoriaIBSUF:   grupo.GIBSUF.MemoriaCalculo,
			MemoriaIBSMun:  grupo.GIBSMun.MemoriaCalculo,
			MemoriaCBS:     grupo.GCBS.MemoriaCalculo,
		}
		for _, erro := range objeto.Erros {
			apurado.Erros = append(apurado.Erros, domain.ErroDoMotor{
				Codigo: erro.Code, Titulo: erro.Title,
				Ponteiro: erro.Pointer, Detalhe: erro.Detail,
			})
		}
		itens[objeto.NObj] = apurado
	}

	carimbo, err := m.Versao(ctx)
	if err != nil {
		return app.ResultadoMotor{}, err
	}

	return app.ResultadoMotor{
		Carimbo: carimbo,
		Itens:   itens,
		// Header avisa que o número saiu de alíquota informada.
		DadosSimulados: cabecalhos.Get("x-warning-dados-simulados") != "",
		Requisicao:     enviado,
		Resposta:       recebido,
	}, nil
}

func parseDecimal(texto string) *decimal.Decimal {
	if texto == "" {
		return nil
	}
	valor, err := decimal.NewFromString(texto)
	if err != nil {
		return nil
	}
	return &valor
}

func texto(valor *decimal.Decimal) string {
	if valor == nil {
		return ""
	}
	return valor.String()
}
