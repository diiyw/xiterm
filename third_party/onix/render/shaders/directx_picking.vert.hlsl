struct Uniforms {
    row_major float4x4 projection;
};

struct Input {
    float3 position : LOC0;
    float4 model0_ : LOC3;
    float4 model1_ : LOC4;
    float4 model2_ : LOC5;
    float4 model3_ : LOC6;
    float4 custom3_ : LOC14;
};

struct Output {
    float4 position : SV_Position;
    nointerpolation uint pickingID : LOC0;
};

cbuffer uniforms : register(b0) { Uniforms uniforms; }

struct VertexOutput_mesh_vertex {
    nointerpolation uint pickingID : LOC0;
    float4 position : SV_Position;
};

VertexOutput_mesh_vertex mesh_vertex(Input input)
{
    Output out_ = (Output)0;

    float4x4 _e5 = uniforms.projection;
    out_.position = mul(float4(input.position, 1.0), mul(float4x4(input.model0_, input.model1_, input.model2_, input.model3_), _e5));
    out_.pickingID = asuint(input.custom3_.w);
    Output _e20 = out_;
    const Output output = _e20;
    const VertexOutput_mesh_vertex output_1 = { output.pickingID, output.position };
    return output_1;
}
