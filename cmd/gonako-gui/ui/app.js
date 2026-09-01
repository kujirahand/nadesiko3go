// なでしこ3 GUI (gonako-gui) Frontend Script

const templateList = [
  {
    id: 'hello',
    category: '基本',
    title: 'こんにちは (基本構文)',
    desc: '画面への文字列表示とループの基本',
    code: `// なでしこ3の基本
「こんにちは、なでしこ！」と表示。
3回
　　「{回数}回目の挨拶」と表示。
ここまで`
  },
  {
    id: 'fizzbuzz',
    category: '制御構文',
    title: 'FizzBuzz (繰り返しと分岐)',
    desc: '1から30までのFizzBuzzを計算して出力',
    code: `// 1から30までのFizzBuzz
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
結果を「, 」で配列結合して表示。`
  },
  {
    id: 'calc',
    category: 'データ構造',
    title: '計算と変数・辞書・配列',
    desc: '辞書オブジェクトや配列の操作',
    code: `// 変数と計算、辞書
A = 100
B = 200
C = A + B
「{A} + {B} = {C}」を表示。

名簿 = {}
名簿["名前"] = "太郎"
名簿["年齢"] = 25
名簿["趣味"] = ["プログラミング", "読書", "旅行"]

「名前: {名簿["名前"]}」と表示。
「趣味数: {名簿["趣味"]の要素数}」と表示。`
  },
  {
    id: 'dialog',
    category: 'システム',
    title: 'ファイル・ハッシュ・UUID',
    desc: 'システム情報取得やSHA256ハッシュ計算',
    code: `// ファイル・ハッシュ・UUID
Msg = 「こんにちは」
Sha = Msgを「sha256」でハッシュ値計算
「SHA256: {Sha}」と表示。

U = ランダムUUID生成
「UUID: {U}」と表示。

「カレント: {カレントディレクトリ取得}」と表示。
「OS: {OS取得} ({OSアーキテクチャ取得})」と表示。`
  },
  {
    id: 'csv',
    category: 'ファイル処理',
    title: 'CSVの生成と解析',
    desc: '2次元配列からCSV変換とCSVパース',
    code: `// CSVの生成と解析
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
  },
  {
    id: 'excel',
    category: 'オフィス',
    title: 'Excelブックの作成 (officelib)',
    desc: 'Excelファイルのセル設定・計算・保存',
    code: `// Excelファイルの作成
Book = エクセルブック作成
Bookの「Sheet1」の「A1」に「品名」をエクセルセル設定
Bookの「Sheet1」の「B1」に「価格」をエクセルセル設定
Bookの「Sheet1」の「A2」に「りんご」をエクセルセル設定
Bookの「Sheet1」の「B2」に150をエクセルセル設定

Bookを「test.xlsx」へエクセル保存
「Excel保存完了: {"test.xlsx"が存在}」と表示。`
  },
  {
    id: 'pdf',
    category: 'オフィス',
    title: 'PDFドキュメント作成 (pdflib)',
    desc: 'PDFページの作成・テキスト描画・保存',
    code: `// PDFドキュメント作成
Doc = PDF新規作成
Docにページ追加
Docの「Hello, Nadesiko3 Go!」をテキスト描画
Docを「output.pdf」へPDF保存
「PDF出力完了: {"output.pdf"が存在}」と表示。`
  },
  {
    id: 'image',
    category: 'グラフィック',
    title: '画像の生成と保存 (imagelib)',
    desc: '画像キャンバスの作成・塗りつぶし・保存',
    code: `// 画像の新規作成と保存
Img = [200, 200]の画像新規作成
Imgの[20, 20, 160, 160]を"#f38ba8"で四角描画
Imgを「image.png」へ画像保存
「画像保存完了: {"image.png"が存在}」と表示。`
  },
  {
    id: 'window_gui',
    category: 'GUI',
    title: 'ウィンドウGUIアプリ (HTML/DOM)',
    desc: 'ボタンや入力フォームを持ったGUI画面',
    code: `// ウィンドウアプリ用コード
「<h1>なでしこ3 GUIアプリ</h1>
<p>ボタンを押してテストしてください。</p>
<button id='btn1' style='padding:8px 16px;background:#f38ba8;color:#fff;border:none;border-radius:4px;cursor:pointer;'>クリック</button>
<div id='res' style='margin-top:10px;font-weight:bold;'></div>
<script>
document.getElementById('btn1').onclick = function() {
  document.getElementById('res').innerText = 'ボタンがクリックされました！ (' + new Date().toLocaleTimeString() + ')';
};
</script>」を表示。`
  }
];

document.addEventListener('DOMContentLoaded', () => {
  const editor = document.getElementById('editor');
  const lineNumbers = document.getElementById('line-numbers');
  const output = document.getElementById('output');
  const windowPreview = document.getElementById('window-preview');
  const btnRun = document.getElementById('btn-run');
  const btnSave = document.getElementById('btn-save');
  const btnClearLog = document.getElementById('btn-clear-log');
  const btnCopyLog = document.getElementById('btn-copy-log');
  const selectAppType = document.getElementById('select-app-type');
  const modeBadge = document.getElementById('mode-badge');
  const charCount = document.getElementById('char-count');
  const cursorPos = document.getElementById('cursor-pos');
  const execStatus = document.getElementById('exec-status');
  const statusMsg = document.getElementById('status-msg');
  const versionInfo = document.getElementById('version-info');
  const activeFileName = document.getElementById('active-file-name');

  // タブボタン
  const tabBtnCmd = document.getElementById('tab-btn-cmd');
  const tabBtnTemplate = document.getElementById('tab-btn-template');
  const tabBtnFile = document.getElementById('tab-btn-file');

  // タブコンテンツ
  const tabContentCmd = document.getElementById('tab-content-cmd');
  const tabContentTemplate = document.getElementById('tab-content-template');
  const tabContentFile = document.getElementById('tab-content-file');

  // 命令タブ要素
  const cmdSearch = document.getElementById('cmd-search');
  const cmdCount = document.getElementById('cmd-count');
  const cmdList = document.getElementById('cmd-list');

  // ひな形タブ要素
  const templateSearch = document.getElementById('template-search');
  const templateCount = document.getElementById('template-count');
  const templateListElem = document.getElementById('template-list');

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
  editor.value = templateList[0].code;
  updateLineNumbers();
  updateCharCount();

  // --- タブ切り替え処理 ---
  function activateTab(activeBtn, activeContent) {
    [tabBtnCmd, tabBtnTemplate, tabBtnFile].forEach(btn => btn.classList.remove('active'));
    [tabContentCmd, tabContentTemplate, tabContentFile].forEach(c => c.classList.remove('active'));
    activeBtn.classList.add('active');
    activeContent.classList.add('active');
  }

  tabBtnCmd.addEventListener('click', () => activateTab(tabBtnCmd, tabContentCmd));
  tabBtnTemplate.addEventListener('click', () => {
    activateTab(tabBtnTemplate, tabContentTemplate);
    renderTemplates(templateList);
  });
  tabBtnFile.addEventListener('click', () => {
    activateTab(tabBtnFile, tabContentFile);
    if (!currentDirPath) {
      loadDirectory(homeDirPath || '$HOME');
    }
  });

  // --- アプリ種類選択 ---
  selectAppType.addEventListener('change', () => {
    const isWindow = selectAppType.value === 'window';
    modeBadge.textContent = isWindow ? 'ウィンドウ' : 'コマンドライン';
    modeBadge.style.color = isWindow ? 'var(--accent-pink)' : 'var(--accent-teal)';
    modeBadge.style.background = isWindow ? 'rgba(243, 139, 168, 0.15)' : 'rgba(148, 226, 213, 0.12)';
    setStatus(`アプリ種類を「${isWindow ? 'ウィンドウ' : 'コマンドライン'}」に変更しました`);
  });

  // --- ひな形一覧の表示と検索 ---
  function renderTemplates(list) {
    templateListElem.innerHTML = '';
    templateCount.textContent = `${list.length}件`;

    list.forEach(t => {
      const item = document.createElement('div');
      item.className = 'list-item';
      item.title = `クリックでエディタに読み込み: ${t.title}`;

      item.innerHTML = `
        <div class="item-left">
          <span class="item-icon">📄</span>
          <div>
            <span class="item-name">${escapeHtml(t.title)}</span>
            <span class="item-desc">${escapeHtml(t.desc)}</span>
          </div>
        </div>
        <span class="template-badge">${escapeHtml(t.category)}</span>
      `;

      item.addEventListener('click', () => {
        editor.value = t.code;
        currentFilePath = '';
        activeFileName.textContent = t.title;
        updateLineNumbers();
        updateCharCount();
        updateCursorPos();
        setStatus(`ひな形「${t.title}」を読み込みました`);
      });

      templateListElem.appendChild(item);
    });
  }

  templateSearch.addEventListener('input', () => {
    const q = templateSearch.value.trim().toLowerCase();
    if (!q) {
      renderTemplates(templateList);
      return;
    }
    const filtered = templateList.filter(t => 
      t.title.toLowerCase().includes(q) || 
      t.desc.toLowerCase().includes(q) || 
      t.category.toLowerCase().includes(q)
    );
    renderTemplates(filtered);
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
        setStatus(`保存完了: ${activeFileName.textContent}`);
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
    if ((e.ctrlKey || e.metaKey) && (e.key === 'Enter' || e.key === 'r' || e.key === 'R')) {
      e.preventDefault();
      runCode();
    }
    if ((e.ctrlKey || e.metaKey) && (e.key === 's' || e.key === 'S')) {
      e.preventDefault();
      saveFile();
    }
  });

  // 実行ボタン
  btnRun.addEventListener('click', runCode);

  // ログクリア
  btnClearLog.addEventListener('click', () => {
    output.textContent = '';
    output.className = 'output';
    windowPreview.innerHTML = '';
    windowPreview.style.display = 'none';
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

    const isWindowMode = selectAppType.value === 'window';
    execStatus.textContent = '実行中...';
    execStatus.className = 'status-indicator running';
    btnRun.disabled = true;
    setStatus(`実行中 (${isWindowMode ? 'ウィンドウ' : 'コマンドライン'})...`);

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
        windowPreview.style.display = 'none';
      } else {
        output.textContent = data.output || '（出力なし）';
        output.className = 'output has-content';
        execStatus.textContent = `完了 (${elapsed}s)`;
        execStatus.className = 'status-indicator success';
        setStatus(`実行完了 (${elapsed}秒)`);

        // ウィンドウモードの場合、HTMLタグが含まれていればプレビュー領域にレンダリング
        if (isWindowMode && data.output && /<[a-z][\s\S]*>/i.test(data.output)) {
          windowPreview.style.display = 'block';
          windowPreview.innerHTML = data.output;
          // scriptタグの動的実行
          const scripts = windowPreview.querySelectorAll('script');
          scripts.forEach(oldScript => {
            const newScript = document.createElement('script');
            Array.from(oldScript.attributes).forEach(attr => newScript.setAttribute(attr.name, attr.value));
            newScript.appendChild(document.createTextNode(oldScript.innerHTML));
            oldScript.parentNode.replaceChild(newScript, oldScript);
          });
        } else {
          windowPreview.style.display = 'none';
        }
      }
    } catch (err) {
      output.textContent = `[システムエラー] ${err.message || err}`;
      output.className = 'output has-error';
      execStatus.textContent = 'エラー';
      execStatus.className = 'status-indicator error';
      setStatus(`システムエラー: ${err}`);
      windowPreview.style.display = 'none';
    } finally {
      btnRun.disabled = false;
    }
  }

  function setStatus(msg) {
    statusMsg.textContent = msg;
  }

  // --- スプリッター処理 ---
  const splitterV = document.getElementById('splitter-v');
  const sidebar = document.getElementById('sidebar');
  let isDraggingV = false;

  splitterV.addEventListener('mousedown', () => {
    isDraggingV = true;
    splitterV.classList.add('dragging');
    document.body.style.cursor = 'col-resize';
  });

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
  renderTemplates(templateList);

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
