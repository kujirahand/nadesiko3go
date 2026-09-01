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
  const btnSave = document.getElementById('btn-save');
  const btnClearLog = document.getElementById('btn-clear-log');
  const btnCopyLog = document.getElementById('btn-copy-log');
  const selectSample = document.getElementById('select-sample');
  const charCount = document.getElementById('char-count');
  const cursorPos = document.getElementById('cursor-pos');
  const execStatus = document.getElementById('exec-status');
  const statusMsg = document.getElementById('status-msg');
  const versionInfo = document.getElementById('version-info');
  const activeFileName = document.getElementById('active-file-name');

  // タブ要素
  const tabBtnCmd = document.getElementById('tab-btn-cmd');
  const tabBtnFile = document.getElementById('tab-btn-file');
  const tabContentCmd = document.getElementById('tab-content-cmd');
  const tabContentFile = document.getElementById('tab-content-file');

  // 命令タブ要素
  const cmdSearch = document.getElementById('cmd-search');
  const cmdCount = document.getElementById('cmd-count');
  const cmdList = document.getElementById('cmd-list');

  // ファイルタブ要素
  const btnFileUp = document.getElementById('btn-file-up');
  const btnFileHome = document.getElementById('btn-file-home');
  const currentDirDisplay = document.getElementById('current-dir-display');
  const fileList = document.getElementById('file-list');

  let allCommands = [];
  let currentDirPath = '';
  let parentDirPath = '';
  let homeDirPath = '';
  let currentFilePath = '';

  // 初期プログラム
  editor.value = samples.hello;
  updateLineNumbers();
  updateCharCount();

  // --- タブ切り替え処理 ---
  tabBtnCmd.addEventListener('click', () => {
    tabBtnCmd.classList.add('active');
    tabBtnFile.classList.remove('active');
    tabContentCmd.classList.add('active');
    tabContentFile.classList.remove('active');
  });

  tabBtnFile.addEventListener('click', () => {
    tabBtnFile.classList.add('active');
    tabBtnCmd.classList.remove('active');
    tabContentFile.classList.add('active');
    tabContentCmd.classList.remove('active');
    if (!currentDirPath) {
      loadDirectory(homeDirPath || '$HOME');
    }
  });

  // --- 命令一覧の読み込みと検索 ---
  async function loadCommands() {
    if (typeof window.getCommandList !== 'function') {
      return;
    }
    try {
      const res = await window.getCommandList();
      allCommands = typeof res === 'string' ? JSON.parse(res) : res;
      renderCommands(allCommands);
    } catch (err) {
      console.error('命令一覧の読み込みエラー:', err);
    }
  }

  function renderCommands(commands) {
    cmdList.innerHTML = '';
    cmdCount.textContent = `${commands.length}件`;

    commands.forEach(cmd => {
      const item = document.createElement('div');
      item.className = 'list-item';
      item.title = `クリックでエディタに挿入: ${cmd.name}`;

      // 助詞のフォーマット (例: [["を", "から"], ["に", "へ"]])
      let josiText = '';
      if (cmd.josi && cmd.josi.length > 0) {
        josiText = cmd.josi.map(group => group[0]).join(' / ');
      }

      item.innerHTML = `
        <div class="item-left">
          <span class="item-icon">⚡</span>
          <span class="item-name">${escapeHtml(cmd.name)}</span>
        </div>
        ${josiText ? `<span class="cmd-item-josi">${escapeHtml(josiText)}</span>` : ''}
      `;

      item.addEventListener('click', () => {
        insertTextAtCursor(cmd.name);
        setStatus(`命令「${cmd.name}」を挿入しました`);
      });

      cmdList.appendChild(item);
    });
  }

  cmdSearch.addEventListener('input', () => {
    const query = cmdSearch.value.trim().toLowerCase();
    if (!query) {
      renderCommands(allCommands);
      return;
    }
    const filtered = allCommands.filter(c => {
      if (c.name.toLowerCase().includes(query)) return true;
      if (c.josi && c.josi.some(group => group.some(j => j.toLowerCase().includes(query)))) return true;
      return false;
    });
    renderCommands(filtered);
  });

  // --- ファイルブラウザ処理 ---
  async function loadDirectory(dirPath) {
    if (typeof window.listFiles !== 'function') {
      fileList.innerHTML = '<div class="list-item"><span class="item-name">（ファイル一覧取得不可）</span></div>';
      return;
    }
    try {
      fileList.innerHTML = '<div class="list-item"><span class="item-name">読み込み中...</span></div>';
      const res = await window.listFiles(dirPath);
      const data = typeof res === 'string' ? JSON.parse(res) : res;

      if (data.error) {
        fileList.innerHTML = `<div class="list-item" style="color:var(--accent-pink);"><span class="item-name">エラー: ${escapeHtml(data.error)}</span></div>`;
        return;
      }

      currentDirPath = data.currentDir;
      parentDirPath = data.parentDir;
      currentDirDisplay.textContent = currentDirPath;
      currentDirDisplay.title = currentDirPath;

      fileList.innerHTML = '';
      if (!data.items || data.items.length === 0) {
        fileList.innerHTML = '<div class="list-item"><span class="item-name" style="color:var(--text-muted);">（ファイルがありません）</span></div>';
        return;
      }

      data.items.forEach(file => {
        const item = document.createElement('div');
        item.className = 'list-item';
        const icon = file.isDir ? '📁' : (file.name.endsWith('.nako3') ? '🌸' : '📄');

        item.innerHTML = `
          <div class="item-left">
            <span class="item-icon">${icon}</span>
            <span class="item-name">${escapeHtml(file.name)}</span>
          </div>
          <span class="file-item-ext">${file.isDir ? 'フォルダ' : formatBytes(file.size)}</span>
        `;

        item.addEventListener('click', () => {
          if (file.isDir) {
            loadDirectory(file.path);
          } else {
            openFile(file.path, file.name);
          }
        });

        fileList.appendChild(item);
      });
    } catch (err) {
      console.error('ファイル一覧エラー:', err);
    }
  }

  btnFileUp.addEventListener('click', () => {
    if (parentDirPath && parentDirPath !== currentDirPath) {
      loadDirectory(parentDirPath);
    }
  });

  btnFileHome.addEventListener('click', () => {
    loadDirectory(homeDirPath || '$HOME');
  });

  async function openFile(filePath, fileName) {
    if (typeof window.readFile !== 'function') return;
    try {
      setStatus(`ファイル読み込み中: ${fileName}...`);
      const res = await window.readFile(filePath);
      const data = typeof res === 'string' ? JSON.parse(res) : res;
      if (data.ok) {
        editor.value = data.content;
        currentFilePath = filePath;
        activeFileName.textContent = fileName;
        activeFileName.title = filePath;
        updateLineNumbers();
        updateCharCount();
        updateCursorPos();
        setStatus(`開きました: ${fileName}`);
      } else {
        alert(`ファイルを開けませんでした: ${data.error}`);
        setStatus(`エラー: ${data.error}`);
      }
    } catch (err) {
      console.error('ファイルオープンエラー:', err);
    }
  }

  async function saveFile() {
    if (!currentFilePath) {
      const name = prompt('保存するファイル名（例: my_script.nako3）:', 'program.nako3');
      if (!name) return;
      currentFilePath = (currentDirPath || homeDirPath) + '/' + name;
      activeFileName.textContent = name;
      activeFileName.title = currentFilePath;
    }

    if (typeof window.saveFile !== 'function') return;
    try {
      setStatus(`保存中: ${currentFilePath}...`);
      const res = await window.saveFile(currentFilePath, editor.value);
      const data = typeof res === 'string' ? JSON.parse(res) : res;
      if (data.ok) {
        const now = new Date();
        const savedAt = [now.getHours(), now.getMinutes(), now.getSeconds()]
          .map(value => String(value).padStart(2, '0'))
          .join(':');
        setStatus(`${savedAt} 保存しました！`);
        if (tabContentFile.classList.contains('active')) {
          loadDirectory(currentDirPath);
        }
      } else {
        alert(`保存に失敗しました: ${data.error}`);
        setStatus(`保存エラー: ${data.error}`);
      }
    } catch (err) {
      console.error('保存エラー:', err);
    }
  }

  btnSave.addEventListener('click', saveFile);

  // --- エディタ操作 ---
  function updateLineNumbers() {
    const lines = editor.value.split('\n').length;
    let numbers = '';
    for (let i = 1; i <= lines; i++) {
      numbers += i + '\n';
    }
    lineNumbers.textContent = numbers;
  }

  function updateCharCount() {
    charCount.textContent = `${editor.value.length} 文字 / ${editor.value.split('\n').length} 行`;
  }

  function updateCursorPos() {
    const text = editor.value.substring(0, editor.selectionStart);
    const lines = text.split('\n');
    const row = lines.length;
    const col = lines[lines.length - 1].length + 1;
    cursorPos.textContent = `行: ${row}, 列: ${col}`;
  }

  function insertTextAtCursor(text) {
    const start = editor.selectionStart;
    const end = editor.selectionEnd;
    editor.value = editor.value.substring(0, start) + text + editor.value.substring(end);
    editor.selectionStart = editor.selectionEnd = start + text.length;
    editor.focus();
    updateLineNumbers();
    updateCharCount();
    updateCursorPos();
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

  // ショートカットキー対応
  editor.addEventListener('keydown', (e) => {
    if (e.key === 'Tab') {
      e.preventDefault();
      insertTextAtCursor('\t');
    }
    // Ctrl+R / Cmd+R / Ctrl+Enter / Cmd+Enter で実行
    if ((e.ctrlKey || e.metaKey) && (e.key === 'Enter' || e.key === 'r' || e.key === 'R')) {
      e.preventDefault();
      runCode();
    }
    // Ctrl+S / Cmd+S で保存
    if ((e.ctrlKey || e.metaKey) && (e.key === 's' || e.key === 'S')) {
      e.preventDefault();
      saveFile();
    }
  });

  // サンプル選択
  selectSample.addEventListener('change', (e) => {
    const key = e.target.value;
    if (key && samples[key]) {
      editor.value = samples[key];
      currentFilePath = '';
      activeFileName.textContent = `サンプル (${e.target.options[e.target.selectedIndex].text})`;
      updateLineNumbers();
      updateCharCount();
      updateCursorPos();
      setStatus(`サンプル「${e.target.options[e.target.selectedIndex].text}」を読み込みました`);
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
    setStatus('ログを消去しました');
  });

  // ログコピー
  btnCopyLog.addEventListener('click', () => {
    navigator.clipboard.writeText(output.textContent).then(() => {
      btnCopyLog.textContent = 'コピー完了!';
      setTimeout(() => {
        btnCopyLog.innerHTML = '📋 コピー';
      }, 1500);
    });
  });

  // --- なでしこプログラム実行処理 ---
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
    setStatus('実行中...');

    const startTime = performance.now();

    try {
      let result;
      if (typeof window.runNakoCode === 'function') {
        result = await window.runNakoCode(code);
      } else {
        result = JSON.stringify({
          ok: true,
          output: "※ WebViewバインディング準備中...\n" + code,
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
        setStatus(`実行エラー (${elapsed}秒)`);
      } else {
        output.textContent = data.output || '（出力なし）';
        output.className = 'output has-content';
        execStatus.textContent = `完了 (${elapsed}s)`;
        execStatus.className = 'status-indicator success';
        setStatus(`実行完了 (${elapsed}秒)`);
      }
    } catch (err) {
      output.textContent = `[システムエラー] ${err.message || err}`;
      output.className = 'output has-error';
      execStatus.textContent = 'エラー';
      execStatus.className = 'status-indicator error';
      setStatus(`システムエラー: ${err}`);
    } finally {
      btnRun.disabled = false;
    }
  }

  function setStatus(msg) {
    statusMsg.textContent = msg;
  }

  // --- スプリッター処理 ---
  // 垂直スプリッター (左サイドバー / 右メイン)
  const splitterV = document.getElementById('splitter-v');
  const sidebar = document.getElementById('sidebar');
  let isDraggingV = false;

  splitterV.addEventListener('mousedown', () => {
    isDraggingV = true;
    splitterV.classList.add('dragging');
    document.body.style.cursor = 'col-resize';
  });

  // 水平スプリッター (上エディタ / 下コンソール)
  const splitterH = document.getElementById('splitter-h');
  const paneEditor = document.querySelector('.pane-editor');
  const mainPane = document.querySelector('.main-pane');
  let isDraggingH = false;

  splitterH.addEventListener('mousedown', () => {
    isDraggingH = true;
    splitterH.classList.add('dragging');
    document.body.style.cursor = 'row-resize';
  });

  window.addEventListener('mousemove', (e) => {
    if (isDraggingV) {
      const containerRect = document.querySelector('.workspace').getBoundingClientRect();
      const newWidth = e.clientX - containerRect.left;
      if (newWidth >= 160 && newWidth <= 600) {
        sidebar.style.width = `${newWidth}px`;
      }
    }
    if (isDraggingH) {
      const paneRect = mainPane.getBoundingClientRect();
      const newHeight = e.clientY - paneRect.top;
      if (newHeight >= 100 && newHeight <= paneRect.height - 80) {
        paneEditor.style.flex = 'none';
        paneEditor.style.height = `${newHeight}px`;
      }
    }
  });

  window.addEventListener('mouseup', () => {
    if (isDraggingV) {
      isDraggingV = false;
      splitterV.classList.remove('dragging');
      document.body.style.cursor = '';
    }
    if (isDraggingH) {
      isDraggingH = false;
      splitterH.classList.remove('dragging');
      document.body.style.cursor = '';
    }
  });

  // --- ユーティリティ ---
  function escapeHtml(str) {
    if (!str) return '';
    return str.replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }

  function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  }

  // --- 初期化呼び出し ---
  if (typeof window.getAppInfo === 'function') {
    window.getAppInfo().then(infoStr => {
      try {
        const info = JSON.parse(infoStr);
        versionInfo.textContent = `gonako-gui v${info.version || '3.6.0'} (${info.os}/${info.arch})`;
        homeDirPath = info.homeDir || '';
        loadCommands();
      } catch {
        versionInfo.textContent = `gonako-gui`;
      }
    });
  } else {
    loadCommands();
  }
});
