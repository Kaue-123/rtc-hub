-- Esquema inicial do RTC Hub.
--
-- Multi-tenant por tenant_id em toda tabela com dado de cliente. Valores
-- monetários e alíquotas são numeric: nunca float, porque a conferência compara
-- valor declarado com valor apurado e um arredondamento binário viraria
-- divergência inexistente.

CREATE TABLE tenant (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome        TEXT        NOT NULL,
    slug        TEXT        NOT NULL UNIQUE,
    ativo       BOOLEAN     NOT NULL DEFAULT TRUE,
    criado_em   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE usuario (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id     BIGINT      NOT NULL REFERENCES tenant (id) ON DELETE CASCADE,
    email         TEXT        NOT NULL,
    nome          TEXT        NOT NULL,
    hash_senha    TEXT        NOT NULL,
    papel         TEXT        NOT NULL DEFAULT 'operador',
    ativo         BOOLEAN     NOT NULL DEFAULT TRUE,
    criado_em     TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- E-mail é único DENTRO do tenant: a mesma pessoa pode atender dois
    -- escritórios com o mesmo endereço.
    UNIQUE (tenant_id, email)
);

CREATE TABLE empresa (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id     BIGINT      NOT NULL REFERENCES tenant (id) ON DELETE CASCADE,
    cnpj          TEXT        NOT NULL,
    razao_social  TEXT        NOT NULL,
    nome_fantasia TEXT,
    crt           TEXT,
    uf            TEXT,
    municipio     INTEGER,
    ativo         BOOLEAN     NOT NULL DEFAULT TRUE,
    criado_em     TIMESTAMPTZ NOT NULL DEFAULT now(),
    atualizado_em TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, cnpj)
);

-- tenant_id nulo = linha padrão, válida para todos.
CREATE TABLE aliquota_vigencia (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id       BIGINT      REFERENCES tenant (id) ON DELETE CASCADE,
    vigencia_inicio DATE        NOT NULL,
    vigencia_fim    DATE,
    uf              TEXT,
    municipio       INTEGER,
    cbs             NUMERIC(9, 4) NOT NULL,
    ibs_estadual    NUMERIC(9, 4) NOT NULL,
    ibs_municipal   NUMERIC(9, 4) NOT NULL,
    fonte           TEXT        NOT NULL,
    observacao      TEXT,
    criado_em       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (vigencia_fim IS NULL OR vigencia_fim >= vigencia_inicio)
);

CREATE INDEX ix_aliquota_janela ON aliquota_vigencia (vigencia_inicio, vigencia_fim);
CREATE INDEX ix_aliquota_tenant ON aliquota_vigencia (tenant_id);

CREATE TABLE lote (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id     BIGINT      NOT NULL REFERENCES tenant (id) ON DELETE CASCADE,
    empresa_id    BIGINT      REFERENCES empresa (id) ON DELETE SET NULL,
    descricao     TEXT,
    estado        TEXT        NOT NULL DEFAULT 'processando',
    criado_em     TIMESTAMPTZ NOT NULL DEFAULT now(),
    concluido_em  TIMESTAMPTZ
);

CREATE INDEX ix_lote_tenant ON lote (tenant_id, criado_em DESC);

CREATE TABLE documento (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id     BIGINT      NOT NULL REFERENCES tenant (id) ON DELETE CASCADE,
    lote_id       BIGINT      NOT NULL REFERENCES lote (id) ON DELETE CASCADE,
    empresa_id    BIGINT      REFERENCES empresa (id) ON DELETE SET NULL,

    arquivo       TEXT        NOT NULL,
    tipo          TEXT,
    modelo        TEXT,
    chave         TEXT,
    numero        TEXT,
    dh_emissao    TIMESTAMPTZ,
    uf            TEXT,
    municipio     INTEGER,
    crt           TEXT,
    emitente_cnpj TEXT,

    estado          TEXT    NOT NULL,
    motor           TEXT,
    motivo          TEXT,
    dados_simulados BOOLEAN NOT NULL DEFAULT FALSE,

    -- Sem este trio não há como explicar, meses depois, por que o número mudou.
    motor_versao_app     TEXT,
    motor_versao_db      TEXT,
    motor_data_versao_db TEXT,
    aliquota_origem_id   BIGINT REFERENCES aliquota_vigencia (id) ON DELETE SET NULL,

    requisicao JSONB,
    resposta   JSONB,

    criado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ix_documento_lote ON documento (lote_id);
CREATE INDEX ix_documento_chave ON documento (tenant_id, chave);
CREATE INDEX ix_documento_emitente ON documento (tenant_id, emitente_cnpj);

CREATE TABLE divergencia (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id     BIGINT  NOT NULL REFERENCES tenant (id) ON DELETE CASCADE,
    documento_id  BIGINT  NOT NULL REFERENCES documento (id) ON DELETE CASCADE,

    item            INTEGER NOT NULL,
    campo           TEXT    NOT NULL,
    categoria       TEXT    NOT NULL,
    severidade      TEXT    NOT NULL,
    declarado       TEXT,
    apurado         TEXT,
    diferenca       NUMERIC(18, 8),
    codigo          TEXT,
    detalhe         TEXT,
    memoria_calculo TEXT
);

CREATE INDEX ix_divergencia_documento ON divergencia (documento_id);
CREATE INDEX ix_divergencia_severidade ON divergencia (tenant_id, severidade);
