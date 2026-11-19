package httpapi

import (
	"io"
	"net/http"

	"github.com/Kaue-123/rtc-hub/internal/adapters/postgres"
	"github.com/Kaue-123/rtc-hub/internal/app"
)

func (s Servidor) criarLote(w http.ResponseWriter, r *http.Request) {
	tenant := tenantDe(r.Context())

	if err := r.ParseMultipartForm(s.TamanhoMaxLoteBytes); err != nil {
		erro(w, http.StatusRequestEntityTooLarge, "lote acima do limite: "+err.Error())
		return
	}
	cabecalhos := r.MultipartForm.File["arquivos"]
	if len(cabecalhos) == 0 {
		erro(w, http.StatusUnprocessableEntity, "envie ao menos um arquivo no campo 'arquivos'")
		return
	}

	arquivos := make([]app.Arquivo, 0, len(cabecalhos))
	for _, cabecalho := range cabecalhos {
		aberto, err := cabecalho.Open()
		if err != nil {
			erro(w, http.StatusBadRequest, cabecalho.Filename+": "+err.Error())
			return
		}
		conteudo, err := io.ReadAll(aberto)
		_ = aberto.Close()
		if err != nil {
			erro(w, http.StatusBadRequest, cabecalho.Filename+": "+err.Error())
			return
		}
		arquivos = append(arquivos, app.Arquivo{Nome: cabecalho.Filename, Conteudo: conteudo})
	}

	loteID, err := s.Conferir.Executar(r.Context(), app.PedidoDeLote{
		TenantID:  tenant.ID,
		Descricao: r.URL.Query().Get("descricao"),
		Arquivos:  arquivos,
	})
	if err != nil {
		erro(w, http.StatusInternalServerError, err.Error())
		return
	}

	resumo, err := s.montarResumo(r, tenant.ID, loteID, false)
	if err != nil {
		erroDoRepo(w, err)
		return
	}
	responder(w, http.StatusCreated, resumo)
}

func (s Servidor) lerLote(w http.ResponseWriter, r *http.Request) {
	tenant := tenantDe(r.Context())
	loteID, ok := idDaURL(w, r)
	if !ok {
		return
	}

	resumo, err := s.montarResumo(r, tenant.ID, loteID, true)
	if err != nil {
		erroDoRepo(w, err)
		return
	}
	responder(w, http.StatusOK, resumo)
}

func (s Servidor) montarResumo(
	r *http.Request, tenantID, loteID int64, detalhado bool,
) (map[string]any, error) {
	lote, documentos, err := s.Repo.LerLote(r.Context(), tenantID, loteID)
	if err != nil {
		return nil, err
	}

	porEstado := map[string]int{}
	porSeveridade := map[string]int{}
	porCategoria := map[string]int{}
	totalDivergencias, conformes := 0, 0

	for _, item := range documentos {
		porEstado[item.Documento.Estado]++
		totalDivergencias += len(item.Divergencias)
		if item.Documento.Estado == app.EstadoConferido && len(item.Divergencias) == 0 {
			conformes++
		}
		for _, divergencia := range item.Divergencias {
			porSeveridade[divergencia.Severidade]++
			porCategoria[divergencia.Categoria]++
		}
	}

	resumo := map[string]any{
		"lote":           lote.ID,
		"estado":         lote.Estado,
		"descricao":      lote.Descricao,
		"documentos":     len(documentos),
		"por_estado":     porEstado,
		"divergencias":   totalDivergencias,
		"por_severidade": porSeveridade,
		"por_categoria":  porCategoria,
		"conformes":      conformes,
	}
	if !detalhado {
		return resumo, nil
	}

	detalhe := make([]map[string]any, 0, len(documentos))
	for _, item := range documentos {
		detalhe = append(detalhe, documentoEmJSON(item))
	}
	resumo["detalhe"] = detalhe
	return resumo, nil
}

func documentoEmJSON(item postgres.DocumentoComDivergencias) map[string]any {
	doc := item.Documento

	divergencias := make([]map[string]any, 0, len(item.Divergencias))
	for _, d := range item.Divergencias {
		registro := map[string]any{
			"item": d.Item, "campo": d.Campo, "categoria": d.Categoria,
			"severidade": d.Severidade, "declarado": d.Declarado, "apurado": d.Apurado,
			"codigo": d.Codigo, "detalhe": d.Detalhe, "memoria_calculo": d.MemoriaCalculo,
		}
		if d.Diferenca.Valid {
			registro["diferenca"] = d.Diferenca.Decimal.String()
		}
		divergencias = append(divergencias, registro)
	}

	return map[string]any{
		"arquivo": doc.Arquivo, "tipo": doc.Tipo, "chave": doc.Chave,
		"estado": doc.Estado, "motor": doc.Motor, "motivo": doc.Motivo,
		"dados_simulados": doc.DadosSimulados,
		"motor_versao": map[string]any{
			"app": doc.MotorVersaoApp, "db": doc.MotorVersaoDb, "data_db": doc.MotorDataVersaoDb,
		},
		"aliquota_origem_id": doc.AliquotaOrigemID,
		"divergencias":       divergencias,
	}
}
