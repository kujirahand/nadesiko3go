// gonako-gui が全ページの先頭で注入するローダー（#63）。
//
// - window.gonako.call(命令名, ...引数) で Go 側の命令を呼ぶ
// - window.gonako.run(code) で Go 側のなでしこを実行し、表示内容を受け取る
// - <script type="なでしこ"> があるページでは wnako3.js を読み込んで実行する
// - wnako3 には PluginGonako（GONAKO呼出 / GONAKO実行 / GONAKOバージョン）を登録する
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

  async function answerDialog(runID, dialog) {
    let text = '';
    let accepted = true;
    if (dialog.kind === 'prompt') {
      const answer = window.prompt(dialog.message, '');
      accepted = answer !== null;
      text = answer || '';
    } else if (dialog.kind === 'confirm') {
      accepted = window.confirm(dialog.message);
    } else {
      window.alert(dialog.message);
    }
    await window.resolveNakoDialog(runID, dialog.id, text, accepted);
  }

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
    'GONAKO呼出': { // @Go側(gonako)の命令CMDを引数ARGS(配列)で呼び出して結果を返す // @GONAKOよびだし
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
      // async などで遅れて読み込まれる場合
      window.addEventListener('load', registerPlugin);
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
