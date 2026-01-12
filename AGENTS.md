# AGENTS.md

## Загальні правила спілкування

- Спілкуватися лише українською мовою.
- Не читати і не обробляти файли з розширенням `.tmp` (ігнорувати їх під час пошуку та аналізу).

## Корисний контекст проєкту

Цей репозиторій — Go‑проксі для OpenAI‑сумісного API поверх Codex OAuth.

Основні ендпоінти:
- `POST /v1/chat/completions`
- `POST /v1/completions`
- `GET /v1/models`

Ключові файли:
- `cmd/codex-proxy/main.go` — точка входу.
- `internal/httpapi/handlers.go` — HTTP‑маршрути.
- `internal/upstream/upstream.go` — upstream виклики.
- `internal/auth/auth.go` — токени з `auth.json`.
- `internal/instructions/instructions.go` — кеш інструкцій.
- `internal/config/config.go` — конфіг із `.env`.

Запуск:
1) `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o codex-proxy ./cmd/codex-proxy`
2) `docker-compose up --build`

Важливі змінні (`configs/.env`):
- `API_KEY` — ключ доступу.
- `AUTH_FILE` — шлях до `~/.codex/auth.json`.
- `INSTR_TTL_HOURS` — TTL кешу інструкцій.
- `LOG_UPSTREAM` — SSE логи.
- `LOG_TOKENS` — логи токенів.
- `ADDR` — адреса сервера.
