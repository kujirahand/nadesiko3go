/**
 * エディタのシンタックスハイライト部品（textarea + 背面オーバーレイ方式）
 *
 * textarea はそのまま入力・キャレット・IME・選択の担当として残し、
 * 文字色を透明にして、真後ろに色付けした <pre> を重ねて表示する。
 * 外部ライブラリに依存しないので、追加のバイナリサイズは JS 1本ぶんだけ。
 *
 * ■ 差し替え用インターフェース
 *   const h = window.GonakoHighlighter.create(textarea, options);
 *   h.refresh();          // textarea.value を反映（プログラムから値を変えた後に呼ぶ）
 *   h.setEnabled(bool);   // 色分けの ON/OFF
 *   h.destroy();          // DOMと購読を元に戻す
 * CodeMirror などに置き換える場合も、この3メソッドを持つオブジェクトを
 * 返す create() を用意すれば、app.js 側は変更しなくてよい。
 *
 * options:
 *   syntax    : tokenize(code) を持つオブジェクト（既定: window.NakoSyntax）
 *   maxLength : この文字数を超えたら色分けを止める（既定: 200000）
 */
(function (global) {
  'use strict';

  const HOST_CLASS = 'editor-host';
  const LAYER_CLASS = 'editor-highlight';
  const COMPOSING_CLASS = 'is-composing';

  function escapeHtml(s) {
    return s.replace(/[&<>]/g, (c) => (c === '&' ? '&amp;' : c === '<' ? '&lt;' : '&gt;'));
  }

  function render(syntax, code) {
    const tokens = syntax.tokenize(code);
    let html = '';
    for (let i = 0; i < tokens.length; i++) {
      const t = tokens[i];
      const text = escapeHtml(t.value);
      html += (t.type === 'plain') ? text : '<span class="tk-' + t.type + '">' + text + '</span>';
    }
    // 末尾の改行だけだと <pre> の最終行が潰れるので番兵を足す
    return html + '\n';
  }

  function create(textarea, options) {
    const opts = options || {};
    const syntax = opts.syntax || global.NakoSyntax;
    const maxLength = opts.maxLength || 200000;
    if (!textarea || !syntax || typeof syntax.tokenize !== 'function') {
      return null; // シンタックス定義がなければ何もしない（素の textarea のまま）
    }

    // textarea を色付けレイヤーと同じ箱に入れ、重ね合わせの基準にする
    const parent = textarea.parentNode;
    const host = document.createElement('div');
    host.className = HOST_CLASS;
    parent.insertBefore(host, textarea);

    const layer = document.createElement('pre');
    layer.className = LAYER_CLASS;
    layer.setAttribute('aria-hidden', 'true');
    host.appendChild(layer);
    host.appendChild(textarea);

    let enabled = true;
    let lastValue = null;
    let frame = 0;

    function syncScroll() {
      layer.scrollTop = textarea.scrollTop;
      layer.scrollLeft = textarea.scrollLeft;
    }

    function paint() {
      frame = 0;
      const value = textarea.value;
      // 大きすぎるファイルは色分けを諦めて、素の表示に戻す
      const active = enabled && value.length <= maxLength;
      host.classList.toggle('is-plain', !active);
      if (!active) { lastValue = null; layer.textContent = ''; return; }
      if (value === lastValue) { syncScroll(); return; }
      lastValue = value;
      layer.innerHTML = render(syntax, value);
      syncScroll();
    }

    function schedule() {
      if (frame) return;
      frame = global.requestAnimationFrame(paint);
    }

    // IME変換中は未確定文字が textarea 側にしか無いので、
    // 一時的に textarea の文字を見えるようにして色分けを止める
    function onCompositionStart() { host.classList.add(COMPOSING_CLASS); }
    function onCompositionEnd() { host.classList.remove(COMPOSING_CLASS); schedule(); }

    textarea.addEventListener('input', schedule);
    textarea.addEventListener('scroll', syncScroll);
    textarea.addEventListener('compositionstart', onCompositionStart);
    textarea.addEventListener('compositionend', onCompositionEnd);

    paint();

    return {
      refresh: schedule,
      setEnabled: function (value) {
        enabled = !!value;
        lastValue = null;
        schedule();
      },
      destroy: function () {
        if (frame) { global.cancelAnimationFrame(frame); frame = 0; }
        textarea.removeEventListener('input', schedule);
        textarea.removeEventListener('scroll', syncScroll);
        textarea.removeEventListener('compositionstart', onCompositionStart);
        textarea.removeEventListener('compositionend', onCompositionEnd);
        parent.insertBefore(textarea, host);
        parent.removeChild(host);
      },
    };
  }

  global.GonakoHighlighter = { create: create };
}(window));
