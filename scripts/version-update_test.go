//go:build ignore

package main

import (
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		want       parsedArgs
		wantErr    bool
	}{
		{
			name: "位置引数なし、フラグなし",
			args: []string{},
			want: parsedArgs{},
		},
		{
			name: "位置引数のみ",
			args: []string{"3.8.2"},
			want: parsedArgs{positional: []string{"3.8.2"}},
		},
		{
			name: "前置フラグ --check",
			args: []string{"--check"},
			want: parsedArgs{check: true},
		},
		{
			name: "前置フラグ --nadesiko",
			args: []string{"--nadesiko", "3.9.0"},
			want: parsedArgs{nadesiko: "3.9.0"},
		},
		{
			name: "前置フラグ --stable",
			args: []string{"--stable", "3.8.2"},
			want: parsedArgs{stable: "3.8.2"},
		},
		{
			name: "位置引数の後に --nadesiko（Issue #211 のケース）",
			args: []string{"3.8.2", "--nadesiko", "3.9.0"},
			want: parsedArgs{
				positional: []string{"3.8.2"},
				nadesiko:   "3.9.0",
			},
		},
		{
			name: "位置引数の後に --check",
			args: []string{"3.8.2", "--check"},
			want: parsedArgs{
				check:      true,
				positional: []string{"3.8.2"},
			},
		},
		{
			name: "フラグが混在",
			args: []string{"--nadesiko", "3.9.0", "3.8.2", "--check"},
			want: parsedArgs{
				check:      true,
				nadesiko:   "3.9.0",
				positional: []string{"3.8.2"},
			},
		},
		{
			name: "終端マーカー --",
			args: []string{"--check", "--", "--nadesiko", "3.9.0"},
			want: parsedArgs{
				check:      true,
				positional: []string{"--nadesiko", "3.9.0"},
			},
		},
		{
			name:    "--nadesiko の後に値がない",
			args:    []string{"--nadesiko"},
			wantErr: true,
		},
		{
			name:    "--stable の後に値がない",
			args:    []string{"--stable"},
			wantErr: true,
		},
		{
			name:    "未知のフラグ",
			args:    []string{"--unknown"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.check != tt.want.check {
				t.Errorf("check = %v, want %v", got.check, tt.want.check)
			}
			if got.nadesiko != tt.want.nadesiko {
				t.Errorf("nadesiko = %q, want %q", got.nadesiko, tt.want.nadesiko)
			}
			if got.stable != tt.want.stable {
				t.Errorf("stable = %q, want %q", got.stable, tt.want.stable)
			}
			if len(got.positional) != len(tt.want.positional) {
				t.Errorf("positional = %v, want %v", got.positional, tt.want.positional)
				return
			}
			for i := range got.positional {
				if got.positional[i] != tt.want.positional[i] {
					t.Errorf("positional[%d] = %q, want %q", i, got.positional[i], tt.want.positional[i])
				}
			}
		})
	}
}
