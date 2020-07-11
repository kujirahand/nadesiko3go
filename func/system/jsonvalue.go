package system

import (
	"fmt"
	"strconv"

	"github.com/kujirahand/nadesiko3go/runeutil"
	"github.com/kujirahand/nadesiko3go/value"
)

type jsonParser struct {
	src []rune
	i   int
}

// JSONDecode : JSON Decoder
func JSONDecode(jsonStr string) (*value.Value, error) {
	parser := jsonParser{
		src: []rune(jsonStr),
		i:   0,
	}
	return parser.parse()
}

func (p *jsonParser) isLive() bool {
	return p.i < len(p.src)
}

func (p *jsonParser) peek() rune {
	return p.src[p.i]
}

func (p *jsonParser) parse() (*value.Value, error) {
	p.skipSpace()
	if !p.isLive() {
		return value.NewNullPtr(), nil
	}
	c := p.peek()
	// check
	if runeutil.IsDigit(c) || c == '+' || c == '-' { // number
		return p.getNumber()
	}
	if c == '"' || c == '\'' { // string
		return p.getString(c)
	}
	if c == '[' {
		return p.getArray()
	}
	if c == '{' {
		return p.getObject()
	}
	// check bool
	if p.eq("true") {
		p.i += len("true")
		return value.NewBoolPtr(true), nil
	}
	if p.eq("false") {
		p.i += len("false")
		return value.NewBoolPtr(false), nil
	}
	// check null
	if p.eq("null") {
		p.i += len("null")
		return value.NewNullPtr(), nil
	}
	// error
	return nil, fmt.Errorf("Unknown Char:" + string(c))
}

func (p *jsonParser) eq(word string) bool {
	if p.i+len(word) > len(p.src) {
		return false
	}
	cword := []rune(word)
	for i := 0; i < len(cword); i++ {
		if cword[i] != p.src[i+p.i] {
			return false
		}
	}
	return true
}

func (p *jsonParser) skipSpace() {
	for p.isLive() {
		c := p.peek()
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			p.i++
			continue
		}
		break
	}
}

func (p *jsonParser) getArray() (*value.Value, error) {
	p.i++ // skip "["
	a := value.NewArrayPtr()
	for p.isLive() {
		p.skipSpace()
		c := p.peek()
		if c == ']' {
			p.i++ // skip "]"
			return a, nil
		}
		v, err := p.parse()
		//println(v.ToString())
		if err != nil {
			return nil, err
		}
		a.Append(v)
		p.skipSpace()
		if p.peek() == ',' {
			p.i++
			continue
		}
		if p.peek() == ']' {
			p.i++ // skip "]"
			return a, nil
		}
		return nil, fmt.Errorf("Arrayで']'がありません")
	}
	return a, nil
}

func (p *jsonParser) getObject() (*value.Value, error) {
	p.i++ // skip "{"
	h := value.NewHashPtr()
	for p.isLive() {
		p.skipSpace()
		c := p.peek()
		if c == '}' {
			p.i++ // skip "}"
			return h, nil
		}
		// key
		key := ""
		if c == '"' || c == '\'' {
			k, err := p.getStringRaw(c)
			if err != nil {
				return nil, fmt.Errorf("Object式でキーがありません。%s", err.Error())
			}
			key = k
		}
		p.skipSpace()
		c = p.peek()
		if c != ':' {
			return nil, fmt.Errorf("Object式で':'がありません。")
		}
		p.i++
		v, err := p.parse()
		if err != nil {
			return nil, err
		}
		h.HashSet(key, v)
		p.skipSpace()
		if p.peek() == ',' {
			p.i++
		}
	}
	return h, nil
}

func (p *jsonParser) getString(quote rune) (*value.Value, error) {
	s, err := p.getStringRaw(quote)
	if err != nil {
		return nil, err
	}
	return value.NewStrPtr(s), nil
}

func (p *jsonParser) getStringRaw(quote rune) (string, error) {
	p.skipSpace()
	p.i++ // skip quote
	s := ""
	for p.isLive() {
		c := p.peek()
		if c == quote {
			p.i++
			break
		}
		// quote
		if c == '\\' {
			p.i++ // skip \\
			c = p.peek()
			p.i++ // skip c
			switch c {
			case 't':
				s += "\t"
			case 'r':
				s += "\r"
			case 'n':
				s += "\n"
			case 'b':
				s += "\b"
			case 'f':
				s += "\f"
			case 'u', 'x':
				// uXXXX
				hex := ""
				for p.isLive() {
					c := p.peek()
					if runeutil.IsHexDigit(c) {
						hex += string(c)
						p.i++
						continue
					}
					break
				}
				i, err := strconv.ParseInt(hex, 16, 32)
				if err == nil {
					s += string(rune(i))
				}
			default:
				s += string(c)
			}
			continue
		}
		s += string(c)
		p.i++
	}
	return s, nil
}

func (p *jsonParser) getNumber() (*value.Value, error) {
	p.skipSpace()
	s := ""
	for p.isLive() {
		c := p.peek()
		if runeutil.IsDigit(c) || c == '.' || c == 'e' || c == 'E' || c == '+' || c == '-' {
			s += string(c)
			p.i++
			continue
		}
		break
	}
	v := value.NewByType(value.Float, s)
	return v, nil
}
