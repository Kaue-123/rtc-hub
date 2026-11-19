// Package rfb fala com os motores oficiais da Receita Federal por HTTP.
//
// Adaptador de saída: satisfaz app.Calculadora sem que o núcleo conheça o
// formato de requisição de nenhum dos dois. Os motores são caixas-pretas
// versionadas — nada do código deles é copiado para cá.
package rfb

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Kaue-123/rtc-hub/internal/app"
	"github.com/Kaue-123/rtc-hub/internal/domain"
)

const tempoLimite = 30 * time.Second

type cliente struct {
	base string
	http *http.Client
}

func novoCliente(base string) cliente {
	return cliente{base: base, http: &http.Client{Timeout: tempoLimite}}
}

func (c cliente) obter(ctx context.Context, caminho string, destino any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+caminho, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", app.ErrMotorIndisponivel, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%w: %s devolveu %d", app.ErrMotorIndisponivel, caminho, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(destino)
}

func (c cliente) postar(ctx context.Context, caminho string, corpo any) ([]byte, []byte, http.Header, error) {
	enviado, err := json.Marshal(corpo)
	if err != nil {
		return nil, nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+caminho, bytes.NewReader(enviado))
	if err != nil {
		return nil, nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return enviado, nil, nil, fmt.Errorf("%w: %v", app.ErrMotorIndisponivel, err)
	}
	defer resp.Body.Close()

	recebido := new(bytes.Buffer)
	if _, err := recebido.ReadFrom(resp.Body); err != nil {
		return enviado, nil, nil, err
	}
	if resp.StatusCode >= 400 {
		corpo := recebido.String()
		if len(corpo) > 300 {
			corpo = corpo[:300]
		}
		return enviado, recebido.Bytes(), resp.Header,
			fmt.Errorf("%w: devolveu %d: %s", app.ErrMotorIndisponivel, resp.StatusCode, corpo)
	}
	return enviado, recebido.Bytes(), resp.Header, nil
}

type respostaVersao struct {
	VersaoApp    string `json:"versaoApp"`
	VersaoDb     string `json:"versaoDb"`
	DataVersaoDb string `json:"dataVersaoDb"`
	Ambiente     string `json:"ambiente"`
}

func (r respostaVersao) carimbo(motor string) domain.CarimboVersao {
	return domain.CarimboVersao{
		Motor: motor, VersaoApp: r.VersaoApp, VersaoDB: r.VersaoDb,
		DataVersaoDB: r.DataVersaoDb, Ambiente: r.Ambiente,
	}
}

// identificador da operação, exigido pelos dois motores. Só precisa ser único
// na requisição; não é chave de nada do nosso lado.
func identificador() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
