package httpapi

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func requisicaoComArquivo(t *testing.T, nome string, bytesDeConteudo int) *http.Request {
	t.Helper()
	corpo := new(bytes.Buffer)
	escritor := multipart.NewWriter(corpo)
	parte, err := escritor.CreateFormFile("arquivos", nome)
	if err != nil {
		t.Fatalf("criar parte: %v", err)
	}
	if _, err := parte.Write(bytes.Repeat([]byte("x"), bytesDeConteudo)); err != nil {
		t.Fatalf("escrever parte: %v", err)
	}
	if err := escritor.Close(); err != nil {
		t.Fatalf("fechar multipart: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/lotes", corpo)
	req.Header.Set("Content-Type", escritor.FormDataContentType())
	return req
}

// ParseMultipartForm não limita o tamanho da requisição: o argumento dela é
// quanto fica em MEMÓRIA, e o resto vai para arquivo temporário em disco. Sem
// MaxBytesReader, lote de qualquer tamanho passava e o 413 nunca disparava por
// excesso — só por multipart malformado, que é outra coisa.
func TestLoteAcimaDoLimiteEhRecusadoCom413(t *testing.T) {
	w := httptest.NewRecorder()
	r := requisicaoComArquivo(t, "nota.xml", 4096)

	if _, ok := lerArquivosDoLote(w, r, 1024); ok {
		t.Fatal("lote de 4 KiB passou por um limite de 1 KiB")
	}
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, esperava 413 — corpo: %s", w.Code, w.Body)
	}
}

func TestLoteDentroDoLimitePassaInteiro(t *testing.T) {
	w := httptest.NewRecorder()
	r := requisicaoComArquivo(t, "nota.xml", 512)

	arquivos, ok := lerArquivosDoLote(w, r, 64<<10)
	if !ok {
		t.Fatalf("recusou lote dentro do limite: %d %s", w.Code, w.Body)
	}
	if len(arquivos) != 1 {
		t.Fatalf("arquivos = %d, esperava 1", len(arquivos))
	}
	if arquivos[0].Nome != "nota.xml" || len(arquivos[0].Conteudo) != 512 {
		t.Errorf("arquivo = %s com %d bytes", arquivos[0].Nome, len(arquivos[0].Conteudo))
	}
}

// Zero no limite recusaria toda requisição; o padrão protege o servidor montado
// sem o campo preenchido.
func TestLimiteZeroCaiNoPadraoEmVezDeRecusarTudo(t *testing.T) {
	w := httptest.NewRecorder()
	r := requisicaoComArquivo(t, "nota.xml", 512)

	if _, ok := lerArquivosDoLote(w, r, 0); !ok {
		t.Errorf("limite zero recusou lote pequeno: %d %s", w.Code, w.Body)
	}
}

func TestLoteSemNenhumArquivoEh422(t *testing.T) {
	corpo := new(bytes.Buffer)
	escritor := multipart.NewWriter(corpo)
	if err := escritor.WriteField("descricao", "setembro"); err != nil {
		t.Fatalf("campo: %v", err)
	}
	if err := escritor.Close(); err != nil {
		t.Fatalf("fechar: %v", err)
	}
	r := httptest.NewRequest(http.MethodPost, "/lotes", corpo)
	r.Header.Set("Content-Type", escritor.FormDataContentType())

	w := httptest.NewRecorder()
	if _, ok := lerArquivosDoLote(w, r, 64<<10); ok {
		t.Fatal("multipart sem arquivo deveria ser recusado")
	}
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, esperava 422", w.Code)
	}
}
