<?php
/**
 * install/gonako.php のテストスクリプト
 * 実行: php install/test_gonako.php
 */

function runTest($name, $closure) {
    try {
        $closure();
        echo "  [PASS] {$name}\n";
    } catch (\Throwable $e) {
        echo "  [FAIL] {$name}: " . $e->getMessage() . "\n";
        exit(1);
    }
}

echo "=== Testing install/gonako.php ===\n";

$target = __DIR__ . '/gonako.php';

// Test 1: curl UA -> install.sh
runTest("curl UA returns bash script", function () use ($target) {
    $out = shell_exec("HTTP_USER_AGENT='curl/8.7.1' php " . escapeshellarg($target));
    if (strpos($out, "nadesiko3go (gonako & gonako-gui) インストーラー (macOS / Linux 用)") === false) {
        throw new Exception("Expected macOS/Linux installer, got:\n" . substr($out, 0, 200));
    }
});

// Test 2: Windows PowerShell UA -> install.ps1
runTest("PowerShell UA returns PowerShell script", function () use ($target) {
    $ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) PowerShell/7.4.2";
    $out = shell_exec("HTTP_USER_AGENT=" . escapeshellarg($ua) . " php " . escapeshellarg($target));
    if (strpos($out, "nadesiko3go (gonako & gonako-gui) インストーラー (Windows / PowerShell 用)") === false) {
        throw new Exception("Expected Windows PowerShell installer, got:\n" . substr($out, 0, 200));
    }
});

// Test 3: Windows browser UA -> install.ps1
runTest("Windows UA returns PowerShell script", function () use ($target) {
    $ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:128.0) Gecko/20100101 Firefox/128.0";
    $out = shell_exec("HTTP_USER_AGENT=" . escapeshellarg($ua) . " php " . escapeshellarg($target));
    if (strpos($out, "nadesiko3go (gonako & gonako-gui) インストーラー (Windows / PowerShell 用)") === false) {
        throw new Exception("Expected Windows PowerShell installer, got:\n" . substr($out, 0, 200));
    }
});

// Test 4: Query parameter ?os=win overrides curl UA
runTest("Query ?os=win returns PowerShell script", function () use ($target) {
    $script = '$_GET["os"]="win"; include ' . var_export($target, true) . ';';
    $cmd = "HTTP_USER_AGENT='curl/8.0' php -r " . escapeshellarg($script);
    $out = shell_exec($cmd);
    if (strpos($out, "nadesiko3go (gonako & gonako-gui) インストーラー (Windows / PowerShell 用)") === false) {
        throw new Exception("Expected Windows PowerShell installer, got:\n" . substr($out, 0, 200));
    }
});

// Test 5: Query parameter ?os=mac overrides Windows UA
runTest("Query ?os=mac overrides Windows UA to bash", function () use ($target) {
    $script = '$_GET["os"]="mac"; include ' . var_export($target, true) . ';';
    $cmd = "HTTP_USER_AGENT='Windows' php -r " . escapeshellarg($script);
    $out = shell_exec($cmd);
    if (strpos($out, "nadesiko3go (gonako & gonako-gui) インストーラー (macOS / Linux 用)") === false) {
        throw new Exception("Expected macOS/Linux installer, got:\n" . substr($out, 0, 200));
    }
});

// Test 6: Remote fetch fallback when standalone
runTest("Standalone file (no local scripts) fetches from GitHub raw", function () use ($target) {
    $tmpDir = sys_get_temp_dir() . '/gonako_test_' . uniqid();
    mkdir($tmpDir);
    $tmpFile = $tmpDir . '/gonako.php';
    copy($target, $tmpFile);

    $outBash = shell_exec("HTTP_USER_AGENT='curl/8.0' php " . escapeshellarg($tmpFile));
    $outPs1 = shell_exec("HTTP_USER_AGENT='PowerShell' php " . escapeshellarg($tmpFile));

    @unlink($tmpFile);
    @rmdir($tmpDir);

    if (strpos($outBash, "nadesiko3go (gonako & gonako-gui) インストーラー (macOS / Linux 用)") === false) {
        throw new Exception("Standalone remote fetch bash failed");
    }
    if (strpos($outPs1, "nadesiko3go (gonako & gonako-gui) インストーラー (Windows / PowerShell 用)") === false) {
        throw new Exception("Standalone remote fetch ps1 failed");
    }
});

// Test 7: 共有一時領域に置かれた改ざんキャッシュは配信しない (#184)
runTest("Tampered shared cache is rejected", function () use ($target) {
    $base = sys_get_temp_dir() . '/gonako_test_' . bin2hex(random_bytes(4));
    $tmpDir = $base . '/app';
    $tmpHome = $base . '/tmp';
    mkdir($tmpDir, 0700, true);
    mkdir($tmpHome, 0700, true);
    $tmpFile = $tmpDir . '/gonako.php';
    copy($target, $tmpFile);

    // 旧実装と同じ固定名・誰でも書き込める権限で、改ざんキャッシュを先に置く
    $old = $tmpHome . '/gonako_install_cache';
    mkdir($old, 0777, true);
    chmod($old, 0777);
    file_put_contents($old . '/install.sh', "echo pwned\n");

    // 現行の命名のディレクトリ・ファイルを他者書き込み可で用意しても拒否する
    $uid = function_exists('posix_geteuid') ? posix_geteuid() : md5($tmpDir);
    $cur = $tmpHome . '/gonako_install_cache_' . $uid;
    mkdir($cur, 0777, true);
    chmod($cur, 0777);
    file_put_contents($cur . '/install.sh', "echo pwned\n");

    $out = shell_exec("TMPDIR=" . escapeshellarg($tmpHome) . " HTTP_USER_AGENT='curl/8.0' php " . escapeshellarg($tmpFile));

    shell_exec("rm -rf " . escapeshellarg($base));

    if (strpos((string)$out, 'echo pwned') !== false) {
        throw new Exception("Tampered cache was served");
    }
    if (strpos((string)$out, "インストーラー (macOS / Linux 用)") === false) {
        throw new Exception("Genuine script was not served, got:\n" . substr((string)$out, 0, 200));
    }
});

// Test 8: 専用キャッシュは 0700 で作られ、次回はキャッシュから配信できる
runTest("Private cache is created with 0700", function () use ($target) {
    $base = sys_get_temp_dir() . '/gonako_test_' . bin2hex(random_bytes(4));
    $tmpDir = $base . '/app';
    $tmpHome = $base . '/tmp';
    mkdir($tmpDir, 0700, true);
    mkdir($tmpHome, 0700, true);
    $tmpFile = $tmpDir . '/gonako.php';
    copy($target, $tmpFile);

    $cmd = "TMPDIR=" . escapeshellarg($tmpHome) . " HTTP_USER_AGENT='curl/8.0' php " . escapeshellarg($tmpFile);
    $out = shell_exec($cmd);
    $dirs = glob($tmpHome . '/gonako_install_cache_*');
    $mode = $dirs ? (fileperms($dirs[0]) & 0777) : -1;
    $files = $dirs ? glob($dirs[0] . '/install.sh') : [];
    $fmode = $files ? (fileperms($files[0]) & 0777) : -1;

    shell_exec("rm -rf " . escapeshellarg($base));

    if (strpos((string)$out, "インストーラー (macOS / Linux 用)") === false) {
        throw new Exception("Standalone fetch failed");
    }
    if ($mode !== 0700) {
        throw new Exception("cache dir mode is " . decoct($mode));
    }
    if ($fmode !== 0600) {
        throw new Exception("cache file mode is " . decoct($fmode));
    }
});

echo "All tests passed successfully!\n";
