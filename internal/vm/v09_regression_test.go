package vm_test

import "testing"

// Issue V09: JSONエンコード整形 がキー順と文字表現を変えていた。
// 実際の入口（なでしこソース）から整形しても、JSONエンコード と同じ表現を保つ。
func TestV09JSONIndentFromSource(t *testing.T) {
	tests := []struct{ name, code, want string }{
		{
			name: "z、aのキー順と<>",
			code: "D={\"z\":\"<\",\"a\":\">\"}\nDをJSONエンコードして表示\nDをJSONエンコード整形して表示",
			want: "{\"z\":\"<\",\"a\":\">\"}\n" +
				"{\n  \"z\": \"<\",\n  \"a\": \">\"\n}",
		},
		{
			name: "入れ子と配列と&",
			code: "A={\"z\":{\"b\":1,\"a\":[1,2]},\"a\":\"&\"}\nAをJSONエンコード整形して表示",
			want: "{\n  \"z\": {\n    \"b\": 1,\n    \"a\": [\n      1,\n      2\n    ]\n  },\n  \"a\": \"&\"\n}",
		},
		{
			name: "日本語はエスケープしない",
			code: "A={\"名\":\"太郎\"}\nAをJSONエンコード整形して表示",
			want: "{\n  \"名\": \"太郎\"\n}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.code); got != tt.want {
				t.Fatalf("code=%q\ngot=%q\nwant=%q", tt.code, got, tt.want)
			}
		})
	}
}
