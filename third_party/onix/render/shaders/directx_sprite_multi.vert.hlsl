struct Uniforms {
    row_major float4x4 projection;
    float4 coverage;
    float4 padding[3];
};

struct Input {
    float3 position : LOC0;
    float2 textureCoord : LOC1;
    float3 model0_ : LOC2;
    float3 model1_ : LOC3;
    float3 texture0_ : LOC4;
    float3 texture1_ : LOC5;
    float4 color : LOC6;
    float textureSlot : LOC11;
    float4 clipRect : LOC9;
    float4 clipParams : LOC10;
};

struct Output {
    float4 position : SV_Position;
    float2 uv : LOC0;
    float4 color : LOC1;
    nointerpolation uint slot : LOC2;
    float2 position2D : LOC3;
    nointerpolation float4 clipRect : LOC4;
    nointerpolation float4 clipParams : LOC5;
};

cbuffer uniforms : register(b0) { Uniforms uniforms; }
Texture2D<float4> tex : register(t0);
SamplerState texSampler : register(s0);
Texture2D<float4> tex1_ : register(t1);
SamplerState sampler1_ : register(s1);
Texture2D<float4> tex2_ : register(t2);
SamplerState sampler2_ : register(s2);
Texture2D<float4> tex3_ : register(t3);
SamplerState sampler3_ : register(s3);

struct VertexOutput_sprite_vertex {
    float2 uv : LOC0;
    float4 color : LOC1;
    nointerpolation uint slot : LOC2;
    float2 position2D : LOC3;
    nointerpolation float4 clipRect : LOC4;
    nointerpolation float4 clipParams : LOC5;
    float4 position : SV_Position;
};

uint naga_f2u32(float value) {
    return uint(clamp(value, 0.0, 4294967000.0));
}

VertexOutput_sprite_vertex sprite_vertex(Input input)
{
    Output out_ = (Output)0;

    float3 p = float3(input.position.xy, 1.0);
    out_.position2D = float2(dot(input.model0_, p), dot(input.model1_, p));
    float4x4 _e15 = uniforms.projection;
    float2 _e17 = out_.position2D;
    out_.position = mul(float4(_e17, 0.0, 1.0), _e15);
    out_.clipRect = input.clipRect;
    out_.clipParams = input.clipParams;
    float3 uv_1 = float3(input.textureCoord, 1.0);
    out_.uv = float2(dot(input.texture0_, uv_1), dot(input.texture1_, uv_1));
    out_.color = input.color;
    out_.slot = naga_f2u32(input.textureSlot);
    Output _e40 = out_;
    const Output output = _e40;
    const VertexOutput_sprite_vertex output_1 = { output.uv, output.color, output.slot, output.position2D, output.clipRect, output.clipParams, output.position };
    return output_1;
}
