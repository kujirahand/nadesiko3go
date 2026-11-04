// #354: 「保存して開く」の保存待ち中にコンテキストメニューが閉じても、
// 操作開始時に選んだファイルを開けることを、製品 app.js の対象範囲を
// 切り出して DOM スタブで実行し確認する。同種の参照をしていた
// menuRevealFinder（Finder表示）もあわせて確認する。
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(
  path.join(__dirname, '../ui/app.js'),
  'utf8',
);
const start = source.indexOf('  function openContextMenu');
const end = source.indexOf('  async function copyFilePath', start);

assert.notEqual(start, -1, 'openContextMenuが見つかりません');
assert.notEqual(end, -1, 'copyFilePathが見つかりません');

const extracted = source.slice(start, end);

const listeners = {};
const testState = {
  confirm: async () => true,
  reveal: async () => {},
  opened: [],
  loadedDirectories: [],
  revealed: [],
  statuses: [],
};

function makeMenuItem(key) {
  return {
    textContent: '',
    addEventListener(type, listener) {
      listeners[key] = listener;
    },
  };
}

const menuOpenEditor = makeMenuItem('openEditor');
const menuRevealFinder = makeMenuItem('revealFinder');

const fileContextMenu = {
  style: {},
};

const context = vm.createContext({
  console,
  fileContextMenu,
  menuOpenEditor,
  menuRevealFinder,
  setStatus: (message) => testState.statuses.push(message),
  testState,
  window: {
    innerWidth: 1024,
    innerHeight: 768,
    revealInFinder: (filePath) => testState.reveal(filePath),
  },
});

vm.runInContext(`
  let selectedContextFile = null;

  async function confirmSaveIfDirty() {
    return testState.confirm();
  }

  function openFile(filePath, name) {
    testState.opened.push({ path: filePath, name });
  }

  function loadDirectory(dirPath) {
    testState.loadedDirectories.push(dirPath);
  }

${extracted}

  globalThis.__contextMenuTest = {
    openContextMenu,
    closeContextMenu,
  };
`, context, { filename: 'context-menu-extracted.js' });

const { openContextMenu, closeContextMenu } = context.__contextMenuTest;

async function main() {
  const fileB = {
    path: '/tmp/B.nako3',
    name: 'B.nako3',
    isDir: false,
  };

  // 保存完了待ちの間にメニューが閉じても、操作開始時のBを開く。
  testState.opened.length = 0;
  testState.confirm = async () => {
    closeContextMenu();
    return true;
  };
  openContextMenu(10, 20, fileB);
  await listeners.openEditor();

  // vm内で作られたオブジェクトはプロトタイプが異なるため、値を取り出して比較する。
  assert.deepEqual(testState.opened.map(({ path: p, name }) => ({ path: p, name })), [{
    path: '/tmp/B.nako3',
    name: 'B.nako3',
  }]);

  // 保存をキャンセルした場合はファイルを開かない。
  testState.opened.length = 0;
  testState.confirm = async () => false;
  openContextMenu(10, 20, fileB);
  await listeners.openEditor();

  assert.deepEqual(testState.opened, []);

  // フォルダでは従来どおり対象フォルダを開く。
  testState.loadedDirectories.length = 0;
  const directory = {
    path: '/tmp/project',
    name: 'project',
    isDir: true,
  };
  openContextMenu(10, 20, directory);
  await listeners.openEditor();

  assert.deepEqual(testState.loadedDirectories, ['/tmp/project']);

  // Finder表示でも、表示待ちの間にメニューが閉じても対象の名前を使う。
  testState.revealed.length = 0;
  testState.statuses.length = 0;
  testState.reveal = async (filePath) => {
    testState.revealed.push(filePath);
    closeContextMenu();
  };
  openContextMenu(10, 20, fileB);
  await listeners.revealFinder();

  assert.deepEqual(testState.revealed, ['/tmp/B.nako3']);
  assert.deepEqual(testState.statuses, ['ファイルを表示しました: B.nako3']);
}

main().catch((err) => {
  console.error(err);
  process.exitCode = 1;
});
