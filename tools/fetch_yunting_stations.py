import argparse
import hashlib
import json
from pathlib import Path
import time
from urllib.parse import urlencode
from urllib.request import Request, urlopen

parser = argparse.ArgumentParser()
parser.add_argument("--output", type=Path)
args = parser.parse_args()
params = {"categoryId": "0", "provinceCode": "0"}
query = urlencode(sorted(params.items()))
timestamp = str(int(time.time() * 1000))
public_web_key = "f0fc4c668392f9f9a447e48584c214ee"
sign = hashlib.md5(f"{query}&timestamp={timestamp}&key={public_web_key}".encode()).hexdigest().upper()
url = "https://ytmsout.radio.cn/web/appBroadcast/list?" + query
request = Request(url, headers={
    "Content-Type": "application/json", "equipmentId": "0000",
    "platformCode": "WEB", "timestamp": timestamp, "sign": sign,
})
with urlopen(request, timeout=20) as response:
    payload = json.load(response)
if payload.get("code") != 0:
    raise SystemExit(json.dumps(payload, ensure_ascii=False))
result = {"source": url, "queried_at": timestamp, "stations": payload["data"]}
if args.output:
    args.output.write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding="utf-8")
print(json.dumps(result, ensure_ascii=False, indent=2))
