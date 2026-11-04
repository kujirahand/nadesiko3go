//go:build windows

#include "drop_windows_interfaces.h"

#include <cctype>
#include <memory>
#include <new>
#include <string>

// S01: native trustの現在値を照会する。失効後は同じoriginのメッセージでも
// ローカルパスを返さないため、drop listenerもこの値を見る。
extern "C" int webview_trust_installed(void *w);

// COMの取得した参照は、成功・失敗のどちらの経路でも必ず解放する。
template <class T> struct GonakoComPtr {
  T *value = nullptr;
  ~GonakoComPtr() {
    if (value) value->Release();
  }
  GonakoComPtr() = default;
  GonakoComPtr(const GonakoComPtr &) = delete;
  GonakoComPtr &operator=(const GonakoComPtr &) = delete;
};

struct GonakoCoTaskMemFree {
  void operator()(wchar_t *value) const { CoTaskMemFree(value); }
};
using GonakoComString = std::unique_ptr<wchar_t, GonakoCoTaskMemFree>;

// パスに含まれる引用符、改行、バックスラッシュもJSONとして正しく送る。
static std::wstring gonakoJSONQuote(const std::wstring &value) {
  const wchar_t hex[] = L"0123456789abcdef";
  std::wstring result = L"\"";
  for (wchar_t ch : value) {
    if (ch == L'"' || ch == L'\\') {
      result += L'\\';
      result += ch;
    } else if (ch < 0x20) {
      result += L"\\u00";
      result += hex[(ch >> 4) & 0xf];
      result += hex[ch & 0xf];
    } else {
      result += ch;
    }
  }
  result += L'"';
  return result;
}


// S01: 保護windowの厳密origin。scheme/host/portの完全一致だけを許可する。
struct GonakoOrigin {
  std::string scheme;
  std::string host;
  std::string port;
};

static std::string gonakoLower(std::string value) {
  for (auto &ch : value) {
    ch = static_cast<char>(std::tolower(static_cast<unsigned char>(ch)));
  }
  return value;
}

static bool gonakoParseAuthority(const std::string &url, std::string &scheme,
                                 std::string &host, std::string &port) {
  scheme.clear();
  host.clear();
  port.clear();
  auto sep = url.find("://");
  if (sep == std::string::npos || sep == 0) return false;
  scheme = gonakoLower(url.substr(0, sep));
  if (scheme != "http" && scheme != "https") return false;
  auto rest = url.substr(sep + 3);
  auto end = rest.find_first_of("/?#");
  auto authority = rest.substr(0, end);
  if (authority.empty()) return false;
  if (authority.find('@') != std::string::npos ||
      authority.find('[') != std::string::npos ||
      authority.find(']') != std::string::npos) {
    return false;
  }
  for (char ch : authority) {
    auto c = static_cast<unsigned char>(ch);
    if (c <= 0x20 || c == 0x7f) return false;
  }
  auto colon = authority.rfind(':');
  if (colon == std::string::npos) {
    host = gonakoLower(authority);
    port = scheme == "https" ? "443" : "80";
  } else {
    host = gonakoLower(authority.substr(0, colon));
    port = authority.substr(colon + 1);
    if (host.empty() || port.empty()) return false;
    for (char ch : port) {
      if (!std::isdigit(static_cast<unsigned char>(ch))) return false;
    }
  }
  return !host.empty();
}

static bool gonakoParseOrigin(const char *origin, GonakoOrigin &out) {
  if (!origin) return false;
  return gonakoParseAuthority(origin, out.scheme, out.host, out.port);
}

static bool gonakoOriginAllows(const GonakoOrigin &origin,
                               const std::string &url) {
  std::string scheme, host, port;
  if (!gonakoParseAuthority(url, scheme, host, port)) return false;
  return scheme == origin.scheme && host == origin.host && port == origin.port;
}

static std::string gonakoNarrow(const wchar_t *value) {
  if (!value) return std::string();
  int length = WideCharToMultiByte(CP_UTF8, 0, value, -1, nullptr, 0, nullptr,
                                   nullptr);
  if (length <= 1) return std::string();
  std::string out(static_cast<size_t>(length), '\0');
  WideCharToMultiByte(CP_UTF8, 0, value, -1, &out[0], length, nullptr, nullptr);
  out.resize(static_cast<size_t>(length - 1));
  return out;
}

class GonakoFileDropListener final
    : public ICoreWebView2WebMessageReceivedEventHandler {
  LONG refs = 1;
  GonakoOrigin origin;
  void *trustHandle = nullptr;

public:
  GonakoFileDropListener(const GonakoOrigin &origin, void *trustHandle)
      : origin(origin), trustHandle(trustHandle) {}
  HRESULT STDMETHODCALLTYPE QueryInterface(REFIID iid, void **out) override {
    if (!out) return E_POINTER;
    *out = nullptr;
    if (IsEqualIID(iid, IID_IUnknown) ||
        IsEqualIID(iid, IID_ICoreWebView2WebMessageReceivedEventHandler)) {
      *out = static_cast<ICoreWebView2WebMessageReceivedEventHandler *>(this);
      AddRef();
      return S_OK;
    }
    return E_NOINTERFACE;
  }

  ULONG STDMETHODCALLTYPE AddRef() override {
    return InterlockedIncrement(&refs);
  }

  ULONG STDMETHODCALLTYPE Release() override {
    ULONG remaining = InterlockedDecrement(&refs);
    if (remaining == 0) delete this;
    return remaining;
  }

  HRESULT STDMETHODCALLTYPE Invoke(
      ICoreWebView2 *sender,
      ICoreWebView2WebMessageReceivedEventArgs *args) override {
    try {
      // S01: trustが失効した後は、キュー済みのメッセージでもローカルパスを
      // 返さない。originだけでは、失効後に同じURLをsourceとするメッセージを
      // 区別できないため、native trustの現在値を先に確認する。
      if (!trustHandle || !webview_trust_installed(trustHandle)) return S_OK;
      LPWSTR source = nullptr;
      if (FAILED(args->get_Source(&source)) || !source) {
        CoTaskMemFree(source);
        return S_OK;
      }
      std::string senderText = gonakoNarrow(source);
      CoTaskMemFree(source);
      if (!gonakoOriginAllows(origin, senderText)) return S_OK;
      LPWSTR rawMessage = nullptr;
      HRESULT result = args->TryGetWebMessageAsString(&rawMessage);
      GonakoComString message(rawMessage);
      if (FAILED(result) || !message) return S_OK;
      const std::wstring prefix = L"gonako-file-drop-paths:";
      const std::wstring request(message.get());
      // 通常のRPCはwebview_goの既存ハンドラーが処理する。
      if (request.compare(0, prefix.size(), prefix) != 0) return S_OK;

      std::wstring response =
          L"{\"type\":\"gonako-file-drop-paths\",\"requestId\":" +
          gonakoJSONQuote(request.substr(prefix.size())) + L",\"paths\":[";
      GonakoComPtr<IGonakoWebMessageReceivedEventArgs2> args2;
      GonakoComPtr<IGonakoWebView2ObjectCollectionView> objects;
      result = args->QueryInterface(IID_GonakoWebMessageReceivedEventArgs2,
                                   reinterpret_cast<void **>(&args2.value));
      if (SUCCEEDED(result) && args2.value &&
          SUCCEEDED(args2.value->get_AdditionalObjects(&objects.value)) &&
          objects.value) {
        UINT32 count = 0;
        if (SUCCEEDED(objects.value->get_Count(&count))) {
          bool first = true;
          for (UINT32 i = 0; i < count; ++i) {
            GonakoComPtr<IUnknown> object;
            GonakoComPtr<IGonakoWebView2File> file;
            if (FAILED(objects.value->GetValueAtIndex(i, &object.value)) ||
                !object.value) continue;
            result = object.value->QueryInterface(
                IID_GonakoWebView2File, reinterpret_cast<void **>(&file.value));
            if (FAILED(result) || !file.value) continue;
            LPWSTR rawPath = nullptr;
            result = file.value->get_Path(&rawPath);
            GonakoComString path(rawPath);
            if (FAILED(result) || !path) continue;
            if (!first) response += L",";
            response += gonakoJSONQuote(path.get());
            first = false;
          }
        }
      }
      response += L"]}";
      return sender->PostWebMessageAsJson(response.c_str());
    } catch (const std::bad_alloc &) {
      return E_OUTOFMEMORY;
    } catch (...) {
      return E_FAIL;
    }
  }
};

// UIスレッドから呼ぶ。コントローラーは借用し、取得したWebView参照は解放する。
// trustHandleはnative trustの照会にだけ使う不透明ハンドルである。
extern "C" long gonakoInstallFileDropListener(void *controller,
                                              const char *originText,
                                              void *trustHandle) {
  if (!controller) return E_POINTER;
  GonakoOrigin origin;
  if (!gonakoParseOrigin(originText, origin)) return E_INVALIDARG;
  GonakoComPtr<ICoreWebView2> view;
  HRESULT result = static_cast<ICoreWebView2Controller *>(controller)
                       ->get_CoreWebView2(&view.value);
  if (FAILED(result)) return result;
  if (!view.value) return E_POINTER;
  auto *listener = new (std::nothrow) GonakoFileDropListener(origin, trustHandle);
  if (!listener) return E_OUTOFMEMORY;
  EventRegistrationToken token{};
  result = view.value->add_WebMessageReceived(listener, &token);
  // 登録成功時はWebViewが参照を保持し、ウィンドウの破棄時に解放する。
  // ハンドラー側はWebView参照を持たないので循環参照にはならない。
  listener->Release();
  return result;
}
