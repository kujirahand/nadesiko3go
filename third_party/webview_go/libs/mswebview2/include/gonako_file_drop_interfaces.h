// WebView2のファイルドロップで使う公開COMインターフェース。
#pragma once

#include <WebView2.h>

static const IID IID_GonakoWebMessageReceivedEventArgs2 = {
    0x06fc7ab7, 0xc90c, 0x4297, {0x93, 0x89, 0x33, 0xca, 0x01, 0xcf, 0x6d, 0x5e}};
static const IID IID_GonakoWebView2ObjectCollectionView = {
    0x0f36fd87, 0x4f69, 0x4415, {0x98, 0xda, 0x88, 0x8f, 0x89, 0xfb, 0x9a, 0x33}};
static const IID IID_GonakoWebView2File = {
    0xf2c19559, 0x6bc1, 0x4583, {0xa7, 0x57, 0x90, 0x02, 0x1b, 0xe9, 0xaf, 0xec}};

struct IGonakoWebView2ObjectCollectionView : IUnknown {
  virtual HRESULT STDMETHODCALLTYPE get_Count(UINT32 *value) = 0;
  virtual HRESULT STDMETHODCALLTYPE GetValueAtIndex(UINT32 index,
                                                      IUnknown **value) = 0;
};

struct IGonakoWebView2File : IUnknown {
  virtual HRESULT STDMETHODCALLTYPE get_Path(LPWSTR *value) = 0;
};

struct IGonakoWebMessageReceivedEventArgs2
    : ICoreWebView2WebMessageReceivedEventArgs {
  virtual HRESULT STDMETHODCALLTYPE get_AdditionalObjects(
      IGonakoWebView2ObjectCollectionView **value) = 0;
};
