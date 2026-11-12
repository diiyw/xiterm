struct Uniforms {
    row_major float4x4 projection;
    float4 padding[4];
};

struct Input {
    float2 textureCoord : LOC0;
    float4 color : LOC1;
    nointerpolation float4 custom0_ : LOC2;
    nointerpolation float4 custom1_ : LOC3;
    nointerpolation float4 custom2_ : LOC4;
    nointerpolation float4 custom3_ : LOC5;
};

cbuffer uniforms : register(b0) { Uniforms uniforms; }
Texture2D<float4> tex : register(t0);
SamplerState texSampler : register(s0);

struct FragmentInput_sprite_fragment {
    float2 textureCoord : LOC0;
    float4 color : LOC1;
    nointerpolation float4 custom0_ : LOC2;
    nointerpolation float4 custom1_ : LOC3;
    nointerpolation float4 custom2_ : LOC4;
    nointerpolation float4 custom3_ : LOC5;
};

float4 sprite_fragment(FragmentInput_sprite_fragment fragmentinput_sprite_fragment) : SV_Target0
{
    Input input = { fragmentinput_sprite_fragment.textureCoord, fragmentinput_sprite_fragment.color, fragmentinput_sprite_fragment.custom0_, fragmentinput_sprite_fragment.custom1_, fragmentinput_sprite_fragment.custom2_, fragmentinput_sprite_fragment.custom3_ };
    float4 sampled = tex.Sample(texSampler, input.textureCoord);
    float _e14 = uniforms.padding[0].x;
    float4 texel = ((_e14 > 0.5) ? float4(1.0, 1.0, 1.0, sampled.x) : sampled);
    float4 color_1 = (texel * input.color);
    return float4((color_1.xyz * color_1.w), color_1.w);
}
