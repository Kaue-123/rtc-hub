package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Kaue-123/rtc-hub/internal/adapters/postgres/db"
	"github.com/Kaue-123/rtc-hub/internal/app"
	"github.com/Kaue-123/rtc-hub/internal/domain"
)

// Repositorio satisfaz app.RepoAliquotas e app.RepoConferencia.
type Repositorio struct {
	pool     *pgxpool.Pool
	consulta *db.Queries
}

func NovoRepositorio(pool *pgxpool.Pool) *Repositorio {
	return &Repositorio{pool: pool, consulta: db.New(pool)}
}

// --------------------------------------------------------------------------
// Alíquotas
// --------------------------------------------------------------------------

func (r *Repositorio) Aplicaveis(
	ctx context.Context, tenantID int64, competencia time.Time, uf string, municipio *int32,
) ([]domain.AliquotaVigencia, error) {
	var ufFiltro *string
	if uf != "" {
		ufFiltro = &uf
	}

	linhas, err := r.consulta.AliquotasAplicaveis(ctx, db.AliquotasAplicaveisParams{
		Competencia: competencia,
		TenantID:    &tenantID,
		UF:          ufFiltro,
		Municipio:   municipio,
	})
	if err != nil {
		return nil, err
	}

	candidatas := make([]domain.AliquotaVigencia, 0, len(linhas))
	for _, linha := range linhas {
		candidatas = append(candidatas, paraDominioAliquota(linha))
	}
	return candidatas, nil
}

func paraDominioAliquota(linha db.AliquotaVigencia) domain.AliquotaVigencia {
	return domain.AliquotaVigencia{
		ID:             linha.ID,
		TenantID:       linha.TenantID,
		VigenciaInicio: linha.VigenciaInicio,
		VigenciaFim:    linha.VigenciaFim,
		UF:             linha.UF,
		Municipio:      linha.Municipio,
		CBS:            linha.Cbs,
		IBSEstadual:    linha.IbsEstadual,
		IBSMunicipal:   linha.IbsMunicipal,
		Fonte:          linha.Fonte,
		Observacao:     linha.Observacao,
	}
}

// --------------------------------------------------------------------------
// Conferência
// --------------------------------------------------------------------------

func (r *Repositorio) CriarLote(
	ctx context.Context, tenantID int64, empresaID *int64, descricao string,
) (int64, error) {
	lote, err := r.consulta.CriarLote(ctx, db.CriarLoteParams{
		TenantID: tenantID, EmpresaID: empresaID, Descricao: texto(descricao),
	})
	if err != nil {
		return 0, err
	}
	return lote.ID, nil
}

func (r *Repositorio) ConcluirLote(ctx context.Context, tenantID, loteID int64) error {
	_, err := r.consulta.ConcluirLote(ctx, db.ConcluirLoteParams{ID: loteID, TenantID: tenantID})
	return err
}

// SalvarDocumento grava documento e divergências na MESMA transação: documento
// sem as divergências dele é pior que documento nenhum, porque parece conforme.
func (r *Repositorio) SalvarDocumento(
	ctx context.Context, tenantID, loteID int64, conferido app.DocumentoConferido,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	consulta := r.consulta.WithTx(tx)
	doc := conferido.Documento

	gravado, err := consulta.CriarDocumento(ctx, db.CriarDocumentoParams{
		TenantID:          tenantID,
		LoteID:            loteID,
		Arquivo:           doc.Arquivo,
		Tipo:              texto(doc.Tipo),
		Modelo:            texto(doc.Modelo),
		Chave:             texto(doc.Chave),
		Numero:            texto(doc.Numero),
		DhEmissao:         doc.DhEmissao,
		UF:                texto(doc.UF),
		Municipio:         doc.Municipio,
		CRT:               texto(doc.CRT),
		EmitenteCnpj:      texto(doc.EmitenteCNPJ),
		Estado:            conferido.Estado,
		Motor:             texto(conferido.Motor),
		Motivo:            texto(conferido.Motivo),
		DadosSimulados:    conferido.DadosSimulados,
		MotorVersaoApp:    texto(conferido.Carimbo.VersaoApp),
		MotorVersaoDb:     texto(conferido.Carimbo.VersaoDB),
		MotorDataVersaoDb: texto(conferido.Carimbo.DataVersaoDB),
		AliquotaOrigemID:  conferido.AliquotaOrigemID,
		Requisicao:        conferido.Requisicao,
		Resposta:          conferido.Resposta,
	})
	if err != nil {
		return fmt.Errorf("gravar documento: %w", err)
	}

	for _, divergencia := range conferido.Divergencias {
		_, err := consulta.CriarDivergencia(ctx, db.CriarDivergenciaParams{
			TenantID:       tenantID,
			DocumentoID:    gravado.ID,
			Item:           int32(divergencia.Item),
			Campo:          divergencia.Campo,
			Categoria:      divergencia.Categoria,
			Severidade:     divergencia.Severidade,
			Declarado:      divergencia.Declarado,
			Apurado:        divergencia.Apurado,
			Diferenca:      nullDecimal(divergencia.Diferenca),
			Codigo:         texto(divergencia.Codigo),
			Detalhe:        texto(divergencia.Detalhe),
			MemoriaCalculo: texto(divergencia.MemoriaCalculo),
		})
		if err != nil {
			return fmt.Errorf("gravar divergência: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// --------------------------------------------------------------------------
// Leitura de lote, para a API
// --------------------------------------------------------------------------

type DocumentoComDivergencias struct {
	Documento    db.Documento
	Divergencias []db.Divergencia
}

func (r *Repositorio) LerLote(
	ctx context.Context, tenantID, loteID int64,
) (db.Lote, []DocumentoComDivergencias, error) {
	lote, err := r.consulta.BuscarLote(ctx, db.BuscarLoteParams{ID: loteID, TenantID: tenantID})
	if err != nil {
		if err == pgx.ErrNoRows {
			return db.Lote{}, nil, ErrNaoEncontrado
		}
		return db.Lote{}, nil, err
	}

	documentos, err := r.consulta.ListarDocumentosDoLote(ctx,
		db.ListarDocumentosDoLoteParams{LoteID: loteID, TenantID: tenantID})
	if err != nil {
		return lote, nil, err
	}
	divergencias, err := r.consulta.ListarDivergenciasDoLote(ctx,
		db.ListarDivergenciasDoLoteParams{LoteID: loteID, TenantID: tenantID})
	if err != nil {
		return lote, nil, err
	}

	porDocumento := make(map[int64][]db.Divergencia, len(documentos))
	for _, divergencia := range divergencias {
		porDocumento[divergencia.DocumentoID] = append(porDocumento[divergencia.DocumentoID], divergencia)
	}

	resultado := make([]DocumentoComDivergencias, 0, len(documentos))
	for _, documento := range documentos {
		resultado = append(resultado, DocumentoComDivergencias{
			Documento: documento, Divergencias: porDocumento[documento.ID],
		})
	}
	return lote, resultado, nil
}

// Consultas expõe o acesso gerado para os casos de uso de cadastro.
func (r *Repositorio) Consultas() *db.Queries { return r.consulta }
