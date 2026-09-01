// なでしこ3 GUI (gonako-gui) Frontend Script

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

  // ハンバーガーメニュー & モーダル要素
  const btnHamburger = document.getElementById('btn-hamburger');
  const hamburgerMenu = document.getElementById('hamburger-menu');
  const menuItemShortcuts = document.getElementById('menu-item-shortcuts');
  const menuItemAbout = document.getElementById('menu-item-about');
  const modalOverlay = document.getElementById('modal-overlay');
  const modalClose = document.getElementById('modal-close');
  const modalTitle = document.getElementById('modal-title');
  const modalBody = document.getElementById('modal-body');

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
  let allTemplates = [];
  let currentDirPath = '';
  let parentDirPath = '';
  let homeDirPath = '';
  let currentFilePath = '';

  // 初期プレースホルダー
  editor.value = `// なでしこ3の基本\n「こんにちは、なでしこ！」と表示。`;
  updateLineNumbers();
  updateCharCount();

  // --- タブ切り替え処理 ---
  function activateTab(activeBtn, activeContent) {
    [tabBtnCmd, tabBtnTemplate, tabBtnFile].forEach(btn => btn.classList.remove('active'));
    [tabContentCmd, tabContentTemplate, tabContentFile].forEach(c => c.classList.remove('active'));
    activeBtn.classList.add('active');
    activeContent.classList.add('active');
  }

  tabBtnCmd.addEventListener('click', () => {
    activateTab(tabBtnCmd, tabContentCmd);
    if (allCommands.length === 0) {
      loadCommands();
    }
  });

  tabBtnTemplate.addEventListener('click', () => {
    activateTab(tabBtnTemplate, tabContentTemplate);
    if (allTemplates.length === 0) {
      loadTemplates();
    } else {
      renderTemplates(allTemplates);
    }
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

  // --- ハンバーガーメニュー・モーダル処理 ---
  function toggleHamburger() {
    const isShown = hamburgerMenu.style.display === 'flex';
    hamburgerMenu.style.display = isShown ? 'none' : 'flex';
  }

  function closeHamburger() {
    if (hamburgerMenu) hamburgerMenu.style.display = 'none';
  }

  btnHamburger.addEventListener('click', (e) => {
    e.stopPropagation();
    toggleHamburger();
  });

  document.addEventListener('click', (e) => {
    if (hamburgerMenu && !hamburgerMenu.contains(e.target) && e.target !== btnHamburger) {
      closeHamburger();
    }
  });

  function openShortcutsModal() {
    closeHamburger();
    modalTitle.textContent = 'ショートカットキー一覧';
    modalBody.innerHTML = `
      <table class="shortcuts-table">
        <thead>
          <tr>
            <th>ショートカットキー</th>
            <th>動作</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td><kbd>F5</kbd></td>
            <td>プログラムを実行</td>
          </tr>
          <tr>
            <td><kbd>Ctrl</kbd> + <kbd>R</kbd> / <kbd>Cmd</kbd> + <kbd>R</kbd></td>
            <td>プログラムを実行</td>
          </tr>
          <tr>
            <td><kbd>Ctrl</kbd> + <kbd>Enter</kbd> / <kbd>Cmd</kbd> + <kbd>Enter</kbd></td>
            <td>プログラムを実行</td>
          </tr>
          <tr>
            <td><kbd>Ctrl</kbd> + <kbd>S</kbd> / <kbd>Cmd</kbd> + <kbd>S</kbd></td>
            <td>ファイルを上書き保存</td>
          </tr>
          <tr>
            <td><kbd>Ctrl</kbd> + <kbd>C</kbd> / <kbd>Cmd</kbd> + <kbd>C</kbd></td>
            <td>選択範囲をコピー</td>
          </tr>
          <tr>
            <td><kbd>Ctrl</kbd> + <kbd>V</kbd> / <kbd>Cmd</kbd> + <kbd>V</kbd></td>
            <td>クリップボードから貼り付け</td>
          </tr>
          <tr>
            <td><kbd>Ctrl</kbd> + <kbd>X</kbd> / <kbd>Cmd</kbd> + <kbd>X</kbd></td>
            <td>選択範囲を切り取り</td>
          </tr>
          <tr>
            <td><kbd>Ctrl</kbd> + <kbd>A</kbd> / <kbd>Cmd</kbd> + <kbd>A</kbd></td>
            <td>すべて選択</td>
          </tr>
          <tr>
            <td><kbd>Tab</kbd></td>
            <td>タブ文字（インデント）を挿入</td>
          </tr>
          <tr>
            <td><kbd>Ctrl</kbd> + <kbd>L</kbd></td>
            <td>実行ログを消去</td>
          </tr>
          <tr>
            <td><kbd>Esc</kbd></td>
            <td>ダイアログを閉じる</td>
          </tr>
        </tbody>
      </table>
    `;
    modalOverlay.style.display = 'flex';
  }

  function openAboutModal() {
    closeHamburger();
    modalTitle.textContent = 'バージョン情報';
    modalBody.innerHTML = `
      <div style="text-align:center;padding:12px 0;">
        <div style="font-size:36px;margin-bottom:8px;">🌸</div>
        <h4 style="font-size:16px;color:var(--accent-pink);margin-bottom:6px;">なでしこ3 GUI (gonako-gui)</h4>
        <p style="font-size:12px;color:var(--text-muted);margin-bottom:12px;">バージョン: v3.6.0 (Go言語版)</p>
        <p style="font-size:12px;line-height:1.6;color:var(--text-main);">
          日本語プログラミング言語「なでしこ3」のGoネイティブデスクトップGUI環境です。
        </p>
      </div>
    `;
    modalOverlay.style.display = 'flex';
  }

  function closeModal() {
    modalOverlay.style.display = 'none';
  }

  menuItemShortcuts.addEventListener('click', openShortcutsModal);
  menuItemAbout.addEventListener('click', openAboutModal);
  modalClose.addEventListener('click', closeModal);
  modalOverlay.addEventListener('click', (e) => {
    if (e.target === modalOverlay) closeModal();
  });

  // --- ひな形一覧の読み込みと検索 ---
  async function loadTemplates() {
    if (typeof window.getTemplateList === 'function') {
      try {
        const res = await window.getTemplateList();
        allTemplates = typeof res === 'string' ? JSON.parse(res) : res;
      } catch (err) {
        console.error('ひな形読み込みエラー:', err);
      }
    }
    if (allTemplates && allTemplates.length > 0) {
      renderTemplates(allTemplates);
      // 初回起動時、先頭のひな形をエディタにセット
      if (editor.value.includes('なでしこ3の基本') && allTemplates[0].code) {
        editor.value = allTemplates[0].code;
        activeFileName.textContent = allTemplates[0].title;
        updateLineNumbers();
        updateCharCount();
      }
    }
  }

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
      renderTemplates(allTemplates);
      return;
    }
    const filtered = allTemplates.filter(t => 
      t.title.toLowerCase().includes(q) || 
      t.desc.toLowerCase().includes(q) || 
      t.category.toLowerCase().includes(q)
    );
    renderTemplates(filtered);
  });

  // --- 命令一覧の読み込みと検索 ---
  async function loadCommands() {
    if (typeof window.getCommandList === 'function') {
      try {
        const res = await window.getCommandList();
        allCommands = typeof res === 'string' ? JSON.parse(res) : res;
      } catch (err) {
        console.error('命令一覧の読み込みエラー:', err);
      }
    } else {
      // ローカルJSONファイルから取得を試行
      try {
        const res = await fetch('command-list.json');
        allCommands = await res.json();
      } catch (err) {}
    }
    if (allCommands && allCommands.length > 0) {
      renderCommands(allCommands);
    }
  }

  // 命令の使い方を実行結果コンソールに表示
  function displayCommandHelp(cmd) {
    let josiText = '';
    if (cmd.josi && cmd.josi.length > 0) {
      josiText = cmd.josi.map(group => `[${group.join(', ')}]`).join(' ');
    } else {
      josiText = '（助詞なし）';
    }

    const template = cmd.template || cmd.name;
    const desc = cmd.desc || '（説明はありません）';
    const category = cmd.category || '基本';
    const yomi = cmd.yomi ? ` (${cmd.yomi})` : '';

    const helpText = [
      `━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`,
      `📖 命令: ${cmd.name}${yomi}`,
      `━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`,
      `【構文】 ${template}`,
      `【助詞】 ${josiText}`,
      `【分類】 ${category}`,
      `【説明】 ${desc}`,
      ``,
      `※ ダブルクリックまたはエディタへのドラッグ＆ドロップで構文を挿入できます。`
    ].join('\n');

    output.textContent = helpText;
    output.className = 'output has-content';
    windowPreview.style.display = 'none';
    execStatus.textContent = '使い方表示';
    execStatus.className = 'status-indicator';
  }

  function renderCommands(commands) {
    cmdList.innerHTML = '';
    cmdCount.textContent = `${commands.length}件`;

    commands.forEach(cmd => {
      const item = document.createElement('div');
      item.className = 'list-item';
      item.draggable = true;
      item.title = `クリック: 使い方を表示 / ダブルクリックまたはドラッグ: 構文を挿入 (${cmd.name})`;

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

      // 1. シングルクリック: 使い方を表示
      item.addEventListener('click', (e) => {
        document.querySelectorAll('#cmd-list .list-item').forEach(el => el.classList.remove('selected'));
        item.classList.add('selected');
        displayCommandHelp(cmd);
        setStatus(`命令「${cmd.name}」の使い方を表示しました (ダブルクリックまたはDnDで挿入)`);
      });

      // 2. ダブルクリック: エディタに「助詞+命令」テンプレートを挿入
      item.addEventListener('dblclick', (e) => {
        e.preventDefault();
        const template = cmd.template || cmd.name;
        insertTextAtCursor(template);
        displayCommandHelp(cmd);
        setStatus(`命令「${cmd.name}」の構文をエディタに挿入しました`);
      });

      // 3. ドラッグ開始: テンプレート文字列とメタデータを設定
      item.addEventListener('dragstart', (e) => {
        const template = cmd.template || cmd.name;
        e.dataTransfer.setData('text/plain', template);
        e.dataTransfer.setData('application/json', JSON.stringify(cmd));
        e.dataTransfer.effectAllowed = 'copy';
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
      if (c.desc && c.desc.toLowerCase().includes(query)) return true;
      if (c.category && c.category.toLowerCase().includes(query)) return true;
      if (c.yomi && c.yomi.toLowerCase().includes(query)) return true;
      if (c.josi && c.josi.some(group => group.some(j => j.toLowerCase().includes(query)))) return true;
      return false;
    });
    renderCommands(filtered);
  });

  // --- エディタへのドラッグ＆ドロップ対応 ---
  editor.addEventListener('dragover', (e) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'copy';
    editor.classList.add('drag-over');
  });

  editor.addEventListener('dragleave', () => {
    editor.classList.remove('drag-over');
  });

  editor.addEventListener('drop', (e) => {
    e.preventDefault();
    editor.classList.remove('drag-over');

    const text = e.dataTransfer.getData('text/plain');
    const jsonStr = e.dataTransfer.getData('application/json');

    if (jsonStr) {
      try {
        const cmd = JSON.parse(jsonStr);
        displayCommandHelp(cmd);
      } catch (err) {}
    }

    if (text) {
      insertTextAtCursor(text);
      setStatus(`構文「${text}」を挿入しました`);
    }
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

  // グローバルおよびエディタのキーボードショートカット対応
  window.addEventListener('keydown', async (e) => {
    const isCmdOrCtrl = e.ctrlKey || e.metaKey;
    const activeEl = document.activeElement;
    const isInput = activeEl && (activeEl.tagName === 'INPUT' || activeEl.tagName === 'TEXTAREA');

    // F5 で実行
    if (e.key === 'F5') {
      e.preventDefault();
      runCode();
      return;
    }
    // Ctrl+R / Cmd+R で実行
    if (isCmdOrCtrl && (e.key === 'r' || e.key === 'R')) {
      e.preventDefault();
      runCode();
      return;
    }
    // Ctrl+Enter / Cmd+Enter で実行
    if (isCmdOrCtrl && e.key === 'Enter') {
      e.preventDefault();
      runCode();
      return;
    }
    // Ctrl+S / Cmd+S で保存
    if (isCmdOrCtrl && (e.key === 's' || e.key === 'S')) {
      e.preventDefault();
      saveFile();
      return;
    }
    // Ctrl+L でログ消去
    if (isCmdOrCtrl && (e.key === 'l' || e.key === 'L')) {
      e.preventDefault();
      btnClearLog.click();
      return;
    }
    // Esc でモーダルを閉じる
    if (e.key === 'Escape') {
      closeModal();
      closeHamburger();
      return;
    }

    // [Cmd]+[A] / [Ctrl]+[A] --- 全選択 (フォーカス中のinput/textarea、またはエディタ)
    if (isCmdOrCtrl && (e.key === 'a' || e.key === 'A')) {
      if (isInput) {
        e.preventDefault();
        activeEl.select();
      } else {
        e.preventDefault();
        editor.focus();
        editor.select();
      }
      return;
    }

    // [Cmd]+[C] / [Ctrl]+[C] --- コピー (フォーカス中のinput/textarea)
    if (isCmdOrCtrl && (e.key === 'c' || e.key === 'C')) {
      const target = isInput ? activeEl : editor;
      const start = target.selectionStart;
      const end = target.selectionEnd;
      if (start !== undefined && end !== undefined && start !== end) {
        const selectedText = target.value.substring(start, end);
        try {
          if (navigator.clipboard && navigator.clipboard.writeText) {
            await navigator.clipboard.writeText(selectedText);
            setStatus('選択範囲をコピーしました');
          }
        } catch (err) {}
      }
      return;
    }

    // [Cmd]+[X] / [Ctrl]+[X] --- 切り取り (フォーカス中のinput/textarea)
    if (isCmdOrCtrl && (e.key === 'x' || e.key === 'X')) {
      const target = isInput ? activeEl : editor;
      const start = target.selectionStart;
      const end = target.selectionEnd;
      if (start !== undefined && end !== undefined && start !== end) {
        const selectedText = target.value.substring(start, end);
        try {
          if (navigator.clipboard && navigator.clipboard.writeText) {
            await navigator.clipboard.writeText(selectedText);
          }
          e.preventDefault();
          target.value = target.value.substring(0, start) + target.value.substring(end);
          target.selectionStart = target.selectionEnd = start;
          if (target === editor) {
            updateLineNumbers();
            updateCharCount();
            updateCursorPos();
          }
          target.dispatchEvent(new Event('input', { bubbles: true }));
          setStatus('選択範囲を切り取りました');
        } catch (err) {}
      }
      return;
    }

    // [Cmd]+[V] / [Ctrl]+[V] --- 貼り付け (フォーカス中のinput/textarea)
    if (isCmdOrCtrl && (e.key === 'v' || e.key === 'V')) {
      const target = isInput ? activeEl : editor;
      try {
        if (navigator.clipboard && navigator.clipboard.readText) {
          const text = await navigator.clipboard.readText();
          if (text) {
            e.preventDefault();
            const start = target.selectionStart !== undefined ? target.selectionStart : target.value.length;
            const end = target.selectionEnd !== undefined ? target.selectionEnd : start;
            target.value = target.value.substring(0, start) + text + target.value.substring(end);
            target.selectionStart = target.selectionEnd = start + text.length;
            if (target === editor) {
              updateLineNumbers();
              updateCharCount();
              updateCursorPos();
            }
            target.dispatchEvent(new Event('input', { bubbles: true }));
            setStatus('クリップボードから貼り付けました');
          }
        }
      } catch (err) {}
      return;
    }
  });

  editor.addEventListener('keydown', (e) => {
    if (e.key === 'Tab') {
      e.preventDefault();
      insertTextAtCursor('\t');
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
  loadTemplates();
  loadCommands();

  if (typeof window.getAppInfo === 'function') {
    window.getAppInfo().then(infoStr => {
      try {
        const info = JSON.parse(infoStr);
        versionInfo.textContent = `gonako-gui v${info.version || '3.6.0'} (${info.os}/${info.arch})`;
        homeDirPath = info.homeDir || '';
      } catch {
        versionInfo.textContent = `gonako-gui`;
      }
    });
  }
});
