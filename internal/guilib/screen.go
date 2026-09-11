package guilib

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Operation is one DOM mutation sent to the WebView frontend.
// Nadesiko programs only see numeric handles; DOM objects never cross the VM
// boundary.
type Operation struct {
	Type       string            `json:"type"`
	Handle     int               `json:"handle,omitempty"`
	Parent     int               `json:"parent,omitempty"`
	Tag        string            `json:"tag,omitempty"`
	Text       string            `json:"text,omitempty"`
	HTML       string            `json:"html,omitempty"`
	Name       string            `json:"name,omitempty"`
	Event      string            `json:"event,omitempty"`
	Styles     map[string]string `json:"styles,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Detached   bool              `json:"detached,omitempty"`
}

type screenNode struct {
	handle     int
	tag        string
	text       string
	html       string
	name       string
	parent     int
	styles     map[string]string
	attributes map[string]string
}

type eventBinding struct {
	ctx stdlib.Context
	fn  *value.Func
}

// Screen is the Go-side model of one rendered GUI. It is deliberately free of
// webview_go so command behaviour can be tested without opening a window.
type Screen struct {
	mu         sync.Mutex
	dispatchMu sync.Mutex
	nextHandle int
	domParent  int
	nodes      map[int]*screenNode
	events     map[int]map[string]eventBinding
	operations []Operation
}

// NewScreen creates an empty GUI model.
func NewScreen() *Screen {
	return &Screen{
		nodes:  map[int]*screenNode{},
		events: map[int]map[string]eventBinding{},
	}
}

func (s *Screen) create(tag, text, html, name string, parent int) int {
	return s.createNode(tag, text, html, name, parent, nil, false)
}

func (s *Screen) createWithAttributes(tag, text, html, name string, parent int, attrs map[string]string) int {
	return s.createNode(tag, text, html, name, parent, attrs, false)
}

func (s *Screen) createDetached(tag string) int {
	return s.createNode(tag, "", "", "", -1, nil, true)
}

func (s *Screen) createNode(tag, text, html, name string, parent int, attrs map[string]string, detached bool) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextHandle++
	h := s.nextHandle
	s.nodes[h] = &screenNode{
		handle: h, tag: tag, text: text, html: html, name: name, parent: parent,
		styles: map[string]string{}, attributes: map[string]string{},
	}
	for key, item := range attrs {
		s.nodes[h].attributes[key] = item
	}
	s.operations = append(s.operations, Operation{
		Type: "create", Handle: h, Parent: parent, Tag: tag, Text: text, HTML: html, Name: name,
		Attributes: attrs, Detached: detached,
	})
	return h
}

func (s *Screen) appendOptions(parent int, options []string) {
	s.mu.Lock()
	if selectNode := s.nodes[parent]; selectNode != nil && selectNode.tag == "select" {
		selectNode.text = ""
		if len(options) > 0 {
			selectNode.text = options[0]
		}
	}
	s.mu.Unlock()
	for _, item := range options {
		s.createWithAttributes("option", item, "", "", parent, map[string]string{"value": item})
	}
}

func (s *Screen) replaceOptions(handle int, options []string) error {
	s.mu.Lock()
	node, err := s.node(handle, "セレクトボックスアイテム設定")
	if err != nil {
		s.mu.Unlock()
		return err
	}
	if node.tag != "select" {
		s.mu.Unlock()
		return errors.New("『セレクトボックスアイテム設定』にはselect要素を指定してください。")
	}
	for childHandle, child := range s.nodes {
		if child.parent == handle {
			delete(s.nodes, childHandle)
			delete(s.events, childHandle)
		}
	}
	s.operations = append(s.operations, Operation{Type: "clear", Handle: handle})
	s.mu.Unlock()
	s.appendOptions(handle, options)
	return nil
}

// DisplayText appends escaped text to the screen. Escaping is performed by
// assigning textContent in JavaScript rather than by constructing HTML here.
func (s *Screen) DisplayText(text string) int { return s.create("div", text, "", "", 0) }

// DisplayHTML appends explicitly trusted HTML to the screen.
func (s *Screen) DisplayHTML(markup string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextHandle++
	wrapperHandle := s.nextHandle
	wrapper := &screenNode{
		handle: wrapperHandle, tag: "div", html: markup,
		styles: map[string]string{}, attributes: map[string]string{},
	}
	s.nodes[wrapperHandle] = wrapper

	context := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	fragments, err := html.ParseFragment(strings.NewReader(markup), context)
	if err == nil {
		nodeRefs := map[int]*html.Node{}
		var register func(*html.Node, int)
		register = func(dom *html.Node, parent int) {
			nextParent := parent
			if dom.Type == html.ElementNode {
				s.nextHandle++
				handle := s.nextHandle
				attrs := map[string]string{}
				filtered := dom.Attr[:0]
				for _, attr := range dom.Attr {
					if attr.Key == "data-gonako-handle" {
						continue
					}
					attrs[attr.Key] = attr.Val
					filtered = append(filtered, attr)
				}
				dom.Attr = append(filtered, html.Attribute{Key: "data-gonako-handle", Val: strconv.Itoa(handle)})
				s.nodes[handle] = &screenNode{
					handle: handle, tag: dom.Data, name: attrs["name"], parent: parent,
					text: attrs["value"], styles: map[string]string{}, attributes: attrs,
				}
				nodeRefs[handle] = dom
				nextParent = handle
			}
			for child := dom.FirstChild; child != nil; child = child.NextSibling {
				register(child, nextParent)
			}
		}
		for _, fragment := range fragments {
			register(fragment, wrapperHandle)
		}
		for handle, dom := range nodeRefs {
			s.nodes[handle].html = renderChildren(dom)
			if s.nodes[handle].tag == "textarea" && s.nodes[handle].text == "" {
				s.nodes[handle].text = textContent(dom)
			}
		}
		var rendered strings.Builder
		for _, fragment := range fragments {
			_ = html.Render(&rendered, fragment)
		}
		wrapper.html = rendered.String()
	}

	s.operations = append(s.operations, Operation{
		Type: "create", Handle: wrapperHandle, Tag: "div", HTML: wrapper.html,
	})
	return wrapperHandle
}

func renderChildren(node *html.Node) string {
	var rendered strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		_ = html.Render(&rendered, child)
	}
	return rendered.String()
}

func textContent(node *html.Node) string {
	var text strings.Builder
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.TextNode {
			text.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return text.String()
}

// DrainOperations returns pending DOM changes and clears the queue.
func (s *Screen) DrainOperations() []Operation {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]Operation(nil), s.operations...)
	s.operations = s.operations[:0]
	return out
}

func (s *Screen) node(handle int, command string) (*screenNode, error) {
	n, ok := s.nodes[handle]
	if !ok {
		return nil, fmt.Errorf("『%s』で画面部品ハンドル『%d』が見つかりません。", command, handle)
	}
	return n, nil
}

// setParent は追加先の控えを更新する。正はシステム変数『DOM親要素』だが、
// VMがその変数に記憶領域を割り当てない場合（プログラム中に名前が現れない場合）に
// 備えて、Screen側にも保持する。ハンドルの存在確認は呼び出し側で済ませておく。
func (s *Screen) setParent(handle int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.domParent = handle
}

func (s *Screen) parent() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.domParent
}

func (s *Screen) hasNode(handle int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.nodes[handle]
	return ok
}

func (s *Screen) connected(handle int) bool {
	for handle > 0 {
		n := s.nodes[handle]
		if n == nil || n.parent < 0 {
			return false
		}
		handle = n.parent
	}
	return handle == 0
}

func (s *Screen) append(handle, parent int, command string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.node(handle, command)
	if err != nil {
		return err
	}
	if parent > 0 {
		if _, err := s.node(parent, command); err != nil {
			return err
		}
	}
	for ancestor := parent; ancestor > 0; {
		if ancestor == handle {
			return fmt.Errorf("『%s』で循環する親子関係は作成できません。", command)
		}
		parentNode := s.nodes[ancestor]
		if parentNode == nil {
			break
		}
		ancestor = parentNode.parent
	}
	n.parent = parent
	s.operations = append(s.operations, Operation{Type: "append", Handle: handle, Parent: parent})
	return nil
}

func (s *Screen) remove(handle int, command string) (map[int]struct{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.node(handle, command); err != nil {
		return nil, err
	}
	removed := map[int]struct{}{handle: {}}
	for changed := true; changed; {
		changed = false
		for child, n := range s.nodes {
			if _, ok := removed[child]; ok {
				continue
			}
			if _, ok := removed[n.parent]; ok {
				removed[child] = struct{}{}
				changed = true
			}
		}
	}
	for removedHandle := range removed {
		delete(s.nodes, removedHandle)
		delete(s.events, removedHandle)
	}
	if _, ok := removed[s.domParent]; ok {
		s.domParent = 0
	}
	s.operations = append(s.operations, Operation{Type: "remove", Handle: handle})
	return removed, nil
}

func (s *Screen) setText(handle int, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.node(handle, "テキスト設定")
	if err != nil {
		return err
	}
	n.text, n.html = text, ""
	s.operations = append(s.operations, Operation{Type: "text", Handle: handle, Text: text})
	return nil
}

func (s *Screen) text(handle int) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.node(handle, "テキスト取得")
	if err != nil {
		return "", err
	}
	if n.tag != "input" && n.tag != "textarea" && n.tag != "select" && n.html != "" {
		return n.html, nil
	}
	return n.text, nil
}

func (s *Screen) setHTML(handle int, html string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.node(handle, "HTML変更")
	if err != nil {
		return err
	}
	n.html, n.text = html, ""
	s.operations = append(s.operations, Operation{Type: "html", Handle: handle, HTML: html})
	return nil
}

func (s *Screen) html(handle int) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.node(handle, "HTML取得")
	if err != nil {
		return "", err
	}
	if n.html != "" {
		return n.html, nil
	}
	return n.text, nil
}

func (s *Screen) focus(handle int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.node(handle, "DOM注目"); err != nil {
		return err
	}
	s.operations = append(s.operations, Operation{Type: "focus", Handle: handle})
	return nil
}

func (s *Screen) queryByID(id string) (int, bool) {
	if id == "" {
		return 0, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for handle := 1; handle <= s.nextHandle; handle++ {
		n := s.nodes[handle]
		if n == nil || !s.connected(handle) {
			continue
		}
		// id属性を持たないノードは対象外。map[string]stringの零値""と
		// 突き合わせると、id無しのノード全部に一致してしまう。
		if got, ok := n.attributes["id"]; ok && got == id {
			return handle, true
		}
	}
	return 0, false
}

func (s *Screen) query(selector string) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for handle := 1; handle <= s.nextHandle; handle++ {
		if n := s.nodes[handle]; n != nil && s.connected(handle) && matchesSelector(n, selector) {
			return handle, true
		}
	}
	return 0, false
}

func (s *Screen) queryAll(selector string) []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	handles := make([]int, 0)
	for handle := 1; handle <= s.nextHandle; handle++ {
		if n := s.nodes[handle]; n != nil && s.connected(handle) && matchesSelector(n, selector) {
			handles = append(handles, handle)
		}
	}
	return handles
}

// matchesSelector supports the simple selectors used by Nadesiko GUI parts:
// tag, #id, .class, tag#id, tag.class and [attribute=value]. Comma-separated
// alternatives are also accepted. Descendant and pseudo selectors require a
// live browser DOM and are intentionally outside the numeric-handle model.
func matchesSelector(n *screenNode, selector string) bool {
	for _, alternative := range strings.Split(selector, ",") {
		if matchesSimpleSelector(n, strings.TrimSpace(alternative)) {
			return true
		}
	}
	return false
}

func matchesSimpleSelector(n *screenNode, selector string) bool {
	if selector == "" || strings.ContainsAny(selector, " >+~:") {
		return false
	}
	if selector == "*" {
		return true
	}

	rest := selector
	tagEnd := strings.IndexAny(rest, "#.[]")
	if tagEnd < 0 {
		return strings.EqualFold(n.tag, rest)
	}
	if tagEnd > 0 {
		if !strings.EqualFold(n.tag, rest[:tagEnd]) {
			return false
		}
		rest = rest[tagEnd:]
	}

	for rest != "" {
		switch rest[0] {
		case '#', '.':
			kind := rest[0]
			rest = rest[1:]
			end := strings.IndexAny(rest, "#.[]")
			if end < 0 {
				end = len(rest)
			}
			name := rest[:end]
			if name == "" {
				return false
			}
			if kind == '#' && n.attributes["id"] != name {
				return false
			}
			if kind == '.' && !hasClass(n.attributes["class"], name) {
				return false
			}
			rest = rest[end:]
		case '[':
			end := strings.IndexByte(rest, ']')
			if end < 0 {
				return false
			}
			expr := strings.TrimSpace(rest[1:end])
			parts := strings.SplitN(expr, "=", 2)
			key := strings.TrimSpace(parts[0])
			actual, exists := n.attributes[key]
			if key == "" || !exists {
				return false
			}
			if len(parts) == 2 {
				want := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
				if actual != want {
					return false
				}
			}
			rest = rest[end+1:]
		default:
			return false
		}
	}
	return true
}

func hasClass(classNames, want string) bool {
	for _, name := range strings.Fields(classNames) {
		if name == want {
			return true
		}
	}
	return false
}

func (s *Screen) setStyles(handle int, styles map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.node(handle, "DOMスタイル一括設定")
	if err != nil {
		return err
	}
	for k, v := range styles {
		n.styles[k] = v
	}
	s.operations = append(s.operations, Operation{Type: "styles", Handle: handle, Styles: styles})
	return nil
}

func (s *Screen) setAttributes(handle int, attrs map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.node(handle, "DOM属性一括設定")
	if err != nil {
		return err
	}
	for k, v := range attrs {
		n.attributes[k] = v
		if k == "name" {
			n.name = v
		}
	}
	s.operations = append(s.operations, Operation{Type: "attributes", Handle: handle, Attributes: attrs})
	return nil
}

func (s *Screen) bind(handle int, event string, binding eventBinding) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.node(handle, eventName(event)); err != nil {
		return err
	}
	if s.events[handle] == nil {
		s.events[handle] = map[string]eventBinding{}
	}
	s.events[handle][event] = binding
	s.operations = append(s.operations, Operation{Type: "listen", Handle: handle, Event: event})
	return nil
}

// DispatchEvent updates the Go-side input snapshot, then runs the registered
// nadesiko callback. Calls are serialized because a VM is single-threaded.
func (s *Screen) DispatchEvent(handle int, event string, values map[string]string) error {
	s.dispatchMu.Lock()
	defer s.dispatchMu.Unlock()

	s.mu.Lock()
	for key, text := range values {
		h, err := strconv.Atoi(key)
		if err != nil {
			continue
		}
		if n := s.nodes[h]; n != nil {
			n.text = text
		}
	}
	binding, ok := s.events[handle][event]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("画面部品『%d』に%sイベントは登録されていません。", handle, event)
	}
	formValues := value.NewDict()
	for key, text := range values {
		formValues.Set(key, value.String(text))
		if h, err := strconv.Atoi(key); err == nil {
			if n := s.nodes[h]; n != nil && n.name != "" {
				formValues.Set(n.name, value.String(text))
			}
		}
	}
	s.mu.Unlock()

	binding.ctx.SetSysVar("対象", value.Number(float64(handle)))
	binding.ctx.SetSysVar("フォーム値", value.DictValue(formValues))
	_, err := binding.ctx.CallFunc(binding.fn, nil)
	return err
}

func eventName(event string) string {
	switch event {
	case "click":
		return "クリック時"
	case "change":
		return "変更時"
	case "submit":
		return "フォーム送信時"
	default:
		return "DOMイベント"
	}
}

func handleValue(v value.Value) (int, error) {
	n, ok := v.Number()
	if !ok || n <= 0 || n != float64(int(n)) {
		return 0, errors.New("画面部品には作成命令が返したハンドルを指定してください。")
	}
	return int(n), nil
}

func stringMap(v value.Value) (map[string]string, error) {
	d, ok := v.Dict()
	if !ok || d == nil {
		return nil, errors.New("設定値には辞書を指定してください。")
	}
	out := make(map[string]string, d.Len())
	for _, key := range d.Keys() {
		item, _ := d.Get(key)
		out[key] = value.ToString(item)
	}
	return out, nil
}

func callable(ctx stdlib.Context, v value.Value) (*value.Func, error) {
	if fn, ok := v.Func(); ok {
		return fn, nil
	}
	if v.Kind() == value.KindString {
		if fn := ctx.FindFunc(value.ToString(v)); fn != nil {
			return fn, nil
		}
	}
	return nil, errors.New("イベントに指定できるのは関数だけです。")
}

func formRows(v value.Value) [][2]string {
	var rows [][2]string
	for _, line := range strings.Split(value.ToString(v), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, initial, _ := strings.Cut(line, "=")
		rows = append(rows, [2]string{strings.TrimSpace(key), initial})
	}
	return rows
}
