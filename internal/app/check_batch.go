package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/Kaue-123/rtc-hub/internal/domain"
)

// Estados de um documento no lote.
const (
	EstadoConferido      = "CONFERIDO"
	EstadoNaoReconhecido = "NAO_RECONHECIDO"
	EstadoForaDeEscopo   = "FORA_DE_ESCOPO"
	EstadoSemAliquota    = "SEM_ALIQUOTA"
	EstadoSemItens       = "SEM_ITENS"
	EstadoFalhaMotor     = "FALHA_MOTOR"
)

type Arquivo struct {
	Nome     string
	Conteudo []byte
}

// ConferirLote orquestra o caso de uso: ler, resolver alíquota, calcular,
// comparar e persistir. Um arquivo ruim não derruba o lote.
type ConferirLote struct {
	Leitor       LeitorDeDocumento
	Calculadoras map[string]Calculadora
	Aliquotas    RepoAliquotas
	Repo         RepoConferencia
}

type PedidoDeLote struct {
	TenantID  int64
	EmpresaID *int64
	Descricao string
	Arquivos  []Arquivo
}

func (c ConferirLote) Executar(ctx context.Context, pedido PedidoDeLote) (int64, error) {
	if len(pedido.Arquivos) == 0 {
		return 0, errors.New("lote sem arquivos")
	}

	loteID, err := c.Repo.CriarLote(ctx, pedido.TenantID, pedido.EmpresaID, pedido.Descricao)
	if err != nil {
		return 0, fmt.Errorf("criar lote: %w", err)
	}

	for _, arquivo := range pedido.Arquivos {
		conferido := c.confereArquivo(ctx, pedido.TenantID, arquivo)
		if err := c.Repo.SalvarDocumento(ctx, pedido.TenantID, loteID, conferido); err != nil {
			return 0, fmt.Errorf("salvar %s: %w", arquivo.Nome, err)
		}
	}

	if err := c.Repo.ConcluirLote(ctx, pedido.TenantID, loteID); err != nil {
		return 0, fmt.Errorf("concluir lote: %w", err)
	}
	return loteID, nil
}

func (c ConferirLote) confereArquivo(ctx context.Context, tenantID int64, arquivo Arquivo) DocumentoConferido {
	doc, err := c.Leitor.Ler(arquivo.Conteudo, arquivo.Nome)
	if err != nil {
		return DocumentoConferido{
			Documento: domain.DocumentoFiscal{Arquivo: arquivo.Nome},
			Estado:    EstadoNaoReconhecido,
			Motivo:    err.Error(),
		}
	}

	resultado := DocumentoConferido{Documento: doc, Motor: doc.Motor()}

	calculadora, existe := c.Calculadoras[doc.Motor()]
	if !existe {
		resultado.Estado = EstadoForaDeEscopo
		resultado.Motivo = fmt.Sprintf(
			"CRT %q fora do escopo do cálculo (NT 2026.005): MEI e regime não identificado não são calculados",
			doc.CRT)
		return resultado
	}
	if len(doc.Itens) == 0 {
		resultado.Estado = EstadoSemItens
		resultado.Motivo = "documento sem itens"
		return resultado
	}

	var aliquota domain.AliquotaVigencia
	if calculadora.ExigeAliquotaInformada(doc) {
		aliquota, err = c.resolveAliquota(ctx, tenantID, doc)
		if err != nil {
			resultado.Estado = EstadoSemAliquota
			resultado.Motivo = err.Error()
			return resultado
		}
		resultado.AliquotaOrigemID = &aliquota.ID
		if aliquota.Observacao != nil {
			resultado.Motivo = *aliquota.Observacao
		}
	}

	saida, err := calculadora.Calcular(ctx, doc, aliquota)
	if err != nil {
		resultado.Estado = EstadoFalhaMotor
		resultado.Motivo = err.Error()
		return resultado
	}

	resultado.Estado = EstadoConferido
	resultado.Carimbo = saida.Carimbo
	resultado.DadosSimulados = saida.DadosSimulados
	resultado.Requisicao = saida.Requisicao
	resultado.Resposta = saida.Resposta
	resultado.Divergencias = domain.Conferir(doc, saida.Itens)
	return resultado
}

func (c ConferirLote) resolveAliquota(
	ctx context.Context, tenantID int64, doc domain.DocumentoFiscal,
) (domain.AliquotaVigencia, error) {
	if doc.DhEmissao == nil {
		return domain.AliquotaVigencia{}, errors.New("documento sem data de emissão")
	}
	competencia := *doc.DhEmissao

	candidatas, err := c.Aliquotas.Aplicaveis(ctx, tenantID, competencia, doc.UF, doc.Municipio)
	if err != nil {
		return domain.AliquotaVigencia{}, fmt.Errorf("consultar alíquotas: %w", err)
	}
	escolhida, err := domain.ResolverAliquota(candidatas, competencia, doc.UF, doc.Municipio)
	if err != nil {
		return domain.AliquotaVigencia{}, fmt.Errorf(
			"%w em %s para uf=%s", err, competencia.Format("2006-01-02"), doc.UF)
	}
	return escolhida, nil
}
