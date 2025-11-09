FROM python:3.11-slim

WORKDIR /app
COPY requirements.txt /app/
RUN pip install --no-cache-dir -r requirements.txt

COPY model.joblib /app/model.joblib
COPY app.py /app/app.py

ENV MODEL_PATH=/app/model.joblib
# Optional: set ML_HMAC_SECRET for HMAC verification
# ENV ML_HMAC_SECRET=change_me

EXPOSE 8000
CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8000"]
