package postgres

import (
	"errors"

	"github.com/shopspring/decimal"
)

// ErrNaoEncontrado traduz pgx.ErrNoRows para a camada de cima, que não deve
// conhecer o driver.
var ErrNaoEncontrado = errors.New("não encontrado")

// texto devolve nil para string vazia: no banco, ausência é NULL, não "".
func texto(valor string) *string {
	if valor == "" {
		return nil
	}
	return &valor
}

func nullDecimal(valor *decimal.Decimal) decimal.NullDecimal {
	if valor == nil {
		return decimal.NullDecimal{}
	}
	return decimal.NullDecimal{Decimal: *valor, Valid: true}
}
