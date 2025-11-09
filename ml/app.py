import os, hmac, hashlib
from fastapi import FastAPI, Request, HTTPException
from pydantic import BaseModel, Field
from typing import List, Optional, Literal, Dict
from datetime import datetime
import joblib

MODEL_PATH = os.getenv("MODEL_PATH", "model.joblib")
HMAC_SECRET = os.getenv("ML_HMAC_SECRET", "")  # optional

app = FastAPI(title="SmartFin ML", version="1.0.0")
clf = joblib.load(MODEL_PATH)

class Tx(BaseModel):
    id: str
    amount_minor: int
    currency: str
    direction: Literal["debit","credit"]
    merchant: Optional[str] = ""
    description: Optional[str] = ""
    booked_at: datetime

class ScoreRequest(BaseModel):
    user_id: str
    window_days: int = Field(30, ge=1, le=365)
    transactions: List[Tx]

def verify_hmac(signature: str, body: bytes) -> None:
    if not HMAC_SECRET:
        return
    if not signature:
        raise HTTPException(status_code=401, detail="missing signature")
    try:
        algo, hexsig = signature.split("=", 1)
    except ValueError:
        raise HTTPException(status_code=400, detail="bad signature format")
    if algo.lower() != "sha256":
        raise HTTPException(status_code=400, detail="unsupported signature algo")
    digest = hmac.new(HMAC_SECRET.encode(), body, hashlib.sha256).hexdigest()
    if not hmac.compare_digest(digest, hexsig):
        raise HTTPException(status_code=401, detail="bad signature")

@app.post("/score")
async def score(req: Request):
    body = await req.body()
    verify_hmac(req.headers.get("x-ml-signature",""), body)
    data = ScoreRequest.model_validate_json(body)

    texts = [((tx.merchant or "") + " " + (tx.description or "")).lower().strip() for tx in data.transactions]
    if not texts:
        return {"categories": [], "top_merchants": [], "cashflow": {"in_minor": 0, "out_minor": 0}, "insights": [], "version": "ml_simple_v1"}

    preds = clf.predict(texts)

    cat_sum, cat_cnt = {}, {}
    merch_sum, merch_cnt = {}, {}
    in_minor = out_minor = 0

    for tx, cat in zip(data.transactions, preds):
        sign = -1 if tx.direction == "debit" else 1
        amt = tx.amount_minor
        if sign > 0:
            in_minor += amt
        else:
            out_minor += amt
        cat_sum[cat] = cat_sum.get(cat, 0) + (amt if (sign < 0 or cat == "Income") else 0)
        cat_cnt[cat] = cat_cnt.get(cat, 0) + 1
        m = (tx.merchant or "UNKNOWN").strip() or "UNKNOWN"
        merch_sum[m] = merch_sum.get(m, 0) + amt
        merch_cnt[m] = merch_cnt.get(m, 0) + 1

    categories = [{"name": k, "sum_minor": v, "tx_count": cat_cnt.get(k,0)} for k, v in sorted(cat_sum.items(), key=lambda kv: kv[1], reverse=True)]
    top_merchants = [{"name": k, "sum_minor": v, "tx_count": merch_cnt.get(k,0)} for k, v in sorted(merch_sum.items(), key=lambda kv: kv[1], reverse=True)[:10]]
    insights = []
    if any(c["name"] == "Subscriptions" and c["sum_minor"] > 0 for c in categories):
        insights.append({"code": "SUBS_SHARE", "title": "Подписки занимают заметную долю расходов", "severity": "info"})

    return {
        "categories": categories,
        "top_merchants": top_merchants,
        "cashflow": {"in_minor": in_minor, "out_minor": out_minor},
        "insights": insights,
        "version": "ml_simple_v1"
    }
