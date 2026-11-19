// Comando rtchub: raiz de composição.
//
// É o único lugar que conhece todas as peças ao mesmo tempo. Amarra os
// adaptadores nas portas e sobe o servidor; nenhuma regra mora aqui.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Kaue-123/rtc-hub/internal/adapters/fiscalxml"
	"github.com/Kaue-123/rtc-hub/internal/adapters/httpapi"
	"github.com/Kaue-123/rtc-hub/internal/adapters/postgres"
	"github.com/Kaue-123/rtc-hub/internal/adapters/rfb"
	"github.com/Kaue-123/rtc-hub/internal/app"
	"github.com/Kaue-123/rtc-hub/internal/domain"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	if err := executar(log); err != nil {
		log.Error("encerrando por erro", "erro", err)
		os.Exit(1)
	}
}

func executar(log *slog.Logger) error {
	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	urlBanco := ambiente("RTC_DATABASE_URL", "")
	if urlBanco == "" {
		return errors.New("RTC_DATABASE_URL não definida")
	}

	pool, err := postgres.Conectar(ctx, urlBanco)
	if err != nil {
		return err
	}
	defer pool.Close()

	aplicadas, err := postgres.Migrar(ctx, pool)
	if err != nil {
		return err
	}
	if len(aplicadas) > 0 {
		log.Info("migrações aplicadas", "quantidade", len(aplicadas), "nomes", aplicadas)
	}

	repo := postgres.NovoRepositorio(pool)
	if criadas, err := postgres.SemearAliquotasPadrao(ctx, pool); err != nil {
		return err
	} else if criadas > 0 {
		log.Info("alíquotas padrão semeadas", "linhas", criadas)
	}
	if err := postgres.GarantirTenantDeDesenvolvimento(ctx, pool); err != nil {
		return err
	}

	motores := map[string]app.Calculadora{
		domain.MotorRegimeRegular: rfb.NovoRegimeRegular(
			ambiente("RTC_MOTOR_REGIME_REGULAR_URL", "http://localhost:18080")),
		domain.MotorSimples: rfb.NovoSimplesNacional(
			ambiente("RTC_MOTOR_SIMPLES_URL", "http://localhost:18082")),
	}

	servidor := httpapi.Servidor{
		Conferir: app.ConferirLote{
			Leitor:       fiscalxml.Leitor{},
			Calculadoras: motores,
			Aliquotas:    repo,
			Repo:         repo,
		},
		Repo:                repo,
		Motores:             motores,
		TamanhoMaxLoteBytes: inteiroDoAmbiente("RTC_TAMANHO_MAX_LOTE_BYTES", 64<<20),
	}

	endereco := ":" + ambiente("RTC_PORTA", "8000")
	servidorHTTP := &http.Server{
		Addr:              endereco,
		Handler:           servidor.Rotas(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	erros := make(chan error, 1)
	go func() {
		log.Info("servindo", "endereco", endereco)
		erros <- servidorHTTP.ListenAndServe()
	}()

	select {
	case err := <-erros:
		return err
	case <-ctx.Done():
		log.Info("sinal recebido, encerrando")
		desligar, cancelar := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancelar()
		return servidorHTTP.Shutdown(desligar)
	}
}

func ambiente(nome, padrao string) string {
	if valor := os.Getenv(nome); valor != "" {
		return valor
	}
	return padrao
}

func inteiroDoAmbiente(nome string, padrao int64) int64 {
	valor, err := strconv.ParseInt(os.Getenv(nome), 10, 64)
	if err != nil {
		return padrao
	}
	return valor
}
