/**
 * なでしこ3 のシンタックス定義（表示用トークナイザ）
 *
 * internal/lexer (nako_lex_rules.mts 相当) の規則を、色分け表示に必要な範囲だけ
 * 簡略化して移植したもの。DOMには一切触らないので、エディタ部品を
 * CodeMirror などに差し替えても、この定義はそのまま使い回せる。
 *
 * 公開API:
 *   window.NakoSyntax.tokenize(code) -> [{ type, value }, ...]
 *   window.NakoSyntax.setCommands(list)   // command-list.json の配列を渡す
 *
 * トークン種別 (type):
 *   comment / string / embed / number / keyword / josi /
 *   func / const / word / operator / deffunc / plain
 */
(function (global) {
  'use strict';

  // --- 予約語 (internal/lexer/replace.go の reservedWords 相当) ---
  const RESERVED = new Set([
    'もし', '回', '回繰返', '間', '間繰返', '繰返', '増繰返', '減繰返', '後判定',
    '反復', '抜', '続', '戻', '先', '次', '代入', '実行速度優先',
    'パフォーマンスモニタ適用', '定', '逐次実行', '条件分岐', '増', '減',
    '変数', '定数', 'エラー監視', 'エラー', '関数', 'インデント構文',
    '非同期モード', 'DNCLモード', 'DNCL2モード', 'モード設定', '取込',
    'モジュール公開既定値', '厳チェック',
  ]);

  // 「それ」「そう」は予約語だが値を持つ特別な変数なので const 扱いで色を分ける
  const SPECIAL_VARS = new Set(['それ', 'そう']);

  // --- 助詞 (internal/lexer/josi/josi.go 相当) ---
  const JOSI_BASE = [
    'について', 'くらい', 'なのか', 'までを', 'までの', 'による', 'として',
    'とは', 'から', 'まで', 'だけ', 'より', 'ほど', 'など',
    'いて', 'えて', 'きて', 'けて', 'して', 'って', 'にて', 'みて',
    'めて', 'ねて', 'では', 'には', 'んで', 'ずつ',
    'は', 'を', 'に', 'へ', 'で', 'と', 'が', 'の',
  ];
  const JOSI_TARAREBA = ['でなければ', 'なければ', 'ならば', 'なら', 'たら', 'れば'];
  const JOSI_REMOVABLE = ['こと', 'である', 'です', 'します', 'でした', 'にゃん'];

  const TARAREBA = new Set(JOSI_TARAREBA);

  // 「もの」付きと素の助詞を交互に並べ、文字数の長い順に安定ソートする (#1614)
  const JOSI_RE = (function () {
    const list = JOSI_BASE.concat(JOSI_TARAREBA, JOSI_REMOVABLE);
    const mono = [];
    list.forEach((j) => { mono.push('もの' + j, j); });
    const indexed = mono.map((j, i) => ({ j, i }));
    indexed.sort((a, b) => (Array.from(b.j).length - Array.from(a.j).length) || (a.i - b.i));
    return new RegExp('^[\\t 　]*(?:' + indexed.map((x) => x.j).join('|') + ')');
  }());

  // --- 字句規則 ---
  const RE_SPACE = /^[ \t　・｜]+/;
  const RE_EOL = /^[\n;。]+/;
  const RE_LINE_COMMENT = /^(?:#|\/\/)[^\n]*/;
  const RE_RANGE_COMMENT = /^\/\*[\s\S]*?(?:\*\/|$)/;
  const RE_PRAGMA_COMMENT = /^(?:!|💡)(?:インデント構文|ここまでだるい|DNCLモード|DNCL2モード|DNCL2)[^\n]*/;
  const RE_DEF_FUNC = /^(?:●テスト:|●)/;
  const RE_NUMBER = /^(?:0[xX][0-9a-fA-F]+(?:_[0-9a-fA-F]+)*|0[oO][0-7]+(?:_[0-7]+)*|0[bB][01]+(?:_[01]+)*|\d+(?:_\d+)*\.(?:\d+(?:_\d+)*)?(?:[eE][+\-]?\d+(?:_\d+)*)?|\.\d+(?:_\d+)*(?:[eE][+\-]?\d+(?:_\d+)*)?|\d+(?:_\d+)*(?:[eE][+\-]?\d+(?:_\d+)*)?)n?/;
  const RE_BLOCK_KEYWORD = /^(?:;;;|ここまで|💧|ここから,?|もしも?|違えば|違)/;
  const RE_SPECIAL_WORD = /^(?:\$\{.+?\}|《.+?》)/;
  const RE_OPERATOR = /^(?:>>>|>>|<<|===|!==|≧|>=|=>|≦|<=|=<|≠|<>|!=|←|<--|==|🟰🟰|=|🟰|>|<|かつ|&&|または|或いは|あるいは|\|\||××|\*\*|×|\*|÷÷|÷|\/|%|\^|&|\+|-|@|\?\?|!|💡|…|\.{2,3}|\{関数\}|[\[\]()｛｝{}|:,、.$])/;

  // 単語を構成する文字 (rules.go の kanakanjiRE 相当)
  const RE_KANAKANJI = /^[\u3005\u4E00-\u9FCF_a-zA-Z0-9\u30A1-\u30F6\u30FC\u2460-\u24FF\u2776-\u277F\u3251-\u32BF]+/;
  const RE_HIRAGANA_HEAD = /^[\u3041-\u3093]/;
  const RE_WORD_SPECIAL = /^(?:かつ|または)/;
  const RE_HIRA_KAN_TAIL = /[ぁ-ん]間$/;
  const RE_IJO_IKA = /^.+(以上|以下|超|未満)$/;

  // 文字列リテラル。値は [開始, 終了, 展開あり?]
  const STRING_DELIMS = [
    ['🌿', '🌿', false],
    ['🌴', '🌴', true],
    ['「', '」', true],
    ['『', '』', false],
    ['“', '”', true],
    ['"', '"', true],
    ["'", "'", false],
  ];

  // --- 命令名（command-list.json から注入する） ---
  let funcNames = new Set();
  let constNames = new Set();

  /** 送り仮名を落とす (rules.go の TrimOkurigana 相当) */
  function trimOkurigana(s) {
    if (!RE_HIRAGANA_HEAD.test(s)) return s.replace(/[ぁ-ん]+/g, '');
    if (/^[ぁ-ん]+$/.test(s)) return s;
    return s.replace(/[ぁ-ん]+$/, '');
  }

  /**
   * 命令一覧を登録する。command-list.json の要素 ({name, type}) の配列を渡す。
   * 登録しなくても動作するが、命令名の色分けは行われない。
   */
  function setCommands(list) {
    funcNames = new Set();
    constNames = new Set();
    if (!Array.isArray(list)) return;
    list.forEach((c) => {
      if (!c || !c.name) return;
      const target = (c.type === 'const' || c.type === 'var') ? constNames : funcNames;
      target.add(c.name);
      target.add(trimOkurigana(c.name));
    });
  }

  /** 語句の種別を決める */
  function classifyWord(word) {
    const trimmed = trimOkurigana(word);
    if (RESERVED.has(word) || RESERVED.has(trimmed)) return 'keyword';
    if (SPECIAL_VARS.has(word)) return 'const';
    if (constNames.has(word) || constNames.has(trimmed)) return 'const';
    if (funcNames.has(word) || funcNames.has(trimmed)) return 'func';
    return 'word';
  }

  /**
   * 語句と直後の助詞を読む (rules.go の cbWordParser 相当)。
   * @returns {{word:string, josi:string}} 読めなければ word も josi も空
   */
  function readWord(src) {
    let word = '';
    let josi = '';
    let s = src;
    while (s.length > 0) {
      if (word !== '') {
        if (RE_WORD_SPECIAL.test(s)) break; // 「かつ」「または」で区切る (#1379)
        const m = JOSI_RE.exec(s);
        if (m) {
          josi = m[0];
          s = s.slice(josi.length);
          if (s.charAt(0) === ',') { josi += ','; } // 助詞直後の「,」を飛ばす (#877)
          break;
        }
      }
      const km = RE_KANAKANJI.exec(s);
      if (km) { word += km[0]; s = s.slice(km[0].length); continue; }
      if (RE_HIRAGANA_HEAD.test(s)) { word += s.charAt(0); s = s.slice(1); continue; }
      break;
    }
    // 「等しい間」のように、ひらがな＋「間」で終わるなら「間」を次のトークンに回す (#831)
    if (RE_HIRA_KAN_TAIL.test(word)) {
      return { word: word.slice(0, -1), josi: '' };
    }
    // 「以上」「以下」「超」「未満」は独立したトークンにする (#918)
    const ii = RE_IJO_IKA.exec(word);
    if (ii) {
      return { word: word.slice(0, word.length - ii[1].length), josi: '' };
    }
    return { word: word, josi: josi };
  }

  /**
   * トークンの直後に続く助詞を読んで out に積む (rules.go の readJosi:true 相当)。
   * 数値・文字列・閉じ括弧の後ろは、語句と同じように助詞が続きうる。
   * @returns {number} 消費した文字数
   */
  function pushJosi(out, s) {
    const m = JOSI_RE.exec(s);
    if (!m) return 0;
    let josi = m[0];
    if (s.charAt(josi.length) === ',') { josi += ','; } // 助詞直後の「,」を飛ばす (#877)
    const bare = josi.replace(/^[\t 　]+/, '').replace(/,$/, '');
    // 「ならば」などの条件助詞は制御構文の一部なのでキーワード色にする
    out.push({ type: TARAREBA.has(bare) ? 'keyword' : 'josi', value: josi });
    return josi.length;
  }

  /** 文字列リテラルを読む。閉じ記号がなければ行末ではなく末尾まで含める */
  function readString(src, delim) {
    const open = delim[0];
    const close = delim[1];
    const body = src.slice(open.length);
    const idx = body.indexOf(close);
    if (idx < 0) return { text: src, closed: false };
    return { text: open + body.slice(0, idx) + close, closed: true };
  }

  /** 文字列展開「{変数}」を別トークンに分ける */
  function pushString(out, text, expandable) {
    if (!expandable || text.indexOf('{') < 0) {
      out.push({ type: 'string', value: text });
      return;
    }
    const re = /\{[^{}\n]*\}/g;
    let last = 0;
    let m;
    while ((m = re.exec(text)) !== null) {
      if (m.index > last) out.push({ type: 'string', value: text.slice(last, m.index) });
      out.push({ type: 'embed', value: m[0] });
      last = m.index + m[0].length;
    }
    if (last < text.length) out.push({ type: 'string', value: text.slice(last) });
  }

  /**
   * ソースをトークン列に分解する。
   * 解析に失敗する文字も必ず plain トークンとして返すので、
   * 連結すると必ず元のソースに戻る（表示が欠けない）。
   */
  function tokenize(code) {
    const out = [];
    let s = code;
    let m;
    while (s.length > 0) {
      if ((m = RE_EOL.exec(s)) || (m = RE_SPACE.exec(s))) {
        out.push({ type: 'plain', value: m[0] });
        s = s.slice(m[0].length);
        continue;
      }
      if ((m = RE_PRAGMA_COMMENT.exec(s)) || (m = RE_LINE_COMMENT.exec(s)) || (m = RE_RANGE_COMMENT.exec(s))) {
        out.push({ type: 'comment', value: m[0] });
        s = s.slice(m[0].length);
        continue;
      }
      const delim = STRING_DELIMS.find((d) => s.startsWith(d[0]));
      if (delim) {
        const str = readString(s, delim);
        pushString(out, str.text, delim[2]);
        s = s.slice(str.text.length);
        s = s.slice(pushJosi(out, s));
        continue;
      }
      if ((m = RE_DEF_FUNC.exec(s))) {
        out.push({ type: 'deffunc', value: m[0] });
        s = s.slice(m[0].length);
        continue;
      }
      if ((m = RE_BLOCK_KEYWORD.exec(s))) {
        out.push({ type: 'keyword', value: m[0] });
        s = s.slice(m[0].length);
        continue;
      }
      if ((m = RE_NUMBER.exec(s))) {
        out.push({ type: 'number', value: m[0] });
        s = s.slice(m[0].length);
        s = s.slice(pushJosi(out, s));
        continue;
      }
      if ((m = RE_SPECIAL_WORD.exec(s))) {
        out.push({ type: 'word', value: m[0] });
        s = s.slice(m[0].length);
        continue;
      }
      if ((m = RE_OPERATOR.exec(s))) {
        out.push({ type: 'operator', value: m[0] });
        s = s.slice(m[0].length);
        // 閉じ括弧の後ろには助詞が続きうる (「(A+B)を表示」など)
        if (/[)\]}｝]$/.test(m[0])) s = s.slice(pushJosi(out, s));
        continue;
      }
      const w = readWord(s);
      if (w.word !== '') {
        out.push({ type: classifyWord(w.word), value: w.word });
        s = s.slice(w.word.length);
        if (w.josi !== '') s = s.slice(pushJosi(out, s));
        continue;
      }
      // 助詞だけの語句 (「〜を」の「を」が単独で現れた場合など)
      const josiLen = pushJosi(out, s);
      if (josiLen > 0) {
        s = s.slice(josiLen);
        continue;
      }
      // どの規則にも当てはまらない文字（絵文字変数など）は素通しする
      const ch = String.fromCodePoint(s.codePointAt(0));
      out.push({ type: 'plain', value: ch });
      s = s.slice(ch.length);
    }
    return out;
  }

  global.NakoSyntax = {
    tokenize: tokenize,
    setCommands: setCommands,
    trimOkurigana: trimOkurigana,
  };
}(window));
