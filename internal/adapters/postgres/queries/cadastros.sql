-- name: CriarTenant :one
INSERT INTO tenant (nome, slug) VALUES ($1, $2) RETURNING *;

-- name: BuscarTenantPorSlug :one
SELECT * FROM tenant WHERE slug = $1 AND ativo;

-- name: BuscarTenant :one
SELECT * FROM tenant WHERE id = $1;

-- name: ListarTenants :many
SELECT * FROM tenant ORDER BY nome;

-- name: AtualizarTenant :one
UPDATE tenant SET nome = $2, ativo = $3 WHERE id = $1 RETURNING *;

-- name: ExcluirTenant :exec
DELETE FROM tenant WHERE id = $1;

-- name: CriarUsuario :one
INSERT INTO usuario (tenant_id, email, nome, hash_senha, papel)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: BuscarUsuarioPorEmail :one
SELECT * FROM usuario WHERE tenant_id = $1 AND email = $2 AND ativo;

-- name: BuscarUsuario :one
SELECT * FROM usuario WHERE id = $1 AND tenant_id = $2;

-- name: ListarUsuarios :many
SELECT * FROM usuario WHERE tenant_id = $1 ORDER BY nome;

-- name: AtualizarUsuario :one
UPDATE usuario SET nome = $3, papel = $4, ativo = $5
WHERE id = $1 AND tenant_id = $2
RETURNING *;

-- name: TrocarSenhaUsuario :exec
UPDATE usuario SET hash_senha = $3 WHERE id = $1 AND tenant_id = $2;

-- name: ExcluirUsuario :exec
DELETE FROM usuario WHERE id = $1 AND tenant_id = $2;

-- name: CriarEmpresa :one
INSERT INTO empresa (tenant_id, cnpj, razao_social, nome_fantasia, crt, uf, municipio)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: BuscarEmpresa :one
SELECT * FROM empresa WHERE id = $1 AND tenant_id = $2;

-- name: BuscarEmpresaPorCNPJ :one
SELECT * FROM empresa WHERE tenant_id = $1 AND cnpj = $2;

-- name: ListarEmpresas :many
SELECT * FROM empresa
WHERE tenant_id = $1
  AND (sqlc.narg('apenas_ativas')::BOOLEAN IS NOT TRUE OR ativo)
  AND (sqlc.narg('busca')::TEXT IS NULL
       OR razao_social ILIKE '%' || sqlc.narg('busca')::TEXT || '%'
       OR cnpj LIKE '%' || sqlc.narg('busca')::TEXT || '%')
ORDER BY razao_social
LIMIT sqlc.arg('limite') OFFSET sqlc.arg('deslocamento');

-- name: AtualizarEmpresa :one
UPDATE empresa
SET razao_social = $3, nome_fantasia = $4, crt = $5, uf = $6, municipio = $7,
    ativo = $8, atualizado_em = now()
WHERE id = $1 AND tenant_id = $2
RETURNING *;

-- name: ExcluirEmpresa :exec
DELETE FROM empresa WHERE id = $1 AND tenant_id = $2;
