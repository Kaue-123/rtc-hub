package internal_test

import (
	"go/build"
	"strings"
	"testing"
)

// A regra que sustenta ports and adapters: a seta aponta para dentro. O núcleo
// não pode conhecer banco, HTTP nem XML — se conhecer, trocar de adaptador
// deixa de ser possível e o teste de domínio passa a exigir infraestrutura.
func TestNucleoNaoImportaAdaptadores(t *testing.T) {
	const modulo = "github.com/Kaue-123/rtc-hub"

	for _, pacote := range []string{"/internal/domain", "/internal/app"} {
		info, err := build.Import(modulo+pacote, "", 0)
		if err != nil {
			t.Fatalf("%s: %v", pacote, err)
		}
		for _, importado := range info.Imports {
			if strings.Contains(importado, "/internal/adapters") {
				t.Errorf("%s importa %s — a dependência está invertida", pacote, importado)
			}
		}
	}
}
