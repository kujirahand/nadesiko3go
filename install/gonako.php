<?php
/**
 * nadesiko3go (gonako) インストーラ配信スクリプト
 *
 * アクセス元のOS (User-Agent) またはクエリパラメータに応じて、
 * Bashスクリプト (macOS / Linux 用) または PowerShellスクリプト (Windows 用) を返します。
 *
 * 使い方:
 *   macOS / Linux (ターミナル):
 *     curl -fsSL https://nadesi.com/install/gonako | bash
 *
 *   Windows (PowerShell):
 *     irm https://nadesi.com/install/gonako | iex
 *
 * クエリパラメータによる明示指定:
 *   ?os=win    または ?type=ps1  -> Windows (PowerShell) 用スクリプト
 *   ?os=mac    または ?type=sh   -> macOS / Linux (Bash) 用スクリプト
 *   ?os=linux  または ?type=bash -> Linux (Bash) 用スクリプト
 */

if (!function_exists('gonakoFetchUrl')) {
    /**
     * URL からコンテンツを取得 (curl または stream_context)
     *
     * @param string $url
     * @return string|false
     */
    function gonakoFetchUrl($url) {
        if (function_exists('curl_init')) {
            $ch = curl_init();
            curl_setopt($ch, CURLOPT_URL, $url);
            curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
            curl_setopt($ch, CURLOPT_FOLLOWLOCATION, true);
            curl_setopt($ch, CURLOPT_TIMEOUT, 10);
            curl_setopt($ch, CURLOPT_CONNECTTIMEOUT, 5);
            curl_setopt($ch, CURLOPT_USERAGENT, 'gonako-installer-dispatcher/1.0');
            $result = curl_exec($ch);
            $code = curl_getinfo($ch, CURLINFO_HTTP_CODE);
            if (PHP_VERSION_ID < 80000) {
                curl_close($ch);
            }
            if ($code === 200 && $result !== false) {
                return $result;
            }
        }

        $ctx = stream_context_create([
            'http' => [
                'timeout' => 10,
                'user_agent' => 'gonako-installer-dispatcher/1.0',
                'follow_location' => 1,
            ],
        ]);
        return @file_get_contents($url, false, $ctx);
    }
}

// 1. OS / スクリプト種別の判定
$isWindows = false;

if (isset($_GET['os'])) {
    $osParam = strtolower(trim((string)$_GET['os']));
    if (in_array($osParam, ['win', 'windows', 'win32', 'win64'], true)) {
        $isWindows = true;
    }
} elseif (isset($_GET['type'])) {
    $typeParam = strtolower(trim((string)$_GET['type']));
    if (in_array($typeParam, ['ps1', 'powershell'], true)) {
        $isWindows = true;
    }
} else {
    // User-Agent による判定
    // PowerShell (irm) や Windows 環境からのアクセスを検出
    $ua = isset($_SERVER['HTTP_USER_AGENT']) ? (string)$_SERVER['HTTP_USER_AGENT'] : '';
    if (preg_match('/(Windows|PowerShell|Win32|Win64)/i', $ua)) {
        $isWindows = true;
    }
}

$scriptName = $isWindows ? 'install.ps1' : 'install.sh';

// 2. スクリプトの読み込み
$content = null;

// (A) ローカルファイルが存在する場合はそちらを最優先で読み込む
$localCandidates = [
    dirname(__DIR__) . '/scripts/' . $scriptName,
    __DIR__ . '/scripts/' . $scriptName,
    __DIR__ . '/' . $scriptName,
];

foreach ($localCandidates as $candidatePath) {
    if (file_exists($candidatePath) && is_readable($candidatePath)) {
        $loaded = @file_get_contents($candidatePath);
        if ($loaded !== false && strlen($loaded) > 0) {
            $content = $loaded;
            break;
        }
    }
}

// (B) ローカルファイルがない場合は GitHub の raw リポジトリから取得 (キャッシュ付き)
if ($content === null) {
    $cacheDir = sys_get_temp_dir() . '/gonako_install_cache';
    if (!is_dir($cacheDir)) {
        @mkdir($cacheDir, 0777, true);
    }
    $cacheFile = $cacheDir . '/' . $scriptName;
    $cacheTtl = 600; // キャッシュ有効期間 (10分)

    // キャッシュが有効ならそれを使用
    if (file_exists($cacheFile) && (time() - filemtime($cacheFile) < $cacheTtl)) {
        $cached = @file_get_contents($cacheFile);
        if ($cached !== false && strlen($cached) > 0) {
            $content = $cached;
        }
    }

    // キャッシュがないか期限切れなら GitHub から取得
    if ($content === null) {
        $rawUrl = 'https://raw.githubusercontent.com/kujirahand/nadesiko3go/master/scripts/' . $scriptName;
        $fetched = gonakoFetchUrl($rawUrl);
        if ($fetched !== false && strlen($fetched) > 0) {
            $content = $fetched;
            @file_put_contents($cacheFile, $content);
        } elseif (file_exists($cacheFile)) {
            // ネットワークエラー時は期限切れキャッシュでも利用
            $content = @file_get_contents($cacheFile);
        }
    }
}

// 3. レスポンスの返却
if (!headers_sent()) {
    header('Content-Type: text/plain; charset=utf-8');
    header('X-Content-Type-Options: nosniff');
    header('Cache-Control: public, max-age=300');
}

if ($content === null || $content === false) {
    if (!headers_sent()) {
        http_response_code(500);
    }
    echo "# エラー: インストールスクリプト (" . htmlspecialchars($scriptName, ENT_QUOTES, 'UTF-8') . ") の取得に失敗しました。\n";
    echo "# リポジトリをご確認ください: https://github.com/kujirahand/nadesiko3go\n";
    exit(1);
}

echo $content;
