package token

// ReservedToken : 予約語(トークン)
var ReservedToken = []string{
	"でなければ",
	"ここから",
	"ここまで",
	"ならば",
	"または",
	"かつ",
	"なら",
	"もし",
	"反復",
	"●",
}

// ReservedWord : 予約語
var ReservedWord = map[string]TType{
	"回":     KAI,
	"間":     AIDA,
	"繰返":    FOR,
	"反復":    FOREACH,
	"抜":     BREAK,
	"続":     CONTINUE,
	"戻":     RETURN,
	"先":     SAKINI,
	"次":     TUGINI,
	"代入":    LET,
	"逐次実行":  TIKUJI,
	"変数":    HENSU,
	"定数":    TEISU,
	"取込":    INCLUDE,
	"エラー監視": ERROR_TRY, // 例外処理:エラーならばと対
	"エラー":   ERROR,
	// "それ": "word",
	// "そう": "word", // 「それ」のエイリアス
	"関数":    DEF_FUNC, // 無名関数の定義用
	"●":     DEF_FUNC,
	"ならば":   THEN,
	"なら":    THEN,
	"でなければ": THEN,
	"もし":    IF,
	"違":     ELSE,
	"ここから":  BEGIN,
	"ここまで":  END,
	"かつ":    AND,
	"または":   OR,
}
