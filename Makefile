# =============================================================================
# Adojobs Bengkalis
#
# Seluruh target berjalan sama persis di lokal maupun di server.
# Yang membedakan hanya variabel ENV:
#
#   make up            -> lingkungan lokal   (ENV=local, nilai bawaan)
#   make up ENV=prod   -> lingkungan server  (lewat SSH ke DEPLOY_HOST)
#
# Tidak ada perintah yang ditulis dua kali untuk dua lingkungan: perbedaannya
# hanya pada berkas compose yang dipakai dan ada/tidaknya awalan SSH.
# =============================================================================

SHELL := /bin/bash
.DEFAULT_GOAL := help

# Muat .env supaya variabel seperti POSTGRES_USER dan DEPLOY_HOST tersedia.
ifneq (,$(wildcard .env))
include .env
export
endif

ENV ?= local

POSTGRES_USER ?= adojobs
POSTGRES_DB   ?= adojobs
DEPLOY_USER   ?= deploy
DEPLOY_PORT   ?= 22
DEPLOY_PATH   ?= /opt/adojobsid
APP_IMAGE     ?= adojobsid-app
APP_TAG       ?= latest
GO_IMAGE      ?= golang:1.26-alpine
# Tag tambahan per deploy untuk rollback; dibekukan sekali per pemanggilan make.
DEPLOY_STAMP  := $(shell date +%Y%m%d-%H%M)

# --- pemilihan lingkungan ----------------------------------------------------
ifeq ($(ENV),prod)
  COMPOSE_FILES := -f docker-compose.yml -f docker-compose.prod.yml
  SSH           := ssh -p $(DEPLOY_PORT) $(DEPLOY_USER)@$(DEPLOY_HOST)
  SSH_TTY       := ssh -t -p $(DEPLOY_PORT) $(DEPLOY_USER)@$(DEPLOY_HOST)
else
  # Di lokal, docker-compose.override.yml dimuat otomatis oleh Docker Compose.
  COMPOSE_FILES :=
  SSH           :=
  SSH_TTY       :=
endif

# compose  : perintah non-interaktif (aman untuk pipe dan redirect)
# composei : perintah interaktif (psql, shell) — butuh TTY
ifeq ($(ENV),prod)
  compose  = $(SSH) 'cd $(DEPLOY_PATH) && docker compose $(COMPOSE_FILES) $(1)'
  composei = $(SSH_TTY) 'cd $(DEPLOY_PATH) && docker compose $(COMPOSE_FILES) $(1)'
else
  compose  = docker compose $(COMPOSE_FILES) $(1)
  composei = docker compose $(COMPOSE_FILES) $(1)
endif

# Menjalankan perintah Go di dalam container sekali pakai, sehingga tidak ada
# toolchain Go yang perlu dipasang di host.
GO_RUN_FLAGS = --rm \
	-v "$(CURDIR)":/src \
	-v adojobsid-gomod:/go/pkg/mod \
	-v adojobsid-gobuild:/root/.cache/go-build \
	-w /src
GO_RUN = docker run $(GO_RUN_FLAGS) $(GO_IMAGE)

.PHONY: help setup up down restart build rebuild ps logs logs-app \
        migrate migrate-down migrate-version seed admin-create shell psql redis-cli \
        test test-e2e cache-clear fmt lint generate css ikon deploy releases rollback deploy-check backup-db restore-db \
        clean

## help: tampilkan daftar perintah
help:
	@echo ""
	@echo "  Adojobs Bengkalis — perintah yang tersedia"
	@echo "  Tambahkan ENV=prod untuk menjalankan di server."
	@echo ""
	@# -h menekan nama berkas: MAKEFILE_LIST juga memuat .env yang ikut di-include.
	@grep -hE '^## ' $(MAKEFILE_LIST) | sed -e 's/## /  /' | column -t -s ':'
	@echo ""

## setup: siapkan .env dari contoh lalu jalankan seluruh service
setup:
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		secret=$$(openssl rand -base64 48 | tr -d '\n/+=' | head -c 48); \
		pgpass=$$(openssl rand -hex 16); \
		redispass=$$(openssl rand -hex 16); \
		sed -i.bak "s|^SESSION_SECRET=.*|SESSION_SECRET=$$secret|" .env; \
		sed -i.bak "s|^POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=$$pgpass|" .env; \
		sed -i.bak "s|^REDIS_PASSWORD=.*|REDIS_PASSWORD=$$redispass|" .env; \
		rm -f .env.bak; \
		echo "  .env dibuat dengan kredensial acak."; \
	else \
		echo "  .env sudah ada, dilewati."; \
	fi
	@$(MAKE) up
	@$(MAKE) migrate
	@$(MAKE) seed
	@echo ""
	@echo "  Siap. Buka http://localhost:$${APP_PORT:-3000}"

## up: jalankan semua service
up:
	@$(call compose,up -d)
	@$(call compose,ps)

## down: hentikan semua service
down:
	@$(call compose,down)

## restart: mulai ulang service app
restart:
	@$(call compose,restart app)

## build: build ulang image app
build:
	@$(call compose,build app)

## rebuild: build ulang image app tanpa cache lalu jalankan
rebuild:
	@$(call compose,build --no-cache app)
	@$(call compose,up -d app)

## ps: status seluruh service
ps:
	@$(call compose,ps)

## logs: tail log semua service
logs:
	@$(call compose,logs -f --tail=100)

## logs-app: tail log service app saja
logs-app:
	@$(call compose,logs -f --tail=100 app)

## migrate: jalankan migrasi database ke versi terbaru (migrasi tersemat di image — `make build` dulu bila ada berkas migrasi baru)
migrate:
	@$(call compose,run --rm app migrate up)

## migrate-down: rollback satu migrasi
migrate-down:
	@$(call compose,run --rm app migrate down)

## migrate-version: tampilkan versi migrasi yang aktif
migrate-version:
	@$(call compose,run --rm app migrate version)

## seed: isi data dummy untuk pengembangan
seed:
	@$(call compose,run --rm app seed)

## admin-create: buat admin pertama dari ADMIN_PHONE & ADMIN_PASSWORD di .env
admin-create:
	@if [ -z "$(ADMIN_PHONE)" ] || [ -z "$(ADMIN_PASSWORD)" ]; then \
		echo "  Isi ADMIN_PHONE dan ADMIN_PASSWORD di .env terlebih dahulu."; exit 1; \
	fi
	@$(call compose,run --rm app admin:create)

## shell: masuk shell container app
shell:
	@$(call composei,exec app sh)

## psql: buka psql ke database di dalam container
psql:
	@$(call composei,exec postgres psql -U $(POSTGRES_USER) -d $(POSTGRES_DB))

## redis-cli: buka redis-cli di dalam container
redis-cli:
	@$(call composei,exec redis sh -c 'redis-cli -a "$$REDIS_PASSWORD"')

## test: jalankan seluruh test Go
test:
	@$(GO_RUN) sh -c "apk add --no-cache git >/dev/null && \
		go install github.com/a-h/templ/cmd/templ@v0.3.1020 >/dev/null 2>&1 && \
		templ generate && go test ./... -count=1"

## fmt: jalankan gofmt/goimports ke seluruh kode
fmt:
	@$(GO_RUN) sh -c "gofmt -w -s . && \
		go run golang.org/x/tools/cmd/goimports@latest -w -local github.com/hermansyah/adojobsid ."
	@echo "  Kode sudah diformat."

## cache-clear: kosongkan cache aplikasi di Redis (hasil pencarian, filter, pengaturan) — sesi login tidak disentuh
# Skrip Lua-nya disimpan di variabel karena mengandung koma; koma literal di
# dalam argumen $(call ...) akan dibaca make sebagai pemisah argumen.
LUA_HAPUS_CACHE = local n=0 for _,k in ipairs(redis.call('KEYS', ARGV[1])) do redis.call('DEL', k) n=n+1 end return n
cache-clear:
	@$(call compose,exec -T redis redis-cli -a "$(REDIS_PASSWORD)" --no-auth-warning EVAL "$(LUA_HAPUS_CACHE)" 0 'cache:*') | sed 's/^/  kunci dihapus: /'

## test-e2e: uji alur end-to-end di atas Postgres & Redis compose (butuh `make up`)
test-e2e:
	@docker run $(GO_RUN_FLAGS) --network $(COMPOSE_PROJECT_NAME)_internal --env-file .env \
		-e E2E=1 -e POSTGRES_HOST=postgres -e REDIS_HOST=redis -e REDIS_DB=15 \
		-e POSTGRES_DB=$(POSTGRES_DB)_e2e -e UPLOAD_DIR=/tmp/e2e-uploads \
		$(GO_IMAGE) sh -c "apk add --no-cache git >/dev/null && \
		go install github.com/a-h/templ/cmd/templ@v0.3.1020 >/dev/null 2>&1 && \
		templ generate && go test ./internal/app/ -count=1 -v"

## lint: periksa format dan jalankan go vet
lint:
	@$(GO_RUN) sh -c "test -z \"\$$(gofmt -l -s .)\" || (gofmt -l -s . && exit 1); go vet ./..."

## generate: hasilkan ulang berkas Go dari template templ
generate:
	@$(GO_RUN) sh -c "go install github.com/a-h/templ/cmd/templ@v0.3.1020 >/dev/null 2>&1 && templ generate"

## ikon: hasilkan ikon PNG aplikasi (PWA/Android) + ekspor set ikon SVG dari icons.templ
ikon:
	@$(GO_RUN) go run ./tools/ikon
	@echo "  Ikon aplikasi di web/static/img, set ikon SVG di web/static/design/ikon"

## css: bangun ulang CSS Tailwind
css:
	@docker run --rm -v "$(CURDIR)":/app -w /app node:22-alpine \
		sh -c "npm install --no-audit --no-fund --loglevel=error && \
		       npx tailwindcss -i ./web/static/css/source.css -o ./web/static/css/app.css --minify"
	@echo "  CSS dibangun ke web/static/css/app.css"

## deploy: build image produksi, kirim ke server, lalu restart service
deploy: deploy-check
	@echo "==> Membangun image produksi $(APP_IMAGE):$(APP_TAG)"
	@docker build \
		--file docker/Dockerfile \
		--build-arg VERSION=$$(git rev-parse --short HEAD 2>/dev/null || echo manual) \
		--tag $(APP_IMAGE):$(APP_TAG) \
		--platform linux/amd64 \
		.
	@echo "==> Menyalin berkas compose ke $(DEPLOY_HOST):$(DEPLOY_PATH)"
	@$(SSH) 'mkdir -p $(DEPLOY_PATH)'
	@scp -P $(DEPLOY_PORT) -q \
		docker-compose.yml docker-compose.prod.yml \
		$(DEPLOY_USER)@$(DEPLOY_HOST):$(DEPLOY_PATH)/
	@echo "==> Menandai image $(APP_IMAGE):$(DEPLOY_STAMP) untuk rollback"
	@docker tag $(APP_IMAGE):$(APP_TAG) $(APP_IMAGE):$(DEPLOY_STAMP)
ifeq ($(strip $(DEPLOY_REGISTRY)),)
	@echo "==> Mengirim image lewat SSH (tanpa registry)"
	@docker save $(APP_IMAGE):$(APP_TAG) $(APP_IMAGE):$(DEPLOY_STAMP) | gzip --fast | \
		$(SSH) 'gunzip | docker load'
else
	@echo "==> Mendorong image ke registry $(DEPLOY_REGISTRY)"
	@docker tag $(APP_IMAGE):$(APP_TAG) $(DEPLOY_REGISTRY)/$(APP_IMAGE):$(APP_TAG)
	@docker push $(DEPLOY_REGISTRY)/$(APP_IMAGE):$(APP_TAG)
	@$(SSH) 'cd $(DEPLOY_PATH) && docker compose $(COMPOSE_FILES) pull app'
endif
	@echo "==> Menjalankan migrasi di server"
	@$(MAKE) --no-print-directory migrate ENV=prod
	@echo "==> Menjalankan ulang service di server"
	@$(MAKE) --no-print-directory up ENV=prod
	@echo "==> Selesai. Versi ini tersimpan sebagai $(APP_IMAGE):$(DEPLOY_STAMP) di server."
	@echo "    Rollback: make rollback ENV=prod TAG=<stamp>   (daftar: make releases ENV=prod)"

## releases: daftar tag image yang tersedia untuk rollback
releases:
	@$(SSH) 'docker image ls $(APP_IMAGE) --format "  {{.Tag}}\t{{.CreatedSince}}" | grep -v latest'

## rollback: kembalikan app ke tag tertentu (TAG=YYYYmmdd-HHMM), tanpa migrasi mundur
rollback:
	@if [ -z "$(TAG)" ]; then echo "  Sebutkan tag: make rollback ENV=prod TAG=20260924-1015"; exit 1; fi
	@echo "==> Menandai $(APP_IMAGE):$(TAG) sebagai $(APP_TAG) lalu menjalankan ulang app"
	@$(SSH) 'docker tag $(APP_IMAGE):$(TAG) $(APP_IMAGE):$(APP_TAG)'
	@$(call compose,up -d app)
	@echo "  Catatan: migrasi database tidak dimundurkan; pastikan versi ini kompatibel dengan skema saat ini."

## deploy-check: pastikan tujuan deploy sudah terkonfigurasi
deploy-check:
	@if [ -z "$(DEPLOY_HOST)" ]; then \
		echo "  DEPLOY_HOST belum diisi di .env."; exit 1; \
	fi
	@if ! $(SSH) 'test -f $(DEPLOY_PATH)/.env' 2>/dev/null; then \
		echo "  Berkas $(DEPLOY_PATH)/.env belum ada di server."; \
		echo "  Salin sekali saat setup awal:"; \
		echo "    ssh -p $(DEPLOY_PORT) $(DEPLOY_USER)@$(DEPLOY_HOST) 'mkdir -p $(DEPLOY_PATH)'"; \
		echo "    scp -P $(DEPLOY_PORT) .env $(DEPLOY_USER)@$(DEPLOY_HOST):$(DEPLOY_PATH)/.env"; \
		exit 1; \
	fi

## backup-db: dump database ke berkas dengan timestamp
backup-db:
	@mkdir -p backups
	@stamp=$$(date +%Y%m%d-%H%M%S); \
	file="backups/$(POSTGRES_DB)-$(ENV)-$$stamp.sql.gz"; \
	echo "==> Mencadangkan database ke $$file"; \
	$(call compose,exec -T postgres pg_dump -U $(POSTGRES_USER) -d $(POSTGRES_DB) --clean --if-exists) \
		| gzip > "$$file"; \
	if [ ! -s "$$file" ]; then rm -f "$$file"; echo "  Cadangan gagal: berkas kosong."; exit 1; fi; \
	echo "  Selesai: $$file ($$(du -h "$$file" | cut -f1))"

## restore-db: pulihkan database dari berkas dump (FILE=backups/xxx.sql.gz)
restore-db:
	@if [ -z "$(FILE)" ]; then \
		echo "  Sebutkan berkasnya: make restore-db FILE=backups/nama.sql.gz"; \
		echo ""; ls -1t backups/*.sql.gz 2>/dev/null | head -10; exit 1; \
	fi
	@if [ ! -f "$(FILE)" ]; then echo "  Berkas tidak ditemukan: $(FILE)"; exit 1; fi
	@echo "  Ini akan MENIMPA database $(POSTGRES_DB) di lingkungan $(ENV)."
	@read -p "  Ketik 'ya' untuk melanjutkan: " jawab; [ "$$jawab" = "ya" ] || { echo "  Dibatalkan."; exit 1; }
	@echo "==> Memulihkan dari $(FILE)"
	@gunzip -c "$(FILE)" | \
		$(call compose,exec -T postgres psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) --quiet)
	@echo "  Pemulihan selesai."

## clean: hentikan service dan hapus volume (DATA IKUT TERHAPUS)
clean:
	@echo "  Ini akan menghapus seluruh data database dan foto di lingkungan $(ENV)."
	@read -p "  Ketik 'ya' untuk melanjutkan: " jawab; [ "$$jawab" = "ya" ] || { echo "  Dibatalkan."; exit 1; }
	@$(call compose,down -v)
