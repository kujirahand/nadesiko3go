package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func canvasPNG(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data.Bytes())
}

// ファイルへ保存してから同じ画像を読み、描画完了後に別形式で保存する。
func TestCanvasImageSampleAndRequests(t *testing.T) {
	code, err := os.ReadFile("ui/samples/18_キャンバス画像描画と保存.nako3")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	first := filepath.Join(dir, "キャンバス.png")
	second := filepath.Join(dir, "キャンバス縮小.jpg")
	source := strings.ReplaceAll(string(code), "キャンバス.png", first)
	source = strings.ReplaceAll(source, "キャンバス縮小.jpg", second)
	session := &guiSession{}
	runID := session.start(source, "canvas.nako3", true, nil, nil)
	collector := newAsyncCollector(session, runID)
	pngData := canvasPNG(t)
	var requests []json.RawMessage
	for i, action := range []string{"save", "image", "save"} {
		status := waitForDialog(t, collector)
		if status.Dialog.Kind != "canvas" {
			t.Fatalf("要求: %#v", status.Dialog)
		}
		var request struct {
			Handle         int
			Action, Source string
			LineWidth      float64
			StrokeColor    string
			Coordinates    []float64
		}
		if err := json.Unmarshal([]byte(status.Dialog.Message), &request); err != nil {
			t.Fatal(err)
		}
		if request.Action != action || request.Handle != []int{1, 2, 2}[i] {
			t.Fatalf("要求%d: %#v", i, request)
		}
		// 要求が届く時点で、先行する作成・描画が必ず画面へ届いている。
		if i == 0 && (len(collector.ops) < 5 || collector.ops[0].Tag != "canvas") {
			t.Fatalf("先行操作: %#v", collector.ops)
		}
		answer := pngData
		if action == "image" {
			if request.LineWidth != 4 || request.StrokeColor != "navy" {
				t.Fatalf("画像枠の設定=%#v", request)
			}
			if request.Source != pngData {
				t.Fatal("保存したPNGが読み込まれていません")
			}
			if len(request.Coordinates) != 4 || request.Coordinates[2] != 160 {
				t.Fatalf("拡大縮小: %#v", request)
			}
			answer = ""
		}
		requests = append(requests, json.RawMessage(status.Dialog.Message))
		if !session.resolveDialog(runID, status.Dialog.ID, answer, true) {
			t.Fatal("要求を解決できません")
		}
	}
	result := waitForAsyncDone(t, collector)
	if result.Result == nil || !result.Result.OK {
		t.Fatalf("実行結果: %#v", result.Result)
	}
	for filename, want := range map[string]string{first: "png", second: "jpeg"} {
		data, err := os.ReadFile(filename)
		if err != nil {
			t.Fatal(err)
		}
		img, format, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if format != want || img.Bounds().Dx() != 2 {
			t.Fatalf("保存画像: %s %v", format, img.Bounds())
		}
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("画面側の検証にはNode.jsが必要です")
	}
	payload, _ := json.Marshal(requests)
	cmd := exec.Command(node, "testdata/canvas-image-regression.cjs")
	cmd.Stdin = bytes.NewReader(payload)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("画像の橋渡し: %v\n%s", err, output)
	}
}

func TestCanvasImageSaveFormatsAndErrors(t *testing.T) {
	for _, ext := range []string{".png", ".jpg", ".gif", ""} {
		t.Run(ext, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "画像"+ext)
			code := `絵=[2,2]のキャンバス作成;絵を「` + path + `」にキャンバス画像保存`
			session := &guiSession{}
			id := session.start(code, "save.nako3", true, nil, nil)
			c := newAsyncCollector(session, id)
			status := waitForDialog(t, c)
			session.resolveDialog(id, status.Dialog.ID, canvasPNG(t), true)
			done := waitForAsyncDone(t, c)
			if !done.Result.OK {
				t.Fatal(done.Result.Error)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			_, format, err := image.DecodeConfig(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			want := "png"
			if ext == ".jpg" {
				want = "jpeg"
			}
			if ext == ".gif" {
				want = "gif"
			}
			if format != want {
				t.Fatalf("形式=%s", format)
			}
		})
	}
	for _, answer := range []struct {
		text     string
		accepted bool
	}{{"壊れた画像", true}, {"キャンバスの取得エラー", false}, {"data:image/png;base64,aW52YWxpZA==", true}} {
		t.Run(answer.text, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "既存.png")
			os.WriteFile(path, []byte("既存の内容"), 0600)
			session := &guiSession{}
			id := session.start(`絵=[2,2]のキャンバス作成;絵を「`+path+`」にキャンバス画像保存`, "save.nako3", true, nil, nil)
			c := newAsyncCollector(session, id)
			status := waitForDialog(t, c)
			session.resolveDialog(id, status.Dialog.ID, answer.text, answer.accepted)
			done := waitForAsyncDone(t, c)
			if done.Result.OK {
				t.Fatal("失敗すべき要求が成功しました")
			}
			data, _ := os.ReadFile(path)
			if string(data) != "既存の内容" {
				t.Fatal("失敗時に既存ファイルを書き換えました")
			}
		})
	}
}

// イベント内の保存要求も、クリックの実行IDへ応答を返して最後まで進める。
func TestCanvasImageSaveInEvent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "クリック.png")
	session := &guiSession{}
	initial := session.run(`絵=[2,2]のキャンバス作成
保存ボタン=「保存」のボタン作成
保存ボタンをクリックした時には
 絵を「`+path+`」にキャンバス画像保存
ここまで`, "canvas.nako3", true, nil, nil)
	if !initial.OK {
		t.Fatal(initial.Error)
	}
	event := session.startEvent(initial.RunID, 2, "click", nil)
	if event.Error != "" {
		t.Fatal(event.Error)
	}
	collector := newAsyncCollector(session, event.RunID)
	status := waitForDialog(t, collector)
	if status.Dialog.Kind != "canvas" {
		t.Fatalf("要求: %#v", status.Dialog)
	}
	session.resolveDialog(event.RunID, status.Dialog.ID, canvasPNG(t), true)
	result := waitForAsyncDone(t, collector)
	if !result.Result.OK {
		t.Fatal(result.Result.Error)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

// PNGだけが透明を保持し、現行JPEG・GIF保存では透明部分が黒になる仕様を固定する。
func TestCanvasImageSaveTransparency(t *testing.T) {
	var source bytes.Buffer
	if err := png.Encode(&source, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	answer := "data:image/png;base64," + base64.StdEncoding.EncodeToString(source.Bytes())
	for _, ext := range []string{".png", ".jpg", ".gif"} {
		t.Run(ext, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "透明"+ext)
			session := &guiSession{}
			id := session.start(`絵=[8,8]のキャンバス作成;絵を「`+path+`」にキャンバス画像保存`, "transparent.nako3", true, nil, nil)
			c := newAsyncCollector(session, id)
			request := waitForDialog(t, c)
			session.resolveDialog(id, request.Dialog.ID, answer, true)
			done := waitForAsyncDone(t, c)
			if !done.Result.OK {
				t.Fatal(done.Result.Error)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			img, _, err := image.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			r, g, b, a := img.At(4, 4).RGBA()
			if ext == ".png" {
				if a != 0 {
					t.Fatalf("PNGの透明度=%d", a)
				}
			} else if r != 0 || g != 0 || b != 0 || a != 65535 {
				t.Fatalf("保存色=%d,%d,%d,%d", r, g, b, a)
			}
		})
	}
}
