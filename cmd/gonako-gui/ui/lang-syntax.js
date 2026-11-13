/**
 * Markdown / HTML / JavaScript 用の色分け定義（表示専用・DOMには触らない）
 *
 *   window.LangSyntax.forExtension(ext) -> { tokenize(code) } または null
 *
 * tokenize の戻り値は nako-syntax.js と同じ [{ type, value }, ...]。
 * type は既存の .tk-* クラスを流用する（comment / string / number / keyword /
 * func / const / operator / deffunc / word / plain）。
 * 連結すると必ず元の文字列に戻る（取りこぼしを作らない）。
 */
(function (global) {
  'use strict';

  function push(out, type, value) {
    if (value === '') return;
    const last = out[out.length - 1];
    if (last && last.type === type) { last.value += value; return; }
    out.push({ type: type, value: value });
  }

  // ---------- JavaScript ----------
  const JS_KEYWORDS = new Set((
    'break case catch class const continue debugger default delete do else export extends ' +
    'finally for function if import in instanceof let new return super switch this throw try ' +
    'typeof var void while with yield async await of static get set'
  ).split(' '));
  const JS_CONSTS = new Set('true false null undefined NaN Infinity'.split(' '));

  function tokenizeJS(code) {
    const out = [];
    let i = 0;
    const n = code.length;
    let prevSig = ''; // 直前の意味のあるトークン（正規表現リテラル判定用）
    while (i < n) {
      const c = code[i];
      const rest2 = code.substr(i, 2);
      if (rest2 === '//') {
        let j = code.indexOf('\n', i);
        if (j < 0) j = n;
        push(out, 'comment', code.slice(i, j)); i = j; continue;
      }
      if (rest2 === '/*') {
        let j = code.indexOf('*/', i + 2);
        j = j < 0 ? n : j + 2;
        push(out, 'comment', code.slice(i, j)); i = j; continue;
      }
      if (c === '"' || c === "'" || c === '`') {
        let j = i + 1;
        while (j < n && code[j] !== c) {
          if (code[j] === '\\') j++;
          else if (c !== '`' && code[j] === '\n') break;
          j++;
        }
        j = Math.min(j + 1, n);
        push(out, 'string', code.slice(i, j)); i = j; prevSig = 'v'; continue;
      }
      if (c === '/' && (prevSig === '' || prevSig === 'o')) {
        // 正規表現リテラル（演算子や括弧の直後だけ）
        let j = i + 1, inClass = false;
        while (j < n && code[j] !== '\n') {
          if (code[j] === '\\') j++;
          else if (code[j] === '[') inClass = true;
          else if (code[j] === ']') inClass = false;
          else if (code[j] === '/' && !inClass) break;
          j++;
        }
        if (j < n && code[j] === '/') {
          j++;
          while (j < n && /[a-z]/i.test(code[j])) j++;
          push(out, 'string', code.slice(i, j)); i = j; prevSig = 'v'; continue;
        }
      }
      let m;
      if (/[0-9]/.test(c) || (c === '.' && /[0-9]/.test(code[i + 1] || ''))) {
        m = /^(0[xX][0-9a-fA-F_]+|0[bB][01_]+|0[oO][0-7_]+|(\d[\d_]*\.?[\d_]*|\.\d[\d_]*)([eE][+-]?\d+)?)n?/.exec(code.slice(i));
        push(out, 'number', m[0]); i += m[0].length; prevSig = 'v'; continue;
      }
      if (/[A-Za-z_$]/.test(c)) {
        m = /^[A-Za-z_$][\w$]*/.exec(code.slice(i));
        const w = m[0];
        let type = 'word';
        if (JS_KEYWORDS.has(w)) { type = 'keyword'; prevSig = 'o'; }
        else if (JS_CONSTS.has(w)) { type = 'const'; prevSig = 'v'; }
        else {
          prevSig = 'v';
          let k = i + w.length;
          while (k < n && (code[k] === ' ' || code[k] === '\t')) k++;
          if (code[k] === '(') type = 'func';
        }
        push(out, type, w); i += w.length; continue;
      }
      if (/\s/.test(c)) { push(out, 'plain', c); i++; continue; }
      if (c === ')' || c === ']' || c === '}') prevSig = 'v';
      else prevSig = 'o';
      push(out, /[+\-*/%=<>!&|^~?:]/.test(c) ? 'operator' : 'plain', c);
      i++;
    }
    return out;
  }

  // ---------- CSS（<style> 内用の簡易版） ----------
  function tokenizeCSS(code) {
    const out = [];
    const re = /\/\*[\s\S]*?(\*\/|$)|"(?:\\.|[^"\\\n])*"?|'(?:\\.|[^'\\\n])*'?|#[0-9a-fA-F]{3,8}\b|-?\d[\d.]*(?:px|em|rem|%|vh|vw|s|ms|deg)?|[A-Za-z-]+|@[\w-]+|[\s\S]/g;
    let m;
    while ((m = re.exec(code)) !== null) {
      const s = m[0];
      let type = 'plain';
      if (s.startsWith('/*')) type = 'comment';
      else if (s[0] === '"' || s[0] === "'") type = 'string';
      else if (s[0] === '#' && s.length > 1) type = 'const';
      else if (/^-?\d/.test(s)) type = 'number';
      else if (s[0] === '@') type = 'keyword';
      else if (/^[A-Za-z-]+$/.test(s) && /^\s*:/.test(code.slice(re.lastIndex))) type = 'func';
      push(out, type, s);
    }
    return out;
  }

  // ---------- HTML ----------
  function tokenizeHTML(code) {
    const out = [];
    let i = 0;
    const n = code.length;
    while (i < n) {
      if (code.startsWith('<!--', i)) {
        let j = code.indexOf('-->', i + 4);
        j = j < 0 ? n : j + 3;
        push(out, 'comment', code.slice(i, j)); i = j; continue;
      }
      if (code[i] === '<' && /[A-Za-z!/?]/.test(code[i + 1] || '')) {
        // タグ全体を読む（属性値内の > は無視）
        const head = /^<[/!?]?[A-Za-z0-9:-]*/.exec(code.slice(i))[0];
        push(out, 'keyword', head);
        i += head.length;
        const tagName = head.replace(/^<\/?/, '').toLowerCase();
        const isOpen = head[1] !== '/';
        while (i < n && code[i] !== '>') {
          const c = code[i];
          if (c === '"' || c === "'") {
            let j = code.indexOf(c, i + 1);
            j = j < 0 ? n : j + 1;
            push(out, 'string', code.slice(i, j)); i = j;
          } else if (/[^\s=/>"']/.test(c)) {
            const w = /^[^\s=/>"']+/.exec(code.slice(i))[0];
            push(out, 'func', w); i += w.length;
          } else {
            push(out, c === '=' ? 'operator' : 'plain', c); i++;
          }
        }
        if (i < n) {
          const selfClose = code[i - 1] === '/';
          push(out, 'keyword', '>'); i++;
          if (isOpen && !selfClose && (tagName === 'script' || tagName === 'style')) {
            const re = new RegExp('</' + tagName + '\\b', 'i');
            const m = re.exec(code.slice(i));
            const end = m ? i + m.index : n;
            const inner = code.slice(i, end);
            const toks = tagName === 'script' ? tokenizeJS(inner) : tokenizeCSS(inner);
            for (const t of toks) push(out, t.type, t.value);
            i = end;
          }
        }
        continue;
      }
      if (code[i] === '&') {
        const m = /^&(#x?[0-9a-fA-F]+|[A-Za-z][A-Za-z0-9]*);/.exec(code.slice(i, i + 12));
        if (m) { push(out, 'const', m[0]); i += m[0].length; continue; }
      }
      push(out, 'plain', code[i]); i++;
    }
    return out;
  }

  // ---------- Markdown ----------
  function tokenizeInlineMD(line, out) {
    // 閉じ記号が無い場合に各位置から行末まで再走査して二次時間になるのを避けるため、
    // 範囲指定はすべて上限付き({0,N})にしている
    const re = /`[^`\n]*`?|\*\*[^*\n]{1,300}\*\*|__[^_\n]{1,300}__|\*[^*\s][^*\n]{0,300}\*|!?\[[^\]\n]{0,300}\]\([^)\n]{0,300}\)|<https?:[^>\n]{0,300}>|https?:\/\/[^\s)]+|<\/?[A-Za-z][^>\n]{0,300}>|[\s\S]/g;
    let m;
    while ((m = re.exec(line)) !== null) {
      const s = m[0];
      let type = 'plain';
      if (s[0] === '`') type = 'string';
      else if (s.startsWith('**') || s.startsWith('__')) type = 'const';
      else if (s[0] === '*' && s.length > 1) type = 'const';
      else if (s[0] === '[' || s.startsWith('![') || s[0] === '<' && s.length > 1 || s.startsWith('http')) type = 'func';
      push(out, type, s);
    }
  }

  function tokenizeMarkdown(code) {
    const out = [];
    const lines = code.split('\n');
    let fence = null; // 開いているコードフェンスの記号
    for (let li = 0; li < lines.length; li++) {
      const line = lines[li];
      const nl = li < lines.length - 1 ? '\n' : '';
      let m;
      if (fence) {
        push(out, 'string', line + nl);
        if (new RegExp('^\\s{0,3}' + fence[0] + '{' + fence.length + ',}\\s*$').test(line)) fence = null;
        continue;
      }
      if ((m = /^\s{0,3}(`{3,}|~{3,})/.exec(line))) {
        fence = m[1];
        push(out, 'string', line + nl);
        continue;
      }
      if ((m = /^(\s{0,3}#{1,6})(\s.*|)$/.exec(line))) {
        push(out, 'keyword', line + nl); continue;
      }
      if (/^\s{0,3}([-*_])(\s*\1){2,}\s*$/.test(line) || /^\s*(=+|-+)\s*$/.test(line) && li > 0) {
        push(out, 'operator', line + nl); continue;
      }
      if ((m = /^(\s{0,3}>\s?)(.*)$/.exec(line))) {
        push(out, 'comment', line + nl); continue;
      }
      if ((m = /^(\s*)([-*+]|\d+[.)])(\s+)(\[[ xX]\]\s+)?/.exec(line))) {
        push(out, 'plain', m[1]);
        push(out, 'operator', m[2]);
        push(out, 'plain', m[3]);
        if (m[4]) push(out, 'const', m[4]);
        tokenizeInlineMD(line.slice(m[0].length), out);
        push(out, 'plain', nl);
        continue;
      }
      if (/^\s*\|.*\|\s*$/.test(line)) {
        // 表: 区切り記号だけ色を付ける
        const parts = line.split('|');
        parts.forEach((p, k) => {
          if (k > 0) push(out, 'operator', '|');
          tokenizeInlineMD(p, out);
        });
        push(out, 'plain', nl);
        continue;
      }
      tokenizeInlineMD(line, out);
      push(out, 'plain', nl);
    }
    return out;
  }

  const SYNTAXES = {
    md: { tokenize: tokenizeMarkdown },
    html: { tokenize: tokenizeHTML },
    js: { tokenize: tokenizeJS },
    css: { tokenize: tokenizeCSS },
  };
  const ALIASES = {
    markdown: 'md', mdown: 'md', htm: 'html', xhtml: 'html',
    mjs: 'js', cjs: 'js', jsx: 'js',
  };

  function forExtension(ext) {
    let e = String(ext || '').toLowerCase().replace(/^\./, '');
    e = ALIASES[e] || e;
    return SYNTAXES[e] || null;
  }

  global.LangSyntax = { forExtension: forExtension };
}(window));
