struct Uniforms {
    row_major float4x4 projection;
    float4 padding[4];
};

struct Input {
    float3 position : LOC0;
    float2 textureCoord : LOC1;
    float3 model0_ : LOC2;
    float3 model1_ : LOC3;
    float3 texture0_ : LOC4;
    float3 texture1_ : LOC5;
    float4 color : LOC6;
    float4 custom0_ : LOC7;
    float4 custom1_ : LOC8;
    float4 custom2_ : LOC9;
    float4 custom3_ : LOC10;
};

struct Output {
    float4 position : SV_Position;
    float2 textureCoord : LOC0;
    float4 color : LOC1;
    nointerpolation float4 custom0_ : LOC2;
    nointerpolation float4 custom1_ : LOC3;
    nointerpolation float4 custom2_ : LOC4;
    nointerpolation float4 custom3_ : LOC5;
};

cbuffer uniforms : register(b0) { Uniforms uniforms; }

struct VertexOutput_sprite_vertex {
    float2 textureCoord : LOC0;
    float4 color : LOC1;
    nointerpolation float4 custom0_ : LOC2;
    nointerpolation float4 custom1_ : LOC3;
    nointerpolation float4 custom2_ : LOC4;
    nointerpolation float4 custom3_ : LOC5;
    float4 position : SV_Position;
};

VertexOutput_sprite_vertex sprite_vertex(Input input)
{
    Output out_ = (Output)0;

    float3 p = float3(input.position.xy, 1.0);
    float4x4 _e9 = uniforms.projection;
    out_.position = mul(float4(dot(input.model0_, p), dot(input.model1_, p), 0.0, 1.0), _e9);
    float3 uv = float3(input.textureCoord, 1.0);
    out_.textureCoord = float2(dot(input.texture0_, uv), dot(input.texture1_, uv));
    out_.color = input.color;
    out_.custom0_ = input.custom0_;
    out_.custom1_ = input.custom1_;
    out_.custom2_ = input.custom2_;
    out_.custom3_ = input.custom3_;
    Output _e37 = out_;
    const Output output = _e37;
    const VertexOutput_sprite_vertex output_1 = { output.textureCoord, output.color, output.custom0_, output.custom1_, output.custom2_, output.custom3_, output.position };
    return output_1;
}
