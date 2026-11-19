-- Devolve as linhas APLICÁVEIS à data e ao recorte geográfico do documento.
-- A escolha entre elas (qual é a mais específica) é regra de domínio e fica no
-- núcleo, não aqui: o adaptador filtra, o domínio decide.
--
-- O filtro geográfico é de aplicabilidade, não de preferência — linha de SP não
-- pode valer para nota do RS nem na falta de outra.
--
-- name: AliquotasAplicaveis :many
SELECT * FROM aliquota_vigencia
WHERE vigencia_inicio <= sqlc.arg('competencia')
  AND (vigencia_fim IS NULL OR vigencia_fim >= sqlc.arg('competencia'))
  AND (tenant_id = sqlc.arg('tenant_id') OR tenant_id IS NULL)
  AND (uf IS NULL OR uf = sqlc.narg('uf'))
  AND (municipio IS NULL OR municipio = sqlc.narg('municipio'));

-- name: CriarAliquota :one
INSERT INTO aliquota_vigencia (
    tenant_id, vigencia_inicio, vigencia_fim, uf, municipio,
    cbs, ibs_estadual, ibs_municipal, fonte, observacao
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: BuscarAliquota :one
SELECT * FROM aliquota_vigencia
WHERE id = $1 AND (tenant_id = sqlc.narg('tenant_id') OR tenant_id IS NULL);

-- name: ListarAliquotas :many
SELECT * FROM aliquota_vigencia
WHERE tenant_id = sqlc.narg('tenant_id') OR tenant_id IS NULL
ORDER BY vigencia_inicio DESC, tenant_id NULLS LAST;

-- name: AtualizarAliquota :one
-- Só linha do próprio tenant pode ser alterada; a padrão é imutável pela API.
UPDATE aliquota_vigencia
SET vigencia_inicio = $3, vigencia_fim = $4, uf = $5, municipio = $6,
    cbs = $7, ibs_estadual = $8, ibs_municipal = $9, observacao = $10
WHERE id = $1 AND tenant_id = $2
RETURNING *;

-- name: ExcluirAliquota :exec
DELETE FROM aliquota_vigencia WHERE id = $1 AND tenant_id = $2;

-- name: ContarAliquotasPadrao :one
SELECT count(*) FROM aliquota_vigencia WHERE tenant_id IS NULL;
