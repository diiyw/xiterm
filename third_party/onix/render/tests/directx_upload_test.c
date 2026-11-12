#include <limits.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "native_buffer.h"

// Only API behavior is mocked; allocation, cursor and binding functions below
// are extracted from directx_native.c. Windows pixel tests exercise D3D itself.
typedef unsigned UINT;
typedef int HRESULT;
typedef struct { UINT ByteWidth, Usage, BindFlags, CPUAccessFlags; } D3D11_BUFFER_DESC;
typedef struct { void *pData; } D3D11_MAPPED_SUBRESOURCE;
typedef struct { uint8_t *data; UINT capacity; } ID3D11Buffer;
typedef ID3D11Buffer ID3D11Resource;
enum { E_INVALIDARG = -1, D3D11_USAGE_DYNAMIC = 2, D3D11_CPU_ACCESS_WRITE = 1,
    D3D11_MAP_WRITE_DISCARD = 4, D3D11_MAP_WRITE_NO_OVERWRITE = 5,
    D3D11_BIND_VERTEX_BUFFER = 1, D3D11_BIND_INDEX_BUFFER = 2,
    DXGI_FORMAT_R32_UINT = 42, D3D11_PRIMITIVE_TOPOLOGY_TRIANGLELIST = 4 };
static unsigned creates, discards, appends, fail_create, fail_map;
static UINT bound_vertex[2], bound_index;
static void require(int ok, const char *message) {
    if (!ok) { fprintf(stderr, "%s\n", message); exit(1); }
}
static int dx_fail(const char *message, HRESULT result) { return 0; }
static int dx_ok(HRESULT result, const char *message) { return result == 0; }
static HRESULT mock_create(void *device, const D3D11_BUFFER_DESC *desc, void *initial, ID3D11Buffer **result) {
    if (fail_create) { fail_create = 0; return -1; }
    ID3D11Buffer *buffer = calloc(1, sizeof(*buffer));
    require(buffer != NULL, "mock buffer allocation");
    buffer->data = malloc(desc->ByteWidth);
    require(buffer->data != NULL, "mock buffer storage");
    buffer->capacity = desc->ByteWidth;
    *result = buffer;
    creates++;
    return 0;
}
static void mock_release(ID3D11Buffer **buffer) {
    if (!*buffer) return;
    free((*buffer)->data); free(*buffer); *buffer = NULL;
}
static HRESULT mock_map(void *context, ID3D11Resource *buffer, UINT subresource, UINT mode, UINT flags, D3D11_MAPPED_SUBRESOURCE *mapped) {
    if (fail_map) { fail_map = 0; return -1; }
    require(buffer != NULL, "map live buffer");
    if (mode == D3D11_MAP_WRITE_DISCARD) { discards++; memset(buffer->data, 0xcd, buffer->capacity); }
    else { require(mode == D3D11_MAP_WRITE_NO_OVERWRITE, "only discard or no-overwrite maps"); appends++; }
    mapped->pData = buffer->data;
    return 0;
}
static void mock_vertices(void *context, UINT start, UINT count, ID3D11Buffer *const *buffers, const UINT *strides, const UINT *offsets) {
    require(start == 0 && count == 2, "bind both vertex streams");
    memcpy(bound_vertex, offsets, sizeof(bound_vertex));
}
static void mock_indices(void *context, ID3D11Buffer *buffer, UINT format, UINT offset) { bound_index = offset; }
#define RELEASE(value) mock_release(&(value))
#define ID3D11Device_CreateBuffer mock_create
#define ID3D11DeviceContext_Map mock_map
#define ID3D11DeviceContext_Unmap(context, buffer, subresource) ((void)0)
#define ID3D11DeviceContext_IASetVertexBuffers mock_vertices
#define ID3D11DeviceContext_IASetIndexBuffer mock_indices
#define ID3D11DeviceContext_IASetPrimitiveTopology(context, topology) ((void)0)
#include "directx_upload.inc"

int main(void) {
    OnixGPUDriver driver = {0};
    uint8_t source[128];
    for (unsigned frame = 0; frame < 96; frame++) {
        dx_begin_uploads(&driver, &driver.instances);
        unsigned before_creates = creates, before_discards = discards, before_appends = appends;
        for (unsigned batch = 0; batch < 900; batch++) {
            memset(source, batch % 251, sizeof(source));
            require(dx_upload(&driver, &driver.instances, source, sizeof(source), D3D11_BIND_VERTEX_BUFFER), "upload stream");
            require(driver.instances.offset % 16 == 0, "aligned instance offset");
            require(memcmp(driver.instances.value->data + driver.instances.offset, source, sizeof(source)) == 0, "upload payload");
            if (frame && batch) {
                require(driver.instances.offset == batch * sizeof(source), "warm frame ranges never overlap");
                require(driver.instances.value->data[0] == 0, "append preserves first upload");
            }
        }
        if (frame) {
            require(creates == before_creates, "warm frames create no GPU buffers");
            require(discards == before_discards + 1 && appends == before_appends + 899, "one discard and 899 appends per warm frame");
        }
    }
    require(driver.instances.capacity == 128u * 1024u, "900 batches warm to 128 KiB");
    require(dx_upload(&driver, &driver.vertices, source, 33, D3D11_BIND_VERTEX_BUFFER), "first vertices");
    require(dx_upload(&driver, &driver.vertices, source, 33, D3D11_BIND_VERTEX_BUFFER), "append vertices");
    require(dx_upload(&driver, &driver.indices, source, 12, D3D11_BIND_INDEX_BUFFER), "first indices");
    require(dx_upload(&driver, &driver.indices, source, 12, D3D11_BIND_INDEX_BUFFER), "append indices");
    dx_bind_geometry(&driver, driver.vertices.value, driver.indices.value, 33, 128, 128);
    require(bound_vertex[0] == 48 && bound_vertex[1] == driver.instances.offset + 128 && bound_index == 16, "dynamic vertex, index and FirstInstance offsets");
    ID3D11Buffer immutable = {0};
    dx_bind_geometry(&driver, &immutable, &immutable, 33, 128, 0);
    require(bound_vertex[0] == 0 && bound_index == 0 && bound_vertex[1] == driver.instances.offset, "resident geometry stays at zero");
    for (unsigned frame = 0; frame < ONIX_BUFFER_TRIM_DELAY + 1; frame++) {
        dx_begin_uploads(&driver, &driver.instances);
        require(dx_upload(&driver, &driver.instances, source, 7, D3D11_BIND_VERTEX_BUFFER), "small workload");
    }
    require(driver.instances.capacity == 64u * 1024u, "low demand releases peak and regrows at minimum");
    uint8_t *large = calloc(1, 2u << 20);
    require(large != NULL && dx_upload(&driver, &driver.instances, large, 2u << 20, D3D11_BIND_VERTEX_BUFFER), "oversized upload");
    dx_begin_uploads(&driver, &driver.instances);
    for (unsigned frame = 0; frame < ONIX_BUFFER_TRIM_DELAY; frame++) dx_begin_uploads(&driver, &driver.instances);
    require(!driver.instances.value && driver.instances.capacity == 0, "idle releases oversized buffer");
    fail_create = 1;
    require(!dx_upload(&driver, &driver.instances, source, 7, D3D11_BIND_VERTEX_BUFFER), "creation failure");
    require(dx_upload(&driver, &driver.instances, source, 7, D3D11_BIND_VERTEX_BUFFER), "retry creation");
    UINT used = driver.instances.used;
    fail_map = 1;
    require(!dx_upload(&driver, &driver.instances, source, 7, D3D11_BIND_VERTEX_BUFFER) && driver.instances.used == used, "failed map does not consume range");
    require(dx_upload(&driver, &driver.instances, source, 7, D3D11_BIND_VERTEX_BUFFER), "retry map");
    // Force wrap at the bulk ceiling: discard rather than overwrite busy data.
    RELEASE(driver.instances.value); driver.instances = (DxBuffer){0};
    require(dx_upload(&driver, &driver.instances, large, 1u << 20, D3D11_BIND_VERTEX_BUFFER), "fill bulk buffer");
    unsigned before = discards;
    require(dx_upload(&driver, &driver.instances, source, sizeof(source), D3D11_BIND_VERTEX_BUFFER), "wrap bulk buffer");
    require(discards == before + 1 && driver.instances.offset == 0, "wrap always discards");
    RELEASE(driver.vertices.value); RELEASE(driver.indices.value); RELEASE(driver.instances.value); free(large);
    puts("DirectX production upload policy (mock D3D): 900 batches -> 1 DISCARD + 899 NO_OVERWRITE, 96 frames without warm resource creation; dynamic/resident bindings, peak/idle release, failures and bulk wrap passed");
    return 0;
}
