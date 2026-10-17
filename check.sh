#!/bin/sh
# 一键校验：语料可重建且两份一致 → 标准答案未过期 → Python 测试 → Go vet + 测试（含与 Python 对拍）
set -eu
cd "$(dirname "$0")"
python3 tools/build_data.py
cmp divination/data/iching.json go/internal/corpus/iching.json
python3 tools/gen_golden.py --check
python3 -m unittest
if command -v go >/dev/null 2>&1; then
  (cd go && go vet ./... && go test -count=1 ./...)
else
  echo "（未找到 go，跳过 Go 版测试）" >&2
fi
echo "全部通过"
