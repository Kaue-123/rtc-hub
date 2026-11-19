// Package parsers converte XML de DF-e em domain.DocumentoFiscal.
//
// A mesma NF-e vem sob enviNFe, nfeProc ou NFe, então o parser procura
// infNFe/infNFSe em vez de exigir uma raiz. O namespace muda por layout, e tudo
// é comparado por nome local. A NFS-e não tem itens: vira um item sintético.
package fiscalxml

import (
	"encoding/xml"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/Kaue-123/rtc-hub/internal/domain"
)

// ErrNaoReconhecido: é XML, mas não é um DF-e que saibamos ler.
var ErrNaoReconhecido = errors.New("xml não reconhecido")

// no é a árvore genérica do documento. Decodificar em struct tipada exigiria um
// tipo por layout e por embrulho; a árvore deixa a busca por nome local.
type no struct {
	XMLName xml.Name
	Texto   string     `xml:",chardata"`
	Attrs   []xml.Attr `xml:",any,attr"`
	Filhos  []no       `xml:",any"`
}

func (n *no) nome() string { return n.XMLName.Local }

func (n *no) attr(nome string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attrs {
		if a.Name.Local == nome {
			return a.Value
		}
	}
	return ""
}

func filho(pai *no, nome string) *no {
	if pai == nil {
		return nil
	}
	for i := range pai.Filhos {
		if pai.Filhos[i].nome() == nome {
			return &pai.Filhos[i]
		}
	}
	return nil
}

func caminho(raiz *no, nomes ...string) *no {
	atual := raiz
	for _, nome := range nomes {
		atual = filho(atual, nome)
		if atual == nil {
			return nil
		}
	}
	return atual
}

func primeiro(raiz *no, nome string) *no {
	if raiz == nil {
		return nil
	}
	if raiz.nome() == nome {
		return raiz
	}
	for i := range raiz.Filhos {
		if achado := primeiro(&raiz.Filhos[i], nome); achado != nil {
			return achado
		}
	}
	return nil
}

func texto(n *no) string {
	if n == nil {
		return ""
	}
	return strings.TrimSpace(n.Texto)
}

func textoDe(raiz *no, nomes ...string) string { return texto(caminho(raiz, nomes...)) }

func dec(valor string) *decimal.Decimal {
	if valor == "" {
		return nil
	}
	d, err := decimal.NewFromString(valor)
	if err != nil {
		return nil
	}
	return &d
}

func decDe(raiz *no, nomes ...string) *decimal.Decimal { return dec(textoDe(raiz, nomes...)) }

func data(valor string) *time.Time {
	if valor == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, valor)
	if err != nil {
		return nil
	}
	return &t
}

func inteiro(valor string) *int {
	n, err := strconv.Atoi(valor)
	if err != nil {
		return nil
	}
	return &n
}

// Código IBGE de município: int32 por ser largura fixa de 7 dígitos.
func inteiro32(valor string) *int32 {
	n, err := strconv.ParseInt(valor, 10, 32)
	if err != nil {
		return nil
	}
	n32 := int32(n)
	return &n32
}

// Parse detecta o layout e devolve o documento.
func Parse(conteudo []byte, arquivo string) (domain.DocumentoFiscal, error) {
	var raiz no
	if err := xml.Unmarshal(conteudo, &raiz); err != nil {
		return domain.DocumentoFiscal{}, fmt.Errorf("%w: xml malformado: %v", ErrNaoReconhecido, err)
	}

	if primeiro(&raiz, "infNFe") != nil {
		return parseNFe(&raiz, arquivo), nil
	}
	if primeiro(&raiz, "infNFSe") != nil {
		return parseNFSe(&raiz, arquivo), nil
	}
	return domain.DocumentoFiscal{}, fmt.Errorf(
		"%w: nem infNFe nem infNFSe encontrados (raiz <%s>)", ErrNaoReconhecido, raiz.nome())
}

// --------------------------------------------------------------------------
// NF-e e NFC-e
// --------------------------------------------------------------------------

func parseNFe(raiz *no, arquivo string) domain.DocumentoFiscal {
	inf := primeiro(raiz, "infNFe")
	ide := filho(inf, "ide")
	emit := filho(inf, "emit")
	ender := filho(emit, "enderEmit")

	modelo := textoDe(ide, "mod")
	tipo := "nfe"
	if modelo == "65" {
		tipo = "nfce"
	}

	uf := textoDe(ender, "UF")
	if uf == "" {
		uf = domain.UFPorCodigo(textoDe(ide, "cUF"))
	}
	municipio := textoDe(ide, "cMunFG")
	if municipio == "" {
		municipio = textoDe(ender, "cMun")
	}

	documento := domain.DocumentoFiscal{
		Arquivo:      arquivo,
		Tipo:         tipo,
		Modelo:       modelo,
		Chave:        strings.TrimPrefix(inf.attr("Id"), "NFe"),
		Numero:       textoDe(ide, "nNF"),
		DhEmissao:    data(textoDe(ide, "dhEmi")),
		UF:           uf,
		Municipio:    inteiro32(municipio),
		CRT:          textoDe(emit, "CRT"),
		EmitenteCNPJ: textoDe(emit, "CNPJ"),
	}

	ordem := 0
	for i := range inf.Filhos {
		if inf.Filhos[i].nome() != "det" {
			continue
		}
		ordem++
		documento.Itens = append(documento.Itens, itemNFe(&inf.Filhos[i], ordem))
	}
	return documento
}

func itemNFe(det *no, ordem int) domain.ItemDeclarado {
	prod := filho(det, "prod")
	imposto := filho(det, "imposto")
	ibscbs := filho(imposto, "IBSCBS")
	grupo := filho(ibscbs, "gIBSCBS")

	numero := ordem
	if n := inteiro(det.attr("nItem")); n != nil {
		numero = *n
	}

	regular := filho(grupo, "gTribRegular")
	return domain.ItemDeclarado{
		Numero:            numero,
		NCM:               textoDe(prod, "NCM"),
		Descricao:         textoDe(prod, "xProd"),
		Quantidade:        decDe(prod, "qCom"),
		Unidade:           textoDe(prod, "uCom"),
		CST:               textoDe(ibscbs, "CST"),
		CClassTrib:        textoDe(ibscbs, "cClassTrib"),
		BaseCalculo:       decDe(grupo, "vBC"),
		ValorIS:           decDe(imposto, "IS", "vIS"),
		CSTRegular:        textoDe(regular, "CSTReg"),
		CClassTribRegular: textoDe(regular, "cClassTribReg"),
		IBSUF:             tributo(filho(grupo, "gIBSUF"), "pIBSUF", "vIBSUF"),
		IBSMun:            tributo(filho(grupo, "gIBSMun"), "pIBSMun", "vIBSMun"),
		CBS:               tributo(filho(grupo, "gCBS"), "pCBS", "vCBS"),
	}
}

func tributo(grupo *no, tagAliquota, tagValor string) domain.TributoDeclarado {
	if grupo == nil {
		return domain.TributoDeclarado{}
	}
	red := filho(grupo, "gRed")
	return domain.TributoDeclarado{
		Aliquota:        decDe(grupo, tagAliquota),
		AliquotaEfetiva: decDe(red, "pAliqEfet"),
		Reducao:         decDe(red, "pRedAliq"),
		Valor:           decDe(grupo, tagValor),
	}
}

// --------------------------------------------------------------------------
// NFS-e
// --------------------------------------------------------------------------

// opSimpNac na DPS: 1 = não optante, 2 = MEI, 3 = ME/EPP.
var crtPorOpSimpNac = map[string]string{"1": "3", "2": "4", "3": "1"}

func parseNFSe(raiz *no, arquivo string) domain.DocumentoFiscal {
	inf := primeiro(raiz, "infNFSe")
	emit := filho(inf, "emit")
	ibscbs := filho(inf, "IBSCBS")
	infDPS := primeiro(raiz, "infDPS")
	serv := filho(infDPS, "serv")
	prest := filho(infDPS, "prest")

	municipio := textoDe(ibscbs, "cLocalidadeIncid")
	if municipio == "" {
		municipio = textoDe(inf, "cLocIncid")
	}

	documento := domain.DocumentoFiscal{
		Arquivo:      arquivo,
		Tipo:         "nfse",
		Modelo:       "nfse",
		Chave:        textoDe(inf, "nDFSe"),
		Numero:       textoDe(inf, "nNFSe"),
		DhEmissao:    data(textoDe(inf, "dhProc")),
		Municipio:    inteiro32(municipio),
		CRT:          crtPorOpSimpNac[textoDe(prest, "regTrib", "opSimpNac")],
		EmitenteCNPJ: textoDe(emit, "CNPJ"),
	}

	valores := filho(ibscbs, "valores")
	tot := filho(ibscbs, "totCIBS")

	// A NFS-e tem DOIS grupos IBSCBS: o de infNFSe traz o apurado, o de dentro
	// da DPS traz o declarado. Classificação só existe no segundo.
	declaracao := caminho(infDPS, "IBSCBS", "valores", "trib", "gIBSCBS")

	descricao := textoDe(serv, "cServ", "xDescServ")
	if descricao == "" {
		descricao = textoDe(inf, "xTribNac")
	}

	item := domain.ItemDeclarado{
		Numero:            1,
		Descricao:         descricao,
		NBS:               textoDe(serv, "cServ", "cNBS"),
		CST:               textoDe(declaracao, "CST"),
		CClassTrib:        textoDe(declaracao, "cClassTrib"),
		CSTRegular:        textoDe(declaracao, "gTribRegular", "CSTReg"),
		CClassTribRegular: textoDe(declaracao, "gTribRegular", "cClassTribReg"),
		BaseCalculo:       decDe(valores, "vBC"),
		// Alíquota em valores/{uf,mun,fed}, valor apurado em totCIBS.
		IBSUF: domain.TributoDeclarado{
			Aliquota:        decDe(valores, "uf", "pIBSUF"),
			AliquotaEfetiva: decDe(valores, "uf", "pAliqEfetUF"),
			Reducao:         decDe(valores, "uf", "pRedAliqUF"),
			Valor:           decDe(tot, "gIBS", "gIBSUFTot", "vIBSUF"),
		},
		IBSMun: domain.TributoDeclarado{
			Aliquota:        decDe(valores, "mun", "pIBSMun"),
			AliquotaEfetiva: decDe(valores, "mun", "pAliqEfetMun"),
			Reducao:         decDe(valores, "mun", "pRedAliqMun"),
			Valor:           decDe(tot, "gIBS", "gIBSMunTot", "vIBSMun"),
		},
		CBS: domain.TributoDeclarado{
			Aliquota:        decDe(valores, "fed", "pCBS"),
			AliquotaEfetiva: decDe(valores, "fed", "pAliqEfetCBS"),
			Reducao:         decDe(valores, "fed", "pRedAliqCBS"),
			Valor:           decDe(tot, "gCBS", "vCBS"),
		},
	}

	documento.Itens = append(documento.Itens, item)
	return documento
}

// Leitor satisfaz app.LeitorDeDocumento.
type Leitor struct{}

func (Leitor) Ler(conteudo []byte, arquivo string) (domain.DocumentoFiscal, error) {
	return Parse(conteudo, arquivo)
}

func (Leitor) EhNaoReconhecido(err error) bool { return errors.Is(err, ErrNaoReconhecido) }
