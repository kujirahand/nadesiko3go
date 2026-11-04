// Issue #353: text / html 操作で中身を置き換えたとき、旧子孫の要素キャッシュが
// 残らないことを、両画面の実物の apply 関数を切り出して実行し確かめる。
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');

function makeElement(handle, tag) {
  return {
    tagName: String(tag || 'div').toUpperCase(),
    dataset: { gonakoHandle: String(handle) },
    style: {},
    children: [],
    parent: null,
    innerHTML: '',
    textContent: '',
    classList: { add: () => {} },
    setAttribute() {},
    matches(sel) {
      return sel.split(',').some(part => part.trim() === this.tagName.toLowerCase());
    },
    appendChild(child) {
      child.parent = this;
      this.children.push(child);
    },
    // 実物と同じく、自身を含む子孫かどうか。
    contains(other) {
      for (let cur = other; cur; cur = cur.parent) if (cur === this) return true;
      return false;
    },
    querySelectorAll() {
      const out = [];
      const walk = el => el.children.forEach(child => { out.push(child); walk(child); });
      walk(this);
      return out;
    },
    // 中身の置き換え: 旧子孫をDOMから外す。
    replaceChildren() { this.children.forEach(c => { c.parent = null; }); this.children = []; },
  };
}

function sliceFunction(source, startMarker, endMarker, file) {
  const begin = source.indexOf(startMarker);
  const finish = source.indexOf(endMarker, begin);
  assert.ok(begin >= 0 && finish > begin, file + ': 関数の切り出し範囲');
  return source.slice(begin, finish);
}

for (const file of ['app.js', 'bundled/app.js']) {
  const source = fs.readFileSync(path.join(__dirname, '../ui', file), 'utf8');
  const isEditor = file === 'app.js';
  const applyName = isEditor ? 'applyGUIOperations' : 'apply';
  const applySrc = isEditor
    ? sliceFunction(source, 'function applyGUIOperations(', 'function selectOutputTab(', file)
    : sliceFunction(source, 'function apply(', 'function ask(', file);

  for (const opType of ['text', 'html']) {
    const cache = new Map();
    const screen = { dataset: {}, querySelector: () => null, appendChild: () => {} };
    const context = vm.createContext({
      document: { addEventListener: () => {}, createElement: tag => makeElement(0, tag) },
      window: {},
      root: screen, windowPreview: screen,
      guiElements: cache, elements: cache,
      send: () => {}, sendGUIEvent: () => {},
    });

    // 親(1) ─ 子(2) ─ 孫(3)。無関係な要素(4)は残るべき。
    const parent = makeElement(1);
    const child = makeElement(2);
    const grandchild = makeElement(3);
    const other = makeElement(4);
    parent.appendChild(child);
    child.appendChild(grandchild);
    [parent, child, grandchild, other].forEach((el, i) => cache.set(i + 1, el));

    const op = opType === 'text'
      ? { type: 'text', handle: 1, text: 'hello' }
      : { type: 'html', handle: 1, html: '<button data-gonako-handle="9">new</button>' };
    vm.runInContext(applySrc + `\n${applyName}([${JSON.stringify(op)}]);`, context);

    const label = `${file} ${opType}`;
    assert.ok(cache.has(1), label + ': 対象の親はキャッシュに残る');
    assert.ok(cache.has(4), label + ': 無関係な要素はキャッシュに残る');
    assert.ok(!cache.has(2), label + ': 子のキャッシュは消える');
    assert.ok(!cache.has(3), label + ': 孫のキャッシュも消える');
  }
}

console.log('dom-sync-regression: ok');
