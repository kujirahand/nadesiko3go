//go:build windows

#include "drop_windows_interfaces.h"

#include <memory>
#include <new>
#include <string>

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

class GonakoFileDropListener final
    : public ICoreWebView2WebMessageReceivedEventHandler {
  LONG refs = 1;

public:
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
extern "C" long gonakoInstallFileDropListener(void *controller) {
  if (!controller) return E_POINTER;
  GonakoComPtr<ICoreWebView2> view;
  HRESULT result = static_cast<ICoreWebView2Controller *>(controller)
                       ->get_CoreWebView2(&view.value);
  if (FAILED(result)) return result;
  if (!view.value) return E_POINTER;
  auto *listener = new (std::nothrow) GonakoFileDropListener();
  if (!listener) return E_OUTOFMEMORY;
  EventRegistrationToken token{};
  result = view.value->add_WebMessageReceived(listener, &token);
  // 登録成功時はWebViewが参照を保持し、ウィンドウの破棄時に解放する。
  // ハンドラー側はWebView参照を持たないので循環参照にはならない。
  listener->Release();
  return result;
}
