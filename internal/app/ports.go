// Package app tem os casos de uso e as portas que eles exigem.
//
// As interfaces são declaradas aqui, por quem consome, e não no pacote que as
// implementa — é assim que Go inverte a dependência. Consequência prática:
// `app` e `domain` não importam nada de `adapters`, e um teste troca
// qualquer porta por um duplo sem tocar em infraestrutura.
package app

import (
	"context"
	"errors"
	"time"

	"github.com/Kaue-123/rtc-hub/internal/domain"
)

// ErrMotorIndisponivel: o motor recusou a requisição ou está fora do ar.
var ErrMotorIndisponivel = errors.New("motor indisponível")

// LeitorDeDocumento converte o arquivo recebido em documento de domínio.
type LeitorDeDocumento interface {
	Ler(conteudo []byte, arquivo string) (domain.DocumentoFiscal, error)
	// EhReconhecivel distingue "não é DF-e" de falha de leitura, para o lote
	// seguir com os outros arquivos.
	EhNaoReconhecido(err error) bool
}

// ResultadoMotor é o que uma calculadora devolve, já normalizado.
type ResultadoMotor struct {
	Carimbo        domain.CarimboVersao
	Itens          map[int]domain.ItemApurado
	DadosSimulados bool
	Requisicao     []byte
	Resposta       []byte
}

// Calculadora é um motor oficial da RFB visto de fora.
type Calculadora interface {
	Nome() string
	Versao(ctx context.Context) (domain.CarimboVersao, error)
	// Calcular recebe a alíquota já resolvida; informar ou não na requisição é
	// decisão do motor, porque a regra muda por ano e por regime.
	Calcular(ctx context.Context, doc domain.DocumentoFiscal, aliquota domain.AliquotaVigencia) (ResultadoMotor, error)
	// ExigeAliquotaInformada diz se este documento precisa de alíquota da nossa
	// tabela. Antes de 2027 o regime regular é proibido de informar; o Simples
	// exige sempre.
	ExigeAliquotaInformada(doc domain.DocumentoFiscal) bool
}

// RepoAliquotas devolve as linhas candidatas; qual vale é regra de domínio.
type RepoAliquotas interface {
	Aplicaveis(ctx context.Context, tenantID int64, competencia time.Time, uf string, municipio *int32) ([]domain.AliquotaVigencia, error)
}

// DocumentoConferido é o que vai para o banco ao fim de um arquivo.
type DocumentoConferido struct {
	Documento        domain.DocumentoFiscal
	Estado           string
	Motor            string
	Motivo           string
	DadosSimulados   bool
	Carimbo          domain.CarimboVersao
	AliquotaOrigemID *int64
	Requisicao       []byte
	Resposta         []byte
	Divergencias     []domain.Divergencia
}

// RepoConferencia persiste o lote. Implementação deve rodar em transação: lote
// pela metade no banco é pior que lote que falhou.
type RepoConferencia interface {
	CriarLote(ctx context.Context, tenantID int64, empresaID *int64, descricao string) (int64, error)
	SalvarDocumento(ctx context.Context, tenantID, loteID int64, doc DocumentoConferido) error
	ConcluirLote(ctx context.Context, tenantID, loteID int64) error
}
