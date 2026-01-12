# Codex Proxy на Go

Мінімальний HTTP‑проксі для OpenAI‑сумісного API поверх Codex OAuth.

## Швидкий старт

```bash
./build
docker-compose up --build
```

Під час першого запуску створиться `configs/.env` з `configs/.env.example`.

## Налаштування

Файл `configs/.env`:
- `API_KEY` — ключ доступу до проксі
- `AUTH_FILE` — шлях до `configs/auth.json`
- `INSTR_TTL_HOURS` — TTL кешу інструкцій
- `LOG_UPSTREAM` — логи SSE (для дебагу)
- `LOG_TOKENS` — логи токенів
- `ADDR` — адреса сервера

## API

- `POST /v1/chat/completions`
- `POST /v1/completions`
- `POST /v1/responses`
- `GET /v1/models`

Приклад:
```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-5.2","use_instructions":true,"messages":[{"role":"user","content":"Привіт"}]}'
```

### /v1/responses та Codex інструкції

Codex потребує системні інструкції. Проксі завжди підставляє власні Codex‑інструкції (еквівалент `use_instructions=true`), а якщо клієнт надіслав `instructions`, вони переносяться в перше повідомлення як `User instructions: ...`.

Для `/v1/responses` проксі автоматично встановлює `store=false`. Якщо клієнт не запитує стрім, проксі все одно робить upstream‑стрім і збирає результат у звичайний JSON‑відповідь.
