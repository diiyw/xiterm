#define COBJMACROS
#include <d3d11.h>
static unsigned stream_discards, stream_appends;
static HRESULT tracked_map(ID3D11DeviceContext *context, ID3D11Resource *resource, UINT subresource,
                           D3D11_MAP mode, UINT flags, D3D11_MAPPED_SUBRESOURCE *mapping) {
    HRESULT result = context->lpVtbl->Map(context, resource, subresource, mode, flags, mapping);
    if (SUCCEEDED(result) && (mode == D3D11_MAP_WRITE_DISCARD || mode == D3D11_MAP_WRITE_NO_OVERWRITE)) {
        D3D11_BUFFER_DESC desc;
        ID3D11Buffer_GetDesc((ID3D11Buffer *)resource, &desc);
        if (desc.BindFlags & (D3D11_BIND_VERTEX_BUFFER | D3D11_BIND_INDEX_BUFFER)) {
            stream_discards += mode == D3D11_MAP_WRITE_DISCARD;
            stream_appends += mode == D3D11_MAP_WRITE_NO_OVERWRITE;
        }
    }
    return result;
}
#undef ID3D11DeviceContext_Map
#define ID3D11DeviceContext_Map tracked_map
#include "../directx_native.c"
static void require(int success, const char *operation) {
    if (!success) {
        fprintf(stderr, "%s: %s\n", operation, onix_gpu_last_error());
        exit(1);
    }
}
static OnixGPUShader load_shader(const char *path) {
    FILE *f = fopen(path, "rb");
    require(f != NULL, path);
    fseek(f, 0, SEEK_END);
    size_t length = ftell(f);
    rewind(f);
    void *data = malloc(length);
    require(data && fread(data, 1, length, f) == length, "read shader");
    fclose(f);
    return (OnixGPUShader){data, length};
}
static void check_mipmap_updates(OnixGPUDriver *d) {
    const uint8_t red[] = {255, 0, 0, 255, 255, 0, 0, 255, 255, 0, 0, 255, 255, 0, 0, 255};
    const uint8_t green[] = {0, 255, 0, 255, 0, 255, 0, 255, 0, 255, 0, 255, 0, 255, 0, 255};
    uint32_t id = onix_gpu_create_texture_with_format(d, 2, 2, 2, 0, 1, red, sizeof(red), 0, 0);
    require(id != 0, "create mipmapped texture");
    OnixGPUTextureUpdate updates[] = {
        {.texture = id, .width = 2, .height = 1, .data_length = 8},
        {.texture = id, .y = 1, .width = 2, .height = 1, .data_offset = 8, .data_length = 8}};
    require(onix_gpu_update_textures(d, updates, 2, green, sizeof(green)), "update mipmapped texture");
    D3D11_TEXTURE2D_DESC desc = {.Width = 1, .Height = 1, .MipLevels = 1, .ArraySize = 1,
                               .Format = DXGI_FORMAT_R8G8B8A8_UNORM, .SampleDesc = {1, 0},
                               .Usage = D3D11_USAGE_STAGING, .CPUAccessFlags = D3D11_CPU_ACCESS_READ};
    ID3D11Texture2D *staging = NULL;
    require(SUCCEEDED(ID3D11Device_CreateTexture2D(d->device, &desc, NULL, &staging)), "create mip readback");
    ID3D11DeviceContext_CopySubresourceRegion(d->context, (ID3D11Resource *)staging, 0, 0, 0, 0,
                                            (ID3D11Resource *)dx_get_texture(d, id)->value, 1, NULL);
    D3D11_MAPPED_SUBRESOURCE mapped;
    require(SUCCEEDED(ID3D11DeviceContext_Map(d->context, (ID3D11Resource *)staging, 0, D3D11_MAP_READ, 0, &mapped)),
            "map mip readback");
    const uint8_t *pixel = mapped.pData;
    require(pixel[0] == 0 && pixel[1] == 255 && pixel[2] == 0 && pixel[3] == 255, "mipmap regenerated after batch update");
    ID3D11DeviceContext_Unmap(d->context, (ID3D11Resource *)staging, 0);
    RELEASE(staging);
    require(onix_gpu_destroy_texture(d, id), "destroy mipmapped texture");
}
static void check_sprites(OnixGPUDriver *d) {
    const uint8_t white[] = {255, 255, 255, 255};
    uint32_t texture = onix_gpu_create_texture_with_format(d, 1, 1, 1, 0, 0, white, 4, 0, 0), color;
    require(texture != 0, "create sprite texture");
    OnixGPUShader vertex = load_shader("shaders/directx_sprite.vert.hlsl"),
                  fragment = load_shader("shaders/directx_sprite.frag.hlsl");
    uint32_t shader = onix_gpu_create_material2d_shader(d, vertex, fragment);
    free((void *)vertex.code);
    free((void *)fragment.code);
    require(shader != 0, "custom sprite shader");
    OnixGPUSpritePush push = {.projection = {[0] = .125f, [5] = .125f, [10] = 1, [12] = -1, [13] = -1, [15] = 1}};
    const float colors[4][4] = {{1, 0, 0, 1}, {0, 1, 0, 1}, {0, 0, 1, 1}, {1, 1, 0, 1}};
    OnixGPUInstance instances[4];
    for (int i = 0; i < 4; i++) {
        instances[i] = (OnixGPUInstance){.model = {4, 0, i * 4, 0, 16, 0}, .texture_model = {1, 0, 0, 0, 1, 0}};
        memcpy(instances[i].color, colors[i], 16);
    }
    instances[0].custom[2][2] = 2;
    instances[0].custom[2][3] = 16;
    instances[0].custom[3][1] = 1;
    OnixNativeSpriteDraw draws[] = {{.texture = texture, .instance_count = 2},
                                    {.shader = shader, .texture = texture, .first_instance = 2, .instance_count = 1},
                                    {.texture = texture, .first_instance = 3, .instance_count = 1}};
    float clear[4] = {0};
    uint32_t target = onix_gpu_create_render_target(d, 16, 16, 0, 1, &color);
    require(target != 0, "sprite target");
    for (int frame = 0; frame < 2; frame++) {
        require(onix_gpu_begin_frame(d, clear) && onix_gpu_begin_render_target(d, target, clear, 0, 1, 1),
                "begin sprite target");
        onix_gpu_set_viewport(d, 0, 0, 16, 16);
        require(onix_gpu_draw_sprites(d, &push, instances, 4, draws, 3), "draw sprites");
        require(onix_gpu_end_render_target(d), "end sprite target");
        for (int i = 0; i < 4; i++) {
            uint8_t pixel[4];
            require(onix_gpu_read_render_target(d, target, i * 4 + 1, 8, 1, 1, pixel, 4), "read sprite");
            for (int c = 0; c < 4; c++)
                require(abs(pixel[c] - (int)(colors[i][c] * 255)) <= 1, "sprite tint and instance offset");
        }
        uint8_t clipped[4];
        require(onix_gpu_read_render_target(d, target, 3, 8, 1, 1, clipped, 4) && clipped[3] == 0, "sprite clipping");
    }
    require(onix_gpu_destroy_render_target(d, target) && onix_gpu_destroy_material2d_shader(d, shader) &&
                onix_gpu_destroy_texture(d, texture),
            "destroy sprite resources");
}
static void check_streams(OnixGPUDriver *d) {
    const uint8_t white[] = {255, 255, 255, 255};
    uint32_t texture = onix_gpu_create_texture_with_format(d, 1, 1, 1, 0, 0, white, 4, 0, 0), color;
    uint32_t target = onix_gpu_create_render_target(d, 16, 16, 0, 0, &color);
    require(texture && target, "stream resources");
    OnixGPUSpritePush push = {.projection = {[0] = .125f, [5] = .125f, [10] = 1, [12] = -1, [13] = -1, [15] = 1}};
    OnixGPUInstance instance = {.model = {16, 0, 0, 0, 16, 0}, .texture_model = {1, 0, 0, 0, 1, 0}};
    OnixNativeSpriteDraw draw = {.texture = texture, .instance_count = 1};
    float clear[4] = {0};
    for (unsigned frame = 0; frame < 5; frame++) {
        require(onix_gpu_begin_frame(d, clear) && onix_gpu_begin_render_target(d, target, clear, 0, 1, 0), "begin stream target");
        onix_gpu_set_viewport(d, 0, 0, 16, 16);
        stream_discards = stream_appends = 0;
        for (unsigned batch = 0; batch < 900; batch++) {
            instance.color[0] = (batch + frame) % 2 == 0;
            instance.color[1] = (batch + frame) % 2 != 0;
            instance.color[3] = 1;
            unsigned tile = batch % 256;
            onix_gpu_set_scissor(d, 1, tile % 16, tile / 16, 1, 1);
            require(onix_gpu_draw_sprites(d, &push, &instance, 1, &draw, 1), "interleaved sprite upload/draw");
        }
        if (frame) require(stream_discards == 1 && stream_appends == 899, "warm stream map counts");
        onix_gpu_set_scissor(d, 0, 0, 0, 0, 0);
        require(onix_gpu_end_render_target(d), "end stream target");
        uint8_t pixels[16 * 16 * 4];
        require(onix_gpu_read_render_target(d, target, 0, 0, 16, 16, pixels, sizeof(pixels)), "stream readback");
        for (unsigned tile = 0; tile < 256; tile++) {
            unsigned channel = (tile + frame) % 2;
            require(pixels[tile * 4 + channel] == 255 && pixels[tile * 4 + 1 - channel] == 0 && pixels[tile * 4 + 3] == 255, "stream tile pixel after growth/reuse");
        }
    }
    require(onix_gpu_destroy_render_target(d, target) && onix_gpu_destroy_texture(d, texture), "destroy stream resources");
}
static void check_picking(OnixGPUDriver *d) {
    uint32_t color, target = onix_gpu_create_render_target(d, 16, 16, 1, 1, &color), identifier = 1234567, pixel = 0;
    require(target != 0, "picking target");
    float clear[4] = {0};
    OnixGPUMeshPush push = {.view_projection = {[0] = 1, [5] = 1, [10] = 1, [15] = 1}};
    OnixGPUVertex3D vertices[] = {{.position = {-1, -1, .5f}}, {.position = {1, -1, .5f}}, {.position = {0, 1, .5f}}};
    uint32_t indices[] = {0, 1, 2};
    OnixGPUInstance3D instance = {.model = {[0] = 1, [5] = 1, [10] = 1, [15] = 1}};
    memcpy(&instance.custom[3][3], &identifier, 4);
    OnixNativeMeshDraw3D draw = {.index_count = 3, .instance_count = 1};
    require(onix_gpu_begin_frame(d, clear) && onix_gpu_begin_render_target(d, target, clear, 0, 1, 1),
            "begin picking target");
    require(onix_gpu_draw_mesh_picking(d, &push, vertices, 3, indices, 3, &instance, 1, &draw, 1), "draw picking");
    require(onix_gpu_end_render_target(d) && onix_gpu_read_render_target(d, target, 8, 8, 1, 1, (uint8_t *)&pixel, 4),
            "read picking");
    require(pixel == identifier, "picking identifier");
    require(onix_gpu_destroy_render_target(d, target), "destroy picking target");
    target = onix_gpu_create_render_target(d, 16, 16, 0, 1, &color);
    const uint8_t white[] = {255, 255, 255, 255};
    draw.texture = onix_gpu_create_texture_with_format(d, 1, 1, 1, 0, 0, white, 4, 0, 0);
    require(target && draw.texture, "mesh resources");
    instance.material[0] = -1;
    instance.base_color[0] = instance.base_color[3] = 1;
    instance.uv_transform[0] = instance.uv_transform[1] = 1;
    require(onix_gpu_begin_frame(d, clear) && onix_gpu_begin_render_target(d, target, clear, 0, 1, 1), "begin mesh target");
    require(onix_gpu_draw_meshes(d, &push, vertices, 3, indices, 3, &instance, 1, NULL, 0, &draw, 1, NULL), "draw unlit mesh");
    uint8_t mesh_pixel[4];
    require(onix_gpu_end_render_target(d) && onix_gpu_read_render_target(d, target, 8, 8, 1, 1, mesh_pixel, 4), "read mesh");
    if (mesh_pixel[0] != 255 || mesh_pixel[1] != 0 || mesh_pixel[3] != 255) {
        fprintf(stderr, "mesh pixel: %u %u %u %u\n", mesh_pixel[0], mesh_pixel[1], mesh_pixel[2], mesh_pixel[3]);
        require(0, "mesh tint and texture binding");
    }
    require(onix_gpu_destroy_render_target(d, target) && onix_gpu_destroy_texture(d, draw.texture), "destroy mesh resources");
}
int main(void) {
    HWND window = CreateWindowExW(0, L"STATIC", L"Onix DirectX regression", WS_OVERLAPPEDWINDOW, 0, 0, 32, 32, NULL,
                                  NULL, GetModuleHandleW(NULL), NULL);
    require(window != NULL, "hidden window");
    OnixGPUShader shaders[] = {
        load_shader("shaders/directx_sprite_multi.vert.cso"), load_shader("shaders/directx_sprite_multi.frag.cso"),
        load_shader("shaders/directx_shape.vert.cso"),        load_shader("shaders/directx_shape.frag.cso"),
        load_shader("shaders/directx_mesh.vert.cso"),         load_shader("shaders/directx_mesh.frag.cso"),
        load_shader("shaders/directx_mesh_unlit.frag.cso"),   load_shader("shaders/directx_mesh_depth.frag.cso"),
        load_shader("shaders/directx_mesh_basic.frag.cso"),   load_shader("shaders/directx_picking.vert.cso"),
        load_shader("shaders/directx_picking.frag.cso")};
    OnixGPUDriver *d = onix_gpu_create(
        (OnixGPUSurface){.kind = ONIX_GPU_SURFACE_WIN32, .window = (uintptr_t)window, .width = 16, .height = 16},
        shaders[0], shaders[1], shaders[2], shaders[3], shaders[4], shaders[5], shaders[6], shaders[7], shaders[8],
        shaders[9], shaders[10]);
    require(d != NULL, "create driver and compile builtin shaders");
    float clear[] = {0, 0, 0, 0};
    require(onix_gpu_begin_frame(d, clear), "begin frame");
    uint32_t color = 0;
    uint32_t target = onix_gpu_create_render_target(d, 16, 16, 0, 1, &color);
    require(target != 0, "create target");
    float red[] = {1, 0, 0, 1};
    require(onix_gpu_begin_render_target(d, target, red, 0, 1, 1), "clear target");
    require(onix_gpu_end_render_target(d), "end target");
    uint8_t pixels[16 * 16 * 4];
    require(onix_gpu_read_render_target(d, target, 0, 0, 16, 16, pixels, sizeof(pixels)), "read target");
    require(pixels[0] == 255 && pixels[1] == 0 && pixels[3] == 255, "red pixel");
    require(onix_gpu_destroy_render_target(d, target), "destroy clear target");
    check_sprites(d);
    check_streams(d);
    check_mipmap_updates(d);
    check_picking(d);
    require(onix_gpu_present(d), "present");
    onix_gpu_destroy(d);
    d = onix_gpu_create(
        (OnixGPUSurface){.kind = ONIX_GPU_SURFACE_WIN32, .window = (uintptr_t)window, .width = 16, .height = 16, .transparent = 1},
        shaders[0], shaders[1], shaders[2], shaders[3], shaders[4], shaders[5], shaders[6], shaders[7], shaders[8],
        shaders[9], shaders[10]);
    require(d != NULL, "create transparent composition");
    onix_gpu_set_corner_radius(d, 4);
    require(onix_gpu_resize(d, 32, 32) && onix_gpu_begin_frame(d, clear) && onix_gpu_present(d), "resize and present transparent window");
    onix_gpu_destroy(d);
    for (int i = 0; i < 11; i++)
        free((void *)shaders[i].code);
    DestroyWindow(window);
    puts("DirectX shaders, sprites, meshes, clipping, mipmaps, picking, readback and transparent presentation passed");
    return 0;
}
