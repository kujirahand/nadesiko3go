package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// installScript はソースをコピーせず、現在のランタイムで起動するスクリプトを配置する。
func installScript(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("install", flag.ContinueOnError)
	flags.SetOutput(stderr)
	binDir := flags.String("bin", "bin", "起動用スクリプトの配置先")
	name := flags.String("name", "", "コマンド名（既定: gonako-ソース名）")
	runtimePath := flags.String("runtime", "", "使用するgonako（既定: 現在の実行ファイル）")
	force := flags.Bool("force", false, "既存ファイルを置き換える")
	source, rest := splitSourceFor(args, "bin", "name", "runtime")
	if err := flags.Parse(rest); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if source == "" || len(flags.Args()) != 0 {
		return errors.New("installにはソースファイルを1つ指定してください")
	}
	ext := filepath.Ext(source)
	if ext != ".nako3" && ext != ".nako" {
		return errors.New("installの対象は.nako3または.nakoファイルです")
	}
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("installの対象は通常のファイルです")
	}
	if *name == "" {
		*name = "gonako-" + strings.TrimSuffix(filepath.Base(source), ext)
	}
	if *name == "." || *name == ".." || strings.ContainsAny(*name, "/\\\r\n\x00") {
		return errors.New("コマンド名にはパスや改行を指定できません")
	}
	if *runtimePath == "" {
		*runtimePath, err = os.Executable()
		if err != nil {
			return err
		}
	}
	*runtimePath, err = filepath.Abs(*runtimePath)
	if err != nil {
		return err
	}
	info, err = os.Stat(*runtimePath)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("ランタイム『%s』が見つかりません", *runtimePath)
	}
	// go runの一時ランタイムなどを明示指定する場合も、その絶対パスを使う。
	script, suffix := installedScript(source, *runtimePath, runtime.GOOS)
	if err := os.MkdirAll(*binDir, 0755); err != nil {
		return err
	}
	output := filepath.Join(*binDir, *name+suffix)
	if !*force {
		file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0755)
		if err != nil {
			return fmt.Errorf("起動用ファイルを作成できません（上書きは--force）: %w", err)
		}
		_, err = io.WriteString(file, script)
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			os.Remove(output)
			return errors.Join(err, closeErr)
		}
	} else {
		// 一時ファイルから置き換え、書込み途中の状態やリンク先への上書きを避ける。
		file, err := os.CreateTemp(*binDir, ".gonako-install-*")
		if err != nil {
			return err
		}
		defer os.Remove(file.Name())
		_, writeErr := io.WriteString(file, script)
		modeErr := file.Chmod(0755)
		closeErr := file.Close()
		if err := errors.Join(writeErr, modeErr, closeErr); err != nil {
			return err
		}
		if err := os.Rename(file.Name(), output); err != nil {
			return err
		}
	}
	fmt.Fprintf(stdout, "起動用ファイルを配置しました: %s\n", output)
	return nil
}

// installedScript は引用符・空白を含むパスでも引数をそのまま渡す。
func installedScript(source, runtimePath, platform string) (string, string) {
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	if platform == "windows" {
		quote = func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
		return "$ErrorActionPreference = 'Stop'\n" +
			"$runtime = $env:GONAKO_RUNTIME\nif (-not $runtime) { $runtime = " + quote(runtimePath) + " }\n" +
			"$previous = $env:GONAKO_DOCTEST_RUNTIME\ntry {\n" +
			"    if (-not $previous) { $env:GONAKO_DOCTEST_RUNTIME = $runtime }\n" +
			"    & $runtime " + quote(source) + " @args\n    $code = $LASTEXITCODE\n" +
			"} finally { $env:GONAKO_DOCTEST_RUNTIME = $previous }\nexit $code\n", ".ps1"
	}
	return "#!/bin/sh\n# gonako installで生成した起動用スクリプト。\nset -eu\n" +
		"runtime=${GONAKO_RUNTIME:-" + quote(runtimePath) + "}\n" +
		"export GONAKO_DOCTEST_RUNTIME=\"${GONAKO_DOCTEST_RUNTIME:-$runtime}\"\n" +
		"exec \"$runtime\" " + quote(source) + " \"$@\"\n", ""
}
