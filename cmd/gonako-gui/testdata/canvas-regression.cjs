// Goで実行したサンプルの操作を、エディタと梱包画面の描画処理へ渡す。
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
const operations = JSON.parse(fs.readFileSync(0, 'utf8'));
for (const [file, start, end, applyName] of [
  ['app.js', 'function applyGUIOperations(', 'function selectOutputTab(', 'applyGUIOperations'],
  ['bundled/app.js', 'function apply(', 'function ask(', 'apply'],
]) {
  const source = fs.readFileSync(path.join(__dirname, '../ui', file), 'utf8');
  const begin = source.indexOf(start);
  const finish = source.indexOf(end, begin);
  assert.ok(begin >= 0 && finish > begin);
  const calls = [];
  const ctx = {};
  const strokes = [];
  const fills = [];
  for (const method of ['save','restore','beginPath','moveTo','lineTo','stroke','fillRect','strokeRect','arc','fill','clearRect']) {
    ctx[method] = (...args) => {
      calls.push([method, ...args]);
      if (method === "fillRect" || method === "fill") fills.push(ctx.fillStyle);
      if (method === "stroke" || method === "strokeRect") strokes.push([ctx.lineWidth, ctx.strokeStyle]);
    };
  }
  const elements = new Map();
  const root = { appendChild() {}, querySelector() { return null; } };
  const context = vm.createContext({
    document: { createElement(tag) { return {
      tagName: tag.toUpperCase(), dataset: {}, classList: { add() {} }, style: {},
      setAttribute(k,v) { this[k] = Number(v) || v; },
      matches() { return false; }, addEventListener() {},
      getContext(type) { assert.equal(tag, 'canvas'); assert.equal(type, '2d'); return ctx; },
    }; } },
    root, windowPreview: root, elements, guiElements: elements, operations,
  });
  vm.runInContext(source.slice(begin,finish) + `\n${applyName}(operations);`,context);
  assert.deepEqual(calls, [
    ['save'], ['fillRect',0,0,320,200], ['strokeRect',0,0,320,200], ['restore'],
    ['save'], ['fillRect',20,20,140,80], ['strokeRect',20,20,140,80], ['restore'],
    ['save'], ['beginPath'], ['arc',240,80,45,0,Math.PI*2], ['fill'], ['stroke'], ['restore'],
    ['save'], ['beginPath'], ['moveTo',20,160], ['lineTo',300,160], ['stroke'], ['restore'],
    ['save'], ['clearRect',0,0,320,200], ['restore'],
  ],file);
  assert.deepEqual(fills, ['white', '#87ceeb', 'orange'], file);
  assert.deepEqual(strokes, [[1, '#000000'], [5, 'navy'], [5, 'navy'], [5, 'navy']], file);
}
