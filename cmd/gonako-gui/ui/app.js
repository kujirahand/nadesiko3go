// なでしこ3 GUI (gonako-gui) Frontend Script

const samples = {
  hello: `// なでしこ3の基本
「こんにちは、なでしこ！」と表示。
3回
　　「{回数}回目の挨拶」と表示。
ここまで`,

  fizzbuzz: `// 1から30までのFizzBuzz
結果 = []
Iを1から30まで繰り返す
　　もし、(I % 15 = 0)ならば
　　　　結果に「FizzBuzz」を配列追加。
　　違えば、もし、(I % 3 = 0)ならば
　　　　結果に「Fizz」を配列追加。
　　違えば、もし、(I % 5 = 0)ならば
　　　　結果に「Buzz」を配列追加。
　　違えば
　　　　結果にIを配列追加。
　　ここまで
ここまで
結果を「, 」で配列結合して表示。`,

  calc: `// 変数と計算、辞書
A = 100
B = 200
C = A + B
「{A} + {B} = {C}」を表示。

名簿 = {}
名簿["名前"] = "太郎"
名簿["年齢"] = 25
名簿["趣味"] = ["プログラミング", "読書", "旅行"]

「名前: {名簿["名前"]}」と表示。
「趣味数: {名簿["趣味"]の要素数}」と表示。`,

  dialog: `// ファイル・ハッシュ・UUID
Msg = 「こんにちは」
Sha = Msgを「sha256」でハッシュ値計算
「SHA256: {Sha}」と表示。

U = ランダムUUID生成
「UUID: {U}」と表示。

「カレント: {カレントディレクトリ取得}」と表示。
「OS: {OS取得} ({OSアーキテクチャ取得})」と表示。`,

  csv: `// CSVの生成と解析
データ = [
  ["名前", "点数"],
  ["太郎", 85],
  ["次郎", 92],
  ["花子", 78]
]
CsvText = データからCSV変換
「--- CSV出力 ---」を表示。
CsvTextを表示。

Parsed = CsvTextをCSV取得
「件数: {Parsedの要素数 - 1}件」を表示。`
};

document.addEventListener('DOMContentLoaded', () => {
  const editor = document.getElementById('editor');
  const lineNumbers = document.getElementById('line-numbers');
  const output = document.getElementById('output');
  const btnRun = document.getElementById('btn-run');
  const btnClearLog = document.getElementById('btn-clear-log');
  const btnCopyLog = document.getElementById('btn-copy-log');
  const selectSample = document.getElementById('select-sample');
  const charCount = document.getElementById('char-count');
  const cursorPos = document.getElementById('cursor-pos');
  const execStatus = document.getElementById('exec-status');
  const versionInfo = document.getElementById('version-info');

  // 初期プログラム
  editor.value = samples.hello;
  updateLineNumbers();
  updateCharCount();

  // 行番号とスクロールの同期
  function updateLineNumbers() {
    const lines = editor.value.split('\n').length;
    let numbers = '';
    for (let i = 1; i <= lines; i++) {
      numbers += i + '\n';
    }
    lineNumbers.textContent = numbers;
  }

  function updateCharCount() {
    charCount.textContent = `${editor.value.length} 文字`;
  }

  function updateCursorPos() {
    const text = editor.value.substring(0, editor.selectionStart);
    const lines = text.split('\n');
    const row = lines.length;
    const col = lines[lines.length - 1].length + 1;
    cursorPos.textContent = `行: ${row}, 列: ${col}`;
  }

  editor.addEventListener('input', () => {
    updateLineNumbers();
    updateCharCount();
  });

  editor.addEventListener('scroll', () => {
    lineNumbers.scrollTop = editor.scrollTop;
  });

  editor.addEventListener('keyup', updateCursorPos);
  editor.addEventListener('click', updateCursorPos);

  // Tabキーでのインデント対応
  editor.addEventListener('keydown', (e) => {
    if (e.key === 'Tab') {
      e.preventDefault();
      const start = editor.selectionStart;
      const end = editor.selectionEnd;
      editor.value = editor.value.substring(0, start) + '\t' + editor.value.substring(end);
      editor.selectionStart = editor.selectionEnd = start + 1;
      updateLineNumbers();
      updateCharCount();
    }
    // Ctrl+Enter または Cmd+Enter で実行
    if ((e.ctrlKey || e.metaKey) && (e.key === 'Enter' || e.key === 'r' || e.key === 'R')) {
      e.preventDefault();
      runCode();
    }
  });

  // サンプル選択
  selectSample.addEventListener('change', (e) => {
    const key = e.target.value;
    if (key && samples[key]) {
      editor.value = samples[key];
      updateLineNumbers();
      updateCharCount();
      updateCursorPos();
    }
  });

  // 実行ボタン
  btnRun.addEventListener('click', runCode);

  // ログクリア
  btnClearLog.addEventListener('click', () => {
    output.textContent = '';
    output.className = 'output';
    execStatus.textContent = '待機中';
    execStatus.className = 'status-indicator';
  });

  // ログコピー
  btnCopyLog.addEventListener('click', () => {
    navigator.clipboard.writeText(output.textContent).then(() => {
      btnCopyLog.textContent = 'コピー完了!';
      setTimeout(() => {
        btnCopyLog.innerHTML = '<span class="icon">📋</span> 結果コピー';
      }, 1500);
    });
  });

  // なでしこプログラム実行処理
  async function runCode() {
    const code = editor.value;
    if (!code.trim()) {
      output.textContent = '（プログラムが空です）';
      output.className = 'output';
      return;
    }

    execStatus.textContent = '実行中...';
    execStatus.className = 'status-indicator running';
    btnRun.disabled = true;

    const startTime = performance.now();

    try {
      let result;
      // Go側でバインドされた関数を呼び出す
      if (typeof window.runNakoCode === 'function') {
        result = await window.runNakoCode(code);
      } else {
        // ブラウザ単体テスト用のフォールバック
        result = JSON.stringify({
          ok: true,
          output: "※ WebViewバインディング準備中...\n入力コード長: " + code.length,
          error: ""
        });
      }

      const elapsed = ((performance.now() - startTime) / 1000).toFixed(3);
      let data = {};
      try {
        data = typeof result === 'string' ? JSON.parse(result) : result;
      } catch {
        data = { ok: true, output: String(result), error: "" };
      }

      if (data.error) {
        output.textContent = (data.output ? data.output + '\n' : '') + data.error;
        output.className = 'output has-error';
        execStatus.textContent = `エラー (${elapsed}s)`;
        execStatus.className = 'status-indicator error';
      } else {
        output.textContent = data.output || '（出力なし）';
        output.className = 'output has-content';
        execStatus.textContent = `完了 (${elapsed}s)`;
        execStatus.className = 'status-indicator success';
      }
    } catch (err) {
      output.textContent = `[システムエラー] ${err.message || err}`;
      output.className = 'output has-error';
      execStatus.textContent = 'エラー';
      execStatus.className = 'status-indicator error';
    } finally {
      btnRun.disabled = false;
    }
  }

  // スプリッタードラッグ処理
  const splitter = document.getElementById('splitter');
  const paneEditor = document.querySelector('.pane-editor');
  let isDragging = false;

  splitter.addEventListener('mousedown', (e) => {
    isDragging = true;
    splitter.classList.add('dragging');
    document.body.style.cursor = 'col-resize';
  });

  window.addEventListener('mousemove', (e) => {
    if (!isDragging) return;
    const containerRect = document.querySelector('.workspace').getBoundingClientRect();
    const newWidth = e.clientX - containerRect.left;
    if (newWidth > 150 && newWidth < containerRect.width - 150) {
      paneEditor.style.flex = 'none';
      paneEditor.style.width = `${newWidth}px`;
    }
  });

  window.addEventListener('mouseup', () => {
    if (isDragging) {
      isDragging = false;
      splitter.classList.remove('dragging');
      document.body.style.cursor = '';
    }
  });

  // バージョン取得
  if (typeof window.getAppInfo === 'function') {
    window.getAppInfo().then(infoStr => {
      try {
        const info = JSON.parse(infoStr);
        versionInfo.textContent = `gonako-gui v${info.version || '3.6.0'} (${info.os}/${info.arch})`;
      } catch {
        versionInfo.textContent = `gonako-gui`;
      }
    });
  }
});
