# SmartFin ML (minimal bundle)

Минимальный ML-сервис для SmartFin:
- Классификация транзакций по категориям (TF-IDF + LogisticRegression).
- Агрегаты: суммы по категориям, топ-мерчанты, кэшфлоу.
- HTTP `/score` (FastAPI) **или** CLI `score.py`.

## Запуск (Docker)
```bash
cd smartfin-ml
docker build -t smartfin-ml:latest .
docker run --rm -p 8000:8000 -e ML_HMAC_SECRET=change_me smartfin-ml:latest
```

## HTTP API
POST `/score`
Headers (опционально):
- `X-ML-Signature: sha256=<hex>` если задан `ML_HMAC_SECRET` (HMAC по сырому body).

## CLI
```bash
echo '{"user_id":"u","window_days":30,"transactions":[{"id":"1","amount_minor":10000,"currency":"RUB","direction":"debit","merchant":"PEREKRESTOK","description":"Покупка","booked_at":"2025-11-01T00:00:00Z"}]}' | \
MODEL_PATH=./model.joblib python score.py
```

## Интеграция с Go
- `ML_BASE_URL=http://ml:8000`
- Заголовок подписи: `X-ML-Signature: sha256=<hex(hmac_sha256(secret, body))>`
- Ответ можно кэшировать в Redis (15–60 секунд).

Репорты обучения: `training_report.txt`. Датасет: `synth_data.csv`.
