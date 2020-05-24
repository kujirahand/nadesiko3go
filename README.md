# nadesiko3go

golangによるなでしこv3実装。
今のところコマンドラインから計算、条件分岐、繰り返しが可能。
FizzBuzzもなんとか動く感じ。

```
[USAGE]
  cnako3go -e "source"
  cnako3go file.nako3

[Options]
  -d	Debug Mode
  -e (source)	Eval Mode
```

## コンパイルの方法

### (1) Go言語

まずはGo言語をインストール。

### (2) goyacc が必要

```
go get golang.org/x/tools/cmd/goyacc
```

### (3) make.shを実行

```
./make.sh
```

すると、cnako3goが生成される。




