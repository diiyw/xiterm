struct Uniforms {
    projection: mat4x4<f32>,
    coverage: vec4<f32>,
    padding: array<vec4<f32>, 3>,
};
@group(0) @binding(0) var<uniform> uniforms: Uniforms;
@group(0) @binding(1) var tex: texture_2d<f32>;
@group(0) @binding(2) var texSampler: sampler;
@group(1) @binding(1) var tex1: texture_2d<f32>;
@group(1) @binding(2) var sampler1: sampler;
@group(2) @binding(1) var tex2: texture_2d<f32>;
@group(2) @binding(2) var sampler2: sampler;
@group(3) @binding(1) var tex3: texture_2d<f32>;
@group(3) @binding(2) var sampler3: sampler;

struct Input {
    @location(0) position: vec3<f32>, @location(1) textureCoord: vec2<f32>,
    @location(2) model0: vec3<f32>, @location(3) model1: vec3<f32>,
    @location(4) texture0: vec3<f32>, @location(5) texture1: vec3<f32>,
    @location(6) color: vec4<f32>, @location(11) textureSlot: f32,
    @location(9) clipRect: vec4<f32>, @location(10) clipParams: vec4<f32>,
};
struct Output {
    @builtin(position) position: vec4<f32>,
    @location(0) uv: vec2<f32>, @location(1) color: vec4<f32>,
    @location(2) @interpolate(flat) slot: u32,
    @location(3) position2D: vec2<f32>,
    @location(4) @interpolate(flat) clipRect: vec4<f32>,
    @location(5) @interpolate(flat) clipParams: vec4<f32>,
};
@vertex fn sprite_vertex(input: Input) -> Output {
    var out: Output;
    let p = vec3<f32>(input.position.xy, 1.0);
    out.position2D = vec2<f32>(dot(input.model0, p), dot(input.model1, p));
    out.position = uniforms.projection * vec4<f32>(out.position2D, 0.0, 1.0);
    out.clipRect = input.clipRect;
    out.clipParams = input.clipParams;
    let uv = vec3<f32>(input.textureCoord, 1.0);
    out.uv = vec2<f32>(dot(input.texture0, uv), dot(input.texture1, uv));
    out.color = input.color;
    out.slot = u32(input.textureSlot);
    return out;
}
@fragment fn sprite_fragment(input: Output) -> @location(0) vec4<f32> {
    let dx = dpdx(input.uv);
    let dy = dpdy(input.uv);
    var sampled: vec4<f32>;
    switch input.slot {
        case 1u: { sampled = textureSampleGrad(tex1, sampler1, input.uv, dx, dy); }
        case 2u: { sampled = textureSampleGrad(tex2, sampler2, input.uv, dx, dy); }
        case 3u: { sampled = textureSampleGrad(tex3, sampler3, input.uv, dx, dy); }
        default: { sampled = textureSampleGrad(tex, texSampler, input.uv, dx, dy); }
    }
    let texel = select(sampled, vec4<f32>(1.0, 1.0, 1.0, sampled.r), uniforms.coverage[input.slot] > 0.5);
    var color = texel * input.color;
    let halfSize = (input.clipRect.zw - input.clipRect.xy) * 0.5;
    let radius = input.clipParams.x;
    let q = abs(input.position2D - (input.clipRect.xy + halfSize)) - halfSize + radius;
    let distance = length(max(q, vec2<f32>(0.0))) + min(max(q.x, q.y), 0.0) - radius;
    let clipCoverage = clamp(0.5 - distance / max(fwidth(distance), 0.0001), 0.0, 1.0);
    color.a *= select(1.0, clipCoverage, input.clipParams.y > 0.5);
    return vec4<f32>(color.rgb * color.a, color.a);
}
