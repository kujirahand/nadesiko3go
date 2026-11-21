# gonako/gonako-guiをLinuxでビルドする手順

Linuxでの手順は [build-gonako-gui.md](build-gonako-gui.md) にまとめてあります。このページは、以前必要だった pkg-config エイリアスについての説明です。

Ubuntu 24.04 には `libwebkit2gtk-4.0-dev` がありません。以前は `webkit2gtk-4.0.pc` を手で作り、中身を 4.1 へ向ける回避策を案内していました。今は不要です。`just gui` と `scripts/go-gui.sh` が、入っているほう（4.0 または 4.1）を選んでビルドします。

```bash
sudo ./scripts/install-gui-deps.sh
just gui
```

システムディレクトリへ `.pc` ファイルを置かなくてよくなりました。OSを入れ直したあとも、上の2コマンドで再現できます。
