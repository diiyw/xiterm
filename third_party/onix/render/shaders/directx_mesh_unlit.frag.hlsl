struct PBRUniforms {
    row_major float4x4 projection;
    float4 cameraPosition;
    float4 ambientLight;
    float4 lightDirection;
    float4 directionalLight;
    float4 lightPosition[16];
    float4 lightDirections[16];
    float4 lightColors[16];
    float4 lightParams[16];
    float4 lightMeta;
    float4 environment;
    float4 renderParams;
    float4 cameraForward;
    float4 lightRight[16];
    float4 lightUp[16];
    row_major float4x4 shadowMatrices[16];
    float4 shadowRects[16];
    float4 shadowParams[16];
    float4 lightShadows[16];
    row_major float4x4 occlusionInverseProjection;
};

struct PBRInput {
    float4 position : SV_Position;
    bool front : SV_IsFrontFace;
    float3 worldPosition : LOC0;
    float2 uv : LOC2;
    float4 baseColor : LOC3;
    nointerpolation float4 material : LOC5;
    nointerpolation float4 custom0_ : LOC6;
    nointerpolation float4 custom1_ : LOC7;
    nointerpolation float4 custom2_ : LOC8;
    nointerpolation float4 custom3_ : LOC9;
};

struct PBRSurface {
    float3 n;
    int _pad1_0;
    float3 coatNormal;
    int _pad2_0;
    float3 tangent;
    int _pad3_0;
    float3 bitangent;
    int _pad4_0;
    float3 base;
    float metallic;
    float roughness;
    int _pad7_0;
    int _pad7_1;
    int _pad7_2;
    float3 f0_;
    float f90_;
    float diffuseF0_;
    float diffuseF90_;
    float coat;
    float coatRoughness;
    float coatFresnel;
    int _pad14_0;
    int _pad14_1;
    int _pad14_2;
    float3 sheen;
    float sheenRoughness;
    float sheenAlbedo;
    float sheenScale;
    float sheenG1_;
    float anisotropy;
};

static const float PI = 3.1415927;
static const bool SHADOW_FLIP_Y = true;

cbuffer pbr : register(b0) { PBRUniforms pbr; }
Texture2D<float4> tex : register(t0);
SamplerState texSampler : register(s0);
Texture2D<float4> normalTex : register(t1);
SamplerState normalSampler : register(s1);
Texture2D<float4> metallicRoughnessTex : register(t2);
SamplerState metallicRoughnessSampler : register(s2);
Texture2D<float4> occlusionTex : register(t3);
SamplerState occlusionSampler : register(s3);
Texture2D<float4> emissiveTex : register(t4);
SamplerState emissiveSampler : register(s4);
Texture2D<float4> specularTex : register(t5);
SamplerState specularSampler : register(s5);
Texture2D<float4> specularColorTex : register(t6);
SamplerState specularColorSampler : register(s6);
Texture2D<float4> environmentTex : register(t7);
SamplerState environmentSampler : register(s7);
Texture2D<float4> shadowTex : register(t8);
SamplerState shadowSampler : register(s8);
Texture2D<float4> screenDepthTex : register(t9);
SamplerState screenDepthSampler : register(s9);
Texture2D<float4> clearcoatTex : register(t10);
SamplerState clearcoatSampler : register(s10);
Texture2D<float4> clearcoatRoughnessTex : register(t11);
SamplerState clearcoatRoughnessSampler : register(s11);
Texture2D<float4> clearcoatNormalTex : register(t12);
SamplerState clearcoatNormalSampler : register(s12);
Texture2D<float4> sheenColorTex : register(t13);
SamplerState sheenColorSampler : register(s13);
Texture2D<float4> sheenRoughnessTex : register(t14);
SamplerState sheenRoughnessSampler : register(s14);
Texture2D<float4> anisotropyTex : register(t15);
SamplerState anisotropySampler : register(s15);

struct FragmentInput_mesh_fragment {
    float3 worldPosition : LOC0;
    float3 onix_unused_1 : LOC1;
    float2 uv_3 : LOC2;
    float4 baseColor : LOC3;
    float4 onix_unused_4 : LOC4;
    nointerpolation float4 material : LOC5;
    nointerpolation float4 custom0_ : LOC6;
    nointerpolation float4 custom1_ : LOC7;
    nointerpolation float4 custom2_ : LOC8;
    nointerpolation float4 custom3_ : LOC9;
    float4 onix_unused_10 : LOC10;
    float4 position_1 : SV_Position;
    bool front : SV_IsFrontFace;
};

float3 pbr_srgb(float3 linear_)
{
    float3 c = max(linear_, (0.0).xxx);
    return ((c <= (0.0031308).xxx) ? (12.92 * c) : ((1.055 * pow(c, (0.41666666).xxx)) - (0.055).xxx));
}

float3 pbr_tone_map(float3 linear_1)
{
    float _e4 = pbr.renderParams.x;
    float _e8 = pbr.renderParams.x;
    float exposure = ((_e8 <= 0.0) ? 1.0 : _e4);
    float3 hdr = clamp((linear_1 * exposure), (0.0).xxx, (1e20).xxx);
    float3 reinhard = (hdr / (hdr + (1.0).xxx));
    float _e27 = pbr.lightShadows[7].w;
    if ((_e27 < 0.5)) {
        return reinhard;
    }
    float3x3 inputMatrix = float3x3(float3(0.59719, 0.076, 0.0284), float3(0.35458, 0.90834, 0.13383), float3(0.04823, 0.01566, 0.83777));
    float3x3 outputMatrix = float3x3(float3(1.60475, -0.10208, -0.00327), float3(-0.53108, 1.10813, -0.07276), float3(-0.07367, -0.00605, 1.07602));
    float3 aces = mul((hdr / (0.6).xxx), inputMatrix);
    float3 reciprocal = ((1.0).xxx / (aces + (1.0).xxx));
    float3 normalized = (aces * reciprocal);
    float3 squared = (normalized * normalized);
    float3 crossTerm = (normalized * reciprocal);
    float3 fit = (((squared + (0.0245786 * crossTerm)) - ((9.0537e-05 * reciprocal) * reciprocal)) / (((0.983729 * squared) + (0.432951 * crossTerm)) + ((0.238081 * reciprocal) * reciprocal)));
    return clamp(mul(fit, outputMatrix), (0.0).xxx, (1.0).xxx);
}

float4 pbr_encode_radiance(float3 linear_2)
{
    float3 c_1 = max(linear_2, (0.0).xxx);
    float exponent = clamp(ceil(log2(max(max(max(c_1.x, c_1.y), c_1.z), 1e-30))), -120.0, 120.0);
    return float4((c_1 * exp2(-(exponent))), ((exponent + 128.0) / 255.0));
}

float4 pbr_output(float3 linear_3, float alpha)
{
    float _e5 = pbr.renderParams.y;
    if ((_e5 == 3.0)) {
        const float4 _e8 = pbr_encode_radiance(linear_3);
        return _e8;
    }
    const float3 _e9 = pbr_tone_map(linear_3);
    const float3 _e10 = pbr_srgb(_e9);
    return float4((_e10 * alpha), alpha);
}

float3 pbr_fog_color()
{
    float _e4 = pbr.lightShadows[0].w;
    float _e9 = pbr.lightShadows[1].w;
    float _e14 = pbr.lightShadows[2].w;
    return float3(_e4, _e9, _e14);
}

float pbr_fog_factor(float3 position)
{
    float mode = pbr.lightShadows[6].w;
    if ((mode < 0.5)) {
        return 0.0;
    }
    float4 _e11 = pbr.cameraPosition;
    float3 delta = (position - _e11.xyz);
    float4 _e17 = pbr.cameraForward;
    float _e25 = pbr.cameraPosition.w;
    float distance_ = ((_e25 < 0.0) ? max(dot(delta, _e17.xyz), 0.0) : length(delta));
    float start = pbr.lightShadows[3].w;
    if ((mode < 1.5)) {
        float _e41 = pbr.lightShadows[4].w;
        return clamp(((distance_ - start) / max((_e41 - start), 1e-06)), 0.0, 1.0);
    }
    float _e56 = pbr.lightShadows[5].w;
    float opticalDepth = (max((distance_ - start), 0.0) * _e56);
    return (1.0 - exp((-(opticalDepth) * opticalDepth)));
}

float4 pbr_decode_radiance(float4 encoded)
{
    return float4((encoded.xyz * exp2((round((encoded.w * 255.0)) - 128.0))), ((encoded.w > 0.0) ? 1.0 : 0.0));
}

float4 pbr_atlas(float2 uv, float level)
{
    float _e5 = pbr.environment.z;
    float _e9 = pbr.environment.w;
    float2 inset = float2(_e5, _e9);
    float2 local_6 = ((clamp(uv, (0.0).xx, (1.0).xx) * ((1.0).xx - (2.0 * inset))) + inset);
    float4 _e32 = environmentTex.SampleLevel(environmentSampler, float2(local_6.x, ((local_6.y + level) / 8.0)), 0.0);
    return _e32;
}

int naga_div(int lhs, int rhs) {
    return lhs / (((lhs == int(-2147483647 - 1) & rhs == -1) | (rhs == 0)) ? 1 : rhs);
}

int2 naga_f2i32(float2 value) {
    return int2(clamp(value, -2147483600.0, 2147483500.0));
}

int naga_f2i32(float value) {
    return int(clamp(value, -2147483600.0, 2147483500.0));
}

uint2 NagaDimensions2D(Texture2D<float4> tex)
{
    uint4 ret;
    tex.GetDimensions(0, ret.x, ret.y, ret.z);
    return ret.xy;
}

float3 pbr_rgbe_atlas(float2 uv_1, float level_1)
{
    int2 dims = int2(NagaDimensions2D(environmentTex));
    float _e8 = pbr.environment.z;
    float _e12 = pbr.environment.w;
    float2 inset_1 = float2(_e8, _e12);
    float2 local_7 = ((clamp(uv_1, (0.0).xx, (1.0).xx) * ((1.0).xx - (2.0 * inset_1))) + inset_1);
    float2 coord = ((float2(local_7.x, ((local_7.y + level_1) / 8.0)) * float2(dims)) - (0.5).xx);
    int2 base_1 = naga_f2i32(floor(coord));
    float2 weight = frac(coord);
    int tileHeight = naga_div(dims.y, int(8));
    int2 first = int2(int(0), asint(asuint(naga_f2i32(level_1)) * asuint(tileHeight)));
    int2 last = int2(asint(asuint(dims.x) - asuint(int(1))), asint(asuint(asint(asuint(first.y) + asuint(tileHeight))) - asuint(int(1))));
    float4 _e58 = environmentTex.Load(int3(clamp(base_1, first, last), int(0)));
    const float4 _e59 = pbr_decode_radiance(_e58);
    float3 a = _e59.xyz;
    float4 _e68 = environmentTex.Load(int3(clamp(asint(asuint(base_1) + asuint(int2(int(1), int(0)))), first, last), int(0)));
    const float4 _e69 = pbr_decode_radiance(_e68);
    float3 b = _e69.xyz;
    float4 _e78 = environmentTex.Load(int3(clamp(asint(asuint(base_1) + asuint(int2(int(0), int(1)))), first, last), int(0)));
    const float4 _e79 = pbr_decode_radiance(_e78);
    float3 c_2 = _e79.xyz;
    float4 _e87 = environmentTex.Load(int3(clamp(asint(asuint(base_1) + asuint((int(1)).xx)), first, last), int(0)));
    const float4 _e88 = pbr_decode_radiance(_e87);
    float3 d = _e88.xyz;
    return lerp(lerp(a, b, weight.x), lerp(c_2, d, weight.x), weight.y);
}

float2 pbr_environment_uv(float3 direction)
{
    float _e11 = pbr.environment.y;
    return float2(frac((((atan2(direction.z, direction.x) / 6.2831855) + 0.5) + (_e11 / 6.2831855))), (acos(clamp(direction.y, -1.0, 1.0)) / PI));
}

int naga_mod(int lhs, int rhs) {
    int divisor = ((lhs == int(-2147483647 - 1) & rhs == -1) | (rhs == 0)) ? 1 : rhs;
    return lhs - (lhs / divisor) * divisor;
}

float3 pbr_panorama(float3 direction_1)
{
    int2 dims_1 = int2(NagaDimensions2D(tex));
    const float2 _e4 = pbr_environment_uv(direction_1);
    float2 coord_1 = ((_e4 * float2(dims_1)) - (0.5).xx);
    int2 origin = naga_f2i32(floor(coord_1));
    float2 weight_1 = frac(coord_1);
    int x0_ = naga_mod(asint(asuint(origin.x) + asuint(dims_1.x)), dims_1.x);
    int x1_ = naga_mod(asint(asuint(x0_) + asuint(int(1))), dims_1.x);
    int y0_ = clamp(origin.y, int(0), asint(asuint(dims_1.y) - asuint(int(1))));
    int y1_ = clamp(asint(asuint(origin.y) + asuint(int(1))), int(0), asint(asuint(dims_1.y) - asuint(int(1))));
    float4 _e39 = tex.Load(int3(int2(x0_, y0_), int(0)));
    const float4 _e40 = pbr_decode_radiance(_e39);
    float3 a_1 = _e40.xyz;
    float4 _e45 = tex.Load(int3(int2(x1_, y0_), int(0)));
    const float4 _e46 = pbr_decode_radiance(_e45);
    float3 b_1 = _e46.xyz;
    float4 _e51 = tex.Load(int3(int2(x0_, y1_), int(0)));
    const float4 _e52 = pbr_decode_radiance(_e51);
    float3 c_3 = _e52.xyz;
    float4 _e57 = tex.Load(int3(int2(x1_, y1_), int(0)));
    const float4 _e58 = pbr_decode_radiance(_e57);
    float3 d_1 = _e58.xyz;
    return lerp(lerp(a_1, b_1, weight_1.x), lerp(c_3, d_1, weight_1.x), weight_1.y);
}

float3 pbr_environment(float3 direction_2, float level_2)
{
    const float2 _e2 = pbr_environment_uv(direction_2);
    float _e7 = pbr.lightShadows[8].w;
    if ((_e7 > 0.5)) {
        float lower = floor(level_2);
        float upper = ceil(level_2);
        const float3 _e12 = pbr_rgbe_atlas(_e2, lower);
        if ((lower == upper)) {
            return _e12;
        }
        const float3 _e14 = pbr_rgbe_atlas(_e2, upper);
        return lerp(_e12, _e14, frac(level_2));
    }
    const float4 _e18 = pbr_atlas(_e2, floor(level_2));
    const float4 _e20 = pbr_atlas(_e2, ceil(level_2));
    return (lerp((_e18.xyz * _e18.w), (_e20.xyz * _e20.w), frac(level_2)) * 16.0);
}

float grid_coverage(float2 point_, float step_, float pixels)
{
    float2 uv_4 = (point_ / (step_).xx);
    float2 _e5 = fwidth(uv_4);
    float2 footprint = max(_e5, (1e-06).xx);
    float2 distance_1 = (abs((frac((uv_4 - (0.5).xx)) - (0.5).xx)) / footprint);
    float2 lines = ((1.0).xx - smoothstep((((pixels * 0.5) - 0.5)).xx, (((pixels * 0.5) + 0.5)).xx, distance_1));
    float2 visibility = ((1.0).xx - smoothstep((0.25).xx, (0.75).xx, footprint));
    return max((lines.x * visibility.x), (lines.y * visibility.y));
}

uint naga_mod(uint lhs, uint rhs) {
    return lhs % (rhs == 0u ? 1u : rhs);
}

uint naga_f2u32(float value) {
    return uint(clamp(value, 0.0, 4294967000.0));
}

float4 editor_grid(PBRInput input_1)
{
    float step_1 = input_1.custom0_.x;
    float blend = input_1.custom0_.y;
    float pixels_1 = input_1.custom0_.z;
    uint axis = naga_f2u32(input_1.custom0_.w);
    float2 point_1 = float2(input_1.worldPosition[min(uint(naga_mod((axis + 1u), 3u)), 2u)], input_1.worldPosition[min(uint(naga_mod((axis + 2u), 3u)), 2u)]);
    const float _e23 = grid_coverage(point_1, step_1, pixels_1);
    const float _e26 = grid_coverage(point_1, (step_1 * 10.0), pixels_1);
    const float _e29 = grid_coverage(point_1, (step_1 * 100.0), pixels_1);
    float coverage = lerp(max((_e23 * 0.24), (_e26 * 0.48)), max((_e26 * 0.24), (_e29 * 0.48)), blend);
    float distance_2 = length((point_1 - input_1.custom1_.xy));
    float fade = (1.0 - smoothstep((input_1.custom1_.z * 0.4), input_1.custom1_.z, distance_2));
    float4 _e56 = pbr.cameraPosition;
    float3 delta_1 = (_e56.xyz - input_1.worldPosition);
    float3 direction_4 = ((input_1.custom2_.w > 0.5) ? input_1.custom2_.xyz : (delta_1 / (max(length(delta_1), 1e-06)).xxx));
    float angle = smoothstep(0.015, 0.15, abs(direction_4[min(uint(axis), 2u)]));
    float alpha_1 = (((coverage * fade) * angle) * input_1.baseColor.w);
    return float4((input_1.baseColor.xyz * alpha_1), alpha_1);
}

float4 fxaa_sample(float2 uv_2, float2 pixel)
{
    float4 _e13 = tex.SampleLevel(texSampler, clamp(uv_2, (pixel * 0.5), ((1.0).xx - (pixel * 0.5))), 0.0);
    return _e13;
}

float fxaa_luma(float4 color)
{
    return (dot(color.xyz, float3(0.299, 0.587, 0.114)) + (color.w * 0.25));
}

uint2 NagaMipDimensions2D(Texture2D<float4> tex, uint mip_level)
{
    uint4 ret;
    tex.GetDimensions(mip_level, ret.x, ret.y, ret.z);
    return ret.xy;
}

float4 fxaa(float2 uvIn)
{
    float2 direction_3 = (float2)0;
    bool local_5 = (bool)0;

    float2 uv_5 = float2(uvIn.x, (SHADOW_FLIP_Y ? (1.0 - uvIn.y) : uvIn.y));
    float2 pixel_1 = ((1.0).xx / float2(NagaMipDimensions2D(tex, int(0))));
    const float4 _e16 = fxaa_sample(uv_5, pixel_1);
    const float4 _e22 = fxaa_sample((uv_5 + (float2(-1.0, -1.0) * pixel_1)), pixel_1);
    const float _e23 = fxaa_luma(_e22);
    const float4 _e29 = fxaa_sample((uv_5 + (float2(1.0, -1.0) * pixel_1)), pixel_1);
    const float _e30 = fxaa_luma(_e29);
    const float4 _e36 = fxaa_sample((uv_5 + (float2(-1.0, 1.0) * pixel_1)), pixel_1);
    const float _e37 = fxaa_luma(_e36);
    const float4 _e39 = fxaa_sample((uv_5 + pixel_1), pixel_1);
    const float _e40 = fxaa_luma(_e39);
    const float _e41 = fxaa_luma(_e16);
    float low = min(_e41, min(min(_e23, _e30), min(_e37, _e40)));
    float high = max(_e41, max(max(_e23, _e30), max(_e37, _e40)));
    if (((high - low) < max(0.03125, (high * 0.125)))) {
        return _e16;
    }
    direction_3 = float2(-((((_e23 + _e30) - _e37) - _e40)), (((_e23 + _e37) - _e30) - _e40));
    float reduction = max(((((_e23 + _e30) + _e37) + _e40) * 0.03125), 0.0078125);
    float2 _e72 = direction_3;
    float _e74 = direction_3.x;
    float _e77 = direction_3.y;
    direction_3 = (clamp((_e72 / ((min(abs(_e74), abs(_e77)) + reduction)).xx), (-8.0).xx, (8.0).xx) * pixel_1);
    float2 _e89 = direction_3;
    const float4 _e94 = fxaa_sample((uv_5 - (_e89 / (6.0).xx)), pixel_1);
    float2 _e95 = direction_3;
    const float4 _e100 = fxaa_sample((uv_5 + (_e95 / (6.0).xx)), pixel_1);
    float4 narrow = ((_e94 + _e100) * 0.5);
    float2 _e106 = direction_3;
    const float4 _e110 = fxaa_sample((uv_5 - (_e106 * 0.5)), pixel_1);
    float2 _e111 = direction_3;
    const float4 _e115 = fxaa_sample((uv_5 + (_e111 * 0.5)), pixel_1);
    float4 wide = ((narrow * 0.5) + ((_e110 + _e115) * 0.25));
    const float _e120 = fxaa_luma(wide);
    if (!((_e120 < low))) {
        local_5 = (_e120 > high);
    } else {
        local_5 = true;
    }
    bool _e127 = local_5;
    return (_e127 ? narrow : wide);
}

float4 mesh_fragment(FragmentInput_mesh_fragment fragmentinput_mesh_fragment) : SV_Target0
{
    PBRInput input = { fragmentinput_mesh_fragment.position_1, fragmentinput_mesh_fragment.front, fragmentinput_mesh_fragment.worldPosition, fragmentinput_mesh_fragment.uv_3, fragmentinput_mesh_fragment.baseColor, fragmentinput_mesh_fragment.material, fragmentinput_mesh_fragment.custom0_, fragmentinput_mesh_fragment.custom1_, fragmentinput_mesh_fragment.custom2_, fragmentinput_mesh_fragment.custom3_ };
    float4 base = (float4)0;
    bool local = (bool)0;
    bool local_1 = (bool)0;
    bool local_2 = (bool)0;
    bool local_3 = (bool)0;
    bool local_4 = (bool)0;

    float _e4 = pbr.renderParams.y;
    if ((_e4 == 7.0)) {
        const float4 _e8 = fxaa(input.uv);
        return _e8;
    }
    float4 texel = tex.Sample(texSampler, input.uv);
    if ((input.material.x < -1.5)) {
        float alpha_2 = (texel.w * input.baseColor.w);
        float3 tint = ((input.custom0_.w > 0.5) ? (((((texel.w).xxx - texel.xyz) * input.custom0_.xyz) + (texel.xyz * input.baseColor.xyz)) * input.baseColor.w) : (((((1.0).xxx - texel.xyz) * input.custom0_.xyz) + (texel.xyz * input.baseColor.xyz)) * alpha_2));
        return float4(tint, alpha_2);
    }
    uint flags = naga_f2u32((input.material.w + 0.5));
    if (((flags & 8192u) != 0u)) {
        const float4 _e64 = editor_grid(input);
        return _e64;
    }
    base = (texel * input.baseColor);
    if (((flags & 3u) == 1u)) {
        float _e75 = base.w;
        local = (_e75 < input.material.z);
    } else {
        local = false;
    }
    bool _e80 = local;
    if (_e80) {
        discard;
    }
    float _e85 = pbr.renderParams.w;
    bool front_1 = ((input.front != (_e85 > 0.5)) != ((flags & 1024u) != 0u));
    if ((input.material.x >= 0.0)) {
        if (((flags & 512u) == 0u)) {
            local_1 = !(front_1);
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
    float _e116 = pbr.renderParams.y;
    if (!((_e116 == 1.0))) {
        float _e125 = pbr.renderParams.y;
        local_2 = (_e125 == 2.0);
    } else {
        local_2 = true;
    }
    bool _e129 = local_2;
    if (!(_e129)) {
        float _e136 = pbr.renderParams.y;
        local_3 = (_e136 == 5.0);
    } else {
        local_3 = true;
    }
    bool _e140 = local_3;
    if (_e140) {
        uint depth = naga_f2u32((clamp(input.position.z, 0.0, 1.0) * 1.6777215e07));
        return float4((float3(float(((depth >> 16u) & 255u)), float(((depth >> 8u) & 255u)), float((depth & 255u))) / (255.0).xxx), 1.0);
    }
    float _e171 = pbr.renderParams.y;
    if ((_e171 == 3.0)) {
        local_4 = (input.material.y < 0.0);
    } else {
        local_4 = false;
    }
    bool _e181 = local_4;
    if (_e181) {
        return (0.0).xxxx;
    }
    if (((flags & 2048u) != 0u)) {
        float4 _e191 = pbr.cameraPosition;
        float3 direction_5 = ((input.custom3_.w > 0.5) ? input.custom3_.xyz : normalize((input.worldPosition - _e191.xyz)));
        if (((flags & 16384u) != 0u)) {
            float3 pole = ((direction_5.y < 0.0) ? input.custom2_.xyz : input.custom0_.xyz);
            const float4 _e221 = pbr_output(lerp(input.custom1_.xyz, pole, sqrt(abs(direction_5.y))), 1.0);
            return _e221;
        }
        if (((flags & 8388608u) != 0u)) {
            const float3 _e226 = pbr_panorama(direction_5);
            const float4 _e228 = pbr_output(_e226, 1.0);
            return _e228;
        }
        const float3 _e230 = pbr_environment(direction_5, 0.0);
        const float4 _e232 = pbr_output(_e230, 1.0);
        return _e232;
    }
    if ((input.material.x < 0.0)) {
        float4 _e237 = base;
        float _e240 = base.w;
        float _e243 = base.w;
        return float4((_e237.xyz * _e240), _e243);
    }
    if (((flags & 256u) != 0u)) {
        float _e252 = pbr.renderParams.y;
        if ((_e252 == 3.0)) {
            float4 _e255 = base;
            const float4 _e257 = pbr_encode_radiance(_e255.xyz);
            return _e257;
        }
        float _e261 = pbr.renderParams.y;
        if ((_e261 == 6.0)) {
            return (1.0).xxxx;
        }
        float4 _e266 = base;
        const float3 _e268 = pbr_fog_color();
        const float3 _e269 = pbr_tone_map(_e268);
        const float _e271 = pbr_fog_factor(input.worldPosition);
        float3 color_1 = lerp(_e266.xyz, _e269, _e271);
        const float3 _e273 = pbr_srgb(color_1);
        float _e275 = base.w;
        float _e278 = base.w;
        return float4((_e273 * _e275), _e278);
    }
    return (0.0).xxxx;
}
