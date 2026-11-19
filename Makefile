UPSTREAM := upstream
GO := $(shell mise where go@1.27)/bin/go

# Commits conferidos em 18/11/2026. Pinados de propósito: a RFB publicou 14
# versões de banco em seis semanas, e uma atualização silenciosa muda o número
# que o produto entrega.
REGIME_REGULAR_REF := 150cccb
SIMPLES_REF        := c5b0f01
DANFSE_REF         := 8d79e6b

.PHONY: upstream teste teste-rapido sqlc api compose-up compose-down

upstream:
	@mkdir -p $(UPSTREAM)
	@for repo in calculadora-rtc-regime-regular calculadora-rtc-simples-nacional calculadora-rtc-gerador-danfse; do \
		test -d $(UPSTREAM)/$$repo || \
		git clone https://github.com/ReceitaFederal/$$repo.git $(UPSTREAM)/$$repo; \
	done
	@cd $(UPSTREAM)/calculadora-rtc-regime-regular && git checkout -q $(REGIME_REGULAR_REF)
	@cd $(UPSTREAM)/calculadora-rtc-simples-nacional && git checkout -q $(SIMPLES_REF)
	@cd $(UPSTREAM)/calculadora-rtc-gerador-danfse && git checkout -q $(DANFSE_REF)

teste:
	$(GO) test ./...

teste-rapido:
	$(GO) test -short ./...

# Regera o acesso a dados a partir do SQL. Rodar depois de mexer em
# migrations/ ou queries/.
sqlc:
	sqlc generate

api:
	$(GO) run ./cmd/rtchub

compose-up:
	docker compose up -d --build

compose-down:
	docker compose down
