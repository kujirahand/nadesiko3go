// Issue #355: 両画面の値収集が checkbox・radio の checked を送ることと、
// Go側からの checked 操作が選択状態へ反映されることを、実物の関数を
// 切り出して実行し確かめる。
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');

function makeElement(type, value, checked, handle) {
  return {
    type, value, checked,
    dataset: { gonakoHandle: String(handle) },
    style: {},
    classList: { add: () => {} },
    setAttribute(name, v) { if (name === 'type') this.type = v; },
    matches(sel) {
      return sel.split(',').some(part => {
        const p = part.trim();
        if (p === 'input' || p === 'textarea' || p === 'select') return p === 'input';
        const m = p.match(/^input\[type="(\w+)"\]$/);
        return m ? this.type === m[1] : false;
      });
    },
  };
}

function sampleElements() {
  return [
    makeElement('checkbox', 'on', false, 8),
    makeElement('checkbox', 'on', true, 9),
    makeElement('radio', 'A', false, 10),
    makeElement('radio', 'B', true, 11),
    makeElement('text', 'hello', false, 12),
  ];
}

function checkCollected(values, file) {
  assert.equal(values['8'].value, 'on', file);
  assert.equal(values['8'].checked, false, file + ': 未選択のcheckboxはchecked:falseを送る');
  assert.equal(values['9'].checked, true, file);
  assert.equal(values['10'].checked, false, file);
  assert.equal(values['11'].checked, true, file + ': radioは選択中のものだけchecked:true');
  assert.equal(values['11'].value, 'B', file);
  assert.equal(values['12'].value, 'hello', file);
  assert.ok(!('checked' in values['12']), file + ': textにはcheckedキーを付けない');
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
  const screen = {
    dataset: {},
    querySelectorAll: () => sampleElements(),
    querySelector: () => null,
    appendChild: () => {},
  };
  const created = new Map();
  const context = vm.createContext({
    document: {
      addEventListener: () => {},
      createElement: tag => {
        const el = makeElement(tag === 'input' ? 'text' : '', '', false, 0);
        el.tagName = String(tag).toUpperCase();
        return el;
      },
    },
    window: {},
    root: screen, windowPreview: screen,
    guiElements: created, elements: created,
    send: () => {}, sendGUIEvent: () => {},
  });

  // 値の収集: エディタは collectGUIValues、梱包は values。
  const collectName = isEditor ? 'collectGUIValues' : 'values';
  const collectSrc = isEditor
    ? sliceFunction(source, 'function collectGUIValues()', 'function appendGUIOutput(', file)
    : sliceFunction(source, 'function values()', 'async function send(', file);
  vm.runInContext(collectSrc + `\n__collected = ${collectName}();`, context);
  checkCollected(context.__collected, file);

  // checked 操作の反映と、作成時の初期選択。
  const applyName = isEditor ? 'applyGUIOperations' : 'apply';
  const applySrc = isEditor
    ? sliceFunction(source, 'function applyGUIOperations(', 'function selectOutputTab(', file)
    : sliceFunction(source, 'function apply(', 'function ask(', file);
  const target = makeElement('checkbox', 'on', true, 5);
  created.set(5, target);
  vm.runInContext(applySrc + `\n${applyName}([{ type: 'checked', handle: 5, checked: false }]);`, context);
  assert.equal(target.checked, false, file + ': checked操作で選択が外れる');

  vm.runInContext(
    applySrc + `\n${applyName}([{ type: 'create', tag: 'input', handle: 6, attributes: { type: 'checkbox' }, checked: true }]);`,
    context);
  const newEl = created.get(6);
  assert.ok(newEl, file + ': 作成操作で要素が登録される');
  assert.equal(newEl.checked, true, file + ': 作成時のcheckedが反映される');
}

console.log('gui-values-regression: ok');
