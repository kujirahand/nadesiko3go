// Goから受け取った操作JSONで受付を登録し、ウィンドウの余白へのドロップを再現する。
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
  const listeners = new Map();
  const calls = [];
  const screen = { dataset: {}, querySelector: () => null };
  const context = vm.createContext({
    document: { addEventListener: (type, fn) => listeners.set(type, fn) },
    root: screen, windowPreview: screen,
    guiElements: new Map(), elements: new Map(),
    send: (...args) => calls.push(args),
    sendGUIEvent: (...args) => calls.push(args),
    operations,
  });
  vm.runInContext(source.slice(begin, finish) + `\n${applyName}(operations);`, context);
  assert.equal(typeof listeners.get('dragover'), 'function', file);
  assert.equal(typeof listeners.get('drop'), 'function', file);
  let prevented = 0;
  listeners.get('dragover')({ preventDefault: () => prevented++ });
  // documentを対象にするので、プレビュー領域外の余白へのドロップも届く。
  listeners.get('drop')({
    preventDefault: () => prevented++,
    dataTransfer: { files: [{ name: '日本語.txt' }, { name: '画像.png' }] },
  });
  assert.equal(prevented, 2, file);
  assert.equal(calls.length, 1, file);
  assert.equal(calls[0][0], 0, file);
  assert.equal(calls[0][1], 'drop', file);
  assert.deepEqual(JSON.parse(calls[0][2].__gonako_drop_files), ['日本語.txt', '画像.png']);
}
