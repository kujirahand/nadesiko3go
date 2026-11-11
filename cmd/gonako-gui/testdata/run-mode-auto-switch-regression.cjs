'use strict';

// #457: ファイルやひな形を開いたとき、先頭の `### mode = ...` 宣言に合わせて
// 実行モードが自動で切り替わることを、製品 app.js の実行モード選択セクションを
// 切り出して DOM スタブで実行し確認する。
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const appPath = path.join(__dirname, '../ui/app.js');
const appSource = fs.readFileSync(appPath, 'utf8');
const startMarker = '  // --- 実行モード選択 ---';
const endMarker = '  // --- 汎用ダイアログ関数';
const start = appSource.indexOf(startMarker);
const end = appSource.indexOf(endMarker);

assert.notEqual(start, -1, '実行モード選択セクションが見つかりません');
assert.notEqual(end, -1, '汎用ダイアログ関数セクションが見つかりません');
assert.ok(start < end, '実行モード選択セクションの範囲が不正です');

let changeHandler;
const cmdSources = [];
const statuses = [];
let filterCalls = 0;

const selectAppType = {
  value: 'window',
  addEventListener(type, handler) {
    if (type === 'change') changeHandler = handler;
  },
  dispatchEvent(event) {
    if (event.type === 'change' && changeHandler) changeHandler(event);
  }
};

const context = vm.createContext({
  Event,
  console,
  selectAppType,
  modeBadge: { style: {}, textContent: '' },
  btnRunModeHelp: { addEventListener() {} },
  closeHamburger() {},
  setCmdSource(...args) {
    cmdSources.push(args);
  },
  applyTemplateFilters() {
    filterCalls++;
  },
  setStatus(message) {
    statuses.push(message);
  }
});

const section = appSource.slice(start, end);
vm.runInContext(`let templateModeFilterEnabled = true;\n${section}`, context);

function evaluate(expression) {
  return vm.runInContext(expression, context);
}

assert.deepEqual(
  Array.from(evaluate('parseDeclaredRunModes("### mode = wnako3")')),
  ['wnako3']
);
assert.deepEqual(
  Array.from(evaluate('parseDeclaredRunModes("### 説明文\\n### tags = GUI,サンプル\\n### mode = cli")')),
  ['cli']
);
assert.deepEqual(
  Array.from(evaluate('parseDeclaredRunModes("#!/usr/bin/env gonako\\n### mode = wnako3")')),
  ['wnako3']
);
assert.deepEqual(
  Array.from(evaluate('parseDeclaredRunModes("### mode = window,cli")')),
  ['window', 'cli']
);
assert.deepEqual(
  Array.from(evaluate('parseDeclaredRunModes("### mode = foo")')),
  []
);
assert.deepEqual(
  Array.from(evaluate('parseDeclaredRunModes("### mode = foo,cli")')),
  ['cli']
);
assert.deepEqual(
  Array.from(evaluate('parseDeclaredRunModes("表示=1\\n### mode = wnako3")')),
  []
);
assert.deepEqual(
  Array.from(evaluate('parseDeclaredRunModes("### mode = window\\n### mode = wnako3")')),
  ['wnako3']
);
assert.deepEqual(
  Array.from(evaluate('parseDeclaredRunModes("### 説明 = wnako3\\n### tags = GUI")')),
  []
);
assert.deepEqual(
  Array.from(evaluate('parseDeclaredRunModes("### mode = wnako3\\r\\n")')),
  ['wnako3']
);
assert.deepEqual(
  Array.from(evaluate('parseDeclaredRunModes("### mode=wnako3")')),
  ['wnako3']
);
assert.deepEqual(
  Array.from(evaluate('parseDeclaredRunModes("")')),
  []
);
assert.deepEqual(
  Array.from(evaluate('parseDeclaredRunModes("// コメント\\n### mode = cli")')),
  ['cli']
);
assert.deepEqual(
  Array.from(evaluate('parseDeclaredRunModes("### 説明\\n#!/bin/sh\\n### mode = cli")')),
  []
);
assert.deepEqual(
  Array.from(evaluate('parseDeclaredRunModes("### mode = wnako3\\n### mode = foo")')),
  []
);
// BOM付きでも宣言を読める（JSのtrimがBOMを取り除くため。Go側より寛容な点）。
assert.deepEqual(
  Array.from(evaluate('parseDeclaredRunModes("\\uFEFF### mode = wnako3")')),
  ['wnako3']
);

assert.equal(
  evaluate('resolveAutoRunMode(["wnako3"], "window")'),
  'wnako3'
);
assert.equal(
  evaluate('resolveAutoRunMode(["window", "cli"], "cli")'),
  ''
);
assert.equal(
  evaluate('resolveAutoRunMode(["window", "cli"], "wnako3")'),
  'window'
);
assert.equal(
  evaluate('resolveAutoRunMode([], "window")'),
  ''
);
assert.equal(
  evaluate('resolveAutoRunMode(["cli"], "cli")'),
  ''
);

selectAppType.value = 'window';
cmdSources.length = 0;
statuses.length = 0;
filterCalls = 0;
vm.runInContext('templateModeFilterEnabled = false;', context);

assert.equal(
  evaluate('applyAutoRunMode("### mode = wnako3")'),
  'wnako3'
);
assert.equal(selectAppType.value, 'wnako3');
assert.deepEqual(cmdSources.at(-1), ['wnako', false]);
assert.equal(evaluate('templateModeFilterEnabled'), true);
assert.equal(filterCalls, 1);

selectAppType.value = 'window';
assert.equal(
  evaluate('applyAutoRunMode("表示=1")'),
  ''
);
assert.equal(selectAppType.value, 'window');

selectAppType.value = 'cli';
assert.equal(
  evaluate('applyAutoRunMode("### mode = window,cli")'),
  ''
);
assert.equal(selectAppType.value, 'cli');

// openFile の配線も実物で確かめる。ファイルを開いたときにだけ切り替わり、
// バイナリでは切り替わらないこと。
const openFileStart = appSource.indexOf('  async function openFile(filePath, fileName) {');
const openFileEnd = appSource.indexOf('  async function saveFileAs()', openFileStart);
assert.notEqual(openFileStart, -1, 'openFileが見つかりません');
assert.notEqual(openFileEnd, -1, 'saveFileAsが見つかりません');

let readFileResult = null;
context.window = {
  readFile: async () => readFileResult,
};
context.editor = { value: '', readOnly: false, placeholder: '' };
context.activeFileName = { title: '' };
context.defaultEditorPlaceholder = '';
context.isBinaryFile = false;
context.savedContent = '';
context.currentFilePath = '';
context.currentFileDisplayName = '';
context.currentFileEncoding = 'UTF-8';
context.currentTemplateBaseName = '';
context.updateFileTitleDisplay = () => {};
context.updateLineNumbers = () => {};
context.updateCharCount = () => {};
context.updateCursorPos = () => {};
context.showAlertDialog = async () => {};

vm.runInContext(appSource.slice(openFileStart, openFileEnd), context);

(async () => {
  // wnako3宣言のあるファイルを開くと、実行モードが切り替わる。
  selectAppType.value = 'window';
  statuses.length = 0;
  readFileResult = {
    ok: true,
    content: '### mode = wnako3\n「テスト」と表示',
    encoding: 'UTF-8',
    converted: false,
  };
  await evaluate('openFile("a.nako3", "a.nako3")');
  assert.equal(selectAppType.value, 'wnako3');
  assert.equal(context.editor.value, readFileResult.content);
  assert.ok(
    statuses.at(-1).includes('開きました: a.nako3'),
    `ステータスに開いた旨がありません: ${statuses.at(-1)}`
  );
  assert.ok(
    statuses.at(-1).includes('実行モードを「ブラウザ(wnako3)」に切り替えました'),
    `ステータスに切替の注記がありません: ${statuses.at(-1)}`
  );

  // 宣言のないファイルでは切り替わらない。
  selectAppType.value = 'cli';
  statuses.length = 0;
  readFileResult = {
    ok: true,
    content: '「テスト」と表示',
    encoding: 'UTF-8',
    converted: false,
  };
  await evaluate('openFile("b.nako3", "b.nako3")');
  assert.equal(selectAppType.value, 'cli');
  assert.ok(!statuses.at(-1).includes('切り替えました'));

  // バイナリファイルでは、内容に宣言らしき文字列があっても切り替えない。
  selectAppType.value = 'window';
  statuses.length = 0;
  readFileResult = {
    ok: true,
    isBinary: true,
    content: '### mode = wnako3',
    encoding: 'UTF-8',
    converted: false,
  };
  await evaluate('openFile("c.png", "c.png")');
  assert.equal(selectAppType.value, 'window');
  assert.ok(statuses.at(-1).includes('バイナリファイルのため編集できません'));

  console.log('run-mode-auto-switch-regression: ok');
})().catch((err) => {
  console.error(err);
  process.exit(1);
});
