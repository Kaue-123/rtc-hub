package httpapi

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/Kaue-123/rtc-hub/internal/adapters/postgres"
	"github.com/Kaue-123/rtc-hub/internal/app"
)

// Quanto do multipart fica em memória. O resto vai para arquivo temporário, que
// o net/http remove ao fim da requisição. NÃO é o teto do lote — o teto é
// TamanhoMaxLoteBytes, imposto por MaxBytesReader.
const memoriaDoMultipart = 8 << 20

// Usado quando o servidor foi montado sem limite: zero em MaxBytesReader
// recusaria toda requisição, que é pior do que um teto generoso.
const tamanhoMaxLotePadrao = 64 << 20

// lerArquivosDoLote consome o multipart dentro do limite e devolve os arquivos.
// Vive fora do handler para ser exercitável sem banco e sem tenant.
func lerArquivosDoLote(w http.ResponseWriter, r *http.Request, limite int64) ([]app.Arquivo, bool) {
	if limite <= 0 {
		limite = tamanhoMaxLotePadrao
	}
	// ParseMultipartForm NÃO limita o tamanho da requisição: o argumento dela é
	// quanto fica em memória, e a documentação é literal quanto ao resto —
	// "the remainder stored on disk in temporary files". Sem MaxBytesReader
	// qualquer upload passava, enchendo o disco, e o 413 que este handler
	// prometia só disparava em multipart malformado.
	r.Body = http.MaxBytesReader(w, r.Body, limite)

	if err := r.ParseMultipartForm(memoriaDoMultipart); err != nil {
		var excedeu *http.MaxBytesError
		if errors.As(err, &excedeu) {
			erro(w, http.StatusRequestEntityTooLarge,
				fmt.Sprintf("lote acima do limite de %d bytes", limite))
			return nil, false
		}
		erro(w, http.StatusBadRequest, "multipart inválido: "+err.Error())
		return nil, false
	}

	cabecalhos := r.MultipartForm.File["arquivos"]
	if len(cabecalhos) == 0 {
		erro(w, http.StatusUnprocessableEntity, "envie ao menos um arquivo no campo 'arquivos'")
		return nil, false
	}

	arquivos := make([]app.Arquivo, 0, len(cabecalhos))
	for _, cabecalho := range cabecalhos {
		aberto, err := cabecalho.Open()
		if err != nil {
			erro(w, http.StatusBadRequest, cabecalho.Filename+": "+err.Error())
			return nil, false
		}
		conteudo, err := io.ReadAll(aberto)
		_ = aberto.Close()
		if err != nil {
			erro(w, http.StatusBadRequest, cabecalho.Filename+": "+err.Error())
			return nil, false
		}
		arquivos = append(arquivos, app.Arquivo{Nome: cabecalho.Filename, Conteudo: conteudo})
	}
	return arquivos, true
}

func (s Servidor) criarLote(w http.ResponseWriter, r *http.Request) {
	tenant := tenantDe(r.Context())

	arquivos, ok := lerArquivosDoLote(w, r, s.TamanhoMaxLoteBytes)
	if !ok {
		return
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
