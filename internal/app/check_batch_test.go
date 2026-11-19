package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/Kaue-123/rtc-hub/internal/app"
	"github.com/Kaue-123/rtc-hub/internal/domain"
)

// Duplos das portas. Nenhuma infraestrutura: nem banco, nem HTTP, nem XML.

type leitorFalso struct {
	doc domain.DocumentoFiscal
	err error
}

func (l leitorFalso) Ler([]byte, string) (domain.DocumentoFiscal, error) { return l.doc, l.err }
func (l leitorFalso) EhNaoReconhecido(error) bool                        { return true }

type calculadoraFalsa struct {
	itens            map[int]domain.ItemApurado
	err              error
	exigeAliquota    bool
	aliquotaRecebida *domain.AliquotaVigencia
	chamadas         int
}

func (c *calculadoraFalsa) Nome() string { return "falsa" }
func (c *calculadoraFalsa) Versao(context.Context) (domain.CarimboVersao, error) {
	return domain.CarimboVersao{VersaoApp: "1.6.1", VersaoDB: "V0059"}, nil
}
func (c *calculadoraFalsa) ExigeAliquotaInformada(domain.DocumentoFiscal) bool {
	return c.exigeAliquota
}
func (c *calculadoraFalsa) Calcular(
	_ context.Context, _ domain.DocumentoFiscal, aliquota domain.AliquotaVigencia,
) (app.ResultadoMotor, error) {
	c.chamadas++
	c.aliquotaRecebida = &aliquota
	if c.err != nil {
		return app.ResultadoMotor{}, c.err
	}
	return app.ResultadoMotor{
		Carimbo: domain.CarimboVersao{VersaoApp: "1.6.1", VersaoDB: "V0059"},
		Itens:   c.itens,
	}, nil
}

type repoAliquotasFalso struct {
	linhas []domain.AliquotaVigencia
	err    error
}

func (r repoAliquotasFalso) Aplicaveis(
	context.Context, int64, time.Time, string, *int32,
) ([]domain.AliquotaVigencia, error) {
	return r.linhas, r.err
}

type repoFalso struct {
	salvos    []app.DocumentoConferido
	concluido bool
}

func (r *repoFalso) CriarLote(context.Context, int64, *int64, string) (int64, error) { return 7, nil }
func (r *repoFalso) SalvarDocumento(_ context.Context, _, _ int64, doc app.DocumentoConferido) error {
	r.salvos = append(r.salvos, doc)
	return nil
}
func (r *repoFalso) ConcluirLote(context.Context, int64, int64) error {
	r.concluido = true
	return nil
}

func documento(crt string) domain.DocumentoFiscal {
	emissao := time.Date(2027, time.March, 10, 0, 0, 0, 0, time.UTC)
	valor := decimal.RequireFromString("10.00")
	return domain.DocumentoFiscal{
		Arquivo: "nota.xml", Tipo: "nfe", CRT: crt, UF: "RS", DhEmissao: &emissao,
		Itens: []domain.ItemDeclarado{{
			Numero: 1, BaseCalculo: &valor,
			CBS: domain.TributoDeclarado{Valor: &valor},
		}},
	}
}

func aliquotaPadrao() domain.AliquotaVigencia {
	return domain.AliquotaVigencia{
		ID:             42,
		VigenciaInicio: time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC),
		CBS:            decimal.RequireFromString("8.8"),
		Fonte:          domain.FonteAliquotaPadrao,
	}
}

func TestLoteConferidoCarimbaVersaoEGuardaDivergencia(t *testing.T) {
	valorApurado := decimal.RequireFromString("9.00")
	motor := &calculadoraFalsa{itens: map[int]domain.ItemApurado{
		1: {Estado: "CALCULADO", CBS: &valorApurado},
	}}
	repo := &repoFalso{}

	caso := app.ConferirLote{
		Leitor:       leitorFalso{doc: documento("3")},
		Calculadoras: map[string]app.Calculadora{domain.MotorRegimeRegular: motor},
		Aliquotas:    repoAliquotasFalso{linhas: []domain.AliquotaVigencia{aliquotaPadrao()}},
		Repo:         repo,
	}

	loteID, err := caso.Executar(context.Background(), app.PedidoDeLote{
		TenantID: 1, Arquivos: []app.Arquivo{{Nome: "nota.xml", Conteudo: []byte("<x/>")}},
	})
	if err != nil {
		t.Fatalf("executar: %v", err)
	}
	if loteID != 7 || !repo.concluido {
		t.Errorf("lote = %d, concluido = %v", loteID, repo.concluido)
	}

	salvo := repo.salvos[0]
	if salvo.Estado != app.EstadoConferido {
		t.Fatalf("estado = %s (%s)", salvo.Estado, salvo.Motivo)
	}
	if salvo.Carimbo.VersaoDB != "V0059" {
		t.Errorf("carimbo = %+v", salvo.Carimbo)
	}
	if len(salvo.Divergencias) != 1 || salvo.Divergencias[0].Campo != "v_cbs" {
		t.Errorf("divergências = %+v", salvo.Divergencias)
	}
}

// MEI nem chega a custar uma chamada ao motor.
func TestMEINaoChegaAChamarOMotor(t *testing.T) {
	motor := &calculadoraFalsa{}
	repo := &repoFalso{}
	caso := app.ConferirLote{
		Leitor:       leitorFalso{doc: documento("4")},
		Calculadoras: map[string]app.Calculadora{domain.MotorRegimeRegular: motor},
		Aliquotas:    repoAliquotasFalso{},
		Repo:         repo,
	}

	if _, err := caso.Executar(context.Background(), app.PedidoDeLote{
		TenantID: 1, Arquivos: []app.Arquivo{{Nome: "x.xml"}},
	}); err != nil {
		t.Fatalf("executar: %v", err)
	}

	if motor.chamadas != 0 {
		t.Errorf("motor foi chamado %d vezes", motor.chamadas)
	}
	if repo.salvos[0].Estado != app.EstadoForaDeEscopo {
		t.Errorf("estado = %s", repo.salvos[0].Estado)
	}
	if repo.salvos[0].Motivo == "" {
		t.Error("fora de escopo tem de dizer por quê")
	}
}

func TestArquivoIlegivelNaoDerrubaOLote(t *testing.T) {
	repo := &repoFalso{}
	caso := app.ConferirLote{
		Leitor:       leitorFalso{err: errors.New("xml malformado")},
		Calculadoras: map[string]app.Calculadora{},
		Aliquotas:    repoAliquotasFalso{},
		Repo:         repo,
	}

	if _, err := caso.Executar(context.Background(), app.PedidoDeLote{
		TenantID: 1, Arquivos: []app.Arquivo{{Nome: "lixo.xml"}, {Nome: "outro.xml"}},
	}); err != nil {
		t.Fatalf("executar: %v", err)
	}
	if len(repo.salvos) != 2 {
		t.Fatalf("salvos = %d, o lote deveria seguir", len(repo.salvos))
	}
	for _, s := range repo.salvos {
		if s.Estado != app.EstadoNaoReconhecido {
			t.Errorf("estado = %s", s.Estado)
		}
	}
}

// Em 2027 a alíquota sai da nossa tabela, e fica registrado de qual linha.
func TestAliquotaResolvidaChegaAoMotorEFicaRegistrada(t *testing.T) {
	motor := &calculadoraFalsa{exigeAliquota: true, itens: map[int]domain.ItemApurado{}}
	repo := &repoFalso{}
	caso := app.ConferirLote{
		Leitor:       leitorFalso{doc: documento("3")},
		Calculadoras: map[string]app.Calculadora{domain.MotorRegimeRegular: motor},
		Aliquotas:    repoAliquotasFalso{linhas: []domain.AliquotaVigencia{aliquotaPadrao()}},
		Repo:         repo,
	}

	if _, err := caso.Executar(context.Background(), app.PedidoDeLote{
		TenantID: 1, Arquivos: []app.Arquivo{{Nome: "x.xml"}},
	}); err != nil {
		t.Fatalf("executar: %v", err)
	}

	if motor.aliquotaRecebida == nil || !motor.aliquotaRecebida.CBS.Equal(decimal.RequireFromString("8.8")) {
		t.Fatalf("alíquota recebida = %+v", motor.aliquotaRecebida)
	}
	if salvo := repo.salvos[0]; salvo.AliquotaOrigemID == nil || *salvo.AliquotaOrigemID != 42 {
		t.Errorf("origem da alíquota = %v", salvo.AliquotaOrigemID)
	}
}

func TestSemAliquotaVigenteORegistroDizPorQue(t *testing.T) {
	motor := &calculadoraFalsa{exigeAliquota: true}
	repo := &repoFalso{}
	caso := app.ConferirLote{
		Leitor:       leitorFalso{doc: documento("3")},
		Calculadoras: map[string]app.Calculadora{domain.MotorRegimeRegular: motor},
		Aliquotas:    repoAliquotasFalso{}, // nenhuma linha
		Repo:         repo,
	}

	if _, err := caso.Executar(context.Background(), app.PedidoDeLote{
		TenantID: 1, Arquivos: []app.Arquivo{{Nome: "x.xml"}},
	}); err != nil {
		t.Fatalf("executar: %v", err)
	}

	salvo := repo.salvos[0]
	if salvo.Estado != app.EstadoSemAliquota {
		t.Fatalf("estado = %s", salvo.Estado)
	}
	if motor.chamadas != 0 {
		t.Error("sem alíquota não se chama o motor")
	}
	if salvo.Motivo == "" {
		t.Error("motivo vazio")
	}
}

func TestMotorForaDoArViraEstadoNaoErroDoLote(t *testing.T) {
	motor := &calculadoraFalsa{err: app.ErrMotorIndisponivel}
	repo := &repoFalso{}
	caso := app.ConferirLote{
		Leitor:       leitorFalso{doc: documento("3")},
		Calculadoras: map[string]app.Calculadora{domain.MotorRegimeRegular: motor},
		Aliquotas:    repoAliquotasFalso{},
		Repo:         repo,
	}

	if _, err := caso.Executar(context.Background(), app.PedidoDeLote{
		TenantID: 1, Arquivos: []app.Arquivo{{Nome: "x.xml"}},
	}); err != nil {
		t.Fatalf("o lote não deveria falhar: %v", err)
	}
	if repo.salvos[0].Estado != app.EstadoFalhaMotor {
		t.Errorf("estado = %s", repo.salvos[0].Estado)
	}
}

func TestLoteVazioEhRecusado(t *testing.T) {
	caso := app.ConferirLote{Repo: &repoFalso{}}
	if _, err := caso.Executar(context.Background(), app.PedidoDeLote{TenantID: 1}); err == nil {
		t.Error("lote sem arquivo deveria ser recusado")
	}
}
