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
  const enabled = [];
  const screen = { dataset: {}, querySelector: () => null };
  const context = vm.createContext({
    document: { addEventListener: (type, fn) => listeners.set(type, fn) },
    root: screen, windowPreview: screen,
    guiElements: new Map(), elements: new Map(),
    send: (...args) => calls.push(args),
    sendGUIEvent: (...args) => calls.push(args),
    window: { enableFileDropPaths: () => { enabled.push(1); return Promise.resolve(); } },
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
  assert.equal(enabled.length, 1, file);
  assert.equal(prevented, 2, file);
  assert.equal(calls.length, 1, file);
  assert.equal(calls[0][0], 0, file);
  assert.equal(calls[0][1], 'drop', file);
  assert.deepEqual(JSON.parse(calls[0][2].__gonako_drop_files), ['日本語.txt', '画像.png']);

  // WebView2ではFileをホストへ渡し、返された絶対パスを採用する。
  // 既存RPCハンドラーからの文字列エコーや別の要求の返信は無視する。
  const messageListeners = new Set();
  const requests = [];
  context.window.chrome = { webview: {
    addEventListener: (type, fn) => { assert.equal(type, 'message'); messageListeners.add(fn); },
    removeEventListener: (type, fn) => { assert.equal(type, 'message'); messageListeners.delete(fn); },
    postMessageWithAdditionalObjects: (message, files) => requests.push({ message, files }),
  } };
  const droppedFiles = [{ name: '日本語.txt' }, { name: '画像.png' }];
  const drop = () => listeners.get('drop')({
    preventDefault: () => {}, dataTransfer: { files: droppedFiles },
  });
  const receive = data => {
    for (const listener of Array.from(messageListeners)) listener({ data });
  };
  drop();
  drop();
  assert.equal(calls.length, 1, file);
  assert.equal(requests.length, 2, file);
  assert.equal(requests[0].files[0], droppedFiles[0], file);
  assert.equal(requests[0].files[1], droppedFiles[1], file);
  assert.equal(messageListeners.size, 2, file);
  receive(requests[0].message);
  receive({ type: 'gonako-file-drop-paths', requestId: '別の要求', paths: ['間違い'] });
  assert.equal(calls.length, 1, file);
  const fullPaths = ['C:\\資料\\日本語.txt', 'C:\\画像\\画像.png'];
  // 連続ドロップの返信順が変わっても、それぞれの要求に対応する。
  for (const index of [1, 0]) {
    receive({
      type: 'gonako-file-drop-paths',
      requestId: requests[index].message.slice('gonako-file-drop-paths:'.length),
      paths: fullPaths,
    });
    assert.deepEqual(JSON.parse(calls[calls.length - 1][2].__gonako_drop_files), fullPaths, file);
  }
  assert.equal(calls.length, 3, file);
  assert.equal(messageListeners.size, 0, file);
}
