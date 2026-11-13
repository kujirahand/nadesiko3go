// 検索・置換ダイアログ。エディタ(textarea)の内容を直接検索・置換する。
// 置換後は 'input' イベントを流し、行番号・色分け・変更状態を app.js 側に更新させる。
(function () {
  'use strict';
  const editor = document.getElementById('editor');
  const panel = document.getElementById('search-panel');
  if (!editor || !panel) return;

  const $ = (id) => document.getElementById(id);
  const findInput = $('search-find');
  const replaceInput = $('search-replace');
  const optCase = $('search-case');
  const optRegex = $('search-regex');
  const summary = $('search-summary');
  const list = $('search-results');
  const MAX_RESULTS = 500;

  let matches = []; // {start, end, line, lineText, lineStart}
  let current = -1;

  function isOpen() { return panel.style.display !== 'none'; }

  // 検索パターンを作る。不正な正規表現は null を返す。
  function buildRegex() {
    const q = findInput.value;
    if (!q) return null;
    const src = optRegex.checked ? q : q.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    try {
      return new RegExp(src, optCase.checked ? 'gu' : 'giu');
    } catch (e) {
      return new RegExp(src, optCase.checked ? 'g' : 'gi');
    }
  }

  function collect() {
    matches = [];
    current = -1;
    summary.textContent = '';
    summary.style.color = '';
    const text = editor.value;
    let re;
    try {
      re = buildRegex();
    } catch (e) {
      summary.textContent = '正規表現が正しくありません';
      summary.style.color = 'var(--accent-pink)';
      render();
      return;
    }
    if (!re) { render(); return; }
    let m;
    let truncated = false;
    while ((m = re.exec(text)) !== null) {
      if (m[0] === '') { re.lastIndex++; continue; } // 空一致は無視
      if (matches.length >= MAX_RESULTS) { truncated = true; break; }
      matches.push({ start: m.index, end: m.index + m[0].length });
    }
    // 行情報を付与 (開始位置は昇順なので1回の走査で足りる)
    let line = 1, pos = 0;
    for (const mt of matches) {
      let idx;
      while ((idx = text.indexOf('\n', pos)) !== -1 && idx < mt.start) { line++; pos = idx + 1; }
      const lineEnd = text.indexOf('\n', mt.start);
      mt.line = line;
      mt.lineStart = pos;
      mt.lineText = text.slice(pos, lineEnd === -1 ? text.length : lineEnd);
    }
    summary.textContent = matches.length === 0 ? '見つかりません'
      : `${matches.length}${truncated ? '件以上' : '件'} 見つかりました`;
    render();
  }

  function render() {
    list.textContent = '';
    matches.forEach((mt, i) => {
      const li = document.createElement('li');
      li.dataset.index = String(i);
      const ln = document.createElement('span');
      ln.className = 'sr-line';
      ln.textContent = `${mt.line}行`;
      const tx = document.createElement('span');
      tx.className = 'sr-text';
      const s = mt.start - mt.lineStart;
      const e = Math.min(mt.end - mt.lineStart, mt.lineText.length);
      tx.appendChild(document.createTextNode(mt.lineText.slice(0, s)));
      const mk = document.createElement('mark');
      mk.textContent = mt.lineText.slice(s, e);
      tx.appendChild(mk);
      tx.appendChild(document.createTextNode(mt.lineText.slice(e)));
      li.append(ln, tx);
      list.appendChild(li);
    });
  }

  function scrollToOffset(offset) {
    const before = editor.value.slice(0, offset);
    const row = before.split('\n').length - 1;
    const lh = parseFloat(getComputedStyle(editor).lineHeight) || 20;
    const top = row * lh;
    if (top < editor.scrollTop || top > editor.scrollTop + editor.clientHeight - lh * 2) {
      editor.scrollTop = Math.max(0, top - editor.clientHeight / 3);
    }
  }

  function select(i, focusEditor) {
    if (i < 0 || i >= matches.length) return;
    current = i;
    const mt = matches[i];
    editor.setSelectionRange(mt.start, mt.end);
    scrollToOffset(mt.start);
    if (focusEditor) editor.focus();
    editor.dispatchEvent(new Event('keyup')); // カーソル位置表示の更新
    list.querySelectorAll('li.current').forEach((el) => el.classList.remove('current'));
    const li = list.children[i];
    if (li) { li.classList.add('current'); li.scrollIntoView({ block: 'nearest' }); }
  }

  // カーソル位置以降で最初の一致 (見つからなければ先頭へ戻る)
  function step(dir) {
    if (matches.length === 0) collect();
    if (matches.length === 0) return;
    let i;
    if (dir > 0) {
      const pos = current >= 0 ? matches[current].end : editor.selectionEnd;
      i = matches.findIndex((m) => m.start >= pos);
      if (i < 0) i = 0;
    } else {
      const pos = current >= 0 ? matches[current].start : editor.selectionStart;
      i = -1;
      for (let k = matches.length - 1; k >= 0; k--) if (matches[k].end <= pos) { i = k; break; }
      if (i < 0) i = matches.length - 1;
    }
    select(i, false);
  }

  function changed() {
    editor.dispatchEvent(new Event('input', { bubbles: true }));
  }

  // 置換文字列。正規表現時は $1 などの参照を展開する。
  function expand(matchText) {
    const rep = replaceInput.value;
    if (!optRegex.checked) return rep;
    const re = buildRegex();
    return matchText.replace(new RegExp(re.source, re.flags.replace('g', '')), rep);
  }

  function replaceOne() {
    if (editor.readOnly) return;
    // 現在の選択が一致していればそれを置換、そうでなければ次の一致へ移動するだけ
    const s = editor.selectionStart, e = editor.selectionEnd;
    const sel = editor.value.slice(s, e);
    const re = buildRegex();
    if (re && sel) {
      const m = new RegExp(re.source, re.flags.replace('g', ''));
      const r = m.exec(sel);
      if (r && r[0] === sel) {
        const text = expand(sel);
        editor.setRangeText(text, s, e, 'end');
        changed();
        collect();
        step(1);
        return;
      }
    }
    collect();
    step(1);
  }

  function replaceAll() {
    if (editor.readOnly) return;
    collect();
    if (matches.length === 0) return;
    const n = matches.length;
    const text = editor.value;
    let out = '', pos = 0;
    for (const mt of matches) {
      out += text.slice(pos, mt.start) + expand(text.slice(mt.start, mt.end));
      pos = mt.end;
    }
    out += text.slice(pos);
    editor.value = out;
    changed();
    collect();
    summary.textContent = `${n}件 置換しました`;
  }

  function open() {
    panel.style.display = '';
    const sel = editor.value.slice(editor.selectionStart, editor.selectionEnd);
    if (sel && !sel.includes('\n')) findInput.value = sel;
    collect();
    findInput.focus();
    findInput.select();
  }

  function close() {
    panel.style.display = 'none';
    editor.focus();
  }

  $('btn-search').addEventListener('click', () => (isOpen() ? close() : open()));
  $('search-close').addEventListener('click', close);
  $('search-next').addEventListener('click', () => step(1));
  $('search-prev').addEventListener('click', () => step(-1));
  $('search-replace-one').addEventListener('click', replaceOne);
  $('search-replace-all').addEventListener('click', replaceAll);
  [findInput, optCase, optRegex].forEach((el) => el.addEventListener('input', collect));
  [optCase, optRegex].forEach((el) => el.addEventListener('change', collect));

  list.addEventListener('click', (ev) => {
    const li = ev.target.closest('li');
    if (li) select(Number(li.dataset.index), true);
  });

  panel.addEventListener('keydown', (ev) => {
    if (ev.key === 'Enter' && (ev.target === findInput || ev.target === replaceInput)) {
      ev.preventDefault();
      if (ev.target === replaceInput) replaceOne();
      else step(ev.shiftKey ? -1 : 1);
    } else if (ev.key === 'Escape') {
      ev.preventDefault();
      ev.stopPropagation();
      close();
    }
  });

  // 編集されたら一致位置が古くなるので再検索する
  editor.addEventListener('input', () => { if (isOpen()) collect(); });

  // Ctrl+F / Cmd+F で開く。app.js のハンドラより先に処理する。
  window.addEventListener('keydown', (ev) => {
    if ((ev.ctrlKey || ev.metaKey) && !ev.shiftKey && !ev.altKey && (ev.key === 'f' || ev.key === 'F')) {
      ev.preventDefault();
      open();
    }
  }, true);
})();
