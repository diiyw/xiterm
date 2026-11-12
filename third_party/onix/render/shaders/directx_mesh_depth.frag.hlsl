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
    float2 uv : LOC2;
    float4 baseColor : LOC3;
    nointerpolation float4 material : LOC5;
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
    float3 onix_unused_0 : LOC0;
    float3 onix_unused_1 : LOC1;
    float2 uv : LOC2;
    float4 baseColor : LOC3;
    float4 onix_unused_4 : LOC4;
    nointerpolation float4 material : LOC5;
    nointerpolation float4 onix_unused_6 : LOC6;
    nointerpolation float4 onix_unused_7 : LOC7;
    nointerpolation float4 onix_unused_8 : LOC8;
    nointerpolation float4 onix_unused_9 : LOC9;
    float4 onix_unused_10 : LOC10;
    float4 position : SV_Position;
    bool front : SV_IsFrontFace;
};

uint naga_f2u32(float value) {
    return uint(clamp(value, 0.0, 4294967000.0));
}

float4 mesh_fragment(FragmentInput_mesh_fragment fragmentinput_mesh_fragment) : SV_Target0
{
    PBRInput input = { fragmentinput_mesh_fragment.position, fragmentinput_mesh_fragment.front, fragmentinput_mesh_fragment.uv, fragmentinput_mesh_fragment.baseColor, fragmentinput_mesh_fragment.material };
    bool local = (bool)0;

    uint flags = naga_f2u32((input.material.w + 0.5));
    float _e10 = pbr.renderParams.w;
    bool front_1 = ((input.front != (_e10 > 0.5)) != ((flags & 1024u) != 0u));
    if (((flags & 512u) == 0u)) {
        local = !(front_1);
    } else {
        local = false;
    }
    bool _e27 = local;
    if (_e27) {
        discard;
    }
    if (((flags & 3u) == 1u)) {
        float4 _e35 = tex.Sample(texSampler, input.uv);
        if (((_e35.w * input.baseColor.w) < input.material.z)) {
            discard;
        }
    }
    uint depth = naga_f2u32((clamp(input.position.z, 0.0, 1.0) * 1.6777215e07));
    return float4((float3(float(((depth >> 16u) & 255u)), float(((depth >> 8u) & 255u)), float((depth & 255u))) / (255.0).xxx), 1.0);
}
