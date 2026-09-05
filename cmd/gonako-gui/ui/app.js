// なでしこ3 GUI (gonako-gui) Frontend Script

document.addEventListener('DOMContentLoaded', () => {
  const editor = document.getElementById('editor');
  const lineNumbers = document.getElementById('line-numbers');
  // シンタックスハイライト部品 (editor-highlight.js)。
  // 別のエディタ部品に差し替えるときは、ここで生成するオブジェクトを
  // refresh() / setEnabled() / destroy() を持つ実装に置き換えればよい。
  const highlighter = (window.GonakoHighlighter && window.GonakoHighlighter.create(editor)) || null;
  const output = document.getElementById('output');
  const windowPreview = document.getElementById('window-preview');
  const btnRun = document.getElementById('btn-run');
  const btnNew = document.getElementById('btn-new');
  const btnOpen = document.getElementById('btn-open');
  const btnSave = document.getElementById('btn-save');
  const btnToggleSidebar = document.getElementById('btn-toggle-sidebar');
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

  // サイドバー & スプリッター要素
  const sidebar = document.getElementById('sidebar');
  const splitterV = document.getElementById('splitter-v');

  // ハンバーガーメニュー & モーダル要素
  const btnHamburger = document.getElementById('btn-hamburger');
  const hamburgerMenu = document.getElementById('hamburger-menu');
  const menuItemNew = document.getElementById('menu-item-new');
  const menuItemOpen = document.getElementById('menu-item-open');
  const menuItemSave = document.getElementById('menu-item-save');
  const menuItemSaveAs = document.getElementById('menu-item-save-as');
  const menuItemShortcuts = document.getElementById('menu-item-shortcuts');
  const menuItemAbout = document.getElementById('menu-item-about');
  const menuItemBuildApp = document.getElementById('menu-item-build-app');
  const menuItemGoBuild = document.getElementById('menu-item-go-build');
  const modalOverlay = document.getElementById('modal-overlay');
  const modalClose = document.getElementById('modal-close');
  const modalTitle = document.getElementById('modal-title');
  const modalBody = document.getElementById('modal-body');

  // 汎用インタラクティブダイアログ要素 (WebViewでalert/confirm/promptが効かない対策)
  const dialogOverlay = document.getElementById('dialog-overlay');
  const dialogTitle = document.getElementById('dialog-title');
  const dialogMessage = document.getElementById('dialog-message');
  const dialogInputWrapper = document.getElementById('dialog-input-wrapper');
  const dialogInput = document.getElementById('dialog-input');
  const dialogBtnCancel = document.getElementById('dialog-btn-cancel');
  const dialogBtnDiscard = document.getElementById('dialog-btn-discard');
  const dialogBtnOk = document.getElementById('dialog-btn-ok');

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
  const cmdSearchClear = document.getElementById('cmd-search-clear');
  const cmdList = document.getElementById('cmd-list');
  const cmdSortGroupBtn = document.getElementById('cmd-sort-group');
  const cmdSortNameBtn = document.getElementById('cmd-sort-name');
  const collapsedCmdGroups = new Set();
  let cmdSortMode = localStorage.getItem('gonako-cmd-sort-mode') || 'group';

  // ひな形タブ要素
  const templateSearch = document.getElementById('template-search');
  const templateCount = document.getElementById('template-count');
  const templateListElem = document.getElementById('template-list');

  // ファイルタブ要素
  const btnFileUp = document.getElementById('btn-file-up');
  const btnFileHome = document.getElementById('btn-file-home');
  const currentDirDisplay = document.getElementById('current-dir-display');
  const fileList = document.getElementById('file-list');
  const btnNewFile = document.getElementById('btn-new-file');

  // ファイル右端「…」用コンテキストメニュー
  const fileContextMenu = document.getElementById('file-context-menu');
  const menuOpenEditor = document.getElementById('menu-open-editor');
  const menuRevealFinder = document.getElementById('menu-reveal-finder');
  const labelRevealFinder = document.getElementById('label-reveal-finder');
  let selectedContextFile = null;

  let allCommands = [];
  let allTemplates = [];
  let currentDirPath = '';
  let parentDirPath = '';
  let homeDirPath = '';
  let desktopDirPath = '';
  let currentFilePath = '';
  let currentFileDisplayName = '新規プログラム.nako3';
  let currentTemplateBaseName = '';
  let savedContent = `// なでしこ3 プログラム\n「こんにちは」と表示。\n`;
  let isBinaryFile = false; // PNGなど文字コード範囲外のファイルを開いている間はtrue
  let currentOS = ''; // getAppInfo() から受け取る 'darwin' / 'windows' / 'linux'
  const defaultEditorPlaceholder = editor.getAttribute('placeholder') || '';

  // 初期プレースホルダー
  editor.value = savedContent;
  updateFileTitleDisplay();
  updateLineNumbers();
  updateCharCount();

  // --- サイドバーの表示/非表示トグル ---
  function toggleSidebar(force) {
    const isCollapsed = force !== undefined ? force : !sidebar.classList.contains('collapsed');
    sidebar.classList.toggle('collapsed', isCollapsed);
    if (splitterV) splitterV.classList.toggle('collapsed', isCollapsed);
    if (btnToggleSidebar) btnToggleSidebar.classList.toggle('active', !isCollapsed);
    try {
      localStorage.setItem('gonako-sidebar-collapsed', isCollapsed ? '1' : '0');
    } catch (e) {}
  }

  if (btnToggleSidebar) {
    btnToggleSidebar.addEventListener('click', () => toggleSidebar());
    // 保存されている状態があれば復元
    if (localStorage.getItem('gonako-sidebar-collapsed') === '1') {
      toggleSidebar(true);
    }
  }

  // --- タブ切り替え処理 ---
  function activateTab(activeBtn, activeContent) {
    [tabBtnCmd, tabBtnTemplate, tabBtnFile].forEach(btn => btn.classList.remove('active'));
    [tabContentCmd, tabContentTemplate, tabContentFile].forEach(c => c.classList.remove('active'));
    activeBtn.classList.add('active');
    activeContent.classList.add('active');
    closeContextMenu();
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
      loadDirectory(desktopDirPath || homeDirPath || '$DESKTOP');
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

  // --- 汎用ダイアログ関数 (WebView対応) ---
  function showSaveConfirmDialog(filename) {
    return new Promise((resolve) => {
      dialogTitle.textContent = '保存確認';
      dialogMessage.textContent = `「${filename}」は変更されています。\n変更を保存しますか？`;
      dialogInputWrapper.style.display = 'none';

      dialogBtnCancel.style.display = 'inline-flex';
      dialogBtnCancel.textContent = 'キャンセル';

      dialogBtnDiscard.style.display = 'inline-flex';
      dialogBtnDiscard.textContent = '保存せず開く';

      dialogBtnOk.style.display = 'inline-flex';
      dialogBtnOk.textContent = '保存して開く';

      dialogOverlay.style.display = 'flex';
      dialogBtnOk.focus();

      function cleanup() {
        dialogOverlay.style.display = 'none';
        dialogBtnCancel.removeEventListener('click', onCancel);
        dialogBtnDiscard.removeEventListener('click', onDiscard);
        dialogBtnOk.removeEventListener('click', onOk);
      }

      function onCancel() {
        cleanup();
        resolve('cancel');
      }
      function onDiscard() {
        cleanup();
        resolve('discard');
      }
      function onOk() {
        cleanup();
        resolve('save');
      }

      dialogBtnCancel.addEventListener('click', onCancel);
      dialogBtnDiscard.addEventListener('click', onDiscard);
      dialogBtnOk.addEventListener('click', onOk);
    });
  }

  function showPromptDialog(title, message, defaultValue = '') {
    return new Promise((resolve) => {
      dialogTitle.textContent = title;
      dialogMessage.textContent = message;
      dialogInputWrapper.style.display = 'block';
      dialogInput.value = defaultValue;

      dialogBtnCancel.style.display = 'inline-flex';
      dialogBtnCancel.textContent = 'キャンセル';
      dialogBtnDiscard.style.display = 'none';
      dialogBtnOk.style.display = 'inline-flex';
      dialogBtnOk.textContent = '保存';

      dialogOverlay.style.display = 'flex';
      dialogInput.focus();
      dialogInput.select();

      function cleanup() {
        dialogOverlay.style.display = 'none';
        dialogBtnCancel.removeEventListener('click', onCancel);
        dialogBtnOk.removeEventListener('click', onOk);
        dialogInput.removeEventListener('keydown', onKeyDown);
      }

      function onCancel() {
        cleanup();
        resolve(null);
      }
      function onOk() {
        const val = dialogInput.value.trim();
        cleanup();
        resolve(val || null);
      }
      function onKeyDown(e) {
        if (e.key === 'Enter') {
          e.preventDefault();
          onOk();
        } else if (e.key === 'Escape') {
          e.preventDefault();
          onCancel();
        }
      }

      dialogBtnCancel.addEventListener('click', onCancel);
      dialogBtnOk.addEventListener('click', onOk);
      dialogInput.addEventListener('keydown', onKeyDown);
    });
  }

  function showAlertDialog(title, message) {
    return new Promise((resolve) => {
      dialogTitle.textContent = title;
      dialogMessage.textContent = message;
      dialogInputWrapper.style.display = 'none';

      dialogBtnCancel.style.display = 'none';
      dialogBtnDiscard.style.display = 'none';
      dialogBtnOk.style.display = 'inline-flex';
      dialogBtnOk.textContent = 'OK';

      dialogOverlay.style.display = 'flex';
      dialogBtnOk.focus();

      function cleanup() {
        dialogOverlay.style.display = 'none';
        dialogBtnOk.removeEventListener('click', onOk);
      }
      function onOk() {
        cleanup();
        resolve();
      }
      dialogBtnOk.addEventListener('click', onOk);
    });
  }

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
    if (fileContextMenu && !fileContextMenu.contains(e.target)) {
      closeContextMenu();
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
            <td><kbd>Ctrl</kbd> + <kbd>N</kbd> / <kbd>Cmd</kbd> + <kbd>N</kbd></td>
            <td>新規作成</td>
          </tr>
          <tr>
            <td><kbd>Ctrl</kbd> + <kbd>O</kbd> / <kbd>Cmd</kbd> + <kbd>O</kbd></td>
            <td>ファイルを開く</td>
          </tr>
          <tr>
            <td><kbd>Ctrl</kbd> + <kbd>S</kbd> / <kbd>Cmd</kbd> + <kbd>S</kbd></td>
            <td>ファイルを保存</td>
          </tr>
          <tr>
            <td><kbd>Ctrl</kbd> + <kbd>Shift</kbd> + <kbd>S</kbd> / <kbd>Cmd</kbd> + <kbd>Shift</kbd> + <kbd>S</kbd></td>
            <td>名前を付けて保存</td>
          </tr>
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
            <td><kbd>Ctrl</kbd> + <kbd>B</kbd> / <kbd>Cmd</kbd> + <kbd>B</kbd></td>
            <td>ツールパネルの表示/非表示</td>
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

  // --- フォルダを実行ファイルに変換 ---
  function pathDirName(p) {
    const i = Math.max(p.lastIndexOf('/'), p.lastIndexOf('\\'));
    return i > 0 ? p.slice(0, i) : p;
  }

  function pathBaseName(p) {
    const i = Math.max(p.lastIndexOf('/'), p.lastIndexOf('\\'));
    return i >= 0 ? p.slice(i + 1) : p;
  }

  // 開いているファイルをメインにして、そのフォルダ以下を1つの実行ファイルに梱包する。
  // .nako3 なら起動と同時にそのプログラムを実行し、.html ならWebViewで開くアプリになる。
  async function buildAppFromCurrentFolder() {
    closeHamburger();
    if (typeof window.buildAppFromFolder !== 'function') {
      await showAlertDialog('変換できません', 'この機能は gonako-gui 上でのみ使えます。');
      return;
    }
    if (!currentFilePath || isBinaryFile) {
      await showAlertDialog('変換できません',
        'メインにするファイル (.nako3 または .html) を、ファイルタブから開いてください。');
      return;
    }

    const ext = (/\.([^.\\/]+)$/.exec(currentFileDisplayName) || ['', ''])[1].toLowerCase();
    const isProgram = ext === 'nako3' || ext === 'nako';
    const isHTML = ext === 'html' || ext === 'htm';
    if (!isProgram && !isHTML) {
      await showAlertDialog('変換できません',
        `『${currentFileDisplayName}』は変換できません。\nなでしこプログラム (.nako3) かHTMLファイル (.html) を開いてください。`);
      return;
    }

    // ディスク上のファイルを梱包するので、編集中の内容は先に保存しておく
    if (editor.value !== savedContent && !(await saveFile())) return;

    const folder = pathDirName(currentFilePath);
    const isWindows = currentOS === 'windows';
    const sep = isWindows ? '\\' : '/';
    // macOSは .app フォルダ、Windowsは .exe、それ以外は拡張子なしの実行ファイル
    const outExt = isWindows ? '.exe' : (currentOS === 'darwin' ? '.app' : '');
    // 出力先は1つ上のフォルダ。梱包するフォルダの中に置くと自分自身を巻き込む
    const defaultOut = `${pathDirName(folder)}${sep}app${outExt}`;

    const kindLabel = isProgram ? 'プログラムを実行するアプリ' : 'HTMLを表示するアプリ';
    const outPath = await showPromptDialog('このフォルダを実行ファイルに変換',
      `フォルダ『${folder}』以下をすべて梱包し、${kindLabel}を作ります。\n` +
      `メインファイル: ${currentFileDisplayName}\n\n出力先のパス:`,
      defaultOut);
    if (!outPath) return;

    const title = pathBaseName(outPath).replace(/\.(exe|app)$/i, '');
    setStatus('実行ファイルに変換中...');
    menuItemBuildApp.disabled = true;
    try {
      const res = await window.buildAppFromFolder(folder, currentFilePath, outPath, title);
      const data = typeof res === 'string' ? JSON.parse(res) : res;
      if (data.ok) {
        setStatus(`変換完了: ${data.path}`);
        await showAlertDialog('変換しました',
          `${data.path}\nサイズ: ${formatBytes(data.size)}`);
        if (typeof window.revealInFinder === 'function') {
          window.revealInFinder(data.path);
        }
      } else {
        setStatus(`変換エラー: ${data.error}`);
        await showAlertDialog('変換エラー', data.error || '変換に失敗しました。');
      }
    } catch (err) {
      console.error('変換エラー:', err);
      await showAlertDialog('変換エラー', `${err.message || err}`);
    } finally {
      menuItemBuildApp.disabled = false;
    }
  }

  // 開いているなでしこプログラムをGo言語のソースに変換し、go buildで
  // ネイティブ実行ファイルにする（AGENTS.md §12・docs/gogen.md）。
  // 実行ファイルの隣に なでしこ3 Go版 のソースが無ければ、Go側が
  // GitHubから最新masterを自動でダウンロードしてから使う。
  async function buildWithGoFromCurrentFile() {
    closeHamburger();
    if (typeof window.buildWithGo !== 'function') {
      await showAlertDialog('ビルドできません', 'この機能は gonako-gui 上でのみ使えます。');
      return;
    }
    if (!currentFilePath || isBinaryFile) {
      await showAlertDialog('ビルドできません',
        'ビルドするファイル (.nako3) を、ファイルタブから開いてください。');
      return;
    }
    const ext = (/\.([^.\\/]+)$/.exec(currentFileDisplayName) || ['', ''])[1].toLowerCase();
    if (ext !== 'nako3' && ext !== 'nako') {
      await showAlertDialog('ビルドできません',
        `『${currentFileDisplayName}』はビルドできません。なでしこプログラム (.nako3) を開いてください。`);
      return;
    }

    // ディスク上のファイルをビルドするので、編集中の内容は先に保存しておく
    if (editor.value !== savedContent && !(await saveFile())) return;

    const folder = pathDirName(currentFilePath);
    const isWindows = currentOS === 'windows';
    const sep = isWindows ? '\\' : '/';
    const defaultOut = `${folder}${sep}${pathBaseName(currentFilePath).replace(/\.(nako3|nako)$/i, '')}${isWindows ? '.exe' : ''}`;

    const outPath = await showPromptDialog('Go言語でビルド',
      `『${currentFileDisplayName}』をGo言語のソースに変換し、go buildでネイティブ実行ファイルにします。\n` +
      'ソースコードが見つからない場合は、実行ファイルと同じフォルダにGitHubから自動でダウンロードします（少し時間がかかります）。\n\n出力先のパス:',
      defaultOut);
    if (!outPath) return;

    setStatus('Go言語でビルド中...（初回はソースのダウンロードも行います）');
    menuItemGoBuild.disabled = true;
    try {
      const res = await window.buildWithGo(currentFilePath, outPath);
      const data = typeof res === 'string' ? JSON.parse(res) : res;
      if (data.ok) {
        setStatus(`ビルド完了: ${data.path}`);
        await showAlertDialog('ビルドしました',
          `${data.path}\nサイズ: ${formatBytes(data.size)}` +
          (data.downloaded ? '\n\n(初回のため、なでしこ3 Go版のソースをダウンロードしました)' : ''));
        if (typeof window.revealInFinder === 'function') {
          window.revealInFinder(data.path);
        }
      } else {
        setStatus(`ビルドエラー: ${data.error}`);
        await showAlertDialog('ビルドエラー', data.error || 'ビルドに失敗しました。');
      }
    } catch (err) {
      console.error('ビルドエラー:', err);
      await showAlertDialog('ビルドエラー', `${err.message || err}`);
    } finally {
      menuItemGoBuild.disabled = false;
    }
  }

  menuItemBuildApp.addEventListener('click', buildAppFromCurrentFolder);
  menuItemGoBuild.addEventListener('click', buildWithGoFromCurrentFile);
  modalClose.addEventListener('click', closeModal);
  modalOverlay.addEventListener('click', (e) => {
    if (e.target === modalOverlay) closeModal();
  });

  // バイナリファイル表示から抜けるとき（ひな形・新規ファイルの読み込み時）に呼ぶ
  function clearBinaryState() {
    if (!isBinaryFile) return;
    isBinaryFile = false;
    editor.readOnly = false;
    editor.placeholder = defaultEditorPlaceholder;
  }

  // --- 変更検知と保存確認 ---
  function updateFileTitleDisplay() {
    if (isBinaryFile) {
      activeFileName.textContent = `(編集不可) ${currentFileDisplayName}`;
      activeFileName.classList.remove('dirty');
      return;
    }
    const isModified = editor.value !== savedContent;
    if (isModified) {
      activeFileName.textContent = `(変更あり) ${currentFileDisplayName}`;
      activeFileName.classList.add('dirty');
    } else {
      activeFileName.textContent = currentFileDisplayName;
      activeFileName.classList.remove('dirty');
    }
  }

  async function confirmSaveIfDirty() {
    // バイナリファイルは編集できないので、保存確認なしで切り替えてよい
    if (isBinaryFile || editor.value === savedContent) {
      return true;
    }
    const action = await showSaveConfirmDialog(currentFileDisplayName);
    if (action === 'cancel') {
      return false;
    }
    if (action === 'save') {
      const saved = await saveFile();
      return saved;
    }
    if (action === 'discard') {
      return true;
    }
    return false;
  }

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

      item.addEventListener('click', async () => {
        if (!(await confirmSaveIfDirty())) return;
        clearBinaryState();
        editor.value = t.code;
        savedContent = t.code;
        currentFilePath = '';
        currentFileDisplayName = t.title;
        currentTemplateBaseName = (t.id || t.title).replace(/\.nako3$/i, '');
        updateFileTitleDisplay();
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
      try {
        const res = await fetch('command-list.json');
        allCommands = await res.json();
      } catch (err) {}
    }
    if (allCommands && allCommands.length > 0) {
      renderCommands(allCommands);
      // 命令名を色分けできるようにシンタックス定義へ登録し、エディタを塗り直す
      if (window.NakoSyntax) {
        window.NakoSyntax.setCommands(allCommands);
        if (highlighter) highlighter.setEnabled(true);
      }
    }
  }

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

  function createCmdItem(cmd) {
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

    item.addEventListener('click', () => {
      document.querySelectorAll('#cmd-list .list-item').forEach(el => el.classList.remove('selected'));
      item.classList.add('selected');
      displayCommandHelp(cmd);
      setStatus(`命令「${cmd.name}」の使い方を表示しました (ダブルクリックまたはDnDで挿入)`);
    });

    item.addEventListener('dblclick', (e) => {
      e.preventDefault();
      const template = cmd.template || cmd.name;
      insertTextAtCursor(template);
      displayCommandHelp(cmd);
      setStatus(`命令「${cmd.name}」の構文をエディタに挿入しました`);
    });

    item.addEventListener('dragstart', (e) => {
      const template = cmd.template || cmd.name;
      e.dataTransfer.setData('text/plain', template);
      e.dataTransfer.setData('application/json', JSON.stringify(cmd));
      e.dataTransfer.effectAllowed = 'copy';
    });

    return item;
  }

  function renderCommandsFlat(commands) {
    const sorted = [...commands].sort((a, b) => a.name.localeCompare(b.name, 'ja'));
    sorted.forEach(cmd => cmdList.appendChild(createCmdItem(cmd)));
  }

  function renderCommandsGrouped(commands) {
    const groups = new Map();
    commands.forEach(cmd => {
      const category = cmd.category || 'その他';
      if (!groups.has(category)) groups.set(category, []);
      groups.get(category).push(cmd);
    });

    const groupNames = [...groups.keys()].sort((a, b) => a.localeCompare(b, 'ja'));

    groupNames.forEach(groupName => {
      const items = groups.get(groupName).sort((a, b) => a.name.localeCompare(b.name, 'ja'));
      const collapsed = collapsedCmdGroups.has(groupName);

      const groupEl = document.createElement('div');
      groupEl.className = 'cmd-group' + (collapsed ? ' collapsed' : '');

      const header = document.createElement('div');
      header.className = 'cmd-group-header';
      header.innerHTML = `
        <span class="cmd-group-toggle">▾</span>
        <span class="cmd-group-name">${escapeHtml(groupName)}</span>
        <span class="cmd-group-count">${items.length}件</span>
      `;
      header.addEventListener('click', () => {
        if (collapsedCmdGroups.has(groupName)) {
          collapsedCmdGroups.delete(groupName);
        } else {
          collapsedCmdGroups.add(groupName);
        }
        groupEl.classList.toggle('collapsed');
      });

      const itemsEl = document.createElement('div');
      itemsEl.className = 'cmd-group-items';
      items.forEach(cmd => itemsEl.appendChild(createCmdItem(cmd)));

      groupEl.appendChild(header);
      groupEl.appendChild(itemsEl);
      cmdList.appendChild(groupEl);
    });
  }

  function renderCommands(commands) {
    cmdList.innerHTML = '';
    cmdCount.textContent = `${commands.length}件`;

    if (cmdSortMode === 'group') {
      renderCommandsGrouped(commands);
    } else {
      renderCommandsFlat(commands);
    }
  }

  function setCmdSortMode(mode) {
    cmdSortMode = mode;
    localStorage.setItem('gonako-cmd-sort-mode', mode);
    cmdSortGroupBtn.classList.toggle('active', mode === 'group');
    cmdSortNameBtn.classList.toggle('active', mode === 'name');
    const query = cmdSearch.value.trim().toLowerCase();
    renderCommands(query ? filterCommands(query) : allCommands);
  }

  function filterCommands(query) {
    return allCommands.filter(c => {
      if (c.name.toLowerCase().includes(query)) return true;
      if (c.desc && c.desc.toLowerCase().includes(query)) return true;
      if (c.category && c.category.toLowerCase().includes(query)) return true;
      if (c.yomi && c.yomi.toLowerCase().includes(query)) return true;
      if (c.josi && c.josi.some(group => group.some(j => j.toLowerCase().includes(query)))) return true;
      return false;
    });
  }

  cmdSortGroupBtn.addEventListener('click', () => setCmdSortMode('group'));
  cmdSortNameBtn.addEventListener('click', () => setCmdSortMode('name'));
  setCmdSortMode(cmdSortMode);

  cmdSearch.addEventListener('input', () => {
    const query = cmdSearch.value.trim().toLowerCase();
    cmdSearchClear.hidden = !query;
    renderCommands(query ? filterCommands(query) : allCommands);
  });

  cmdSearchClear.addEventListener('click', () => {
    cmdSearch.value = '';
    cmdSearchClear.hidden = true;
    renderCommands(allCommands);
    cmdSearch.focus();
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

  // --- ファイルブラウザ・コンテキストメニュー処理 ---
  function openContextMenu(x, y, file) {
    selectedContextFile = file;
    menuOpenEditor.textContent = file.isDir ? '📁 フォルダを開く' : '📝 エディタで開く';
    fileContextMenu.style.left = `${Math.min(x, window.innerWidth - 180)}px`;
    fileContextMenu.style.top = `${Math.min(y, window.innerHeight - 100)}px`;
    fileContextMenu.style.display = 'block';
  }

  function closeContextMenu() {
    if (fileContextMenu) fileContextMenu.style.display = 'none';
    selectedContextFile = null;
  }

  menuOpenEditor.addEventListener('click', async () => {
    if (!selectedContextFile) return;
    if (selectedContextFile.isDir) {
      loadDirectory(selectedContextFile.path);
    } else {
      if (!(await confirmSaveIfDirty())) return;
      openFile(selectedContextFile.path, selectedContextFile.name);
    }
    closeContextMenu();
  });

  menuRevealFinder.addEventListener('click', async () => {
    if (!selectedContextFile || typeof window.revealInFinder !== 'function') return;
    try {
      await window.revealInFinder(selectedContextFile.path);
      setStatus(`ファイルを表示しました: ${selectedContextFile.name}`);
    } catch (err) {
      console.error('Finder表示エラー:', err);
    }
    closeContextMenu();
  });

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
          <div class="item-right">
            <span class="file-item-ext">${file.isDir ? 'フォルダ' : formatBytes(file.size)}</span>
            <button class="item-more-btn" title="メニュー">…</button>
          </div>
        `;

        item.addEventListener('click', async (e) => {
          if (e.target.classList.contains('item-more-btn')) return;
          if (file.isDir) {
            loadDirectory(file.path);
          } else {
            if (!(await confirmSaveIfDirty())) return;
            openFile(file.path, file.name);
          }
        });

        const moreBtn = item.querySelector('.item-more-btn');
        moreBtn.addEventListener('click', (e) => {
          e.stopPropagation();
          const rect = moreBtn.getBoundingClientRect();
          openContextMenu(rect.left, rect.bottom + 4, file);
        });

        item.addEventListener('contextmenu', (e) => {
          e.preventDefault();
          e.stopPropagation();
          openContextMenu(e.clientX, e.clientY, file);
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
    loadDirectory(desktopDirPath || homeDirPath || '$DESKTOP');
  });

  // 「＋ 新規ファイル」ボタンのクリック処理 (YYYY-MM-DD-新規-1.nako3 を自動生成)
  btnNewFile.addEventListener('click', async () => {
    if (typeof window.createNewFile !== 'function') {
      await showAlertDialog('エラー', '新規ファイル作成機能が利用できません');
      return;
    }
    if (!(await confirmSaveIfDirty())) return;

    try {
      setStatus('新規ファイルを作成中...');
      const res = await window.createNewFile(currentDirPath || desktopDirPath || homeDirPath);
      const data = typeof res === 'string' ? JSON.parse(res) : res;
      if (data.ok) {
        await loadDirectory(currentDirPath);
        await openFile(data.path, data.name);
        setStatus(`新規ファイル「${data.name}」を作成して開きました`);
      } else {
        await showAlertDialog('作成エラー', `新規ファイルを作成できませんでした: ${data.error}`);
        setStatus(`作成エラー: ${data.error}`);
      }
    } catch (err) {
      console.error('新規ファイル作成エラー:', err);
    }
  });

  async function newFile() {
    closeHamburger();
    if (!(await confirmSaveIfDirty())) return;
    clearBinaryState();
    editor.value = `// なでしこ3 プログラム\n「こんにちは」と表示。\n`;
    savedContent = editor.value;
    currentFilePath = '';
    currentFileDisplayName = '新規プログラム.nako3';
    currentTemplateBaseName = '';
    activeFileName.title = '';
    updateFileTitleDisplay();
    updateLineNumbers();
    updateCharCount();
    updateCursorPos();
    editor.focus();
    setStatus('新規ファイルを作成しました');
  }

  async function openFileDialogAction() {
    closeHamburger();
    if (!(await confirmSaveIfDirty())) return;
    if (typeof window.showOpenFileDialog === 'function') {
      try {
        const baseDir = currentFilePath ? pathDirName(currentFilePath) : (desktopDirPath || homeDirPath);
        const res = await window.showOpenFileDialog(baseDir);
        const data = typeof res === 'string' ? JSON.parse(res) : res;
        if (data.ok) {
          if (data.canceled || !data.path) {
            return;
          }
          await openFile(data.path, pathBaseName(data.path));
          return;
        } else if (data.error) {
          console.error('OSダイアログエラー:', data.error);
          setStatus(`ダイアログエラー: ${data.error}`);
        }
      } catch (err) {
        console.error('OSダイアログ呼び出しエラー:', err);
      }
    }
    // フォールバック: ファイルタブに切り替え
    activateTab(tabBtnFile, tabContentFile);
    toggleSidebar(false);
  }

  async function openFile(filePath, fileName) {
    if (typeof window.readFile !== 'function') return;
    try {
      setStatus(`ファイル読み込み中: ${fileName}...`);
      const res = await window.readFile(filePath);
      const data = typeof res === 'string' ? JSON.parse(res) : res;
      if (data.ok) {
        isBinaryFile = !!data.isBinary;
        editor.value = isBinaryFile ? '' : data.content;
        editor.readOnly = isBinaryFile;
        editor.placeholder = isBinaryFile
          ? 'このファイルはバイナリ形式のため、エディタでは表示・編集できません。'
          : defaultEditorPlaceholder;
        savedContent = editor.value;
        currentFilePath = filePath;
        currentFileDisplayName = fileName;
        currentTemplateBaseName = '';
        activeFileName.title = filePath;
        updateFileTitleDisplay();
        updateLineNumbers();
        updateCharCount();
        updateCursorPos();
        setStatus(isBinaryFile ? `編集不可(バイナリ): ${fileName}` : `開きました: ${fileName}`);
      } else {
        await showAlertDialog('エラー', `ファイルを開けませんでした: ${data.error}`);
        setStatus(`エラー: ${data.error}`);
      }
    } catch (err) {
      console.error('ファイルオープンエラー:', err);
    }
  }

  async function saveFileAs() {
    if (isBinaryFile) return false;
    closeHamburger();

    const baseDir = currentFilePath ? pathDirName(currentFilePath) : (desktopDirPath || homeDirPath);
    let defaultName = currentFileDisplayName || '新規プログラム.nako3';
    if (defaultName.startsWith('(変更あり) ')) {
      defaultName = defaultName.replace('(変更あり) ', '');
    } else if (defaultName.startsWith('(編集不可) ')) {
      defaultName = defaultName.replace('(編集不可) ', '');
    }

    let targetPath = '';
    if (typeof window.showSaveFileDialog === 'function') {
      try {
        const res = await window.showSaveFileDialog(baseDir, defaultName);
        const data = typeof res === 'string' ? JSON.parse(res) : res;
        if (data.ok) {
          if (data.canceled || !data.path) {
            return false;
          }
          targetPath = data.path;
        } else if (data.error) {
          console.error('OSダイアログ保存エラー:', data.error);
        }
      } catch (err) {
        console.error('OSダイアログ呼び出しエラー:', err);
      }
    }

    if (!targetPath) {
      // フォールバック: Webプロンプトダイアログ
      const name = await showPromptDialog('名前を付けて保存', '保存するファイル名を入力してください:', defaultName);
      if (!name) return false;
      targetPath = baseDir + '/' + name;
    }

    return await doSaveToPath(targetPath);
  }

  async function saveFile() {
    if (isBinaryFile) return false;
    closeHamburger();
    if (!currentFilePath) {
      return await saveFileAs();
    }
    return await doSaveToPath(currentFilePath);
  }

  async function doSaveToPath(targetPath) {
    if (typeof window.saveFile !== 'function') return false;
    try {
      const fileName = pathBaseName(targetPath);
      setStatus(`保存中: ${fileName}...`);
      const res = await window.saveFile(targetPath, editor.value);
      const data = typeof res === 'string' ? JSON.parse(res) : res;
      if (data.ok) {
        savedContent = editor.value;
        currentFilePath = targetPath;
        currentFileDisplayName = fileName;
        currentTemplateBaseName = '';
        activeFileName.title = currentFilePath;
        updateFileTitleDisplay();
        setStatus(`保存完了: ${fileName}`);
        if (tabContentFile && tabContentFile.classList.contains('active')) {
          loadDirectory(currentDirPath || pathDirName(targetPath));
        }
        return true;
      } else {
        await showAlertDialog('保存エラー', `保存に失敗しました: ${data.error}`);
        setStatus(`保存エラー: ${data.error}`);
        return false;
      }
    } catch (err) {
      console.error('保存エラー:', err);
      await showAlertDialog('システムエラー', `保存エラー: ${err.message || err}`);
      return false;
    }
  }

  if (btnNew) btnNew.addEventListener('click', newFile);
  if (btnOpen) btnOpen.addEventListener('click', openFileDialogAction);
  if (btnSave) btnSave.addEventListener('click', saveFile);
  if (menuItemNew) menuItemNew.addEventListener('click', newFile);
  if (menuItemOpen) menuItemOpen.addEventListener('click', openFileDialogAction);
  if (menuItemSave) menuItemSave.addEventListener('click', saveFile);
  if (menuItemSaveAs) menuItemSaveAs.addEventListener('click', saveFileAs);

  // --- エディタ操作 ---
  // エディタの表示（行番号と色分け）を更新する。
  // editor.value を書き換えたところは必ずここを通す。
  function updateLineNumbers() {
    const lines = editor.value.split('\n').length;
    let numbers = '';
    for (let i = 1; i <= lines; i++) {
      numbers += i + '\n';
    }
    lineNumbers.textContent = numbers;
    if (highlighter) highlighter.refresh();
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
    updateFileTitleDisplay();
    updateLineNumbers();
    updateCharCount();
    updateCursorPos();
  }

  editor.addEventListener('input', () => {
    updateFileTitleDisplay();
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

    // Ctrl+N / Cmd+N で新規作成
    if (isCmdOrCtrl && !e.shiftKey && (e.key === 'n' || e.key === 'N')) {
      e.preventDefault();
      newFile();
      return;
    }
    // Ctrl+O / Cmd+O でファイルを開く
    if (isCmdOrCtrl && !e.shiftKey && (e.key === 'o' || e.key === 'O')) {
      e.preventDefault();
      openFileDialogAction();
      return;
    }
    // Ctrl+S / Cmd+S で保存 (Shift付きなら名前を付けて保存)
    if (isCmdOrCtrl && (e.key === 's' || e.key === 'S')) {
      e.preventDefault();
      if (e.shiftKey) {
        saveFileAs();
      } else {
        saveFile();
      }
      return;
    }
    // Ctrl+B / Cmd+B でサイドバー切り替え
    if (isCmdOrCtrl && !e.shiftKey && (e.key === 'b' || e.key === 'B')) {
      e.preventDefault();
      toggleSidebar();
      return;
    }
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
    // Ctrl+L でログ消去
    if (isCmdOrCtrl && (e.key === 'l' || e.key === 'L')) {
      e.preventDefault();
      btnClearLog.click();
      return;
    }
    // Esc でモーダル・コンテキストメニューを閉じる
    if (e.key === 'Escape') {
      closeModal();
      closeHamburger();
      closeContextMenu();
      return;
    }

    // [Cmd]+[A] / [Ctrl]+[A] --- 全選択
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

    // [Cmd]+[C] / [Ctrl]+[C] --- コピー
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

    // [Cmd]+[X] / [Ctrl]+[X] --- 切り取り
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
            updateFileTitleDisplay();
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

    // [Cmd]+[V] / [Ctrl]+[V] --- 貼り付け
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
              updateFileTitleDisplay();
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
        result = await window.runNakoCode(code, currentFilePath || '');
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

        if (isWindowMode && data.output && /<[a-z][\s\S]*>/i.test(data.output)) {
          windowPreview.style.display = 'block';
          windowPreview.innerHTML = data.output;
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
        currentOS = info.os || '';
        versionInfo.textContent = `gonako-gui v${info.version || '3.6.0'} (${info.os}/${info.arch})`;
        homeDirPath = info.homeDir || '';
        desktopDirPath = info.desktopDir || homeDirPath;
        if (!currentDirPath) {
          currentDirPath = desktopDirPath;
          currentDirDisplay.textContent = currentDirPath;
          currentDirDisplay.title = currentDirPath;
        }
        if (labelRevealFinder) {
          if (info.os === 'darwin') {
            labelRevealFinder.textContent = 'Finderで表示';
          } else if (info.os === 'windows') {
            labelRevealFinder.textContent = 'エクスプローラーで表示';
          } else {
            labelRevealFinder.textContent = 'ファイルマネージャーで表示';
          }
        }
        if (info.initialFile) {
          openFile(info.initialFile, pathBaseName(info.initialFile));
        }
      } catch {
        versionInfo.textContent = `gonako-gui`;
      }
    });
  }
});
