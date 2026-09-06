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

echo "All tests passed successfully!\n";
