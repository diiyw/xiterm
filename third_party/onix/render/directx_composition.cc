//go:build cgo && windows && !onix_metal && !onix_vulkan && !onix_opengl && !onix_webgpu && !onix_webgl

#include <windows.h>
#include <dcomp.h>
#include <stdlib.h>

struct OnixComposition {
    IDCompositionDevice *device;
    IDCompositionTarget *target;
    IDCompositionVisual *visual;
    IDCompositionRectangleClip *clip;
};

extern "C" void onix_dx_destroy_composition(void *handle) {
    OnixComposition *c = (OnixComposition *)handle;
    if (!c)
        return;
    if (c->clip)
        c->clip->Release();
    if (c->visual)
        c->visual->Release();
    if (c->target)
        c->target->Release();
    if (c->device)
        c->device->Release();
    free(c);
}

extern "C" HRESULT onix_dx_create_composition(IDXGIDevice *device, HWND window, IUnknown *content, void **output) {
    OnixComposition *c = (OnixComposition *)calloc(1, sizeof(*c));
    if (!c)
        return E_OUTOFMEMORY;
    HRESULT result = DCompositionCreateDevice(device, __uuidof(IDCompositionDevice), (void **)&c->device);
    if (SUCCEEDED(result))
        result = c->device->CreateTargetForHwnd(window, TRUE, &c->target);
    if (SUCCEEDED(result))
        result = c->device->CreateVisual(&c->visual);
    if (SUCCEEDED(result))
        result = c->visual->SetContent(content);
    if (SUCCEEDED(result))
        result = c->target->SetRoot(c->visual);
    if (SUCCEEDED(result))
        result = c->device->Commit();
    if (FAILED(result)) {
        onix_dx_destroy_composition(c);
        return result;
    }
    *output = c;
    return S_OK;
}

extern "C" void onix_dx_clip_composition(void *handle, int width, int height, int radius) {
    OnixComposition *c = (OnixComposition *)handle;
    if (!c)
        return;
    if (!radius) {
        c->visual->SetClip((IDCompositionClip *)NULL);
        c->device->Commit();
        return;
    }
    if (!c->clip && FAILED(c->device->CreateRectangleClip(&c->clip)))
        return;
    c->clip->SetLeft(0.f);
    c->clip->SetTop(0.f);
    c->clip->SetRight((float)width);
    c->clip->SetBottom((float)height);
    float r = (float)radius;
    c->clip->SetTopLeftRadiusX(r);
    c->clip->SetTopLeftRadiusY(r);
    c->clip->SetTopRightRadiusX(r);
    c->clip->SetTopRightRadiusY(r);
    c->clip->SetBottomLeftRadiusX(r);
    c->clip->SetBottomLeftRadiusY(r);
    c->clip->SetBottomRightRadiusX(r);
    c->clip->SetBottomRightRadiusY(r);
    c->visual->SetClip(c->clip);
    c->device->Commit();
}
