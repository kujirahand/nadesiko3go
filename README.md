# nadesiko3go

golangによるなでしこv3実装。
今のところコマンドラインから簡単な計算が実行できる。

```
[USAGE]
  cnako3 -e "source"
  cnako3 file.nako3

[Options]
  -d	Debug Mode
  -e (source)	Eval Mode
```

## コンパイルの方法

### (1) direnv が必要

- ``direnv`` をインストール
  - macOS  : ``brew install direnv``
  - ubuntu : ``apt install direnv``
- シェルにフックを設定(設定ファイルに以下を記述)
  - bash : ``eval "$(direnv hook bash)"``
  - zsh  : ``eval "$(direnv hook zsh)"``
- ``direnv allow`` を実行する

### (2) goyacc が必要

```
go get golang.org/x/tools/cmd/goyacc
```

### (3) make.shを実行

```
./make.sh
```

すると、binディレクトリにcnako3ができる。




