// gonako-gui が全ページの先頭で注入するローダー（#63）。
//
// - window.gonako.call(命令名, ...引数) で Go 側の命令を呼ぶ
// - window.gonako.run(code) で Go 側のなでしこを実行し、表示内容を受け取る
// - <script type="なでしこ"> があるページでは wnako3.js を読み込んで実行する
// - wnako3 には PluginGonako（GONAKO関数実行 / GONAKO実行 / GONAKOバージョン）を登録する
//
// macOSのWKWebViewはJavaScriptのalert/confirm/promptを実装していない
// （WKUIDelegateがrunJavaScript*Panel系メソッドを持たない）ため、
// wnako3自身の『言』『尋』『文字尋』『二択』（window.alert等を直接呼ぶ）は
// 何も表示せず素通りしてしまう。ここで自前のダイアログ（showGonakoDialog）を
// 実装し、PluginGonakoで同名の命令を上書きしてGo側（gonako）の同名命令を
// 呼び出すようにする（Go側もこのダイアログを介して応答する。bridge.goを参照）。
//
// 直前に gonako-gui が window.__gonakoConfig を設定している。
(function () {
  'use strict';
  if (window.gonako) return;

  const config = window.__gonakoConfig || {};
  const assetBase = '/__gonako/wnako3/';
  const nakoScriptType = /^(なでしこ|nako|nadesiko)3?$/;

  // startNakoCommand が返したIDと、結果を待つPromiseの対応
  const pending = new Map();
  // Bindの戻り値より先に届いた結果
  const early = new Map();

  function finish(entry, raw) {
    let result;
    try {
      result = JSON.parse(raw);
    } catch (e) {
      entry.reject(e);
      return;
    }
    if (result.output) console.log(result.output);
    if (result.ok) {
      entry.resolve(result.value === undefined ? null : result.value);
    } else {
      entry.reject(new Error(result.error || 'Go側の命令が失敗しました'));
    }
  }

  window.__gonakoCommandDone = function (id, raw) {
    const entry = pending.get(id);
    if (!entry) {
      early.set(id, raw);
      return;
    }
    pending.delete(id);
    finish(entry, raw);
  };

  function requireBinding(name) {
    if (typeof window[name] !== 'function') {
      throw new Error(`gonako-guiの機能『${name}』が使えません`);
    }
  }

  // Go側の命令を1つ呼ぶ
  async function call(name, ...args) {
    requireBinding('startNakoCommand');
    const id = await window.startNakoCommand(String(name), JSON.stringify(args));
    return new Promise((resolve, reject) => {
      const entry = { resolve, reject };
      if (early.has(id)) {
        const raw = early.get(id);
        early.delete(id);
        finish(entry, raw);
        return;
      }
      pending.set(id, entry);
    });
  }

  // ネイティブのalert/confirm/promptに頼らない自前のダイアログ。
  // macOSのWKWebViewはこれらを実装していないため、呼んでも何も起きない。
  // kind: 'alert' | 'confirm' | 'prompt' | 'buttons' | 'list'。戻り値は{text, accepted}。
  // 呼び出しは内部でキューに並べ、常に1つずつ表示する（同時に2つ出さない）。
  let dialogQueue = Promise.resolve();
  function showGonakoDialog(kind, message) {
    const task = dialogQueue.then(() => showGonakoDialogNow(kind, message));
    // 1つの失敗が後続の表示を止めないよう、キュー自体は握りつぶして繋ぐ。
    dialogQueue = task.catch(() => {});
    return task;
  }
  // 「ボタン選択」「リスト選択」用のダイアログ。messageには{label, items}を
  // JSON化したものが入っている（internal/guilib のcmdButtonChoice/cmdListChoice参照）。
  // 右上の[x]、またはリストのキャンセルは{ text: '', accepted: false }を返す。
  function showGonakoChoiceDialogNow(kind, message) {
    return new Promise(resolve => {
      let payload = {};
      try { payload = JSON.parse(message || '{}'); } catch (e) { payload = {}; }
      const items = Array.isArray(payload.items) ? payload.items : [];
      const isButtons = kind === 'buttons';

      const overlay = document.createElement('div');
      overlay.style.cssText = 'position:fixed;inset:0;z-index:2147483647;'
        + 'background:rgba(0,0,0,.35);display:flex;align-items:center;justify-content:center;'
        + 'font-family:system-ui,-apple-system,"Hiragino Sans","Yu Gothic UI",sans-serif;';
      const box = document.createElement('div');
      box.style.cssText = 'position:relative;background:#fff;color:#222;min-width:280px;max-width:420px;'
        + 'padding:20px;border-radius:8px;box-shadow:0 8px 32px rgba(0,0,0,.3);';

      const closeBtn = document.createElement('button');
      closeBtn.textContent = '×';
      closeBtn.style.cssText = 'position:absolute;top:8px;right:8px;width:24px;height:24px;'
        + 'border:none;background:transparent;font-size:18px;line-height:1;cursor:pointer;color:#888;';
      box.appendChild(closeBtn);

      if (payload.label) {
        const label = document.createElement('div');
        label.textContent = payload.label;
        label.style.cssText = 'white-space:pre-wrap;word-break:break-word;margin:0 20px 14px 0;font-size:14px;line-height:1.5;';
        box.appendChild(label);
      }

      function finish(text, accepted) {
        document.removeEventListener('keydown', onKeyDown, true);
        overlay.remove();
        resolve({ text, accepted });
      }
      closeBtn.addEventListener('click', () => finish('', false));

      let list = null;
      if (isButtons) {
        const buttons = document.createElement('div');
        buttons.style.cssText = 'display:flex;flex-wrap:wrap;gap:8px;';
        items.forEach(label => {
          const btn = document.createElement('button');
          btn.textContent = label;
          btn.style.cssText = 'padding:6px 14px;font-size:13px;border-radius:4px;cursor:pointer;'
            + 'background:#f0f0f0;color:#222;border:1px solid #ccc;';
          btn.addEventListener('click', () => finish(label, true));
          buttons.appendChild(btn);
        });
        box.appendChild(buttons);
      } else {
        list = document.createElement('select');
        list.size = Math.min(Math.max(items.length, 3), 8);
        list.style.cssText = 'width:100%;box-sizing:border-box;padding:4px;font-size:14px;'
          + 'border:1px solid #ccc;border-radius:4px;margin-bottom:14px;';
        items.forEach(label => {
          const opt = document.createElement('option');
          opt.value = label;
          opt.textContent = label;
          list.appendChild(opt);
        });
        if (items.length > 0) list.selectedIndex = 0;
        box.appendChild(list);

        const buttons = document.createElement('div');
        buttons.style.cssText = 'display:flex;justify-content:flex-end;gap:8px;';
        const cancelBtn = document.createElement('button');
        cancelBtn.textContent = 'キャンセル';
        cancelBtn.style.cssText = 'padding:6px 14px;font-size:13px;border-radius:4px;cursor:pointer;'
          + 'background:#f0f0f0;color:#222;border:1px solid #ccc;';
        cancelBtn.addEventListener('click', () => finish('', false));
        const okBtn = document.createElement('button');
        okBtn.textContent = 'OK';
        okBtn.style.cssText = 'padding:6px 14px;font-size:13px;border-radius:4px;cursor:pointer;'
          + 'background:#e64553;color:#fff;border:none;';
        okBtn.addEventListener('click', () => finish(list.value, true));
        buttons.appendChild(cancelBtn);
        buttons.appendChild(okBtn);
        box.appendChild(buttons);
      }

      function onKeyDown(e) {
        if (e.key === 'Escape') {
          e.preventDefault();
          finish('', false);
        }
      }

      overlay.appendChild(box);
      (document.body || document.documentElement).appendChild(overlay);
      document.addEventListener('keydown', onKeyDown, true);
      (list || overlay.querySelector('button')).focus();
    });
  }

  // 「カスタムダイアログ表示」用のダイアログ。messageには{html, items}を
  // JSON化したものが入っている（internal/guilib のcmdCustomDialog参照）。
  // htmlはそのまま本文としてDOMに挿入し、itemsをボタンラベルとして並べる。
  // 右上の[x]は{ text: '', accepted: false }を返す。
  function showGonakoCustomDialogNow(message) {
    return new Promise(resolve => {
      let payload = {};
      try { payload = JSON.parse(message || '{}'); } catch (e) { payload = {}; }
      const items = Array.isArray(payload.items) ? payload.items : [];

      const overlay = document.createElement('div');
      overlay.style.cssText = 'position:fixed;inset:0;z-index:2147483647;'
        + 'background:rgba(0,0,0,.35);display:flex;align-items:center;justify-content:center;'
        + 'font-family:system-ui,-apple-system,"Hiragino Sans","Yu Gothic UI",sans-serif;';
      const box = document.createElement('div');
      box.style.cssText = 'position:relative;background:#fff;color:#222;min-width:280px;max-width:520px;'
        + 'padding:20px;border-radius:8px;box-shadow:0 8px 32px rgba(0,0,0,.3);';

      const closeBtn = document.createElement('button');
      closeBtn.textContent = '×';
      closeBtn.style.cssText = 'position:absolute;top:8px;right:8px;width:24px;height:24px;'
        + 'border:none;background:transparent;font-size:18px;line-height:1;cursor:pointer;color:#888;';
      box.appendChild(closeBtn);

      const body = document.createElement('div');
      body.innerHTML = payload.html || '';
      body.style.cssText = 'margin:0 20px 14px 0;font-size:14px;line-height:1.5;';
      box.appendChild(body);

      function finish(text, accepted) {
        document.removeEventListener('keydown', onKeyDown, true);
        overlay.remove();
        resolve({ text, accepted });
      }
      closeBtn.addEventListener('click', () => finish('', false));

      const buttons = document.createElement('div');
      buttons.style.cssText = 'display:flex;flex-wrap:wrap;gap:8px;';
      items.forEach(label => {
        const btn = document.createElement('button');
        btn.textContent = label;
        btn.style.cssText = 'padding:6px 14px;font-size:13px;border-radius:4px;cursor:pointer;'
          + 'background:#f0f0f0;color:#222;border:1px solid #ccc;';
        btn.addEventListener('click', () => finish(label, true));
        buttons.appendChild(btn);
      });
      box.appendChild(buttons);

      function onKeyDown(e) {
        if (e.key === 'Escape') {
          e.preventDefault();
          finish('', false);
        }
      }

      overlay.appendChild(box);
      (document.body || document.documentElement).appendChild(overlay);
      document.addEventListener('keydown', onKeyDown, true);
      (overlay.querySelector('button')).focus();
    });
  }

  function showGonakoDialogNow(kind, message) {
    if (kind === 'buttons' || kind === 'list') {
      return showGonakoChoiceDialogNow(kind, message);
    }
    if (kind === 'custom') {
      return showGonakoCustomDialogNow(message);
    }
    return new Promise(resolve => {
      const overlay = document.createElement('div');
      overlay.style.cssText = 'position:fixed;inset:0;z-index:2147483647;'
        + 'background:rgba(0,0,0,.35);display:flex;align-items:center;justify-content:center;'
        + 'font-family:system-ui,-apple-system,"Hiragino Sans","Yu Gothic UI",sans-serif;';
      const box = document.createElement('div');
      box.style.cssText = 'background:#fff;color:#222;min-width:280px;max-width:420px;'
        + 'padding:20px;border-radius:8px;box-shadow:0 8px 32px rgba(0,0,0,.3);';
      const text = document.createElement('div');
      text.textContent = message;
      text.style.cssText = 'white-space:pre-wrap;word-break:break-word;margin-bottom:14px;font-size:14px;line-height:1.5;';
      box.appendChild(text);

      let input = null;
      // IME変換中のEnterで確定してしまわないよう、compositionイベントの
      // 状態とkeyCode=229も併用する（WKWebViewではcomposition中でも
      // isComposingが取れない場合があるため。app.jsの同種対策と同じ）。
      let imeComposing = false;
      if (kind === 'prompt') {
        input = document.createElement('input');
        input.type = 'text';
        input.style.cssText = 'width:100%;box-sizing:border-box;padding:6px 8px;font-size:14px;'
          + 'border:1px solid #ccc;border-radius:4px;margin-bottom:14px;';
        input.addEventListener('compositionstart', () => { imeComposing = true; });
        input.addEventListener('compositionend', () => { imeComposing = false; });
        box.appendChild(input);
      }
      function isIMEKeyEvent(e) {
        return e.isComposing || imeComposing || e.keyCode === 229;
      }

      const buttons = document.createElement('div');
      buttons.style.cssText = 'display:flex;justify-content:flex-end;gap:8px;';
      function makeButton(label, primary) {
        const btn = document.createElement('button');
        btn.textContent = label;
        btn.style.cssText = 'padding:6px 14px;font-size:13px;border-radius:4px;cursor:pointer;'
          + (primary ? 'background:#e64553;color:#fff;border:none;' : 'background:#f0f0f0;color:#222;border:1px solid #ccc;');
        return btn;
      }

      function finish(accepted) {
        document.removeEventListener('keydown', onKeyDown, true);
        overlay.remove();
        resolve({ text: input ? input.value : '', accepted });
      }
      function onKeyDown(e) {
        if (isIMEKeyEvent(e)) return;
        if (e.key === 'Enter' && (kind !== 'prompt' || document.activeElement === input)) {
          e.preventDefault();
          finish(true);
        } else if (e.key === 'Escape') {
          e.preventDefault();
          finish(kind === 'alert');
        }
      }

      if (kind !== 'alert') {
        buttons.appendChild(makeButton('キャンセル', false)).addEventListener('click', () => finish(false));
      }
      buttons.appendChild(makeButton('OK', true)).addEventListener('click', () => finish(true));
      box.appendChild(buttons);
      overlay.appendChild(box);
      (document.body || document.documentElement).appendChild(overlay);
      document.addEventListener('keydown', onKeyDown, true);
      (input || overlay.querySelector('button:last-of-type')).focus();
    });
  }

  // Go側のVM（window.gonako.run経由）が出したダイアログに応答する。
  async function answerDialog(runID, dialog) {
    const { text, accepted } = await showGonakoDialog(dialog.kind, dialog.message);
    await window.resolveNakoDialog(runID, dialog.id, text, accepted);
  }

  // GONAKO関数実行のブリッジVM（bridge.go）が出したダイアログに応答する。
  // 『言』『尋』『文字尋』『二択』をwnako3から上書きするための実装。
  window.__gonakoBridgeDialog = async function (id, kind, message) {
    const { text, accepted } = await showGonakoDialog(kind, message);
    if (window.resolveGonakoDialog) await window.resolveGonakoDialog(id, text, accepted);
  };

  // Go側でなでしこのプログラムを実行し、表示された文字列を返す
  async function run(code) {
    requireBinding('startNakoCode');
    requireBinding('pollNakoRun');
    const runID = await window.startNakoCode(String(code));
    let output = '';
    let answered = 0;
    for (;;) {
      const status = JSON.parse(await window.pollNakoRun(runID));
      if (status.output) output += status.output;
      if (status.dialog && status.dialog.id !== answered) {
        answered = status.dialog.id;
        await answerDialog(runID, status.dialog);
      }
      if (status.done) {
        const result = status.result || {};
        if (result.output) output += result.output;
        if (!result.ok) throw new Error(result.error || 'なでしこの実行に失敗しました');
        return output;
      }
      await new Promise(resolve => setTimeout(resolve, 30));
    }
  }

  window.gonako = {
    call,
    run,
    version: config.gonakoVersion || '',
    wnako3Version: config.wnako3Version || '',
    // wnako3 を自前で動かすページ（ui/wnako3run.html）が、実行前に確実に登録するため
    registerWNako3Plugin: () => registerPlugin(),
  };

  // wnako3 へ登録するプラグイン
  const pluginGonako = {
    'meta': {
      type: 'const',
      value: {
        pluginName: 'plugin_gonako',
        description: 'gonako-guiのGo側の命令を呼び出す',
        pluginVersion: config.gonakoVersion || '',
        nakoRuntime: ['wnako'],
        nakoVersion: '3.6.0',
      },
    },
    'GONAKOバージョン': { type: 'const', value: config.gonakoVersion || '' }, // @GONAKOばーじょん
    'GONAKO関数実行': { // @Go側(gonako)の命令CMDを引数ARGS(配列)で呼び出して結果を返す // @GONAKOかんすうじっこう
      type: 'func',
      josi: [['を', 'の'], ['で']],
      pure: true,
      asyncFn: true,
      fn: async function (cmd, args, sys) {
        const list = Array.isArray(args) ? args : [args];
        return call(cmd, ...list);
      },
    },
    'GONAKO実行': { // @Go側(gonako)でなでしこのプログラムCODEを実行し、表示した内容を返す // @GONAKOじっこう
      type: 'func',
      josi: [['を', 'で']],
      pure: true,
      asyncFn: true,
      fn: async function (code, sys) {
        return run(code);
      },
    },
    // 以下は本家(plugin_browser)の『言』『尋』『文字尋』『二択』を上書きする。
    // wnako3自身の実装はwindow.alert/prompt/confirmを直接呼ぶが、
    // macOSのWKWebViewはこれらを実装しておらず何も表示されない。
    // gonako（Go側）の同名命令をGONAKO関数実行で呼び出し、応答は
    // showGonakoDialog（自前のダイアログ）で受け取る。
    '言': { // @メッセージダイアログにSを表示 // @いう
      type: 'func',
      josi: [['と', 'を']],
      pure: true,
      asyncFn: true,
      fn: async function (s, sys) {
        await call('言', s);
      },
      return_none: true,
    },
    '尋': { // @メッセージSと入力ボックスを出して尋ねる // @たずねる
      type: 'func',
      josi: [['と', 'を']],
      pure: true,
      asyncFn: true,
      fn: async function (s, sys) {
        return call('尋', s);
      },
    },
    '文字尋': { // @メッセージSと入力ボックスを出して尋ねる。返り値は常に文字列 // @もじたずねる
      type: 'func',
      josi: [['と', 'を']],
      pure: true,
      asyncFn: true,
      fn: async function (s, sys) {
        return call('文字尋', s);
      },
    },
    '二択': { // @メッセージSと[OK][キャンセル]のダイアログを出して尋ねる // @にたく
      type: 'func',
      josi: [['で', 'の', 'と', 'を']],
      pure: true,
      asyncFn: true,
      fn: async function (s, sys) {
        return call('二択', s);
      },
    },
    // 『ボタン選択』『リスト選択』はwnako3(本家)には無いgonako独自の命令なので、
    // 上書きではなく新規登録する。実体はGo側(internal/guilib)にあり、
    // GONAKO関数実行と同じ経路(commandBridge)で呼び出す。
    'ボタン選択': { // @候補配列Sからボタンを選ばせて選んだ文字列を返す。[x]で閉じると空文字列を返す // @ぼたんせんたく
      type: 'func',
      josi: [['の']],
      pure: true,
      asyncFn: true,
      fn: async function (s, sys) {
        return call('ボタン選択', s);
      },
    },
    'リスト選択': { // @候補配列Sからリストで選ばせて選んだ文字列を返す。キャンセルまたは[x]で閉じると空文字列を返す // @りすとせんたく
      type: 'func',
      josi: [['の']],
      pure: true,
      asyncFn: true,
      fn: async function (s, sys) {
        return call('リスト選択', s);
      },
    },
  };

  let registered = false;
  function registerPlugin() {
    if (registered || !navigator.nako3) return;
    navigator.nako3.addPluginObject('PluginGonako', pluginGonako);
    registered = true;
  }

  function scripts() {
    return Array.from(document.getElementsByTagName('script'));
  }

  document.addEventListener('DOMContentLoaded', () => {
    // ページ自身が wnako3.js を読み込んでいる。wnako3 の自動実行より先に
    // このリスナーが動くので、ここで登録すれば間に合う。
    if (navigator.nako3) {
      registerPlugin();
      return;
    }
    if (scripts().some(s => /(^|\/)wnako3\.js([?#&]|$)/.test(s.src))) {
      // async などで DOMContentLoaded より後に読み込まれる場合。wnako3 は読み込まれた
      // 時点で DOMContentLoaded リスナーを登録するので、自動実行（wnako3.js?run）は
      // 起きない。ここで登録し、ページが自動実行を求めていれば一度だけ実行する。
      // DOMContentLoaded の時点で navigator.nako3 が無かったので、二重実行にはならない。
      window.addEventListener('load', () => {
        if (!navigator.nako3) return;
        registerPlugin();
        if (navigator.nako3.checkScriptTagParam()) navigator.nako3.runNakoScript();
      }, { once: true });
      return;
    }
    if (!config.wnako3 && !scripts().some(s => nakoScriptType.test(s.type))) return;

    const script = document.createElement('script');
    script.src = assetBase + 'wnako3.js';
    script.onload = () => {
      registerPlugin();
      // DOMContentLoaded 後に読み込んだので、wnako3 の自動実行は起きない。ここで実行する。
      navigator.nako3.runNakoScript();
    };
    script.onerror = () => console.error('[gonako] wnako3.js を読み込めません');
    (document.head || document.documentElement).appendChild(script);
  });
})();
