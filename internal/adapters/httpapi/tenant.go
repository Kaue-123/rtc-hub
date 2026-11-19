package httpapi

import (
	"context"
	"net/http"

	"github.com/Kaue-123/rtc-hub/internal/adapters/postgres/db"
)

type chaveContexto string

const chaveTenant chaveContexto = "tenant"

// resolverTenant põe o tenant no contexto.
//
// ⚠️ PROVISÓRIO. Lê o cabeçalho X-Tenant e confia nele — qualquer um informa o
// slug de qualquer outro e lê os lotes alheios. Isto existe para o fluxo rodar
// enquanto a autenticação não entra, e NÃO pode ir para produção. Quando entrar,
// o tenant passa a sair do token e só este arquivo muda.
func (s Servidor) resolverTenant(proximo http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slug := r.Header.Get("X-Tenant")
		if slug == "" {
			slug = "dev"
		}

		tenant, err := s.consultas().BuscarTenantPorSlug(r.Context(), slug)
		if err != nil {
			erro(w, http.StatusNotFound, "tenant "+slug+" não encontrado ou inativo")
			return
		}

		ctx := context.WithValue(r.Context(), chaveTenant, tenant)
		proximo.ServeHTTP(w, r.WithContext(ctx))
	})
}

func tenantDe(ctx context.Context) db.Tenant {
	tenant, _ := ctx.Value(chaveTenant).(db.Tenant)
	return tenant
}
