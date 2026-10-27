"""Прочитать один снимок или PDF и завершиться.

Процесс не остаётся жить: очередь бэка запускает его только на задачу.
"""

import json
import os
import sys

os.environ.setdefault("GLOG_minloglevel", "3")
os.environ.setdefault("FLAGS_minloglevel", "3")
os.environ.setdefault("PADDLE_PDX_DISABLE_MODEL_SOURCE_CHECK", "True")

MIN_SCORE = 0.5


def fields(res):
    data = res
    if hasattr(res, "json"):
        raw = res.json
        data = raw() if callable(raw) else raw
    if isinstance(data, dict) and isinstance(data.get("res"), dict):
        data = data["res"]
    if isinstance(data, dict):
        return list(data.get("rec_texts") or []), list(data.get("rec_scores") or [])
    try:
        return list(res["rec_texts"]), list(res["rec_scores"])
    except Exception:
        return [], []


def lines_of(result):
    lines = []
    for res in result or []:
        texts, scores = fields(res)
        for index, text in enumerate(texts):
            text = str(text).strip()
            if not text:
                continue
            if index < len(scores) and scores[index] is not None and float(scores[index]) < MIN_SCORE:
                continue
            lines.append(text)
    return lines


def emit(payload):
    print("SERVYS_OCR " + json.dumps(payload, ensure_ascii=False), flush=True)


def main():
    if len(sys.argv) != 2:
        raise SystemExit("нужен путь к снимку или pdf")
    from paddleocr import PaddleOCR

    reader = PaddleOCR(
        lang="ru",
        device="cpu",
        use_doc_orientation_classify=True,
        use_doc_unwarping=False,
        use_textline_orientation=True,
    )
    emit({"lines": lines_of(reader.predict(sys.argv[1]))})


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        emit({"error": str(exc)})
        sys.exit(1)
