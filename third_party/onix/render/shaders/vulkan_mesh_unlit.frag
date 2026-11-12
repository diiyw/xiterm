#version 450
// Generated unlit variant from mesh.frag.wgsl.




struct PBRUniforms {
    mat4x4 projection;
    vec4 cameraPosition;
    vec4 ambientLight;
    vec4 lightDirection;
    vec4 directionalLight;
    vec4 lightPosition[16];
    vec4 lightDirections[16];
    vec4 lightColors[16];
    vec4 lightParams[16];
    vec4 lightMeta;
    vec4 environment;
    vec4 renderParams;
    vec4 cameraForward;
    vec4 lightRight[16];
    vec4 lightUp[16];
    mat4x4 shadowMatrices[16];
    vec4 shadowRects[16];
    vec4 shadowParams[16];
    vec4 lightShadows[16];
    mat4x4 occlusionInverseProjection;
};
struct PBRInput {
    vec4 position;
    bool front;
    vec3 worldPosition;
    vec2 uv;
    vec4 baseColor;
    vec4 material;
    vec4 custom0_;
    vec4 custom1_;
    vec4 custom2_;
    vec4 custom3_;
};
const float PI = 3.1415927;
const bool SHADOW_FLIP_Y = true;

layout(set=2,binding=1,std140) uniform PBRUniforms_block_0Fragment { PBRUniforms _group_0_binding_0_fs; };

layout(set=0,binding=0) uniform sampler2D tex;

layout(set=3,binding=5) uniform sampler2D environmentTex;

layout(location=0) smooth in vec3 _vs2fs_location0;
layout(location=2) smooth in vec2 _vs2fs_location2;
layout(location=3) smooth in vec4 _vs2fs_location3;
layout(location=5) flat in vec4 _vs2fs_location5;
layout(location=6) flat in vec4 _vs2fs_location6;
layout(location=7) flat in vec4 _vs2fs_location7;
layout(location=8) flat in vec4 _vs2fs_location8;
layout(location=9) flat in vec4 _vs2fs_location9;
layout(location = 0) out vec4 _fs2p_location0;

vec3 pbr_srgb(vec3 linear) {
    vec3 c = max(linear, vec3(0.0));
    return mix(((1.055 * pow(c, vec3(0.41666666))) - vec3(0.055)), (12.92 * c), lessThanEqual(c, vec3(0.0031308)));
}

vec3 pbr_tone_map(vec3 linear_1) {
    float _e4 = _group_0_binding_0_fs.renderParams.x;
    float _e8 = _group_0_binding_0_fs.renderParams.x;
    float exposure = ((_e8 <= 0.0) ? 1.0 : _e4);
    vec3 hdr = clamp((linear_1 * exposure), vec3(0.0), vec3(1e20));
    vec3 reinhard = (hdr / (hdr + vec3(1.0)));
    float _e27 = _group_0_binding_0_fs.lightShadows[7].w;
    if ((_e27 < 0.5)) {
        return reinhard;
    }
    mat3x3 inputMatrix = mat3x3(vec3(0.59719, 0.076, 0.0284), vec3(0.35458, 0.90834, 0.13383), vec3(0.04823, 0.01566, 0.83777));
    mat3x3 outputMatrix = mat3x3(vec3(1.60475, -0.10208, -0.00327), vec3(-0.53108, 1.10813, -0.07276), vec3(-0.07367, -0.00605, 1.07602));
    vec3 aces = (inputMatrix * (hdr / vec3(0.6)));
    vec3 reciprocal = (vec3(1.0) / (aces + vec3(1.0)));
    vec3 normalized = (aces * reciprocal);
    vec3 squared = (normalized * normalized);
    vec3 crossTerm = (normalized * reciprocal);
    vec3 fit = (((squared + (0.0245786 * crossTerm)) - ((9.0537e-5 * reciprocal) * reciprocal)) / (((0.983729 * squared) + (0.432951 * crossTerm)) + ((0.238081 * reciprocal) * reciprocal)));
    return clamp((outputMatrix * fit), vec3(0.0), vec3(1.0));
}

vec4 pbr_encode_radiance(vec3 linear_2) {
    vec3 c_1 = max(linear_2, vec3(0.0));
    float exponent = clamp(ceil(log2(max(max(max(c_1.x, c_1.y), c_1.z), 1e-30))), -120.0, 120.0);
    return vec4((c_1 * exp2(-(exponent))), ((exponent + 128.0) / 255.0));
}

vec4 pbr_output(vec3 linear_3, float alpha) {
    float _e5 = _group_0_binding_0_fs.renderParams.y;
    if ((_e5 == 3.0)) {
        vec4 _e8 = pbr_encode_radiance(linear_3);
        return _e8;
    }
    vec3 _e9 = pbr_tone_map(linear_3);
    vec3 _e10 = pbr_srgb(_e9);
    return vec4((_e10 * alpha), alpha);
}

vec3 pbr_fog_color() {
    float _e4 = _group_0_binding_0_fs.lightShadows[0].w;
    float _e9 = _group_0_binding_0_fs.lightShadows[1].w;
    float _e14 = _group_0_binding_0_fs.lightShadows[2].w;
    return vec3(_e4, _e9, _e14);
}

float pbr_fog_factor(vec3 position) {
    float mode = _group_0_binding_0_fs.lightShadows[6].w;
    if ((mode < 0.5)) {
        return 0.0;
    }
    vec4 _e11 = _group_0_binding_0_fs.cameraPosition;
    vec3 delta = (position - _e11.xyz);
    vec4 _e17 = _group_0_binding_0_fs.cameraForward;
    float _e25 = _group_0_binding_0_fs.cameraPosition.w;
    float distance_ = ((_e25 < 0.0) ? max(dot(delta, _e17.xyz), 0.0) : length(delta));
    float start = _group_0_binding_0_fs.lightShadows[3].w;
    if ((mode < 1.5)) {
        float _e41 = _group_0_binding_0_fs.lightShadows[4].w;
        return clamp(((distance_ - start) / max((_e41 - start), 1e-6)), 0.0, 1.0);
    }
    float _e56 = _group_0_binding_0_fs.lightShadows[5].w;
    float opticalDepth = (max((distance_ - start), 0.0) * _e56);
    return (1.0 - exp((-(opticalDepth) * opticalDepth)));
}

vec4 pbr_decode_radiance(vec4 encoded) {
    return vec4((encoded.xyz * exp2((roundEven((encoded.w * 255.0)) - 128.0))), ((encoded.w > 0.0) ? 1.0 : 0.0));
}

vec4 pbr_atlas(vec2 uv, float level) {
    float _e5 = _group_0_binding_0_fs.environment.z;
    float _e9 = _group_0_binding_0_fs.environment.w;
    vec2 inset = vec2(_e5, _e9);
    vec2 local_6 = ((clamp(uv, vec2(0.0), vec2(1.0)) * (vec2(1.0) - (2.0 * inset))) + inset);
    vec4 _e32 = textureLod(environmentTex, vec2(vec2(local_6.x, ((local_6.y + level) / 8.0))), 0.0);
    return _e32;
}

vec3 pbr_rgbe_atlas(vec2 uv_1, float level_1) {
    ivec2 dims = ivec2(uvec2(textureSize(environmentTex, 0).xy));
    float _e8 = _group_0_binding_0_fs.environment.z;
    float _e12 = _group_0_binding_0_fs.environment.w;
    vec2 inset_1 = vec2(_e8, _e12);
    vec2 local_7 = ((clamp(uv_1, vec2(0.0), vec2(1.0)) * (vec2(1.0) - (2.0 * inset_1))) + inset_1);
    vec2 coord = ((vec2(local_7.x, ((local_7.y + level_1) / 8.0)) * vec2(dims)) - vec2(0.5));
    ivec2 base_1 = ivec2(floor(coord));
    vec2 weight = fract(coord);
    int tileHeight = (dims.y / 8);
    ivec2 first = ivec2(0, (int(level_1) * tileHeight));
    ivec2 last = ivec2((dims.x - 1), ((first.y + tileHeight) - 1));
    vec4 _e58 = texelFetch(environmentTex, min(max(base_1, first), last), 0);
    vec4 _e59 = pbr_decode_radiance(_e58);
    vec3 a = _e59.xyz;
    vec4 _e68 = texelFetch(environmentTex, min(max((base_1 + ivec2(1, 0)), first), last), 0);
    vec4 _e69 = pbr_decode_radiance(_e68);
    vec3 b = _e69.xyz;
    vec4 _e78 = texelFetch(environmentTex, min(max((base_1 + ivec2(0, 1)), first), last), 0);
    vec4 _e79 = pbr_decode_radiance(_e78);
    vec3 c_2 = _e79.xyz;
    vec4 _e87 = texelFetch(environmentTex, min(max((base_1 + ivec2(1)), first), last), 0);
    vec4 _e88 = pbr_decode_radiance(_e87);
    vec3 d = _e88.xyz;
    return mix(mix(a, b, weight.x), mix(c_2, d, weight.x), weight.y);
}

vec2 pbr_environment_uv(vec3 direction) {
    float _e11 = _group_0_binding_0_fs.environment.y;
    return vec2(fract((((atan(direction.z, direction.x) / 6.2831855) + 0.5) + (_e11 / 6.2831855))), (acos(clamp(direction.y, -1.0, 1.0)) / PI));
}

vec3 pbr_panorama(vec3 direction_1) {
    ivec2 dims_1 = ivec2(uvec2(textureSize(tex, 0).xy));
    vec2 _e4 = pbr_environment_uv(direction_1);
    vec2 coord_1 = ((_e4 * vec2(dims_1)) - vec2(0.5));
    ivec2 origin = ivec2(floor(coord_1));
    vec2 weight_1 = fract(coord_1);
    int x0_ = ((origin.x + dims_1.x) % dims_1.x);
    int x1_ = ((x0_ + 1) % dims_1.x);
    int y0_ = min(max(origin.y, 0), (dims_1.y - 1));
    int y1_ = min(max((origin.y + 1), 0), (dims_1.y - 1));
    vec4 _e39 = texelFetch(tex, ivec2(x0_, y0_), 0);
    vec4 _e40 = pbr_decode_radiance(_e39);
    vec3 a_1 = _e40.xyz;
    vec4 _e45 = texelFetch(tex, ivec2(x1_, y0_), 0);
    vec4 _e46 = pbr_decode_radiance(_e45);
    vec3 b_1 = _e46.xyz;
    vec4 _e51 = texelFetch(tex, ivec2(x0_, y1_), 0);
    vec4 _e52 = pbr_decode_radiance(_e51);
    vec3 c_3 = _e52.xyz;
    vec4 _e57 = texelFetch(tex, ivec2(x1_, y1_), 0);
    vec4 _e58 = pbr_decode_radiance(_e57);
    vec3 d_1 = _e58.xyz;
    return mix(mix(a_1, b_1, weight_1.x), mix(c_3, d_1, weight_1.x), weight_1.y);
}

vec3 pbr_environment(vec3 direction_2, float level_2) {
    vec2 _e2 = pbr_environment_uv(direction_2);
    float _e7 = _group_0_binding_0_fs.lightShadows[8].w;
    if ((_e7 > 0.5)) {
        float lower = floor(level_2);
        float upper = ceil(level_2);
        vec3 _e12 = pbr_rgbe_atlas(_e2, lower);
        if ((lower == upper)) {
            return _e12;
        }
        vec3 _e14 = pbr_rgbe_atlas(_e2, upper);
        return mix(_e12, _e14, fract(level_2));
    }
    vec4 _e18 = pbr_atlas(_e2, floor(level_2));
    vec4 _e20 = pbr_atlas(_e2, ceil(level_2));
    return (mix((_e18.xyz * _e18.w), (_e20.xyz * _e20.w), fract(level_2)) * 16.0);
}

float grid_coverage(vec2 point, float step_, float pixels) {
    vec2 uv_3 = (point / vec2(step_));
    vec2 _e5 = fwidth(uv_3);
    vec2 footprint = max(_e5, vec2(1e-6));
    vec2 distance_1 = (abs((fract((uv_3 - vec2(0.5))) - vec2(0.5))) / footprint);
    vec2 lines = (vec2(1.0) - smoothstep(vec2(((pixels * 0.5) - 0.5)), vec2(((pixels * 0.5) + 0.5)), distance_1));
    vec2 visibility = (vec2(1.0) - smoothstep(vec2(0.25), vec2(0.75), footprint));
    return max((lines.x * visibility.x), (lines.y * visibility.y));
}

vec4 editor_grid(PBRInput input_1) {
    float step_1 = input_1.custom0_.x;
    float blend = input_1.custom0_.y;
    float pixels_1 = input_1.custom0_.z;
    uint axis = uint(input_1.custom0_.w);
    vec2 point_1 = vec2(input_1.worldPosition[((axis + 1u) % 3u)], input_1.worldPosition[((axis + 2u) % 3u)]);
    float _e23 = grid_coverage(point_1, step_1, pixels_1);
    float _e26 = grid_coverage(point_1, (step_1 * 10.0), pixels_1);
    float _e29 = grid_coverage(point_1, (step_1 * 100.0), pixels_1);
    float coverage = mix(max((_e23 * 0.24), (_e26 * 0.48)), max((_e26 * 0.24), (_e29 * 0.48)), blend);
    float distance_2 = length((point_1 - input_1.custom1_.xy));
    float fade = (1.0 - smoothstep((input_1.custom1_.z * 0.4), input_1.custom1_.z, distance_2));
    vec4 _e56 = _group_0_binding_0_fs.cameraPosition;
    vec3 delta_1 = (_e56.xyz - input_1.worldPosition);
    vec3 direction_4 = ((input_1.custom2_.w > 0.5) ? input_1.custom2_.xyz : (delta_1 / vec3(max(length(delta_1), 1e-6))));
    float angle = smoothstep(0.015, 0.15, abs(direction_4[axis]));
    float alpha_1 = (((coverage * fade) * angle) * input_1.baseColor.w);
    return vec4((input_1.baseColor.xyz * alpha_1), alpha_1);
}

vec4 fxaa_sample(vec2 uv_2, vec2 pixel) {
    vec4 _e13 = textureLod(tex, vec2(clamp(uv_2, (pixel * 0.5), (vec2(1.0) - (pixel * 0.5)))), 0.0);
    return _e13;
}

float fxaa_luma(vec4 color) {
    return (dot(color.xyz, vec3(0.299, 0.587, 0.114)) + (color.w * 0.25));
}

vec4 fxaa(vec2 uvIn) {
    vec2 direction_3 = vec2(0.0);
    bool local_5 = false;
    vec2 uv_4 = vec2(uvIn.x, (SHADOW_FLIP_Y ? (1.0 - uvIn.y) : uvIn.y));
    vec2 pixel_1 = (vec2(1.0) / vec2(uvec2(textureSize(tex, 0).xy)));
    vec4 _e16 = fxaa_sample(uv_4, pixel_1);
    vec4 _e22 = fxaa_sample((uv_4 + (vec2(-1.0, -1.0) * pixel_1)), pixel_1);
    float _e23 = fxaa_luma(_e22);
    vec4 _e29 = fxaa_sample((uv_4 + (vec2(1.0, -1.0) * pixel_1)), pixel_1);
    float _e30 = fxaa_luma(_e29);
    vec4 _e36 = fxaa_sample((uv_4 + (vec2(-1.0, 1.0) * pixel_1)), pixel_1);
    float _e37 = fxaa_luma(_e36);
    vec4 _e39 = fxaa_sample((uv_4 + pixel_1), pixel_1);
    float _e40 = fxaa_luma(_e39);
    float _e41 = fxaa_luma(_e16);
    float low = min(_e41, min(min(_e23, _e30), min(_e37, _e40)));
    float high = max(_e41, max(max(_e23, _e30), max(_e37, _e40)));
    if (((high - low) < max(0.03125, (high * 0.125)))) {
        return _e16;
    }
    direction_3 = vec2(-((((_e23 + _e30) - _e37) - _e40)), (((_e23 + _e37) - _e30) - _e40));
    float reduction = max(((((_e23 + _e30) + _e37) + _e40) * 0.03125), 0.0078125);
    vec2 _e72 = direction_3;
    float _e74 = direction_3.x;
    float _e77 = direction_3.y;
    direction_3 = (clamp((_e72 / vec2((min(abs(_e74), abs(_e77)) + reduction))), vec2(-8.0), vec2(8.0)) * pixel_1);
    vec2 _e89 = direction_3;
    vec4 _e94 = fxaa_sample((uv_4 - (_e89 / vec2(6.0))), pixel_1);
    vec2 _e95 = direction_3;
    vec4 _e100 = fxaa_sample((uv_4 + (_e95 / vec2(6.0))), pixel_1);
    vec4 narrow = ((_e94 + _e100) * 0.5);
    vec2 _e106 = direction_3;
    vec4 _e110 = fxaa_sample((uv_4 - (_e106 * 0.5)), pixel_1);
    vec2 _e111 = direction_3;
    vec4 _e115 = fxaa_sample((uv_4 + (_e111 * 0.5)), pixel_1);
    vec4 wide = ((narrow * 0.5) + ((_e110 + _e115) * 0.25));
    float _e120 = fxaa_luma(wide);
    if (!((_e120 < low))) {
        local_5 = (_e120 > high);
    } else {
        local_5 = true;
    }
    bool _e127 = local_5;
    return (_e127 ? narrow : wide);
}

void main() {
    PBRInput input_ = PBRInput(gl_FragCoord, gl_FrontFacing, _vs2fs_location0, _vs2fs_location2, _vs2fs_location3, _vs2fs_location5, _vs2fs_location6, _vs2fs_location7, _vs2fs_location8, _vs2fs_location9);
    vec4 base = vec4(0.0);
    bool local = false;
    bool local_1 = false;
    bool local_2 = false;
    bool local_3 = false;
    bool local_4 = false;
    float _e4 = _group_0_binding_0_fs.renderParams.y;
    if ((_e4 == 7.0)) {
        vec4 _e8 = fxaa(input_.uv);
        _fs2p_location0 = _e8;
        return;
    }
    vec4 texel = texture(tex, vec2(input_.uv));
    if ((input_.material.x < -1.5)) {
        float alpha_2 = (texel.w * input_.baseColor.w);
        vec3 tint = ((input_.custom0_.w > 0.5) ? ((((vec3(texel.w) - texel.xyz) * input_.custom0_.xyz) + (texel.xyz * input_.baseColor.xyz)) * input_.baseColor.w) : ((((vec3(1.0) - texel.xyz) * input_.custom0_.xyz) + (texel.xyz * input_.baseColor.xyz)) * alpha_2));
        _fs2p_location0 = vec4(tint, alpha_2);
        return;
    }
    uint flags = uint((input_.material.w + 0.5));
    if (((flags & 8192u) != 0u)) {
        vec4 _e64 = editor_grid(input_);
        _fs2p_location0 = _e64;
        return;
    }
    base = (texel * input_.baseColor);
    if (((flags & 3u) == 1u)) {
        float _e75 = base.w;
        local = (_e75 < input_.material.z);
    } else {
        local = false;
    }
    bool _e80 = local;
    if (_e80) {
        discard;
    }
    float _e85 = _group_0_binding_0_fs.renderParams.w;
    bool front = ((input_.front != (_e85 > 0.5)) != ((flags & 1024u) != 0u));
    if ((input_.material.x >= 0.0)) {
        if (((flags & 512u) == 0u)) {
            local_1 = !(front);
        } else {
            local_1 = false;
        }
        bool _e106 = local_1;
        if (_e106) {
            discard;
        }
        if (((flags & 3u) != 2u)) {
            base.w = 1.0;
        }
    }
    float _e116 = _group_0_binding_0_fs.renderParams.y;
    if (!((_e116 == 1.0))) {
        float _e125 = _group_0_binding_0_fs.renderParams.y;
        local_2 = (_e125 == 2.0);
    } else {
        local_2 = true;
    }
    bool _e129 = local_2;
    if (!(_e129)) {
        float _e136 = _group_0_binding_0_fs.renderParams.y;
        local_3 = (_e136 == 5.0);
    } else {
        local_3 = true;
    }
    bool _e140 = local_3;
    if (_e140) {
        uint depth = uint((clamp(input_.position.z, 0.0, 1.0) * 16777215.0));
        _fs2p_location0 = vec4((vec3(float(((depth >> 16u) & 255u)), float(((depth >> 8u) & 255u)), float((depth & 255u))) / vec3(255.0)), 1.0);
        return;
    }
    float _e171 = _group_0_binding_0_fs.renderParams.y;
    if ((_e171 == 3.0)) {
        local_4 = (input_.material.y < 0.0);
    } else {
        local_4 = false;
    }
    bool _e181 = local_4;
    if (_e181) {
        _fs2p_location0 = vec4(0.0);
        return;
    }
    if (((flags & 2048u) != 0u)) {
        vec4 _e191 = _group_0_binding_0_fs.cameraPosition;
        vec3 direction_5 = ((input_.custom3_.w > 0.5) ? input_.custom3_.xyz : normalize((input_.worldPosition - _e191.xyz)));
        if (((flags & 16384u) != 0u)) {
            vec3 pole = ((direction_5.y < 0.0) ? input_.custom2_.xyz : input_.custom0_.xyz);
            vec4 _e221 = pbr_output(mix(input_.custom1_.xyz, pole, sqrt(abs(direction_5.y))), 1.0);
            _fs2p_location0 = _e221;
            return;
        }
        if (((flags & 8388608u) != 0u)) {
            vec3 _e226 = pbr_panorama(direction_5);
            vec4 _e228 = pbr_output(_e226, 1.0);
            _fs2p_location0 = _e228;
            return;
        }
        vec3 _e230 = pbr_environment(direction_5, 0.0);
        vec4 _e232 = pbr_output(_e230, 1.0);
        _fs2p_location0 = _e232;
        return;
    }
    if ((input_.material.x < 0.0)) {
        vec4 _e237 = base;
        float _e240 = base.w;
        float _e243 = base.w;
        _fs2p_location0 = vec4((_e237.xyz * _e240), _e243);
        return;
    }
    if (((flags & 256u) != 0u)) {
        float _e252 = _group_0_binding_0_fs.renderParams.y;
        if ((_e252 == 3.0)) {
            vec4 _e255 = base;
            vec4 _e257 = pbr_encode_radiance(_e255.xyz);
            _fs2p_location0 = _e257;
            return;
        }
        float _e261 = _group_0_binding_0_fs.renderParams.y;
        if ((_e261 == 6.0)) {
            _fs2p_location0 = vec4(1.0);
            return;
        }
        vec4 _e266 = base;
        vec3 _e268 = pbr_fog_color();
        vec3 _e269 = pbr_tone_map(_e268);
        float _e271 = pbr_fog_factor(input_.worldPosition);
        vec3 color_1 = mix(_e266.xyz, _e269, _e271);
        vec3 _e273 = pbr_srgb(color_1);
        float _e275 = base.w;
        float _e278 = base.w;
        _fs2p_location0 = vec4((_e273 * _e275), _e278);
        return;
    }
    _fs2p_location0 = vec4(0.0);
    return;
}
