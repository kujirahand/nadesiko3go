// 梱包プログラムの実行画面。bundled.go が app.css とともに app.html へ
// 埋め込み、WebView に渡す（WebView の SetHtml は単一のHTML文字列しか
// 受け取れないため、外部ファイル参照ではなく埋め込みにしている）。
//
// Go側とのやり取りは main.go が Bind した4つの関数だけ。
//   pollNakoRun(runId)                          途中経過を取り出す
//   startNakoEvent(runId, handle, name, values) イベントハンドラを起動する
//   resolveNakoDialog(runId, id, text, ok)      ダイアログの応答を返す
//   closeBundledWindow()                        画面を使わずに終わったら閉じる

const root = document.getElementById('gonako-screen');
const runId = Number(document.body.dataset.runId);
let state = { runId };

// 入力欄の現在値を、なでしこ側のハンドル番号をキーにして集める。
function values() {
  const v = {};
  root.querySelectorAll('[data-gonako-handle]').forEach(e => {
    if (e.matches('input,textarea,select')) v[e.dataset.gonakoHandle] = e.value;
  });
  return v;
}

// イベントも通常実行と同じポーリング経路に載せる。同期実行すると、
// ハンドラ内の『言う』がダイアログの応答を待つ一方、応答を返す画面は
// この関数の戻りを待ち続け、ウィンドウごと固まる（#59）。
async function send(h, n) {
  const err = document.getElementById('gonako-error');
  const raw = await window.startNakoEvent(state.runId, Number(h), n, values());
  const st = typeof raw === 'string' ? JSON.parse(raw) : raw;
  if (st.error) {
    err.textContent = st.error;
    return;
  }
  const id = st.runId;
  for (;;) {
    const p = await window.pollNakoRun(id);
    const s = typeof p === 'string' ? JSON.parse(p) : p;
    if (s.operations && s.operations.length) apply(s.operations);
    if (s.dialog) {
      const a = await ask(s.dialog);
      await window.resolveNakoDialog(id, s.dialog.id, a.text, a.accepted);
      continue;
    }
    if (s.done) {
      const r = s.result || {};
      if (r.error) err.textContent = r.error;
      return;
    }
    await new Promise(x => setTimeout(x, 20));
  }
}

// なでしこ側から届いた画面操作を1つずつDOMに反映する。
function apply(ops) {
  ops.forEach(o => {
    const q = '[data-gonako-handle="' + o.handle + '"]';
    if (o.type === 'create') {
      const p = o.parent ? root.querySelector('[data-gonako-handle="' + o.parent + '"]') : root;
      if (!p) return;
      let e;
      if (o.tag === 'submit') {
        e = document.createElement('button');
        e.type = 'submit';
      } else {
        e = document.createElement(o.tag || 'div');
        if (o.tag === 'input') e.type = 'text';
      }
      e.dataset.gonakoHandle = String(o.handle);
      e.classList.add('gonako-part');
      if (o.name) e.name = o.name;
      if (o.html) e.innerHTML = o.html;
      else if (e.matches('input,textarea,select')) e.value = o.text || '';
      else e.textContent = o.text || '';
      p.appendChild(e);
      return;
    }
    const e = root.querySelector(q);
    if (!e) return;
    if (o.type === 'text') {
      if (e.matches('input,textarea,select')) e.value = o.text || '';
      else e.textContent = o.text || '';
    } else if (o.type === 'html') {
      e.innerHTML = o.html || '';
    } else if (o.type === 'styles') {
      Object.entries(o.styles || {}).forEach(([k, v]) => e.style[k] = v);
    } else if (o.type === 'attributes') {
      Object.entries(o.attributes || {}).forEach(([k, v]) => e.setAttribute(k, v));
    } else if (o.type === 'listen' && !e.dataset['gonakoEvent' + o.event]) {
      // 同じイベントを二重に登録しない
      e.dataset['gonakoEvent' + o.event] = '1';
      e.addEventListener(o.event, x => {
        if (o.event === 'submit') x.preventDefault();
        send(o.handle, o.event);
      });
    } else if (o.type === 'focus') {
      e.focus();
    }
  });
}

// 『言う』『尋ねる』『二択』のダイアログ。ネイティブの alert/prompt/confirm は
// WebView をブロックしてポーリングが止まるので使わない。
function ask(d) {
  return new Promise(resolve => {
    const overlay = document.getElementById('overlay');
    const input = document.getElementById('dialog-input');
    const cancel = document.getElementById('dialog-cancel');
    const ok = document.getElementById('dialog-ok');
    let composing = false;
    document.getElementById('dialog-title').textContent =
      d.kind === 'prompt' ? '入力' : d.kind === 'confirm' ? '確認' : 'メッセージ';
    document.getElementById('dialog-message').textContent = d.message || '';
    input.style.display = d.kind === 'prompt' ? 'block' : 'none';
    input.value = '';
    cancel.style.display = d.kind === 'alert' ? 'none' : 'inline-block';
    overlay.style.display = 'flex';
    // 日本語入力の変換確定のEnterでダイアログを閉じてしまわないようにする
    input.oncompositionstart = () => { composing = true; };
    input.oncompositionend = () => { composing = false; };
    const done = (accepted) => {
      overlay.style.display = 'none';
      ok.onclick = null;
      cancel.onclick = null;
      input.onkeydown = null;
      input.oncompositionstart = null;
      input.oncompositionend = null;
      resolve({ text: d.kind === 'prompt' ? input.value : '', accepted });
    };
    ok.onclick = () => done(true);
    cancel.onclick = () => done(false);
    input.onkeydown = e => {
      if (e.isComposing || composing || e.keyCode === 229) return;
      if (e.key === 'Enter') done(true);
      else if (e.key === 'Escape') done(false);
    };
    (d.kind === 'prompt' ? input : ok).focus();
  });
}

// 出力と画面操作はポーリングのたびに届き、読まなければ消える。done を
// 見る前に必ず適用すること（→ AsyncRunStatus のコメント）。ウィンドウを
// 閉じてよいかの判定も、実行中に届いた分を数えた ops で行う。
async function run() {
  let out = '';
  let ops = 0;
  for (;;) {
    const raw = await window.pollNakoRun(runId);
    const s = typeof raw === 'string' ? JSON.parse(raw) : raw;
    if (s.output) out += s.output;
    if (s.operations && s.operations.length) {
      ops += s.operations.length;
      apply(s.operations);
    }
    if (s.dialog) {
      const a = await ask(s.dialog);
      await window.resolveNakoDialog(runId, s.dialog.id, a.text, a.accepted);
      continue;
    }
    if (s.done) {
      const r = s.result || {};
      state = r;
      if (r.error) {
        document.getElementById('gonako-error').textContent = (out ? out + '\n' : '') + r.error;
      }
      // 画面を一度も使っていないプログラムは、見せる物がないので閉じる
      if (!r.error && ops === 0) await window.closeBundledWindow();
      return;
    }
    await new Promise(x => setTimeout(x, 20));
  }
}

run();
