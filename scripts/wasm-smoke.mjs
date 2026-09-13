// WebAssembly版(gonako.wasm)をNode.jsで読み込み、gonako.run が動くか確かめる。
// 使い方: node scripts/wasm-smoke.mjs [bin/wasm]
import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";

const dir = path.resolve(process.argv[2] ?? "bin/wasm");
createRequire(import.meta.url)(path.join(dir, "wasm_exec.js"));

const ready = new Promise((resolve) => { globalThis.gonakoReady = resolve; });
const go = new Go();
const { instance } = await WebAssembly.instantiate(
  fs.readFileSync(path.join(dir, "gonako.wasm")), go.importObject);
go.run(instance);
const gonako = await ready;

let failed = 0;
async function check(name, code, want) {
  const printed = [];
  const result = await gonako.run(code, { onPrint: (s) => printed.push(s) });
  const got = { ok: result.ok, output: result.output,
    kind: result.error?.kind ?? "", line: result.error?.line ?? 0 };
  const diffs = Object.entries(want).filter(([k, v]) => got[k] !== v);
  if (want.output !== undefined && printed.map((s) => s + "\n").join("") !== want.output) {
    diffs.push(["onPrint", printed]);
  }
  if (diffs.length) {
    failed++;
    console.log(`NG ${name}:`, JSON.stringify(got), JSON.stringify(result.error));
  } else {
    console.log(`OK ${name}`);
  }
}

console.log(`gonako v${gonako.version}`);
await check("表示", "「こんにちは」を表示。\n1+2を表示。", { ok: true, output: "こんにちは\n3\n" });
await check("秒待機", "「前」を表示。\n0.1秒待機。\n「後」を表示。", { ok: true, output: "前\n後\n" });
await check("言う(ダイアログ無し)", "「やあ」と言う。", { ok: true, output: "やあ\n" });
await check("エラー", "「a」を表示。\n存在しない命令。", { ok: false, line: 2 });
await check("nodelibは無し", "「.」のファイル名一覧取得して表示。", { ok: false });

// 同時に呼んでも、呼び出した順に1本ずつ実行されることを確かめる (#89 レビュー指摘)。
// Aは待機ありで先に呼び、Bは待機無しで後から呼ぶ。goroutineの生成順や
// mutexの取得順に頼っていると、待機の無いBが先に終わってしまう。
{
  const order = [];
  const pA = gonako.run("「A」を表示。\n0.2秒待機。").then(() => order.push("A"));
  const pB = gonako.run("「B」を表示。").then(() => order.push("B"));
  await Promise.all([pA, pB]);
  if (order.join(",") === "A,B") {
    console.log("OK 呼び出し順を保つ");
  } else {
    failed++;
    console.log("NG 呼び出し順を保つ:", order);
  }
}

process.exit(failed ? 1 : 0);
