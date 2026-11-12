//go:build cgo && windows && !onix_metal && !onix_vulkan && !onix_opengl && !onix_webgpu && !onix_webgl

#define COBJMACROS
#include <d3d11.h>
#include <d3dcompiler.h>
#include <dxgi1_2.h>
#include <windows.h>
HRESULT onix_dx_create_composition(IDXGIDevice *, HWND, IUnknown *, void **);
void onix_dx_destroy_composition(void *);
void onix_dx_clip_composition(void *, int, int, int);
#include "native_gpu.h"
#include "native_slots.h"
#include "native_buffer.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define RELEASE(p)                                                                                                     \
    do {                                                                                                               \
        if (p) {                                                                                                       \
            IUnknown_Release((IUnknown *)(p));                                                                         \
            (p) = NULL;                                                                                                \
        }                                                                                                              \
    } while (0)
#define LIMIT 65536
static _Thread_local char dx_error[8192];
static int dx_fail(const char *message, HRESULT result) {
    snprintf(dx_error, sizeof(dx_error), "%s (0x%08lx)", message, (unsigned long)result);
    return 0;
}
static int dx_ok(HRESULT result, const char *message) { return SUCCEEDED(result) ? 1 : dx_fail(message, result); }
const char *onix_gpu_last_error(void) { return dx_error; }
typedef struct {
    ID3D11Buffer *value;
    UINT capacity, used, offset;
    uint64_t demand;
    OnixBufferRetention retention;
} DxBuffer;
typedef struct {
    ID3D11Texture2D *value;
    ID3D11ShaderResourceView *view;
    ID3D11SamplerState *sampler;
    IDXGIKeyedMutex *mutex;
    UINT width, height, format;
    uint32_t free_next, mip_next;
    int mipmap, mips_dirty;
} DxTexture;
typedef struct {
    ID3D11RenderTargetView *color;
    ID3D11Texture2D *depth;
    ID3D11DepthStencilView *depth_view;
    uint32_t texture, width, height, format, free_next;
} DxTarget;
typedef struct {
    ID3D11Buffer *vertices, *indices, *deformation;
    ID3D11ShaderResourceView *deformation_view;
    UINT vertex_count, index_count;
    uint32_t free_next;
} DxMesh;
typedef struct {
    ID3D11VertexShader *vertex;
    ID3D11PixelShader *fragment;
    ID3D11InputLayout *layout;
    uint32_t free_next;
} DxProgram;
ONIX_NATIVE_SLOTS(dx_texture, DxTexture, 32, LIMIT)
ONIX_NATIVE_SLOTS(dx_target, DxTarget, 8, LIMIT)
ONIX_NATIVE_SLOTS(dx_mesh, DxMesh, 16, LIMIT)
ONIX_NATIVE_SLOTS(dx_program, DxProgram, 8, LIMIT)
struct OnixGPUDriver {
    ID3D11Device *device;
    ID3D11DeviceContext *context;
    IDXGISwapChain1 *swapchain;
    void *composition;
    int corner_radius;
    ID3D11BlendState *blends[5];
    ID3D11DepthStencilState *depth_states[3];
    ID3D11RasterizerState *rasterizers[2];
    ID3D11Buffer *uniforms;
    DxTarget surface;
    uint32_t active_target;
    DxTexture *textures;
    OnixNativeSlots texture_slots;
    DxTarget *targets;
    OnixNativeSlots target_slots;
    DxMesh *meshes;
    OnixNativeSlots mesh_slots;
    DxProgram *programs;
    OnixNativeSlots program_slots;
    DxProgram sprite, shape, mesh[4], picking;
    DxBuffer vertices, indices, instances, deformation;
    ID3D11ShaderResourceView *deformation_view;
    ID3D11Buffer *quad_vertices, *quad_indices;
    uint8_t *update_data;
    size_t update_capacity, update_size;
    OnixGPUTextureUpdate *updates;
    uint32_t update_count, update_slots;
    ID3D11Texture2D *readback;
    UINT readback_width, readback_height;
    DXGI_FORMAT readback_format;
    UINT width, height;
    int vsync;
    D3D11_VIEWPORT viewport;
    D3D11_RECT scissor;
    int scissor_enabled, blend_mode;
    float blend_color[4];
    char device_name[128];
};
static DxTexture *dx_get_texture(OnixGPUDriver *d, uint32_t id) {
    return id && id <= d->texture_slots.count && d->textures[id - 1].value ? &d->textures[id - 1] : NULL;
}
static DxTarget *dx_get_target(OnixGPUDriver *d, uint32_t id) {
    return id && id <= d->target_slots.count && d->targets[id - 1].color ? &d->targets[id - 1] : NULL;
}
static DxMesh *dx_get_mesh(OnixGPUDriver *d, uint32_t id) {
    return id && id <= d->mesh_slots.count && d->meshes[id - 1].vertices ? &d->meshes[id - 1] : NULL;
}
static DxProgram *dx_get_program(OnixGPUDriver *d, uint32_t id) {
    return id && id <= d->program_slots.count && d->programs[id - 1].vertex ? &d->programs[id - 1] : NULL;
}
static void dx_release_program(DxProgram *p) {
    RELEASE(p->vertex);
    RELEASE(p->fragment);
    RELEASE(p->layout);
}
static void dx_release_texture(DxTexture *p) {
    RELEASE(p->view);
    RELEASE(p->sampler);
    RELEASE(p->mutex);
    RELEASE(p->value);
}
static void dx_release_target(DxTarget *p) {
    RELEASE(p->color);
    RELEASE(p->depth_view);
    RELEASE(p->depth);
}
static void dx_release_mesh(DxMesh *p) {
    RELEASE(p->vertices);
    RELEASE(p->indices);
    RELEASE(p->deformation_view);
    RELEASE(p->deformation);
}
static ID3DBlob *dx_compile(OnixGPUShader shader, const char *entry, const char *profile) {
    ID3DBlob *code = NULL, *messages = NULL;
    if (shader.size >= 4 && memcmp(shader.code, "DXBC", 4) == 0) {
        if (!dx_ok(D3DCreateBlob(shader.size, &code), "load shader bytecode"))
            return NULL;
        memcpy(ID3D10Blob_GetBufferPointer(code), shader.code, shader.size);
        return code;
    }
    HRESULT result = D3DCompile(shader.code, shader.size, NULL, NULL, NULL, entry, profile,
                                D3DCOMPILE_ENABLE_STRICTNESS | D3DCOMPILE_OPTIMIZATION_LEVEL3, 0, &code, &messages);
    if (FAILED(result)) {
        if (messages) {
            const char *text = (char *)ID3D10Blob_GetBufferPointer(messages);
            const char *error = strstr(text, "error X");
            snprintf(dx_error, sizeof(dx_error), "%s: %s", entry, error ? error : text);
        } else
            dx_fail("D3DCompile", result);
        RELEASE(code);
    }
    RELEASE(messages);
    return code;
}
static int dx_create_program(OnixGPUDriver *d, DxProgram *p, OnixGPUShader vertex, OnixGPUShader fragment, int kind) {
    ID3DBlob *vs = dx_compile(vertex,
                              kind == 2   ? "mesh_vertex"
                              : kind == 1 ? "shape_vertex"
                                          : "sprite_vertex",
                              "vs_5_0");
    if (!vs)
        return 0;
    ID3DBlob *ps = dx_compile(fragment,
                              kind == 2   ? "mesh_fragment"
                              : kind == 1 ? "shape_fragment"
                                          : "sprite_fragment",
                              "ps_5_0");
    if (!ps) {
        RELEASE(vs);
        return 0;
    }
    HRESULT result = ID3D11Device_CreateVertexShader(d->device, ID3D10Blob_GetBufferPointer(vs),
                                                     ID3D10Blob_GetBufferSize(vs), NULL, &p->vertex);
    if (SUCCEEDED(result))
        result = ID3D11Device_CreatePixelShader(d->device, ID3D10Blob_GetBufferPointer(ps),
                                                ID3D10Blob_GetBufferSize(ps), NULL, &p->fragment);
    D3D11_INPUT_ELEMENT_DESC layout[16];
    UINT count = 0;
#define ATTR(loc, slot, fmt, offset)                                                                                   \
    layout[count++] =                                                                                                  \
        (D3D11_INPUT_ELEMENT_DESC){                                                                                    \
            "LOC",       loc, fmt, slot, offset, slot ? D3D11_INPUT_PER_INSTANCE_DATA : D3D11_INPUT_PER_VERTEX_DATA,   \
            slot ? 1 : 0}
    if (kind == 2) {
        ATTR(0, 0, DXGI_FORMAT_R32G32B32_FLOAT, 0);
        ATTR(1, 0, DXGI_FORMAT_R32G32B32_FLOAT, 12);
        ATTR(2, 0, DXGI_FORMAT_R32G32_FLOAT, 24);
        ATTR(15, 0, DXGI_FORMAT_R32G32B32A32_FLOAT, 32);
        for (UINT i = 3; i < 15; i++) {
            ATTR(i, 1, DXGI_FORMAT_R32G32B32A32_FLOAT, (i - 3) * 16);
        }
    } else {
        ATTR(0, 0, DXGI_FORMAT_R32G32B32_FLOAT, 0);
        ATTR(1, 0, DXGI_FORMAT_R32G32_FLOAT, 12);
        for (UINT i = 2; i < 6; i++) {
            ATTR(i, 1, DXGI_FORMAT_R32G32B32_FLOAT, (i - 2) * 12);
        }
        ATTR(6, 1, DXGI_FORMAT_R32G32B32A32_FLOAT, 48);
        if (kind == 1) {
            ATTR(7, 1, DXGI_FORMAT_R32G32B32A32_FLOAT, 64);
            ATTR(8, 1, DXGI_FORMAT_R32G32B32A32_FLOAT, 80);
            ATTR(9, 1, DXGI_FORMAT_R32_UINT, 96);
        } else {
            for (UINT i = 7; i < 11; i++) {
                ATTR(i, 1, DXGI_FORMAT_R32G32B32A32_FLOAT, 64 + (i - 7) * 16);
            }
            ATTR(11, 1, DXGI_FORMAT_R32_FLOAT, 128);
        }
    }
#undef ATTR
    if (SUCCEEDED(result))
        result = ID3D11Device_CreateInputLayout(d->device, layout, count, ID3D10Blob_GetBufferPointer(vs),
                                                ID3D10Blob_GetBufferSize(vs), &p->layout);
    RELEASE(vs);
    RELEASE(ps);
    if (!dx_ok(result, "create shader pipeline")) {
        dx_release_program(p);
        return 0;
    }
    return 1;
}
static int dx_upload(OnixGPUDriver *d, DxBuffer *buffer, const void *data, size_t bytes, UINT bind) {
    if (!bytes || bytes > UINT_MAX)
        return dx_fail("invalid upload size", E_INVALIDARG);
    uint64_t offset = ((uint64_t)buffer->used + 15) & ~UINT64_C(15);
    if (offset + bytes > buffer->capacity) {
        uint64_t capacity = onix_buffer_block_capacity(bytes, 64u << 10, buffer->capacity, 1u << 20);
        if (!capacity || capacity > UINT_MAX)
            return dx_fail("upload too large", E_INVALIDARG);
        if (capacity != buffer->capacity) {
            D3D11_BUFFER_DESC desc = {.ByteWidth = (UINT)capacity,
                                      .Usage = D3D11_USAGE_DYNAMIC,
                                      .BindFlags = bind,
                                      .CPUAccessFlags = D3D11_CPU_ACCESS_WRITE};
            ID3D11Buffer *value = NULL;
            if (!dx_ok(ID3D11Device_CreateBuffer(d->device, &desc, NULL, &value), "create upload buffer"))
                return 0;
            RELEASE(buffer->value);
            buffer->value = value;
            buffer->capacity = (UINT)capacity;
        }
        offset = 0;
    }
    D3D11_MAPPED_SUBRESOURCE mapped;
    // Only vertex/index buffers use this path. Append disjoint ranges until
    // full; DISCARD then lets D3D preserve storage still consumed by the GPU.
    if (!dx_ok(ID3D11DeviceContext_Map(d->context, (ID3D11Resource *)buffer->value, 0,
                                       offset ? D3D11_MAP_WRITE_NO_OVERWRITE : D3D11_MAP_WRITE_DISCARD, 0,
                                       &mapped),
               "map upload"))
        return 0;
    memcpy((uint8_t *)mapped.pData + offset, data, bytes);
    ID3D11DeviceContext_Unmap(d->context, (ID3D11Resource *)buffer->value, 0);
    buffer->offset = (UINT)offset;
    buffer->used = (UINT)(offset + bytes);
    buffer->demand += (bytes + 15u) & ~(size_t)15u;
    return 1;
}
static void dx_begin_uploads(OnixGPUDriver *d, DxBuffer *buffer) {
    if (onix_buffer_trim_capacity(&buffer->retention, buffer->capacity, buffer->demand, 64u << 10)) {
        // The immediate context retains its last vertex/index bindings.
        // Detach them so an idle view can release the historical peak too.
        ID3D11Buffer *empty[2] = {0};
        UINT zero[2] = {0};
        ID3D11DeviceContext_IASetVertexBuffers(d->context, 0, 2, empty, zero, zero);
        ID3D11DeviceContext_IASetIndexBuffer(d->context, NULL, DXGI_FORMAT_R32_UINT, 0);
        RELEASE(buffer->value);
        buffer->capacity = 0;
    }
    buffer->used = buffer->offset = 0;
    buffer->demand = 0;
}
static ID3D11Buffer *dx_static_buffer(OnixGPUDriver *d, const void *data, UINT bytes, UINT bind) {
    ID3D11Buffer *value = NULL;
    D3D11_BUFFER_DESC desc = {.ByteWidth = bytes, .Usage = D3D11_USAGE_IMMUTABLE, .BindFlags = bind};
    D3D11_SUBRESOURCE_DATA initial = {.pSysMem = data};
    if (!dx_ok(ID3D11Device_CreateBuffer(d->device, &desc, &initial, &value), "create static buffer"))
        return NULL;
    return value;
}
static int dx_uniforms(OnixGPUDriver *d, const void *data, size_t size) {
    D3D11_MAPPED_SUBRESOURCE mapped;
    if (!dx_ok(
            ID3D11DeviceContext_Map(d->context, (ID3D11Resource *)d->uniforms, 0, D3D11_MAP_WRITE_DISCARD, 0, &mapped),
            "map uniforms"))
        return 0;
    memset(mapped.pData, 0, ONIX_GPU_MESH_LIGHTING_BYTES);
    memcpy(mapped.pData, data, size);
    ID3D11DeviceContext_Unmap(d->context, (ID3D11Resource *)d->uniforms, 0);
    ID3D11DeviceContext_VSSetConstantBuffers(d->context, 0, 1, &d->uniforms);
    ID3D11DeviceContext_PSSetConstantBuffers(d->context, 0, 1, &d->uniforms);
    return 1;
}
static void dx_bind_program(OnixGPUDriver *d, DxProgram *p) {
    ID3D11DeviceContext_IASetInputLayout(d->context, p->layout);
    ID3D11DeviceContext_VSSetShader(d->context, p->vertex, NULL, 0);
    ID3D11DeviceContext_PSSetShader(d->context, p->fragment, NULL, 0);
}
static void dx_bind_geometry(OnixGPUDriver *d, ID3D11Buffer *vertices, ID3D11Buffer *indices, UINT vertex_stride,
                             UINT instance_stride, UINT instance_offset) {
    ID3D11Buffer *buffers[] = {vertices, d->instances.value};
    UINT vertex_offset = vertices == d->vertices.value ? d->vertices.offset : 0;
    UINT index_offset = indices == d->indices.value ? d->indices.offset : 0;
    UINT strides[] = {vertex_stride, instance_stride}, offsets[] = {vertex_offset, d->instances.offset + instance_offset};
    ID3D11DeviceContext_IASetVertexBuffers(d->context, 0, 2, buffers, strides, offsets);
    ID3D11DeviceContext_IASetIndexBuffer(d->context, indices, DXGI_FORMAT_R32_UINT, index_offset);
    ID3D11DeviceContext_IASetPrimitiveTopology(d->context, D3D11_PRIMITIVE_TOPOLOGY_TRIANGLELIST);
}
static void dx_bind_textures(OnixGPUDriver *d, const uint32_t *textures, UINT count) {
    ID3D11ShaderResourceView *views[16] = {0};
    ID3D11SamplerState *samplers[16] = {0};
    for (UINT i = 0; i < count; i++) {
        DxTexture *t = dx_get_texture(d, textures[i]);
        if (t) {
            views[i] = t->view;
            samplers[i] = t->sampler;
        }
    }
    ID3D11DeviceContext_PSSetShaderResources(d->context, 0, count, views);
    ID3D11DeviceContext_VSSetShaderResources(d->context, 0, count, views);
    ID3D11DeviceContext_PSSetSamplers(d->context, 0, count, samplers);
    ID3D11DeviceContext_VSSetSamplers(d->context, 0, count, samplers);
}
static void dx_blend(OnixGPUDriver *d, int mode) {
    ID3D11DeviceContext_OMSetBlendState(d->context, d->blends[mode], d->blend_color, UINT_MAX);
}
static int dx_depth(OnixGPUDriver *d, DxTarget *target) {
    if (!target->depth) {
        D3D11_TEXTURE2D_DESC desc = {.Width = target->width,
                                     .Height = target->height,
                                     .MipLevels = 1,
                                     .ArraySize = 1,
                                     .Format = DXGI_FORMAT_D32_FLOAT,
                                     .SampleDesc = {1, 0},
                                     .BindFlags = D3D11_BIND_DEPTH_STENCIL};
        if (!dx_ok(ID3D11Device_CreateTexture2D(d->device, &desc, NULL, &target->depth), "create depth"))
            return 0;
        if (!dx_ok(ID3D11Device_CreateDepthStencilView(d->device, (ID3D11Resource *)target->depth, NULL,
                                                       &target->depth_view),
                   "create depth view"))
            return 0;
        ID3D11DeviceContext_ClearDepthStencilView(d->context, target->depth_view, D3D11_CLEAR_DEPTH, 0, 0);
    }
    return 1;
}
static void dx_bind_target(OnixGPUDriver *d, DxTarget *target) {
    ID3D11ShaderResourceView *empty[18] = {0};
    ID3D11DeviceContext_PSSetShaderResources(d->context, 0, 18, empty);
    ID3D11DeviceContext_VSSetShaderResources(d->context, 0, 18, empty);
    ID3D11DeviceContext_OMSetRenderTargets(d->context, 1, &target->color, target->depth_view);
}
static int dx_surface(OnixGPUDriver *d) {
    ID3D11Texture2D *texture = NULL;
    HRESULT result = IDXGISwapChain1_GetBuffer(d->swapchain, 0, &IID_ID3D11Texture2D, (void **)&texture);
    if (SUCCEEDED(result))
        result = ID3D11Device_CreateRenderTargetView(d->device, (ID3D11Resource *)texture, NULL, &d->surface.color);
    RELEASE(texture);
    d->surface.width = d->width;
    d->surface.height = d->height;
    return dx_ok(result, "create swapchain view");
}
const char *onix_gpu_device_name(OnixGPUDriver *d) { return d->device_name; }
OnixGPUDriver *onix_gpu_create(OnixGPUSurface surface, OnixGPUShader sprite_v, OnixGPUShader sprite_f,
                               OnixGPUShader shape_v, OnixGPUShader shape_f, OnixGPUShader mesh_v, OnixGPUShader mesh_f,
                               OnixGPUShader unlit_f, OnixGPUShader depth_f, OnixGPUShader basic_f,
                               OnixGPUShader picking_v, OnixGPUShader picking_f) {
    OnixGPUDriver *d = calloc(1, sizeof(*d));
    if (!d)
        return NULL;
    d->width = surface.width;
    d->height = surface.height;
    d->vsync = 1;
    D3D_FEATURE_LEVEL level = D3D_FEATURE_LEVEL_11_0;
    HRESULT result = D3D11CreateDevice(NULL, D3D_DRIVER_TYPE_HARDWARE, NULL, D3D11_CREATE_DEVICE_BGRA_SUPPORT, &level,
                                       1, D3D11_SDK_VERSION, &d->device, NULL, &d->context);
    if (!dx_ok(result, "D3D11CreateDevice"))
        goto fail;
    IDXGIDevice *device = NULL;
    IDXGIAdapter *adapter = NULL;
    IDXGIFactory2 *factory = NULL;
    result = ID3D11Device_QueryInterface(d->device, &IID_IDXGIDevice, (void **)&device);
    if (SUCCEEDED(result))
        result = IDXGIDevice_GetAdapter(device, &adapter);
    if (SUCCEEDED(result))
        result = IDXGIAdapter_GetParent(adapter, &IID_IDXGIFactory2, (void **)&factory);
    if (adapter) {
        DXGI_ADAPTER_DESC desc;
        if (SUCCEEDED(IDXGIAdapter_GetDesc(adapter, &desc)))
            WideCharToMultiByte(CP_UTF8, 0, desc.Description, -1, d->device_name, sizeof(d->device_name), NULL, NULL);
    }
    DXGI_SWAP_CHAIN_DESC1 desc = {.Width = d->width,
                                  .Height = d->height,
                                  .Format = DXGI_FORMAT_B8G8R8A8_UNORM,
                                  .SampleDesc = {1, 0},
                                  .BufferUsage = DXGI_USAGE_RENDER_TARGET_OUTPUT,
                                  .BufferCount = 2,
                                  .Scaling = DXGI_SCALING_STRETCH,
                                  .SwapEffect = DXGI_SWAP_EFFECT_FLIP_SEQUENTIAL,
                                  .AlphaMode =
                                      surface.transparent ? DXGI_ALPHA_MODE_PREMULTIPLIED : DXGI_ALPHA_MODE_IGNORE};
    if (SUCCEEDED(result) && surface.transparent) {
        result =
            IDXGIFactory2_CreateSwapChainForComposition(factory, (IUnknown *)d->device, &desc, NULL, &d->swapchain);
        if (SUCCEEDED(result))
            result =
                onix_dx_create_composition(device, (HWND)surface.window, (IUnknown *)d->swapchain, &d->composition);
    } else if (SUCCEEDED(result))
        result = IDXGIFactory2_CreateSwapChainForHwnd(factory, (IUnknown *)d->device, (HWND)surface.window, &desc, NULL,
                                                      NULL, &d->swapchain);
    RELEASE(factory);
    RELEASE(adapter);
    RELEASE(device);
    if (!dx_ok(result, "create DirectX swapchain") || !dx_surface(d))
        goto fail;
    D3D11_BUFFER_DESC uniforms = {.ByteWidth = ONIX_GPU_MESH_LIGHTING_BYTES,
                                  .Usage = D3D11_USAGE_DYNAMIC,
                                  .BindFlags = D3D11_BIND_CONSTANT_BUFFER,
                                  .CPUAccessFlags = D3D11_CPU_ACCESS_WRITE};
    if (!dx_ok(ID3D11Device_CreateBuffer(d->device, &uniforms, NULL, &d->uniforms), "create uniforms"))
        goto fail;
    for (int i = 0; i < 5; i++) {
        D3D11_BLEND_DESC blend = {0};
        D3D11_RENDER_TARGET_BLEND_DESC *t = &blend.RenderTarget[0];
        t->BlendEnable = TRUE;
        t->RenderTargetWriteMask = D3D11_COLOR_WRITE_ENABLE_ALL;
        t->BlendOp = t->BlendOpAlpha = D3D11_BLEND_OP_ADD;
        t->SrcBlend = t->SrcBlendAlpha = D3D11_BLEND_ONE;
        t->DestBlend = t->DestBlendAlpha = D3D11_BLEND_INV_SRC_ALPHA;
        if (i == 1) {
            t->SrcBlend = D3D11_BLEND_BLEND_FACTOR;
            t->DestBlend = D3D11_BLEND_INV_SRC_COLOR;
            t->SrcBlendAlpha = D3D11_BLEND_ZERO;
            t->DestBlendAlpha = D3D11_BLEND_ONE;
        }
        if (i == 2)
            t->DestBlend = t->DestBlendAlpha = D3D11_BLEND_ONE;
        if (i == 3) {
            t->SrcBlend = D3D11_BLEND_DEST_COLOR;
            t->DestBlend = D3D11_BLEND_INV_SRC_ALPHA;
        }
        if (i == 4)
            t->DestBlend = D3D11_BLEND_INV_SRC_COLOR;
        if (!dx_ok(ID3D11Device_CreateBlendState(d->device, &blend, &d->blends[i]), "create blend state"))
            goto fail;
    }
    for (int i = 0; i < 3; i++) {
        D3D11_DEPTH_STENCIL_DESC depth = {.DepthEnable = i != 0,
                                          .DepthWriteMask =
                                              i == 2 ? D3D11_DEPTH_WRITE_MASK_ZERO : D3D11_DEPTH_WRITE_MASK_ALL,
                                          .DepthFunc = D3D11_COMPARISON_GREATER};
        if (!dx_ok(ID3D11Device_CreateDepthStencilState(d->device, &depth, &d->depth_states[i]), "create depth state"))
            goto fail;
    }
    for (int i = 0; i < 2; i++) {
        D3D11_RASTERIZER_DESC raster = {
            .FillMode = D3D11_FILL_SOLID, .CullMode = D3D11_CULL_NONE, .DepthClipEnable = TRUE, .ScissorEnable = i};
        if (!dx_ok(ID3D11Device_CreateRasterizerState(d->device, &raster, &d->rasterizers[i]), "create rasterizer"))
            goto fail;
    }
    float quad[] = {0, 0, 0, 0, 0, 1, 0, 0, 1, 0, 1, 1, 0, 1, 1, 0, 1, 0, 0, 1};
    uint32_t indices[] = {0, 1, 2, 0, 2, 3};
    d->quad_vertices = dx_static_buffer(d, quad, sizeof(quad), D3D11_BIND_VERTEX_BUFFER);
    d->quad_indices = dx_static_buffer(d, indices, sizeof(indices), D3D11_BIND_INDEX_BUFFER);
    if (!d->quad_vertices || !d->quad_indices)
        goto fail;
    if (!dx_create_program(d, &d->sprite, sprite_v, sprite_f, 0) ||
        !dx_create_program(d, &d->shape, shape_v, shape_f, 1) ||
        !dx_create_program(d, &d->mesh[0], mesh_v, mesh_f, 2) ||
        !dx_create_program(d, &d->mesh[1], mesh_v, unlit_f, 2) ||
        !dx_create_program(d, &d->mesh[2], mesh_v, depth_f, 2) ||
        !dx_create_program(d, &d->mesh[3], mesh_v, basic_f, 2) ||
        !dx_create_program(d, &d->picking, picking_v, picking_f, 2))
        goto fail;
    d->viewport = (D3D11_VIEWPORT){0, 0, d->width, d->height, 0, 1};
    return d;
fail:
    onix_gpu_destroy(d);
    return NULL;
}
void onix_gpu_buffer_memory(OnixGPUDriver *d, OnixBufferMemory *memory) {
    memset(memory, 0, sizeof(*memory));
    memory->instances = d->vertices.capacity + d->indices.capacity + d->instances.capacity + d->deformation.capacity;
    memory->staging = d->update_capacity;
    memory->scratch = (uint64_t)d->update_slots * sizeof(*d->updates);
    memory->readback = (uint64_t)d->readback_width * d->readback_height * 4;
}
void onix_gpu_destroy(OnixGPUDriver *d) {
    if (!d)
        return;
    if (d->context) {
        ID3D11DeviceContext_ClearState(d->context);
        ID3D11DeviceContext_Flush(d->context);
    }
    for (UINT i = 0; i < d->texture_slots.count; i++)
        dx_release_texture(&d->textures[i]);
    for (UINT i = 0; i < d->target_slots.count; i++)
        dx_release_target(&d->targets[i]);
    for (UINT i = 0; i < d->mesh_slots.count; i++)
        dx_release_mesh(&d->meshes[i]);
    for (UINT i = 0; i < d->program_slots.count; i++)
        dx_release_program(&d->programs[i]);
    free(d->textures);
    free(d->targets);
    free(d->meshes);
    free(d->programs);
    free(d->update_data);
    free(d->updates);
    dx_release_program(&d->sprite);
    dx_release_program(&d->shape);
    dx_release_program(&d->picking);
    for (int i = 0; i < 4; i++)
        dx_release_program(&d->mesh[i]);
    for (int i = 0; i < 5; i++)
        RELEASE(d->blends[i]);
    for (int i = 0; i < 3; i++)
        RELEASE(d->depth_states[i]);
    for (int i = 0; i < 2; i++)
        RELEASE(d->rasterizers[i]);
    RELEASE(d->uniforms);
    RELEASE(d->vertices.value);
    RELEASE(d->indices.value);
    RELEASE(d->instances.value);
    RELEASE(d->deformation_view);
    RELEASE(d->deformation.value);
    RELEASE(d->readback);
    RELEASE(d->quad_vertices);
    RELEASE(d->quad_indices);
    dx_release_target(&d->surface);
    onix_dx_destroy_composition(d->composition);
    RELEASE(d->swapchain);
    RELEASE(d->context);
    RELEASE(d->device);
    free(d);
}
int onix_gpu_resize(OnixGPUDriver *d, uint32_t width, uint32_t height) {
    if (d->active_target)
        return dx_fail("cannot resize during a target pass", E_INVALIDARG);
    ID3D11DeviceContext_OMSetRenderTargets(d->context, 0, NULL, NULL);
    dx_release_target(&d->surface);
    d->width = width;
    d->height = height;
    if (!dx_ok(IDXGISwapChain1_ResizeBuffers(d->swapchain, 0, width, height, DXGI_FORMAT_UNKNOWN, 0),
               "resize swapchain"))
        return 0;
    onix_dx_clip_composition(d->composition, width, height, d->corner_radius);
    return dx_surface(d);
}
int onix_gpu_begin_frame(OnixGPUDriver *d, const float clear[4]) {
    dx_begin_uploads(d, &d->vertices);
    dx_begin_uploads(d, &d->indices);
    dx_begin_uploads(d, &d->instances);
    d->active_target = 0;
    dx_bind_target(d, &d->surface);
    ID3D11DeviceContext_ClearRenderTargetView(d->context, d->surface.color, clear);
    if (d->surface.depth_view)
        ID3D11DeviceContext_ClearDepthStencilView(d->context, d->surface.depth_view, D3D11_CLEAR_DEPTH, 0, 0);
    ID3D11DeviceContext_RSSetViewports(d->context, 1, &d->viewport);
    ID3D11DeviceContext_RSSetState(d->context, d->rasterizers[d->scissor_enabled]);
    if (d->scissor_enabled)
        ID3D11DeviceContext_RSSetScissorRects(d->context, 1, &d->scissor);
    return 1;
}
void onix_gpu_set_sprite_blend(OnixGPUDriver *d, int mode, float r, float g, float b, float a) {
    d->blend_mode = mode;
    d->blend_color[0] = r;
    d->blend_color[1] = g;
    d->blend_color[2] = b;
    d->blend_color[3] = a;
    dx_blend(d, mode);
}
void onix_gpu_set_viewport(OnixGPUDriver *d, int32_t x, int32_t y, int32_t width, int32_t height) {
    d->viewport = (D3D11_VIEWPORT){x, y, width, height, 0, 1};
    ID3D11DeviceContext_RSSetViewports(d->context, 1, &d->viewport);
}
void onix_gpu_set_scissor(OnixGPUDriver *d, int enabled, int32_t x, int32_t y, int32_t width, int32_t height) {
    d->scissor_enabled = enabled;
    d->scissor = (D3D11_RECT){x, y, x + width, y + height};
    ID3D11DeviceContext_RSSetState(d->context, d->rasterizers[enabled != 0]);
    if (enabled)
        ID3D11DeviceContext_RSSetScissorRects(d->context, 1, &d->scissor);
}
void onix_gpu_set_vsync(OnixGPUDriver *d, int enabled) { d->vsync = enabled; }
void onix_gpu_set_corner_radius(OnixGPUDriver *d, int radius) {
    d->corner_radius = radius;
    onix_dx_clip_composition(d->composition, d->width, d->height, radius);
}
int onix_gpu_present(OnixGPUDriver *d) {
    if (d->active_target)
        return dx_fail("cannot present during a target pass", E_INVALIDARG);
    return dx_ok(IDXGISwapChain1_Present(d->swapchain, d->vsync ? 1 : 0, 0), "present");
}

static DXGI_FORMAT dx_texture_format(uint32_t format) {
    static const DXGI_FORMAT formats[] = {
        DXGI_FORMAT_R8G8B8A8_UNORM, DXGI_FORMAT_R8_UNORM,  DXGI_FORMAT_R8G8B8A8_UNORM_SRGB, DXGI_FORMAT_BC1_UNORM,
        DXGI_FORMAT_BC1_UNORM_SRGB, DXGI_FORMAT_BC3_UNORM, DXGI_FORMAT_BC3_UNORM_SRGB,      DXGI_FORMAT_BC4_UNORM,
        DXGI_FORMAT_BC5_UNORM,      DXGI_FORMAT_BC7_UNORM, DXGI_FORMAT_BC7_UNORM_SRGB};
    return format < 11 ? formats[format] : DXGI_FORMAT_UNKNOWN;
}
static UINT dx_texel_bytes(uint32_t format) { return format == 1 ? 1 : 4; }
static UINT dx_block_bytes(uint32_t format) { return format == 3 || format == 4 || format == 7 ? 8 : 16; }
uint32_t onix_gpu_texture_mip_level_count(uint32_t width, uint32_t height) {
    UINT count = 1;
    while (width > 1 || height > 1) {
        width = width > 1 ? width / 2 : 1;
        height = height > 1 ? height / 2 : 1;
        count++;
    }
    return count;
}
uint64_t onix_gpu_texture_chain_size(uint32_t format, uint32_t width, uint32_t height, uint32_t levels) {
    uint64_t bytes = 0;
    for (UINT i = 0; i < levels; i++) {
        bytes += format >= 3 ? (uint64_t)((width + 3) / 4) * ((height + 3) / 4) * dx_block_bytes(format)
                             : (uint64_t)width * height * dx_texel_bytes(format);
        width = width > 1 ? width / 2 : 1;
        height = height > 1 ? height / 2 : 1;
    }
    return bytes;
}
static int dx_texture_view(OnixGPUDriver *d, DxTexture *texture, int mipmap, uint32_t wrap_u, uint32_t wrap_v) {
    if (!dx_ok(ID3D11Device_CreateShaderResourceView(d->device, (ID3D11Resource *)texture->value, NULL, &texture->view),
               "create texture view"))
        return 0;
    static const D3D11_TEXTURE_ADDRESS_MODE modes[] = {D3D11_TEXTURE_ADDRESS_CLAMP, D3D11_TEXTURE_ADDRESS_WRAP,
                                                       D3D11_TEXTURE_ADDRESS_MIRROR};
    D3D11_SAMPLER_DESC sampler = {.Filter =
                                      mipmap ? D3D11_FILTER_MIN_MAG_MIP_LINEAR : D3D11_FILTER_MIN_MAG_LINEAR_MIP_POINT,
                                  .AddressU = modes[wrap_u],
                                  .AddressV = modes[wrap_v],
                                  .AddressW = D3D11_TEXTURE_ADDRESS_CLAMP,
                                  .MaxLOD = mipmap ? D3D11_FLOAT32_MAX : 0};
    return dx_ok(ID3D11Device_CreateSamplerState(d->device, &sampler, &texture->sampler), "create sampler");
}
uint32_t onix_gpu_create_texture_with_format(OnixGPUDriver *d, uint32_t width, uint32_t height, uint32_t levels,
                                             uint32_t format, int mipmap, const uint8_t *pixels, size_t length,
                                             uint32_t wrap_u, uint32_t wrap_v) {
    uint32_t slot;
    if (!dx_texture_acquire(&d->textures, &d->texture_slots, &slot))
        return 0;
    DXGI_FORMAT native_format = dx_texture_format(format);
    if (native_format == DXGI_FORMAT_UNKNOWN || !width || !height || !levels || levels > 32 || wrap_u > 2 ||
        wrap_v > 2) {
        dx_fail("unsupported texture", E_INVALIDARG);
        return 0;
    }
    int generate = mipmap && format < 3 && levels > 1;
    DxTexture texture = {.width = width, .height = height, .format = format, .mipmap = generate};
    D3D11_TEXTURE2D_DESC desc = {.Width = width,
                                 .Height = height,
                                 .MipLevels = levels,
                                 .ArraySize = 1,
                                 .Format = native_format,
                                 .SampleDesc = {1, 0},
                                 .Usage = D3D11_USAGE_DEFAULT,
                                 .BindFlags = D3D11_BIND_SHADER_RESOURCE | (generate ? D3D11_BIND_RENDER_TARGET : 0),
                                 .MiscFlags = generate ? D3D11_RESOURCE_MISC_GENERATE_MIPS : 0};
    D3D11_SUBRESOURCE_DATA data[32] = {0};
    size_t offset = 0;
    UINT w = width, h = height;
    for (UINT i = 0; i < (generate ? 1 : levels); i++) {
        size_t bytes = onix_gpu_texture_chain_size(format, w, h, 1);
        if (pixels && (offset > length || bytes > length - offset)) {
            dx_fail("invalid texture payload", E_INVALIDARG);
            return 0;
        }
        data[i] = (D3D11_SUBRESOURCE_DATA){.pSysMem = pixels ? pixels + offset : NULL,
                                           .SysMemPitch = format >= 3 ? ((w + 3) / 4) * dx_block_bytes(format)
                                                                      : w * dx_texel_bytes(format)};
        offset += bytes;
        w = w > 1 ? w / 2 : 1;
        h = h > 1 ? h / 2 : 1;
    }
    if (!dx_ok(ID3D11Device_CreateTexture2D(d->device, &desc, pixels && !generate ? data : NULL, &texture.value),
               "create texture") ||
        !dx_texture_view(d, &texture, mipmap, wrap_u, wrap_v)) {
        dx_release_texture(&texture);
        return 0;
    }
    if (generate && pixels) {
        ID3D11DeviceContext_UpdateSubresource(d->context, (ID3D11Resource *)texture.value, 0, NULL, pixels,
                                              width * dx_texel_bytes(format), 0);
        ID3D11DeviceContext_GenerateMips(d->context, texture.view);
    }
    dx_texture_insert(d->textures, &d->texture_slots, slot, texture);
    return slot + 1;
}
int onix_gpu_destroy_texture(OnixGPUDriver *d, uint32_t id) {
    DxTexture *t = dx_get_texture(d, id);
    if (!t)
        return dx_fail("invalid texture", E_INVALIDARG);
    dx_release_texture(t);
    dx_texture_recycle(d->textures, &d->texture_slots, id - 1);
    return 1;
}
static int dx_validate_updates(OnixGPUDriver *d, const OnixGPUTextureUpdate *updates, uint32_t count, size_t length) {
    for (UINT i = 0; i < count; i++) {
        const OnixGPUTextureUpdate *u = &updates[i];
        DxTexture *t = dx_get_texture(d, u->texture);
        if (!t || t->format >= 3 || u->x < 0 || u->y < 0 || (uint64_t)u->x + u->width > t->width ||
            (uint64_t)u->y + u->height > t->height ||
            u->data_length != (uint64_t)u->width * u->height * dx_texel_bytes(t->format) || u->data_offset > length ||
            u->data_length > length - u->data_offset)
            return dx_fail("invalid texture update", E_INVALIDARG);
    }
    return 1;
}
int onix_gpu_update_textures(OnixGPUDriver *d, const OnixGPUTextureUpdate *updates, uint32_t count,
                             const uint8_t *pixels, size_t length) {
    if (!dx_validate_updates(d, updates, count, length))
        return 0;
    uint32_t mip_head = 0;
    for (UINT i = 0; i < count; i++) {
        const OnixGPUTextureUpdate *u = &updates[i];
        DxTexture *t = dx_get_texture(d, u->texture);
        D3D11_BOX box = {u->x, u->y, 0, u->x + u->width, u->y + u->height, 1};
        ID3D11DeviceContext_UpdateSubresource(d->context, (ID3D11Resource *)t->value, 0, &box, pixels + u->data_offset,
                                              u->width * dx_texel_bytes(t->format), 0);
        if (t->mipmap && !t->mips_dirty) {
            t->mips_dirty = 1;
            t->mip_next = mip_head;
            mip_head = u->texture;
        }
    }
    while (mip_head) {
        DxTexture *t = dx_get_texture(d, mip_head);
        ID3D11DeviceContext_GenerateMips(d->context, t->view);
        mip_head = t->mip_next;
        t->mips_dirty = 0;
    }
    return 1;
}
uint8_t *onix_gpu_prepare_texture_updates(OnixGPUDriver *d, const OnixGPUTextureUpdate *updates, uint32_t count,
                                          size_t length) {
    if (!dx_validate_updates(d, updates, count, length))
        return NULL;
    if (d->update_capacity < length) {
        uint8_t *data = realloc(d->update_data, length);
        if (!data) {
            dx_fail("allocate texture upload", E_OUTOFMEMORY);
            return NULL;
        }
        d->update_data = data;
        d->update_capacity = length;
    }
    if (d->update_slots < count) {
        OnixGPUTextureUpdate *data = realloc(d->updates, (size_t)count * sizeof(*updates));
        if (!data) {
            dx_fail("allocate texture updates", E_OUTOFMEMORY);
            return NULL;
        }
        d->updates = data;
        d->update_slots = count;
    }
    memcpy(d->updates, updates, (size_t)count * sizeof(*updates));
    d->update_count = count;
    d->update_size = length;
    return d->update_data;
}
int onix_gpu_commit_texture_updates(OnixGPUDriver *d) {
    return onix_gpu_update_textures(d, d->updates, d->update_count, d->update_data, d->update_size);
}
uint32_t onix_gpu_create_external_d3d11_texture(OnixGPUDriver *d, uintptr_t handle, uint32_t width, uint32_t height) {
    if (!handle)
        return 0;
    IDXGIResource *resource = NULL;
    HANDLE shared = NULL;
    HRESULT result = ID3D11Texture2D_QueryInterface((ID3D11Texture2D *)handle, &IID_IDXGIResource, (void **)&resource);
    if (SUCCEEDED(result))
        result = IDXGIResource_GetSharedHandle(resource, &shared);
    RELEASE(resource);
    DxTexture texture = {.width = width, .height = height};
    if (SUCCEEDED(result))
        result = ID3D11Device_OpenSharedResource(d->device, shared, &IID_ID3D11Texture2D, (void **)&texture.value);
    if (!dx_ok(result, "open shared D3D11 texture") || !dx_texture_view(d, &texture, 0, 0, 0)) {
        dx_release_texture(&texture);
        return 0;
    }
    ID3D11Texture2D_QueryInterface(texture.value, &IID_IDXGIKeyedMutex, (void **)&texture.mutex);
    uint32_t slot;
    if (!dx_texture_acquire(&d->textures, &d->texture_slots, &slot)) {
        dx_release_texture(&texture);
        return 0;
    }
    dx_texture_insert(d->textures, &d->texture_slots, slot, texture);
    return slot + 1;
}
uint32_t onix_gpu_create_external_dmabuf_texture(OnixGPUDriver *d, int fd, uint32_t width, uint32_t height,
                                                 uint32_t stride, uint32_t offset, uint64_t modifier) {
    dx_fail("DMA-BUF is unavailable on DirectX", E_NOTIMPL);
    return 0;
}
int onix_gpu_refresh_external_texture(OnixGPUDriver *d, uint32_t texture) { return dx_get_texture(d, texture) != NULL; }
static int dx_create_target(OnixGPUDriver *d, DxTarget *target, uint32_t width, uint32_t height, uint32_t format,
                            int depth) {
    if (!width || !height || format > 2 || (format == 2 && depth))
        return dx_fail("invalid target descriptor", E_INVALIDARG);
    DxTexture texture = {.width = width, .height = height, .format = format == 2 ? ONIX_TEXTURE_FORMAT_R8 : ONIX_TEXTURE_FORMAT_RGBA8};
    D3D11_TEXTURE2D_DESC desc = {.Width = width,
                                 .Height = height,
                                 .MipLevels = 1,
                                 .ArraySize = 1,
                                 .Format = format == 2 ? DXGI_FORMAT_R8_UNORM : format == 1 ? DXGI_FORMAT_R32_UINT : DXGI_FORMAT_R8G8B8A8_UNORM,
                                 .SampleDesc = {1, 0},
                                 .BindFlags = D3D11_BIND_SHADER_RESOURCE | D3D11_BIND_RENDER_TARGET};
    if (!dx_ok(ID3D11Device_CreateTexture2D(d->device, &desc, NULL, &texture.value), "create target texture") ||
        !dx_texture_view(d, &texture, 0, 0, 0)) {
        dx_release_texture(&texture);
        return 0;
    }
    uint32_t slot;
    if (!dx_texture_acquire(&d->textures, &d->texture_slots, &slot)) {
        dx_release_texture(&texture);
        return 0;
    }
    *target = (DxTarget){.texture = slot + 1, .width = width, .height = height, .format = format};
    if (!dx_ok(ID3D11Device_CreateRenderTargetView(d->device, (ID3D11Resource *)texture.value, NULL, &target->color),
               "create target view") ||
        (depth && !dx_depth(d, target))) {
        dx_release_target(target);
        dx_release_texture(&texture);
        return 0;
    }
    dx_texture_insert(d->textures, &d->texture_slots, slot, texture);
    return 1;
}
uint32_t onix_gpu_create_render_target(OnixGPUDriver *d, uint32_t width, uint32_t height, uint32_t format, int depth,
                                       uint32_t *color) {
    uint32_t slot;
    if (!dx_target_acquire(&d->targets, &d->target_slots, &slot))
        return 0;
    DxTarget target = {0};
    if (!dx_create_target(d, &target, width, height, format, depth))
        return 0;
    dx_target_insert(d->targets, &d->target_slots, slot, target);
    *color = target.texture;
    return slot + 1;
}
int onix_gpu_resize_render_target(OnixGPUDriver *d, uint32_t id, uint32_t width, uint32_t height, uint32_t format,
                                  int depth, uint32_t *color) {
    DxTarget *target = dx_get_target(d, id);
    if (!target || d->active_target == id)
        return dx_fail("invalid target resize", E_INVALIDARG);
    DxTarget replacement = {0};
    if (!dx_create_target(d, &replacement, width, height, format, depth))
        return 0;
    onix_gpu_destroy_texture(d, target->texture);
    dx_release_target(target);
    *target = replacement;
    *color = target->texture;
    return 1;
}
int onix_gpu_destroy_render_target(OnixGPUDriver *d, uint32_t id) {
    DxTarget *target = dx_get_target(d, id);
    if (!target || d->active_target == id)
        return dx_fail("invalid target destruction", E_INVALIDARG);
    onix_gpu_destroy_texture(d, target->texture);
    dx_release_target(target);
    dx_target_recycle(d->targets, &d->target_slots, id - 1);
    return 1;
}
int onix_gpu_begin_render_target(OnixGPUDriver *d, uint32_t id, const float clear[4], float depth, int clear_color,
                                 int clear_depth) {
    DxTarget *target = dx_get_target(d, id);
    if (!target || d->active_target)
        return dx_fail("invalid target pass", E_INVALIDARG);
    d->active_target = id;
    dx_bind_target(d, target);
    if (clear_color)
        ID3D11DeviceContext_ClearRenderTargetView(d->context, target->color, clear);
    if (clear_depth && target->depth_view)
        ID3D11DeviceContext_ClearDepthStencilView(d->context, target->depth_view, D3D11_CLEAR_DEPTH, depth, 0);
    return 1;
}
int onix_gpu_end_render_target(OnixGPUDriver *d) {
    if (!d->active_target)
        return dx_fail("no active target", E_INVALIDARG);
    d->active_target = 0;
    dx_bind_target(d, &d->surface);
    return 1;
}
int onix_gpu_read_render_target(OnixGPUDriver *d, uint32_t id, int32_t x, int32_t y, uint32_t width, uint32_t height,
                                uint8_t *destination, size_t length) {
    DxTarget *target = dx_get_target(d, id);
    if (!target || d->active_target || x < 0 || y < 0 || (uint64_t)x + width > target->width ||
        (uint64_t)y + height > target->height || length < (uint64_t)width * height * (target->format == 2 ? 1 : 4))
        return dx_fail("invalid target readback", E_INVALIDARG);
    DXGI_FORMAT format = target->format == 2 ? DXGI_FORMAT_R8_UNORM : target->format == 1 ? DXGI_FORMAT_R32_UINT : DXGI_FORMAT_R8G8B8A8_UNORM;
    size_t row_bytes = (size_t)width * (target->format == 2 ? 1 : 4);
    if (!d->readback || d->readback_width != width || d->readback_height != height || d->readback_format != format) {
        RELEASE(d->readback);
        D3D11_TEXTURE2D_DESC desc = {.Width = width,
                                     .Height = height,
                                     .MipLevels = 1,
                                     .ArraySize = 1,
                                     .Format = format,
                                     .SampleDesc = {1, 0},
                                     .Usage = D3D11_USAGE_STAGING,
                                     .CPUAccessFlags = D3D11_CPU_ACCESS_READ};
        if (!dx_ok(ID3D11Device_CreateTexture2D(d->device, &desc, NULL, &d->readback), "create readback"))
            return 0;
        d->readback_width = width;
        d->readback_height = height;
        d->readback_format = format;
    }
    DxTexture *texture = dx_get_texture(d, target->texture);
    D3D11_BOX box = {x, y, 0, x + width, y + height, 1};
    ID3D11DeviceContext_CopySubresourceRegion(d->context, (ID3D11Resource *)d->readback, 0, 0, 0, 0,
                                              (ID3D11Resource *)texture->value, 0, &box);
    D3D11_MAPPED_SUBRESOURCE mapped;
    if (!dx_ok(ID3D11DeviceContext_Map(d->context, (ID3D11Resource *)d->readback, 0, D3D11_MAP_READ, 0, &mapped),
               "map readback"))
        return 0;
    for (UINT row = 0; row < height; row++)
        memcpy(destination + (size_t)row * row_bytes, (uint8_t *)mapped.pData + (size_t)row * mapped.RowPitch,
               row_bytes);
    ID3D11DeviceContext_Unmap(d->context, (ID3D11Resource *)d->readback, 0);
    return 1;
}
uint32_t onix_gpu_create_mesh(OnixGPUDriver *d, const OnixGPUVertex3D *vertices, uint32_t vertex_count,
                              const uint32_t *indices, uint32_t index_count) {
    if (!vertices || !indices || !vertex_count || !index_count || vertex_count > UINT_MAX / sizeof(*vertices) ||
        index_count > UINT_MAX / 4)
        return 0;
    uint32_t slot;
    if (!dx_mesh_acquire(&d->meshes, &d->mesh_slots, &slot))
        return 0;
    DxMesh mesh = {.vertex_count = vertex_count, .index_count = index_count};
    mesh.vertices = dx_static_buffer(d, vertices, vertex_count * sizeof(*vertices), D3D11_BIND_VERTEX_BUFFER);
    mesh.indices = dx_static_buffer(d, indices, index_count * 4, D3D11_BIND_INDEX_BUFFER);
    if (!mesh.vertices || !mesh.indices) {
        dx_release_mesh(&mesh);
        return 0;
    }
    dx_mesh_insert(d->meshes, &d->mesh_slots, slot, mesh);
    return slot + 1;
}
static int dx_storage_buffer(OnixGPUDriver *d, ID3D11Buffer **buffer, ID3D11ShaderResourceView **view,
                             const uint32_t *words, uint32_t count) {
    if (!count || count > UINT_MAX / 4)
        return 0;
    D3D11_BUFFER_DESC desc = {.ByteWidth = count * 4,
                              .Usage = D3D11_USAGE_DEFAULT,
                              .BindFlags = D3D11_BIND_SHADER_RESOURCE,
                              .MiscFlags = D3D11_RESOURCE_MISC_BUFFER_ALLOW_RAW_VIEWS};
    D3D11_SUBRESOURCE_DATA data = {.pSysMem = words};
    if (!dx_ok(ID3D11Device_CreateBuffer(d->device, &desc, &data, buffer), "create deformation buffer"))
        return 0;
    D3D11_SHADER_RESOURCE_VIEW_DESC srv = {.Format = DXGI_FORMAT_R32_TYPELESS,
                                           .ViewDimension = D3D11_SRV_DIMENSION_BUFFEREX,
                                           .BufferEx = {.NumElements = count, .Flags = D3D11_BUFFEREX_SRV_FLAG_RAW}};
    return dx_ok(ID3D11Device_CreateShaderResourceView(d->device, (ID3D11Resource *)*buffer, &srv, view),
                 "create deformation view");
}
int onix_gpu_set_mesh_deformation(OnixGPUDriver *d, uint32_t id, const uint32_t *words, uint32_t word_count,
                                  uint32_t vertex_count) {
    DxMesh *m = dx_get_mesh(d, id);
    if (!m || m->deformation || m->vertex_count != vertex_count)
        return dx_fail("invalid deformation mesh", E_INVALIDARG);
    return dx_storage_buffer(d, &m->deformation, &m->deformation_view, words, word_count);
}
int onix_gpu_destroy_mesh(OnixGPUDriver *d, uint32_t id) {
    DxMesh *mesh = dx_get_mesh(d, id);
    if (!mesh)
        return dx_fail("invalid mesh", E_INVALIDARG);
    dx_release_mesh(mesh);
    dx_mesh_recycle(d->meshes, &d->mesh_slots, id - 1);
    return 1;
}
static uint32_t dx_custom_program(OnixGPUDriver *d, OnixGPUShader vertex, OnixGPUShader fragment, int kind) {
    uint32_t slot;
    if (!dx_program_acquire(&d->programs, &d->program_slots, &slot))
        return 0;
    DxProgram program = {0};
    if (!dx_create_program(d, &program, vertex, fragment, kind))
        return 0;
    dx_program_insert(d->programs, &d->program_slots, slot, program);
    return slot + 1;
}
uint32_t onix_gpu_create_mesh_shader(OnixGPUDriver *d, OnixGPUShader vertex, OnixGPUShader fragment) {
    return dx_custom_program(d, vertex, fragment, 2);
}
uint32_t onix_gpu_create_material2d_shader(OnixGPUDriver *d, OnixGPUShader vertex, OnixGPUShader fragment) {
    return dx_custom_program(d, vertex, fragment, 0);
}
int onix_gpu_destroy_mesh_shader(OnixGPUDriver *d, uint32_t id) {
    DxProgram *p = dx_get_program(d, id);
    if (!p)
        return dx_fail("invalid shader", E_INVALIDARG);
    dx_release_program(p);
    dx_program_recycle(d->programs, &d->program_slots, id - 1);
    return 1;
}
int onix_gpu_destroy_material2d_shader(OnixGPUDriver *d, uint32_t id) { return onix_gpu_destroy_mesh_shader(d, id); }
int onix_gpu_draw_sprites(OnixGPUDriver *d, const OnixGPUSpritePush *push, const OnixGPUInstance *instances,
                          uint32_t instance_count, const OnixNativeSpriteDraw *draws, uint32_t draw_count) {
    if (!dx_upload(d, &d->instances, instances, (size_t)instance_count * sizeof(*instances), D3D11_BIND_VERTEX_BUFFER))
        return 0;
    ID3D11DeviceContext_OMSetDepthStencilState(d->context, d->depth_states[0], 0);
    dx_blend(d, d->blend_mode);
    for (UINT i = 0; i < draw_count; i++) {
        const OnixNativeSpriteDraw *draw = &draws[i];
        if ((uint64_t)draw->first_instance + draw->instance_count > instance_count)
            return dx_fail("invalid sprite range", E_INVALIDARG);
        DxProgram *p = draw->shader ? dx_get_program(d, draw->shader) : &d->sprite;
        if (!p)
            return dx_fail("invalid sprite shader", E_INVALIDARG);
        uint32_t textures[] = {draw->texture, draw->additional_textures[0], draw->additional_textures[1],
                               draw->additional_textures[2]};
        float uniforms[32] = {0};
        memcpy(uniforms, push->projection, 64);
        for (UINT t = 0; t < 4; t++) {
            if (!textures[t])
                textures[t] = draw->texture;
            DxTexture *texture = dx_get_texture(d, textures[t]);
            if (!texture)
                return dx_fail("invalid sprite texture", E_INVALIDARG);
            uniforms[16 + t] = texture->format == 1 ? 1 : 0;
        }
        // Shared surfaces can appear in more than one texture slot; acquire once.
        UINT locked = 0;
        for (UINT t = 0; t < 4; t++) {
            DxTexture *texture = dx_get_texture(d, textures[t]);
            int duplicate = 0;
            for (UINT j = 0; j < t; j++)
                duplicate |= textures[j] == textures[t];
            if (texture->mutex && !duplicate) {
                if (IDXGIKeyedMutex_AcquireSync(texture->mutex, 0, 1000) != S_OK) {
                    for (UINT j = 0; j < t; j++)
                        if (locked & (1u << j))
                            IDXGIKeyedMutex_ReleaseSync(dx_get_texture(d, textures[j])->mutex, 0);
                    return dx_fail("lock shared texture", E_FAIL);
                }
                locked |= 1u << t;
            }
        }
        int ok = dx_uniforms(d, uniforms, sizeof(uniforms));
        if (ok) {
            dx_bind_program(d, p);
            dx_bind_textures(d, textures, 4);
            dx_bind_geometry(d, d->quad_vertices, d->quad_indices, 20, sizeof(*instances),
                             draw->first_instance * sizeof(*instances));
            ID3D11DeviceContext_DrawIndexedInstanced(d->context, 6, draw->instance_count, 0, 0, 0);
        }
        for (UINT t = 0; t < 4; t++)
            if (locked & (1u << t))
                IDXGIKeyedMutex_ReleaseSync(dx_get_texture(d, textures[t])->mutex, 0);
        if (!ok)
            return 0;
    }
    return 1;
}
int onix_gpu_draw_shapes(OnixGPUDriver *d, const OnixGPUSpritePush *push, const OnixGPUShapeInstance *instances,
                         uint32_t instance_count) {
    if (!dx_upload(d, &d->instances, instances, (size_t)instance_count * sizeof(*instances),
                   D3D11_BIND_VERTEX_BUFFER) ||
        !dx_uniforms(d, push->projection, 64))
        return 0;
    ID3D11DeviceContext_OMSetDepthStencilState(d->context, d->depth_states[0], 0);
    dx_blend(d, d->blend_mode);
    dx_bind_program(d, &d->shape);
    dx_bind_geometry(d, d->quad_vertices, d->quad_indices, 20, sizeof(*instances), 0);
    ID3D11DeviceContext_DrawIndexedInstanced(d->context, 6, instance_count, 0, 0, 0);
    return 1;
}
static int dx_draw_meshes(OnixGPUDriver *d, const OnixGPUMeshPush *push, const OnixGPUVertex3D *vertices,
                          uint32_t vertex_count, const uint32_t *indices, uint32_t index_count,
                          const OnixGPUInstance3D *instances, uint32_t instance_count, const uint32_t *words,
                          uint32_t word_count, const OnixNativeMeshDraw3D *draws, uint32_t draw_count,
                          const float *lighting, int picking) {
    if (!dx_upload(d, &d->instances, instances, (size_t)instance_count * sizeof(*instances), D3D11_BIND_VERTEX_BUFFER))
        return 0;
    if (vertex_count &&
        (!dx_upload(d, &d->vertices, vertices, (size_t)vertex_count * sizeof(*vertices), D3D11_BIND_VERTEX_BUFFER) ||
         !dx_upload(d, &d->indices, indices, (size_t)index_count * 4, D3D11_BIND_INDEX_BUFFER)))
        return 0;
    DxTarget *target = d->active_target ? dx_get_target(d, d->active_target) : &d->surface;
    if (!target || !dx_depth(d, target))
        return 0;
    ID3D11DeviceContext_OMSetRenderTargets(d->context, 1, &target->color, target->depth_view);
    if (word_count) {
        if (d->deformation.capacity < word_count * 4) {
            RELEASE(d->deformation_view);
            RELEASE(d->deformation.value);
            d->deformation.capacity = 0;
            if (!dx_storage_buffer(d, &d->deformation.value, &d->deformation_view, words, word_count))
                return 0;
            d->deformation.capacity = word_count * 4;
        } else {
            D3D11_BOX box = {0, 0, 0, word_count * 4, 1, 1};
            ID3D11DeviceContext_UpdateSubresource(d->context, (ID3D11Resource *)d->deformation.value, 0, &box, words, 0,
                                                  0);
        }
    }
    float uniforms[ONIX_GPU_MESH_LIGHTING_BYTES / 4] = {0};
    if (lighting)
        memcpy(uniforms, lighting, sizeof(uniforms));
    else
        memcpy(uniforms, push, sizeof(*push));
    for (UINT i = 0; i < draw_count; i++) {
        const OnixNativeMeshDraw3D *draw = &draws[i];
        if ((uint64_t)draw->first_instance + draw->instance_count > instance_count)
            return dx_fail("invalid mesh instances", E_INVALIDARG);
        DxMesh *mesh = draw->mesh ? dx_get_mesh(d, draw->mesh) : NULL;
        if (draw->mesh && !mesh)
            return dx_fail("invalid mesh handle", E_INVALIDARG);
        UINT available = mesh ? mesh->index_count : index_count;
        if ((uint64_t)draw->first_index + draw->index_count > available)
            return dx_fail("invalid mesh indices", E_INVALIDARG);
        DxProgram *p = picking        ? &d->picking
                       : draw->shader ? dx_get_program(d, draw->shader)
                                      : &d->mesh[onix_mesh_builtin_variant(instances, draw->first_instance,
                                                                           draw->instance_count, (int)uniforms[297])];
        if (!p)
            return dx_fail("invalid mesh shader", E_INVALIDARG);
        uniforms[31] = (float)draw->deformation_index;
        if (!dx_uniforms(d, uniforms, sizeof(uniforms)))
            return 0;
        uint32_t textures[16] = {draw->texture, draw->normal_texture, draw->metallic_roughness_texture};
        memcpy(textures + 3, draw->extra_textures, sizeof(draw->extra_textures));
        dx_bind_textures(d, textures, 16);
        ID3D11ShaderResourceView *data[] = {d->deformation_view, mesh ? mesh->deformation_view : NULL};
        ID3D11DeviceContext_VSSetShaderResources(d->context, 16, 2, data);
        ID3D11DeviceContext_OMSetDepthStencilState(d->context, d->depth_states[draw->alpha_mode == 2 ? 2 : 1], 0);
        if (picking || draw->alpha_mode != 2)
            ID3D11DeviceContext_OMSetBlendState(d->context, NULL, NULL, UINT_MAX);
        else
            dx_blend(d, draw->blend_mode ? draw->blend_mode + 1 : 0);
        dx_bind_program(d, p);
        dx_bind_geometry(d, mesh ? mesh->vertices : d->vertices.value, mesh ? mesh->indices : d->indices.value,
                         sizeof(*vertices), sizeof(*instances), draw->first_instance * sizeof(*instances));
        ID3D11DeviceContext_DrawIndexedInstanced(d->context, draw->index_count, draw->instance_count, draw->first_index,
                                                 draw->base_vertex, 0);
    }
    return 1;
}
int onix_gpu_draw_meshes(OnixGPUDriver *d, const OnixGPUMeshPush *push, const OnixGPUVertex3D *vertices,
                         uint32_t vertex_count, const uint32_t *indices, uint32_t index_count,
                         const OnixGPUInstance3D *instances, uint32_t instance_count, const uint32_t *words,
                         uint32_t word_count, const OnixNativeMeshDraw3D *draws, uint32_t draw_count,
                         const float *lighting) {
    return dx_draw_meshes(d, push, vertices, vertex_count, indices, index_count, instances, instance_count, words,
                          word_count, draws, draw_count, lighting, 0);
}
int onix_gpu_draw_mesh_picking(OnixGPUDriver *d, const OnixGPUMeshPush *push, const OnixGPUVertex3D *vertices,
                               uint32_t vertex_count, const uint32_t *indices, uint32_t index_count,
                               const OnixGPUInstance3D *instances, uint32_t instance_count,
                               const OnixNativeMeshDraw3D *draws, uint32_t draw_count) {
    return dx_draw_meshes(d, push, vertices, vertex_count, indices, index_count, instances, instance_count, NULL, 0,
                          draws, draw_count, NULL, 1);
}
