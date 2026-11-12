#version 450
layout(location=0) in vec2 uv;
layout(location=1) in vec4 tint;
layout(location=2) flat in int slot;
layout(location=3) in vec2 position2D;
layout(location=4) flat in vec4 clipRect;
layout(location=5) flat in vec4 clipParams;
layout(set=0, binding=0) uniform sampler2D tex;
layout(set=1, binding=0) uniform sampler2D tex1;
layout(set=2, binding=0) uniform sampler2D tex2;
layout(set=3, binding=0) uniform sampler2D tex3;
layout(location=0) out vec4 color;
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
	color = sampled * tint;
	if (clipParams.y > 0.5) color.a *= roundedClipCoverage(position2D, clipRect, clipParams.x);
    color.rgb *= color.a;
}
