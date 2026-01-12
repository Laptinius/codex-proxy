# AGENTS.md

## Загальні правила спілкування

- Спілкуватися лише українською мовою.
- Не читати і не обробляти файли з розширенням `.tmp` (ігнорувати їх під час пошуку та аналізу).

## Корисний контекст проєкту

Цей репозиторій — Go‑проксі для OpenAI‑сумісного API поверх Codex OAuth.

Основні ендпоінти:
- `POST /v1/chat/completions`
- `POST /v1/completions`
- `POST /v1/responses`
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
- `AUTH_FILE` — шлях до `configs/auth.json`.
- `INSTR_TTL_HOURS` — TTL кешу інструкцій.
- `LOG_UPSTREAM` — SSE логи.
- `LOG_TOKENS` — логи токенів.
- `ADDR` — адреса сервера.

## Codex: обов’язкові умови для працездатності

- **Instructions**: Codex потребує системні інструкції завжди. Проксі підставляє власні Codex‑інструкції у поле `instructions` незалежно від запиту.
- **Клієнтські `instructions`**: якщо клієнт передав `instructions`, їх потрібно переносити в перше повідомлення `input` як текст `User instructions: ...`.
- **Streaming**: upstream Codex вимагає `stream=true`. Навіть якщо клієнт не просив стрім, проксі має стрімити до upstream і збирати результат у звичайну відповідь.
- **Store**: upstream Codex вимагає `store=false`. Якщо поле відсутнє — підставляти `false`.
- **Авторизація**: клієнтський доступ — через `Authorization: Bearer $API_KEY`, а OAuth‑токени Codex беруться з `configs/auth.json` на стороні проксі.
