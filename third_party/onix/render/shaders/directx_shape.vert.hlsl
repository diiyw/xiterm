struct Uniforms {
    row_major float4x4 projection;
    float4 padding[4];
};

struct Input {
    float3 position : LOC0;
    float2 localCoord : LOC1;
    float3 model0_ : LOC2;
    float3 model1_ : LOC3;
    float3 local0_ : LOC4;
    float3 local1_ : LOC5;
    float4 fill : LOC6;
    float4 border : LOC7;
    float4 params : LOC8;
    nointerpolation uint kind : LOC9;
};

struct Output {
    float4 position : SV_Position;
    float2 local : LOC0;
    nointerpolation float4 fill : LOC1;
    nointerpolation float4 border : LOC2;
    nointerpolation float4 params : LOC3;
    nointerpolation uint kind : LOC4;
};

cbuffer uniforms : register(b0) { Uniforms uniforms; }

struct VertexOutput_shape_vertex {
    float2 local : LOC0;
    nointerpolation float4 fill : LOC1;
    nointerpolation float4 border : LOC2;
    nointerpolation float4 params : LOC3;
    nointerpolation uint kind : LOC4;
    float4 position : SV_Position;
};

VertexOutput_shape_vertex shape_vertex(Input input)
{
    Output out_ = (Output)0;

    float3 p = float3(input.position.xy, 1.0);
    float3 uv = float3(input.localCoord, 1.0);
    float4x4 _e12 = uniforms.projection;
    out_.position = mul(float4(dot(input.model0_, p), dot(input.model1_, p), 0.0, 1.0), _e12);
    out_.local = (float2(dot(input.local0_, uv), dot(input.local1_, uv)) * input.params.xy);
    out_.fill = input.fill;
    out_.border = input.border;
    out_.params = input.params;
    out_.kind = input.kind;
    Output _e38 = out_;
    const Output output = _e38;
    const VertexOutput_shape_vertex output_1 = { output.local, output.fill, output.border, output.params, output.kind, output.position };
    return output_1;
}
