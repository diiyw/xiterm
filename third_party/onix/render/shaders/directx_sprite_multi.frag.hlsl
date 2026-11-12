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

struct FragmentInput_sprite_fragment {
    float2 uv : LOC0;
    float4 color_1 : LOC1;
    nointerpolation uint slot : LOC2;
    float2 position2D : LOC3;
    nointerpolation float4 clipRect : LOC4;
    nointerpolation float4 clipParams : LOC5;
    float4 position : SV_Position;
};

float4 sprite_fragment(FragmentInput_sprite_fragment fragmentinput_sprite_fragment) : SV_Target0
{
    Output input = { fragmentinput_sprite_fragment.position, fragmentinput_sprite_fragment.uv, fragmentinput_sprite_fragment.color_1, fragmentinput_sprite_fragment.slot, fragmentinput_sprite_fragment.position2D, fragmentinput_sprite_fragment.clipRect, fragmentinput_sprite_fragment.clipParams };
    float4 sampled = (float4)0;
    float4 color = (float4)0;

    float2 dx = ddx(input.uv);
    float2 dy = ddy(input.uv);
    switch(input.slot) {
        case 1u: {
            float4 _e10 = tex1_.SampleGrad(sampler1_, input.uv, dx, dy);
            sampled = _e10;
            break;
        }
        case 2u: {
            float4 _e14 = tex2_.SampleGrad(sampler2_, input.uv, dx, dy);
            sampled = _e14;
            break;
        }
        case 3u: {
            float4 _e18 = tex3_.SampleGrad(sampler3_, input.uv, dx, dy);
            sampled = _e18;
            break;
        }
        default: {
            float4 _e22 = tex.SampleGrad(texSampler, input.uv, dx, dy);
            sampled = _e22;
            break;
        }
    }
    float4 _e23 = sampled;
    float _e25 = sampled.x;
    float _e34 = uniforms.coverage[input.slot];
    float4 texel = ((_e34 > 0.5) ? float4(1.0, 1.0, 1.0, _e25) : _e23);
    color = (texel * input.color);
    float2 halfSize = ((input.clipRect.zw - input.clipRect.xy) * 0.5);
    float radius = input.clipParams.x;
    float2 q = ((abs((input.position2D - (input.clipRect.xy + halfSize))) - halfSize) + (radius).xx);
    float distance_ = ((length(max(q, (0.0).xx)) + min(max(q.x, q.y), 0.0)) - radius);
    float _e70 = fwidth(distance_);
    float clipCoverage = clamp((0.5 - (distance_ / max(_e70, 0.0001))), 0.0, 1.0);
    float _e80 = color.w;
    color.w = (_e80 * ((input.clipParams.y > 0.5) ? clipCoverage : 1.0));
    float4 _e88 = color;
    float _e91 = color.w;
    float _e94 = color.w;
    return float4((_e88.xyz * _e91), _e94);
}
