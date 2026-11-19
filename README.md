# RTC Hub

Conferência de documentos fiscais contra as calculadoras oficiais da Reforma
Tributária do Consumo. Você sobe os XMLs que emitiu; o sistema recalcula cada
item pelo motor da Receita Federal e devolve onde o declarado não bate com o
apurado — com a memória de cálculo e o enquadramento legal que explicam a
acusação.

Os três motores da RFB (Portaria Cotec nº 342, de 12/11/2026, todos MIT) entram
**como vieram**, em containers pinados por commit. Nenhuma linha do código deles
é alterada ou fundida aqui.

> **Este projeto não é um produto da Receita Federal e não tem endosso dela nem
> do SERPRO.** Os motores oficiais são software livre sob licença MIT, que
> autoriza o uso aqui feito mas não autoriza sugerir aprovação oficial. O
> `make upstream` os clona do GitHub da RFB nos commits pinados; nenhum código
> deles é redistribuído neste repositório. Conferência feita aqui não é parecer
> fiscal, e as alíquotas de 2027 que o produto carrega são estimativa própria —
> leia "O que o produto carrega por decisão".

## Arquitetura

Ports and adapters. A regra que sustenta tudo: **a seta aponta para dentro**.

```
cmd/rtchub/            raiz de composição: amarra as peças, não tem regra
internal/
├── domain/            entidades e regras puras. Zero import de infraestrutura
├── app/               casos de uso + as PORTAS (interfaces)
└── adapters/
    ├── httpapi/       entrada: chi
    ├── fiscalxml/     entrada: XML de DF-e → domínio
    ├── rfb/           saída: HTTP para os motores da Receita
    └── postgres/      saída: sqlc + migrações versionadas
        ├── db/        gerado pelo sqlc, não editar à mão
        ├── migrations/
        └── queries/
```

As interfaces são declaradas em `app`, por quem consome, não no pacote que as
implementa — é assim que Go inverte a dependência, e é o que permite testar o
caso de uso inteiro sem banco, sem HTTP e sem XML.

`internal/architecture_test.go` trava a regra: se `domain` ou `app` importar
`adapters`, o teste quebra.

As pastas e os pacotes são em inglês; os identificadores seguem em português,
com os termos fiscais (`CST`, `cClassTrib`, `NCM`, `vIBSUF`) exatamente como a
NT 2026.005 e o schema do XML os nomeiam.

## Subir

```bash
make upstream          # clona os três repos da RFB nos commits pinados
make teste             # 34 testes
echo "POSTGRES_PASSWORD=$(openssl rand -hex 16)" > .env
make compose-up
```

Portas: API 18090, regime regular 18080, DANFSe 18081, Simples 18082, Postgres
15432. Não são as padrão porque a 8080 desta máquina já está ocupada.

## Usar

```bash
curl -X POST "http://localhost:18090/lotes?descricao=setembro" \
  -H "X-Tenant: dev" -F "arquivos=@nota1.xml" -F "arquivos=@nota2.xml"
```

```json
{"lote":1,"documentos":4,"conformes":2,"divergencias":1,
 "por_estado":{"CONFERIDO":3,"NAO_RECONHECIDO":1},
 "por_categoria":{"valor_tributo":1}}
```

`GET /lotes/{id}` traz o detalhe por documento, com a divergência, a memória de
cálculo e a versão do motor que produziu o número. Há CRUD em `/empresas` e
`/aliquotas`.

## O que o produto carrega por decisão

**O motor não tem alíquota de 2027.** A documentação da RFB é literal: "A partir
de 2027, `aliquotasNominais` é obrigatório; antes disso, é proibido". Daí
`domain.ResolverAliquota`: resolve do mais específico para o mais geral
(município → UF → geral, tenant antes do padrão), recusa em vez de chutar quando
nenhuma vigência cobre a data, e **não aplica regra de outra UF** só por ela ser
mais específica. As linhas padrão de 2027 são as dos exemplos da RFB, **não
alíquota fixada em lei** — cada linha carrega `fonte` e `observacao`, e o
documento guarda de qual linha o número saiu.

**HTTP 200 não quer dizer calculado.** Os endpoints de observabilidade devolvem
200 com itens em erro, e os totais somam apenas os itens calculados — item
recusado sumiria do relatório. `domain.Conferir` trata cada `estadoItem` e item
omitido vira divergência, nunca silêncio.

**Sem carimbo de versão não há como explicar o número.** O código aberto está
atrás do ambiente oficial: medido em 18/11/2026, repositório em app 1.6.1 / banco
V0059 (30/09) contra 1.7.1 / V0073 (09/11) do piloto. Dois lotes conferidos antes
e depois de atualizar o motor podem divergir legitimamente, e é por isso que
`motor_versao_app` e `motor_versao_db` vão junto de cada documento.

## Armadilhas que custaram tempo

- **A NFS-e tem dois grupos `IBSCBS`**: `infNFSe/IBSCBS` traz o apurado, e
  `infNFSe/DPS/infDPS/IBSCBS/valores/trib/gIBSCBS` traz o declarado (CST e
  cClassTrib). Procurar a classificação no primeiro devolve nada, e o motor
  recusa o item com `VAL-001`.
- **Campo vazio não é campo ausente.** `"uf": ""` volta `VAL-004`; a ausência
  passa. O zero value de `string` em Go serializa como `""`, então é fácil
  reintroduzir — tem teste.
- **`gTribRegular` é obrigatório para algumas cClassTrib.** Não repassá-lo como
  `tributacaoRegular` faz o motor recusar documento correto.
- **A mesma NF-e vem embrulhada de três formas** (`enviNFe`, `nfeProc`, `NFe`).
  O parser procura `infNFe`, nunca a raiz.
- **`decimal.String()` corta zeros à direita**, e `10.00` viraria `10` no
  relatório. Ver `domain.comEscala`.
- **O repositório do Simples Nacional não traz as migrations** (issue #5). Os
  dois apps Java leem o mesmo `calculadora-pro.db`, e o banco sai das migrations
  do regime regular — é o que o `simples-nacional.Dockerfile` faz.
- **`./mvnw` dentro da imagem `maven:*`** quebra por `MAVEN_CONFIG`.

## Pendências

- **Autenticação não existe.** O tenant vem no cabeçalho `X-Tenant`, sem prova
  nenhuma — `internal/adapters/httpapi/tenant.go` é o único arquivo que muda
  quando entrar. Não pode subir assim.
- CRUD de tenants e usuários: o esquema existe, as consultas estão geradas, os
  handlers ainda não. Usuário depende da decisão de hash de senha.
- Processamento é síncrono na requisição; lote grande precisa de fila.
- Só NF-e, NFC-e e NFS-e são lidos. O motor aceita também CT-e, CT-e
  simplificado, BP-e, BP-e TM e NF3e, com exemplos em `upstream/`.
- `cAtivSN` não é extraído do XML, então documento de optante do Simples volta
  `NAO_CALCULADO` por item.
- O gerador de DANFSe está no compose e ainda não é usado pelo fluxo.
