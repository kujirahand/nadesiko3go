package guilib

import (
	"errors"
	"fmt"
	"sort"
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
	Canvas     *CanvasDrawing    `json:"canvas,omitempty"`
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
	// Checked はcheckbox・radioの選択状態。falseも意味を持つ(「外す」操作)ので
	// ポインタにして、省略(nil)と区別する。
	Checked *bool `json:"checked,omitempty"`
}

// EventField は画面から届く入力部品1つ分の状態。
// Checked はcheckbox・radioのときだけ入り、それ以外(text・select等)ではnilになる。
// 「選択されていない(false)」と「選択という概念がない(nil)」を型で区別するため、
// boolではなくポインタにしている。
type EventField struct {
	Value   string `json:"value"`
	Checked *bool  `json:"checked,omitempty"`
}

// EventPayload は画面部品のイベントとともにGo側へ渡す状態一式。
// Values のキーは画面部品のハンドル(10進文字列)。DropFiles は drop イベントで
// ドロップされたファイルの配列で、入力部品の値とは別の専用フィールドにしてある。
type EventPayload struct {
	Values    map[string]EventField `json:"values,omitempty"`
	DropFiles []string              `json:"dropFiles,omitempty"`
}

type screenNode struct {
	strokeWidth float64
	strokeColor string
	fillColor   string
	handle      int
	tag         string
	text        string
	html        string
	name        string
	checked     bool
	parent      int
	styles      map[string]string
	attributes  map[string]string
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
	// HTMLと同じく、checked属性があれば(値が何であれ)選択済みで始める。
	_, checked := attrs["checked"]
	s.nodes[h].checked = checked
	op := Operation{
		Type: "create", Handle: h, Parent: parent, Tag: tag, Text: text, HTML: html, Name: name,
		Attributes: attrs, Detached: detached,
	}
	if checked {
		op.Checked = &checked
	}
	s.operations = append(s.operations, op)
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
				_, checked := attrs["checked"]
				s.nodes[handle] = &screenNode{
					handle: handle, tag: dom.Data, name: attrs["name"], parent: parent,
					text: attrs["value"], checked: checked, styles: map[string]string{}, attributes: attrs,
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

// checkable はcheckboxまたはradioのinput要素か。選択状態を持つのはこの2種だけ。
func (n *screenNode) checkable() bool {
	if n.tag != "input" {
		return false
	}
	switch strings.ToLower(n.attributes["type"]) {
	case "checkbox", "radio":
		return true
	}
	return false
}

// formScope はnodeが属するform要素のハンドルを返す(無ければ0)。
// radioの排他はブラウザと同じく、同じformの同名の中でだけ働かせる。
func (s *Screen) formScope(n *screenNode) int {
	for h := n.parent; h > 0; {
		p := s.nodes[h]
		if p == nil {
			break
		}
		if p.tag == "form" {
			return h
		}
		h = p.parent
	}
	return 0
}

// setChecked は選択状態を変える。radioを選ぶと、同じform内の同名のradioは外れる
// (画面側のDOMも同じ動きをするので、Go側のモデルをそろえるだけで操作は1つ送る)。
func (s *Screen) setChecked(handle int, on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.node(handle, "チェック設定")
	if err != nil {
		return err
	}
	if !n.checkable() {
		return errors.New("『チェック設定』にはcheckboxまたはradioの部品を指定してください。")
	}
	n.checked = on
	if on && strings.EqualFold(n.attributes["type"], "radio") && n.name != "" {
		scope := s.formScope(n)
		for h, other := range s.nodes {
			if h != handle && other.checkable() && strings.EqualFold(other.attributes["type"], "radio") &&
				other.name == n.name && s.formScope(other) == scope {
				other.checked = false
			}
		}
	}
	s.operations = append(s.operations, Operation{Type: "checked", Handle: handle, Checked: &on})
	return nil
}

func (s *Screen) isChecked(handle int) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.node(handle, "チェック取得")
	if err != nil {
		return false, err
	}
	if !n.checkable() {
		return false, errors.New("『チェック取得』にはcheckboxまたはradioの部品を指定してください。")
	}
	return n.checked, nil
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
		if k == "checked" {
			// checked属性の指定は選択済みにする(HTMLの属性の意味に合わせる)。
			n.checked = true
		}
	}
	s.operations = append(s.operations, Operation{Type: "attributes", Handle: handle, Attributes: attrs})
	return nil
}

func (s *Screen) bind(handle int, event string, binding eventBinding) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if handle != 0 {
		if _, err := s.node(handle, eventName(event)); err != nil {
			return err
		}
	} else if event != "drop" {
		return fmt.Errorf("ウィンドウに%sイベントは登録できません。", event)
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
//
// checkbox・radioの契約(Issue #355):
//   - 『フォーム値』は従来どおり文字列の辞書(キーはハンドルと name)。
//     checkbox・radioは選択中のものだけ載せ、未選択は載せない。HTMLのフォーム送信と
//     同じ慣習で、click/change/submit のどのイベントでも同じ規則にする。
//     radioは同名のうち選択中の1つの値が name に入る(反復順に依存しない)。
//   - 『フォームチェック』は checkbox・radio の選択状態(真偽)の辞書で、未選択も
//     偽として必ず載る。キーはハンドルと name(同名が複数あれば1つでも選択中なら真)。
//   - イベント外では『チェック取得』『チェック設定』で同じ状態を読み書きできる。
func (s *Screen) DispatchEvent(handle int, event string, payload EventPayload) error {
	s.dispatchMu.Lock()
	defer s.dispatchMu.Unlock()

	// 結果を決定的にするため、ハンドルの昇順で処理する。
	keys := make([]int, 0, len(payload.Values))
	for key := range payload.Values {
		if h, err := strconv.Atoi(key); err == nil {
			keys = append(keys, h)
		}
	}
	sort.Ints(keys)

	s.mu.Lock()
	for _, h := range keys {
		field := payload.Values[strconv.Itoa(h)]
		if n := s.nodes[h]; n != nil {
			n.text = field.Value
			if field.Checked != nil && n.checkable() {
				n.checked = *field.Checked
			}
		}
	}
	binding, ok := s.events[handle][event]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("画面部品『%d』に%sイベントは登録されていません。", handle, event)
	}
	formValues := value.NewDict()
	formChecks := value.NewDict()
	for _, h := range keys {
		key := strconv.Itoa(h)
		text := payload.Values[key].Value
		n := s.nodes[h]
		if n == nil || !n.checkable() {
			formValues.Set(key, value.String(text))
			if n != nil && n.name != "" {
				formValues.Set(n.name, value.String(text))
			}
			continue
		}
		formChecks.Set(key, value.Bool(n.checked))
		if n.checked {
			formValues.Set(key, value.String(text))
			if n.name != "" {
				formValues.Set(n.name, value.String(text))
				formChecks.Set(n.name, value.Bool(true))
			}
		} else if n.name != "" {
			// 同名で選択中のものが先にあれば、真のままにする。
			if _, exists := formChecks.Get(n.name); !exists {
				formChecks.Set(n.name, value.Bool(false))
			}
		}
	}
	s.mu.Unlock()

	binding.ctx.SetSysVar("対象", value.Number(float64(handle)))
	binding.ctx.SetSysVar("フォーム値", value.DictValue(formValues))
	binding.ctx.SetSysVar("フォームチェック", value.DictValue(formChecks))
	if event == "drop" {
		items := make([]value.Value, len(payload.DropFiles))
		for i, path := range payload.DropFiles {
			items[i] = value.String(path)
		}
		binding.ctx.SetSysVar("ドロップファイル", value.ArrayValue(value.NewArray(items...)))
	}
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
