package httpapi

import (
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/Kaue-123/rtc-hub/internal/adapters/postgres/db"
	"github.com/Kaue-123/rtc-hub/internal/domain"
)

// --------------------------------------------------------------------------
// Empresas
// --------------------------------------------------------------------------

type empresaEntrada struct {
	CNPJ         string `json:"cnpj"`
	RazaoSocial  string `json:"razao_social"`
	NomeFantasia string `json:"nome_fantasia"`
	CRT          string `json:"crt"`
	UF           string `json:"uf"`
	Municipio    *int32 `json:"municipio"`
	Ativo        *bool  `json:"ativo"`
}

func (s Servidor) criarEmpresa(w http.ResponseWriter, r *http.Request) {
	var entrada empresaEntrada
	if !lerCorpo(w, r, &entrada) {
		return
	}
	if entrada.CNPJ == "" || entrada.RazaoSocial == "" {
		erro(w, http.StatusUnprocessableEntity, "cnpj e razao_social são obrigatórios")
		return
	}

	empresa, err := s.consultas().CriarEmpresa(r.Context(), db.CriarEmpresaParams{
		TenantID:     tenantDe(r.Context()).ID,
		CNPJ:         entrada.CNPJ,
		RazaoSocial:  entrada.RazaoSocial,
		NomeFantasia: textoOuNil(entrada.NomeFantasia),
		CRT:          textoOuNil(entrada.CRT),
		UF:           textoOuNil(entrada.UF),
		Municipio:    entrada.Municipio,
	})
	if err != nil {
		erro(w, http.StatusConflict, err.Error())
		return
	}
	responder(w, http.StatusCreated, empresa)
}

func (s Servidor) listarEmpresas(w http.ResponseWriter, r *http.Request) {
	empresas, err := s.consultas().ListarEmpresas(r.Context(), db.ListarEmpresasParams{
		TenantID:     tenantDe(r.Context()).ID,
		ApenasAtivas: textoOuNilBool(r.URL.Query().Get("ativas")),
		Busca:        textoOuNil(r.URL.Query().Get("busca")),
		Limite:       inteiroDaQuery(r, "limite", 50),
		Deslocamento: inteiroDaQuery(r, "deslocamento", 0),
	})
	if err != nil {
		erroDoRepo(w, err)
		return
	}
	responder(w, http.StatusOK, empresas)
}

func (s Servidor) lerEmpresa(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaURL(w, r)
	if !ok {
		return
	}
	empresa, err := s.consultas().BuscarEmpresa(r.Context(), db.BuscarEmpresaParams{
		ID: id, TenantID: tenantDe(r.Context()).ID,
	})
	if err != nil {
		erro(w, http.StatusNotFound, "empresa não encontrada")
		return
	}
	responder(w, http.StatusOK, empresa)
}

func (s Servidor) atualizarEmpresa(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaURL(w, r)
	if !ok {
		return
	}
	var entrada empresaEntrada
	if !lerCorpo(w, r, &entrada) {
		return
	}
	ativo := true
	if entrada.Ativo != nil {
		ativo = *entrada.Ativo
	}

	empresa, err := s.consultas().AtualizarEmpresa(r.Context(), db.AtualizarEmpresaParams{
		ID: id, TenantID: tenantDe(r.Context()).ID,
		RazaoSocial:  entrada.RazaoSocial,
		NomeFantasia: textoOuNil(entrada.NomeFantasia),
		CRT:          textoOuNil(entrada.CRT),
		UF:           textoOuNil(entrada.UF),
		Municipio:    entrada.Municipio,
		Ativo:        ativo,
	})
	if err != nil {
		erro(w, http.StatusNotFound, "empresa não encontrada")
		return
	}
	responder(w, http.StatusOK, empresa)
}

func (s Servidor) excluirEmpresa(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaURL(w, r)
	if !ok {
		return
	}
	err := s.consultas().ExcluirEmpresa(r.Context(), db.ExcluirEmpresaParams{
		ID: id, TenantID: tenantDe(r.Context()).ID,
	})
	if err != nil {
		erroDoRepo(w, err)
		return
	}
	responder(w, http.StatusNoContent, nil)
}

// --------------------------------------------------------------------------
// Alíquotas
// --------------------------------------------------------------------------

type aliquotaEntrada struct {
	VigenciaInicio string `json:"vigencia_inicio"`
	VigenciaFim    string `json:"vigencia_fim"`
	UF             string `json:"uf"`
	Municipio      *int32 `json:"municipio"`
	CBS            string `json:"cbs"`
	IBSEstadual    string `json:"ibs_estadual"`
	IBSMunicipal   string `json:"ibs_municipal"`
	Observacao     string `json:"observacao"`
}

func (e aliquotaEntrada) datas() (time.Time, *time.Time, error) {
	inicio, err := time.Parse("2006-01-02", e.VigenciaInicio)
	if err != nil {
		return time.Time{}, nil, err
	}
	if e.VigenciaFim == "" {
		return inicio, nil, nil
	}
	fim, err := time.Parse("2006-01-02", e.VigenciaFim)
	if err != nil {
		return time.Time{}, nil, err
	}
	return inicio, &fim, nil
}

func (e aliquotaEntrada) valores() (decimal.Decimal, decimal.Decimal, decimal.Decimal, error) {
	cbs, err := decimal.NewFromString(e.CBS)
	if err != nil {
		return cbs, cbs, cbs, err
	}
	estadual, err := decimal.NewFromString(e.IBSEstadual)
	if err != nil {
		return cbs, estadual, estadual, err
	}
	municipal, err := decimal.NewFromString(e.IBSMunicipal)
	return cbs, estadual, municipal, err
}

func (s Servidor) criarAliquota(w http.ResponseWriter, r *http.Request) {
	var entrada aliquotaEntrada
	if !lerCorpo(w, r, &entrada) {
		return
	}
	inicio, fim, err := entrada.datas()
	if err != nil {
		erro(w, http.StatusUnprocessableEntity, "vigência inválida: "+err.Error())
		return
	}
	cbs, estadual, municipal, err := entrada.valores()
	if err != nil {
		erro(w, http.StatusUnprocessableEntity, "alíquota inválida: "+err.Error())
		return
	}

	tenantID := tenantDe(r.Context()).ID
	aliquota, err := s.consultas().CriarAliquota(r.Context(), db.CriarAliquotaParams{
		TenantID:       &tenantID,
		VigenciaInicio: inicio,
		VigenciaFim:    fim,
		UF:             textoOuNil(entrada.UF),
		Municipio:      entrada.Municipio,
		Cbs:            cbs,
		IbsEstadual:    estadual,
		IbsMunicipal:   municipal,
		// Linha criada pela API é sempre do tenant: a padrão nasce da semente e
		// não pode ser alterada por quem usa o produto.
		Fonte:      domain.FonteAliquotaTenant,
		Observacao: textoOuNil(entrada.Observacao),
	})
	if err != nil {
		erro(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	responder(w, http.StatusCreated, aliquota)
}

func (s Servidor) listarAliquotas(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantDe(r.Context()).ID
	linhas, err := s.consultas().ListarAliquotas(r.Context(), &tenantID)
	if err != nil {
		erroDoRepo(w, err)
		return
	}
	responder(w, http.StatusOK, linhas)
}

func (s Servidor) atualizarAliquota(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaURL(w, r)
	if !ok {
		return
	}
	var entrada aliquotaEntrada
	if !lerCorpo(w, r, &entrada) {
		return
	}
	inicio, fim, err := entrada.datas()
	if err != nil {
		erro(w, http.StatusUnprocessableEntity, "vigência inválida: "+err.Error())
		return
	}
	cbs, estadual, municipal, err := entrada.valores()
	if err != nil {
		erro(w, http.StatusUnprocessableEntity, "alíquota inválida: "+err.Error())
		return
	}

	tenantID := tenantDe(r.Context()).ID
	aliquota, err := s.consultas().AtualizarAliquota(r.Context(), db.AtualizarAliquotaParams{
		ID: id, TenantID: &tenantID,
		VigenciaInicio: inicio, VigenciaFim: fim,
		UF: textoOuNil(entrada.UF), Municipio: entrada.Municipio,
		Cbs: cbs, IbsEstadual: estadual, IbsMunicipal: municipal,
		Observacao: textoOuNil(entrada.Observacao),
	})
	if err != nil {
		// A linha padrão tem tenant_id nulo e não casa com o WHERE — alterar a
		// alíquota padrão pela API é 404 de propósito.
		erro(w, http.StatusNotFound, "alíquota do tenant não encontrada")
		return
	}
	responder(w, http.StatusOK, aliquota)
}

func (s Servidor) excluirAliquota(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaURL(w, r)
	if !ok {
		return
	}
	tenantID := tenantDe(r.Context()).ID
	if err := s.consultas().ExcluirAliquota(r.Context(), db.ExcluirAliquotaParams{
		ID: id, TenantID: &tenantID,
	}); err != nil {
		erroDoRepo(w, err)
		return
	}
	responder(w, http.StatusNoContent, nil)
}

func textoOuNilBool(valor string) *bool {
	if valor == "" {
		return nil
	}
	v := valor == "true" || valor == "1"
	return &v
}
