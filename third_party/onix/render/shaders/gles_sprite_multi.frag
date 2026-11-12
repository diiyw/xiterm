#version 300 es
precision highp float;
precision highp int;
in vec2 uv;
in vec2 position2D;
in vec4 tint;
flat in int slot;
flat in vec4 clipRect;
flat in vec4 clipParams;
uniform sampler2D tex;
uniform sampler2D tex1;
uniform sampler2D tex2;
uniform sampler2D tex3;
uniform vec4 coverage;
out vec4 color;
float roundedClipCoverage(vec2 point, vec4 rect, float radius) {
    vec2 halfSize = (rect.zw - rect.xy) * 0.5;
    vec2 q = abs(point - (rect.xy + halfSize)) - halfSize + radius;
    float distance = length(max(q, vec2(0.0))) + min(max(q.x, q.y), 0.0) - radius;
    return clamp(0.5 - distance / max(fwidth(distance), 0.0001), 0.0, 1.0);
}
void main() {
    vec2 dx = dFdx(uv), dy = dFdy(uv);
    vec4 sampled;
    switch (slot) {
        case 1: sampled = textureGrad(tex1, uv, dx, dy); break;
        case 2: sampled = textureGrad(tex2, uv, dx, dy); break;
        case 3: sampled = textureGrad(tex3, uv, dx, dy); break;
        default: sampled = textureGrad(tex, uv, dx, dy); break;
    }
    if (coverage[slot] > 0.5) sampled = vec4(1.0, 1.0, 1.0, sampled.r);
    color = sampled * tint;
    if (clipParams.y > 0.5) color.a *= roundedClipCoverage(position2D, clipRect, clipParams.x);
    color.rgb *= color.a;
}
