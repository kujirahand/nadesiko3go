// Issue #353: text / html 操作で中身を置き換えたとき、旧子孫の要素キャッシュが
// 残らないことを、両画面の実物の apply 関数を切り出して実行し確かめる。
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');

function makeElement(handle, tag) {
  const el = {
    tagName: String(tag || 'div').toUpperCase(),
    dataset: { gonakoHandle: String(handle) },
    style: {},
    children: [],
    parent: null,
    _innerHTML: '',
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
  // innerHTML への代入は、実物と同じく旧子孫を外し、data-gonako-handle を
  // 持つ新しい要素を子として生成する（後続の操作で検索できるように）。
  Object.defineProperty(el, 'innerHTML', {
    get() { return this._innerHTML; },
    set(v) {
      this._innerHTML = v || '';
      this.replaceChildren();
      const re = /<(\w+)[^>]*data-gonako-handle="(\d+)"/g;
      let m;
      while ((m = re.exec(this._innerHTML))) {
        this.appendChild(makeElement(Number(m[2]), m[1]));
      }
    },
  });
  return el;
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
    // 画面のルート。querySelector は実物と同じく、現在のツリー内から
    // data-gonako-handle で要素を探す（ツリーから外れた要素は見つからない）。
    const screen = {
      dataset: {},
      children: [],
      appendChild(child) { child.parent = this; this.children.push(child); },
      querySelector(sel) {
        const m = /data-gonako-handle="(\d+)"/.exec(sel);
        if (!m) return null;
        let found = null;
        const walk = el => el.children.forEach(c => {
          if (found) return;
          if (c.dataset.gonakoHandle === m[1]) { found = c; return; }
          walk(c);
        });
        walk(this);
        return found;
      },
    };
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
    screen.appendChild(parent);
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

    if (opType === 'html') {
      // 置換で生まれた要素(9)が、次の操作で検索できて操作できること。
      assert.ok(!cache.has(9), label + ': 新要素はまだキャッシュに無い');
      vm.runInContext(
        applySrc + `\n${applyName}([{ type: 'text', handle: 9, text: 'new2' }]);`, context);
      const created = cache.get(9);
      assert.ok(created, label + ': 新要素が検索されてキャッシュに入る');
      assert.equal(created.textContent, 'new2', label + ': 新要素へtext操作が届く');
      // 消えた旧要素(2)への操作は、ツリーに無いので届かないこと。
      vm.runInContext(
        applySrc + `\n${applyName}([{ type: 'text', handle: 2, text: 'ghost' }]);`, context);
      assert.ok(!cache.has(2), label + ': 旧要素は再びキャッシュに入らない');
      assert.notEqual(child.textContent, 'ghost', label + ': 旧要素へ操作が届かない');
    }
  }
}

console.log('dom-sync-regression: ok');
