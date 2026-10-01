package versionupdate

import (
	"reflect"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    ParsedArgs
		wantErr bool
	}{
		{
			name: "位置引数なし、フラグなし",
			args: []string{},
			want: ParsedArgs{},
		},
		{
			name: "位置引数のみ",
			args: []string{"3.8.2"},
			want: ParsedArgs{Positional: []string{"3.8.2"}},
		},
		{
			name: "前置フラグ --check",
			args: []string{"--check"},
			want: ParsedArgs{Check: true},
		},
		{
			name: "前置フラグ --nadesiko",
			args: []string{"--nadesiko", "3.9.0"},
			want: ParsedArgs{Nadesiko: "3.9.0"},
		},
		{
			name: "前置フラグ --stable",
			args: []string{"--stable", "3.8.2"},
			want: ParsedArgs{Stable: "3.8.2"},
		},
		{
			name: "位置引数の後に --nadesiko（Issue #211 のケース）",
			args: []string{"3.8.2", "--nadesiko", "3.9.0"},
			want: ParsedArgs{
				Positional: []string{"3.8.2"},
				Nadesiko:   "3.9.0",
			},
		},
		{
			name: "位置引数の後に --check",
			args: []string{"3.8.2", "--check"},
			want: ParsedArgs{
				Check:      true,
				Positional: []string{"3.8.2"},
			},
		},
		{
			name: "フラグが混在",
			args: []string{"--nadesiko", "3.9.0", "3.8.2", "--check"},
			want: ParsedArgs{
				Check:      true,
				Nadesiko:   "3.9.0",
				Positional: []string{"3.8.2"},
			},
		},
		{
			name: "終端マーカー --",
			args: []string{"--check", "--", "--nadesiko", "3.9.0"},
			want: ParsedArgs{
				Check:      true,
				Positional: []string{"--nadesiko", "3.9.0"},
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
			got, err := ParseArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.Check != tt.want.Check {
				t.Errorf("Check = %v, want %v", got.Check, tt.want.Check)
			}
			if got.Nadesiko != tt.want.Nadesiko {
				t.Errorf("Nadesiko = %q, want %q", got.Nadesiko, tt.want.Nadesiko)
			}
			if got.Stable != tt.want.Stable {
				t.Errorf("Stable = %q, want %q", got.Stable, tt.want.Stable)
			}
			if !reflect.DeepEqual(got.Positional, tt.want.Positional) {
				t.Errorf("Positional = %v, want %v", got.Positional, tt.want.Positional)
			}
		})
	}
}
