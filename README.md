# nadesiko3go

golangによるなでしこv3実装。
今のところコマンドラインから計算、条件分岐、繰り返しが可能。
FizzBuzzもなんとか動く感じ。

```
[USAGE]
  nadesiko3go -e "source"
  nadesiko3go file.nako3
[Options]
  -d	Debug Mode
  -e (source)	Eval Mode
```

## 簡単なビルド方法

Go言語がインストールされている状態にて。

```
$ go get golang.org/x/tools/cmd/goyacc
$ go get github.com/kujirahand/nadesiko3go
$ go install github.com/kujirahand/nadesiko3go
```

## GitHubからリポジトリを取得するコンパイルの方法

現在のところ、なでしこ3(nodejs)のコマンドライン版cnako3が必要。

### (1) goyacc を入手

```
$ go get golang.org/x/tools/cmd/goyacc
```

### (2) git clone

```
$ cd $HOME/src
$ mkdir -p github.com/kujirahand
$ cd github.com/kujirahand
$ git clone git@github.com:kujirahand/nadesiko3go.git
$ cd nadesiko3go
```

### (3) make

```
$ ./make.sh
```





