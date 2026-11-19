-- name: CriarLote :one
INSERT INTO lote (tenant_id, empresa_id, descricao) VALUES ($1, $2, $3) RETURNING *;

-- name: ConcluirLote :one
UPDATE lote SET estado = 'concluido', concluido_em = now()
WHERE id = $1 AND tenant_id = $2
RETURNING *;

-- name: BuscarLote :one
SELECT * FROM lote WHERE id = $1 AND tenant_id = $2;

-- name: ListarLotes :many
SELECT * FROM lote
WHERE tenant_id = $1
  AND (sqlc.narg('empresa_id')::BIGINT IS NULL OR empresa_id = sqlc.narg('empresa_id')::BIGINT)
  AND (sqlc.narg('desde')::TIMESTAMPTZ IS NULL OR criado_em >= sqlc.narg('desde')::TIMESTAMPTZ)
  AND (sqlc.narg('ate')::TIMESTAMPTZ IS NULL OR criado_em <= sqlc.narg('ate')::TIMESTAMPTZ)
ORDER BY criado_em DESC
LIMIT sqlc.arg('limite') OFFSET sqlc.arg('deslocamento');

-- name: ExcluirLote :exec
DELETE FROM lote WHERE id = $1 AND tenant_id = $2;

-- name: CriarDocumento :one
INSERT INTO documento (
    tenant_id, lote_id, empresa_id, arquivo, tipo, modelo, chave, numero,
    dh_emissao, uf, municipio, crt, emitente_cnpj, estado, motor, motivo,
    dados_simulados, motor_versao_app, motor_versao_db, motor_data_versao_db,
    aliquota_origem_id, requisicao, resposta
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
    $17, $18, $19, $20, $21, $22, $23
)
RETURNING *;

-- name: ListarDocumentosDoLote :many
SELECT * FROM documento WHERE lote_id = $1 AND tenant_id = $2 ORDER BY id;

-- name: BuscarDocumento :one
SELECT * FROM documento WHERE id = $1 AND tenant_id = $2;

-- name: ExcluirDocumentosDoLote :exec
DELETE FROM documento WHERE lote_id = $1 AND tenant_id = $2;

-- name: CriarDivergencia :one
INSERT INTO divergencia (
    tenant_id, documento_id, item, campo, categoria, severidade,
    declarado, apurado, diferenca, codigo, detalhe, memoria_calculo
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: ListarDivergenciasDoLote :many
SELECT d.* FROM divergencia d
JOIN documento doc ON doc.id = d.documento_id
WHERE doc.lote_id = $1
  AND d.tenant_id = $2
  AND (sqlc.narg('severidade')::TEXT IS NULL OR d.severidade = sqlc.narg('severidade')::TEXT)
ORDER BY d.documento_id, d.item;

-- name: ResumoDoLote :many
SELECT
    doc.estado,
    count(*)                                       AS documentos,
    count(*) FILTER (WHERE div.id IS NULL)         AS sem_divergencia,
    count(div.id)                                  AS divergencias
FROM documento doc
LEFT JOIN divergencia div ON div.documento_id = doc.id
WHERE doc.lote_id = $1 AND doc.tenant_id = $2
GROUP BY doc.estado;

-- name: ContagemPorCategoria :many
SELECT d.categoria, d.severidade, count(*) AS total
FROM divergencia d
JOIN documento doc ON doc.id = d.documento_id
WHERE doc.lote_id = $1 AND d.tenant_id = $2
GROUP BY d.categoria, d.severidade;
