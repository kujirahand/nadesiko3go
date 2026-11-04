// #354: 「保存して開く」の保存待ち中にコンテキストメニューが閉じても、
// 操作開始時に選んだファイルを開けることを、製品 app.js の対象範囲を
// 切り出して DOM スタブで実行し確認する。
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(
  path.join(__dirname, '../ui/app.js'),
  'utf8',
);
const start = source.indexOf('  function openContextMenu');
const end = source.indexOf('  menuRevealFinder.addEventListener', start);

assert.notEqual(start, -1, 'openContextMenuが見つかりません');
assert.notEqual(end, -1, 'menuRevealFinderハンドラーが見つかりません');

const extracted = source.slice(start, end);

const listeners = {};
const testState = {
  confirm: async () => true,
  opened: [],
  loadedDirectories: [],
};

const menuOpenEditor = {
  textContent: '',
  addEventListener(type, listener) {
    listeners[type] = listener;
  },
};

const fileContextMenu = {
  style: {},
};

const context = vm.createContext({
  console,
  fileContextMenu,
  menuOpenEditor,
  testState,
  window: {
    innerWidth: 1024,
    innerHeight: 768,
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
  await listeners.click();

  // vm内で作られたオブジェクトはプロトタイプが異なるため、値を取り出して比較する。
  assert.deepEqual(testState.opened.map(({ path: p, name }) => ({ path: p, name })), [{
    path: '/tmp/B.nako3',
    name: 'B.nako3',
  }]);

  // 保存をキャンセルした場合はファイルを開かない。
  testState.opened.length = 0;
  testState.confirm = async () => false;
  openContextMenu(10, 20, fileB);
  await listeners.click();

  assert.deepEqual(testState.opened, []);

  // フォルダでは従来どおり対象フォルダを開く。
  testState.loadedDirectories.length = 0;
  const directory = {
    path: '/tmp/project',
    name: 'project',
    isDir: true,
  };
  openContextMenu(10, 20, directory);
  await listeners.click();

  assert.deepEqual(testState.loadedDirectories, ['/tmp/project']);
}

main().catch((err) => {
  console.error(err);
  process.exitCode = 1;
});
