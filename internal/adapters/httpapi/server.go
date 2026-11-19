// Package httpapi é o adaptador de entrada HTTP.
//
// Chi fica contido aqui: os casos de uso recebem tipos próprios e não sabem que
// existe HTTP. Trocar de roteador, ou pendurar uma segunda entrada (CLI, fila,
// tela), não encosta no núcleo.
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Kaue-123/rtc-hub/internal/adapters/postgres"
	"github.com/Kaue-123/rtc-hub/internal/adapters/postgres/db"
	"github.com/Kaue-123/rtc-hub/internal/app"
)

type Servidor struct {
	Conferir            app.ConferirLote
	Repo                *postgres.Repositorio
	Motores             map[string]app.Calculadora
	TamanhoMaxLoteBytes int64
}

func (s Servidor) Rotas() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer)

	r.Get("/saude", s.saude)

	// resolverTenant é o lugar onde a autenticação vai entrar. Hoje confia no
	// cabeçalho, o que NÃO pode ir para produção.
	r.Route("/lotes", func(r chi.Router) {
		r.Use(s.resolverTenant)
		r.Post("/", s.criarLote)
		r.Get("/{id}", s.lerLote)
	})
	r.Route("/empresas", func(r chi.Router) {
		r.Use(s.resolverTenant)
		r.Post("/", s.criarEmpresa)
		r.Get("/", s.listarEmpresas)
		r.Get("/{id}", s.lerEmpresa)
		r.Put("/{id}", s.atualizarEmpresa)
		r.Delete("/{id}", s.excluirEmpresa)
	})
	r.Route("/aliquotas", func(r chi.Router) {
		r.Use(s.resolverTenant)
		r.Post("/", s.criarAliquota)
		r.Get("/", s.listarAliquotas)
		r.Put("/{id}", s.atualizarAliquota)
		r.Delete("/{id}", s.excluirAliquota)
	})

	return r
}

func (s Servidor) saude(w http.ResponseWriter, r *http.Request) {
	estado := map[string]any{"status": "ok"}
	motores := map[string]any{}

	for nome, motor := range s.Motores {
		carimbo, err := motor.Versao(r.Context())
		if err != nil {
			motores[nome] = map[string]string{"erro": err.Error()}
			estado["status"] = "degradado" // motor fora não derruba o BFF
			continue
		}
		motores[nome] = map[string]string{
			"versaoApp": carimbo.VersaoApp, "versaoDb": carimbo.VersaoDB,
			"dataVersaoDb": carimbo.DataVersaoDB,
		}
	}
	estado["motores"] = motores
	responder(w, http.StatusOK, estado)
}

// --------------------------------------------------------------------------
// Utilidades de resposta
// --------------------------------------------------------------------------

func responder(w http.ResponseWriter, status int, corpo any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if corpo != nil {
		_ = json.NewEncoder(w).Encode(corpo)
	}
}

func erro(w http.ResponseWriter, status int, mensagem string) {
	responder(w, status, map[string]string{"erro": mensagem})
}

func erroDoRepo(w http.ResponseWriter, err error) {
	if errors.Is(err, postgres.ErrNaoEncontrado) {
		erro(w, http.StatusNotFound, "não encontrado")
		return
	}
	erro(w, http.StatusInternalServerError, err.Error())
}

func lerCorpo(w http.ResponseWriter, r *http.Request, destino any) bool {
	if err := json.NewDecoder(r.Body).Decode(destino); err != nil {
		erro(w, http.StatusUnprocessableEntity, "corpo inválido: "+err.Error())
		return false
	}
	return true
}

func idDaURL(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		erro(w, http.StatusBadRequest, "id inválido")
		return 0, false
	}
	return id, true
}

func inteiroDaQuery(r *http.Request, nome string, padrao int32) int32 {
	valor, err := strconv.ParseInt(r.URL.Query().Get(nome), 10, 32)
	if err != nil {
		return padrao
	}
	return int32(valor)
}

func textoOuNil(valor string) *string {
	if valor == "" {
		return nil
	}
	return &valor
}

// consultas é atalho para o acesso gerado.
func (s Servidor) consultas() *db.Queries { return s.Repo.Consultas() }
