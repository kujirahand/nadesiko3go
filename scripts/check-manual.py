#!/usr/bin/env python3
"""命令一覧にある関数・定数・変数のマニュアルページを確認する。"""

import json
import sys
from collections import Counter
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
LIST = ROOT / "internal/commanddoc/command-list.json"
MANUAL = ROOT / "manual/gonako"
TYPES = {"func": "関数", "const": "定数", "var": "変数"}


def main():
    try:
        commands = json.loads(LIST.read_text(encoding="utf-8"))
        if not isinstance(commands, list) or not commands:
            raise ValueError("命令一覧が空または配列ではありません")
        if not MANUAL.is_dir():
            raise ValueError(f"マニュアルディレクトリがありません: {MANUAL}")
        counts = Counter()
        missing = []
        for command in commands:
            name, kind = command["name"], command["type"]
            if not isinstance(name, str) or not name or Path(name).name != name or kind not in TYPES:
                raise ValueError(f"不正な命令データ: {command!r}")
            counts[kind] += 1
            if not (MANUAL / (name + ".txt")).is_file():
                missing.append((kind, name))
    except (OSError, ValueError, KeyError, TypeError) as error:
        print(f"確認できません: {error}", file=sys.stderr)
        return 2

    for kind, name in sorted(missing):
        print(f"未作成 [{TYPES[kind]}] {name}: manual/gonako/{name}.txt")
    for kind, label in TYPES.items():
        absent = sum(1 for item, _ in missing if item == kind)
        print(f"{label}: {counts[kind]} 件中 {absent} 件が未作成")
    print("基準: internal/commanddoc/command-list.json（必要に応じて just gen-command-list で更新）")
    return 1 if missing else 0


if __name__ == "__main__":
    sys.exit(main())
