#!/usr/bin/env python3
import sys, json, joblib, os

MODEL_PATH = os.getenv("MODEL_PATH", "model.joblib")
clf = joblib.load(MODEL_PATH)

def to_text(tx):
    return ((tx.get("merchant") or "") + " " + (tx.get("description") or "")).lower().strip()

def main():
    body = sys.stdin.read()
    payload = json.loads(body)
    txs = payload.get("transactions", [])
    texts = [to_text(tx) for tx in txs]
    if not texts:
        print(json.dumps({"categories": [], "top_merchants": [], "cashflow": {"in_minor": 0, "out_minor": 0}, "insights": [], "version": "ml_simple_v1"}))
        return
    preds = clf.predict(texts)

    cat_sum = {}; cat_cnt = {}; merch_sum = {}; merch_cnt = {}; in_minor=0; out_minor=0
    for tx, cat in zip(txs, preds):
        direction = tx.get("direction","debit")
        sign = -1 if direction=="debit" else 1
        amt = int(tx.get("amount_minor",0))
        if sign>0: in_minor += amt
        else: out_minor += amt
        cat_sum[cat] = cat_sum.get(cat,0) + (amt if sign<0 or cat=="Income" else 0)
        cat_cnt[cat] = cat_cnt.get(cat,0) + 1
        m = (tx.get("merchant") or "UNKNOWN").strip() or "UNKNOWN"
        merch_sum[m] = merch_sum.get(m,0) + amt
        merch_cnt[m] = merch_cnt.get(m,0) + 1

    categories = [{"name":k,"sum_minor":v,"tx_count":cat_cnt.get(k,0)} for k,v in sorted(cat_sum.items(), key=lambda kv: kv[1], reverse=True)]
    top_merchants = [{"name":k,"sum_minor":v,"tx_count":merch_cnt.get(k,0)} for k,v in sorted(merch_sum.items(), key=lambda kv: kv[1], reverse=True)[:10]]
    res = {"categories":categories,"top_merchants":top_merchants,"cashflow":{"in_minor":in_minor,"out_minor":out_minor},"insights":[],"version":"ml_simple_v1"}
    print(json.dumps(res, ensure_ascii=False))

if __name__ == "__main__":
    main()
