package token

// Josi : 助詞一覧
var Josi = []string{
	"について", "くらい", "なのか", "までを", "までの",
	"とは", "から", "まで", "だけ", "より", "ほど", "など",
	"いて", "えて", "きて", "けて", "して", "って", "にて", "みて",
	"めて", "ねて", "では", "には", "は~",
	"は", "を", "に", "へ", "で", "と", "が", "の",
}

// JosiRenbun : 上記助詞で、連文認定する助詞
var JosiRenbun = []string{
	"いて", "えて", "きて", "けて", "して", "って", "にて", "みて", "めて", "ねて", "には",
}

// JosiTarareba : たら・れば
var JosiTarareba = []string{
	"でなければ", "ならば", "なら", "たら", "れば",
}