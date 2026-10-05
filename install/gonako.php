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

if (!function_exists('gonakoMyUid')) {
    /**
     * PHP実行ユーザーの uid を返す (判定できなければ null)
     * posix 拡張が無い環境では、自分で作った一時ファイルの所有者から求める
     *
     * @return int|null
     */
    function gonakoMyUid() {
        if (function_exists('posix_geteuid')) {
            return posix_geteuid();
        }
        $probe = @tempnam(sys_get_temp_dir(), 'gonako_uid_');
        if ($probe === false) {
            return null;
        }
        $uid = @fileowner($probe);
        @unlink($probe);
        return $uid === false ? null : $uid;
    }
}

if (!function_exists('gonakoIsTrusted')) {
    /**
     * パスが「PHP実行ユーザー自身の所有」かつ「他者が書き込めない」ものか確認する
     *
     * @param string $path
     * @return bool
     */
    function gonakoIsTrusted($path) {
        if (is_link($path)) {
            return false;
        }
        $st = @stat($path);
        if ($st === false) {
            return false;
        }
        $me = gonakoMyUid();
        if ($me === null || $st['uid'] !== $me) {
            // 所有者を検証できない場合は信頼しない
            return false;
        }
        // グループ・その他に書き込み権限があれば信頼しない
        return ($st['mode'] & 0022) === 0;
    }
}

if (!function_exists('gonakoCacheDir')) {
    /**
     * 実行ユーザー専用のキャッシュディレクトリを返す (安全に用意できなければ null)
     *
     * @return string|null
     */
    function gonakoCacheDir() {
        $uid = gonakoMyUid();
        if ($uid === null) {
            return null;
        }
        $dir = sys_get_temp_dir() . '/gonako_install_cache_' . $uid;
        if (!is_dir($dir) && !is_link($dir)) {
            @mkdir($dir, 0700, true);
        }
        if (!is_dir($dir) || !gonakoIsTrusted($dir)) {
            return null;
        }
        return $dir;
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
    // 実行ユーザー専用 (0700) で、所有者・権限を検証できた場合だけキャッシュを使う
    $cacheDir = gonakoCacheDir();
    $cacheFile = $cacheDir !== null ? $cacheDir . '/' . $scriptName : null;
    $cacheTtl = 600; // キャッシュ有効期間 (10分)
    if ($cacheFile !== null && file_exists($cacheFile) && !gonakoIsTrusted($cacheFile)) {
        // 他者が置いた・書き換え可能なキャッシュは破棄して使わない
        @unlink($cacheFile);
    }
    $cacheUsable = $cacheFile !== null && file_exists($cacheFile) && gonakoIsTrusted($cacheFile);

    // キャッシュが有効ならそれを使用
    if ($cacheUsable && (time() - filemtime($cacheFile) < $cacheTtl)) {
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
            if ($cacheFile !== null) {
                // 一時ファイルに書いてから置き換える (0600)
                $tmp = $cacheFile . '.' . bin2hex(random_bytes(8)) . '.tmp';
                if (@file_put_contents($tmp, $content) !== false) {
                    @chmod($tmp, 0600);
                    if (!@rename($tmp, $cacheFile)) {
                        @unlink($tmp);
                    }
                }
            }
        } elseif ($cacheUsable && file_exists($cacheFile)) {
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
