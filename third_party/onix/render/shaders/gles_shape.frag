#version 300 es
precision highp float;
precision highp int;
in vec2 vLocal;
flat in vec4 vFill;
flat in vec4 vBorder;
flat in vec4 vParams;
flat in uint vKind;
out vec4 color;

float sdRoundedBox(vec2 point, vec2 size, float radius) {
    radius = clamp(radius, 0.0, min(size.x, size.y) * 0.5);
    vec2 q = abs(point) - size * 0.5 + radius;
    return length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - radius;
}

float sdSegment(vec2 point, vec2 a, vec2 b) {
    vec2 pa = point - a;
    vec2 ba = b - a;
    float h = clamp(dot(pa, ba) / dot(ba, ba), 0.0, 1.0);
    return length(pa - ba * h);
}

float coverage(float distance) {
    return clamp(0.5 - distance / max(fwidth(distance), 0.0001), 0.0, 1.0);
}

vec4 premultiply(vec4 value) {
    return vec4(value.rgb * value.a, value.a);
}

void main() {
	vec2 size = vParams.xy;
	if (vKind == 3u) {
		float corner = vParams.w;
		vec2 center = corner < 0.5 ? size : corner < 1.5 ? vec2(0.0, size.y) : corner < 2.5 ? vec2(size.x, 0.0) : vec2(0.0);
		float distance = length(vLocal - center) - vParams.z;
		color = premultiply(vFill) * coverage(distance);
		return;
	}
	if (vKind == 2u) {
        vec2 a = size * vec2(0.18, 0.56);
        vec2 b = size * vec2(0.42, 0.78);
        vec2 c = size * vec2(0.84, 0.22);
        float thickness = max(min(size.x, size.y) * 0.14, 1.4);
        float distance = min(sdSegment(vLocal, a, b), sdSegment(vLocal, b, c)) - thickness * 0.5;
        color = premultiply(vFill) * coverage(distance);
		return;
	}
	if (vKind == 4u) {
        float direction = vParams.w;
        vec2 a, b, d;
        if (direction < 0.5) {
            a = vec2(size.x * 0.5, size.y * 0.40); b = vec2(size.x * 0.20, size.y * 0.73); d = vec2(size.x * 0.80, size.y * 0.73);
        } else if (direction < 1.5) {
            a = vec2(size.x * 0.20, size.y * 0.20); b = vec2(size.x * 0.80, size.y * 0.20); d = vec2(size.x * 0.5, size.y * 0.50);
        } else if (direction < 2.5) {
            a = vec2(size.x * 0.73, size.y * 0.20); b = vec2(size.x * 0.73, size.y * 0.80); d = vec2(size.x * 0.40, size.y * 0.5);
        } else {
            a = vec2(size.x * 0.27, size.y * 0.20); b = vec2(size.x * 0.27, size.y * 0.80); d = vec2(size.x * 0.60, size.y * 0.5);
        }
        float e0 = (b.x - a.x) * (vLocal.y - a.y) - (b.y - a.y) * (vLocal.x - a.x);
        float e1 = (d.x - b.x) * (vLocal.y - b.y) - (d.y - b.y) * (vLocal.x - b.x);
        float e2 = (a.x - d.x) * (vLocal.y - d.y) - (a.y - d.y) * (vLocal.x - d.x);
        float orientation = sign((b.x - a.x) * (d.y - a.y) - (b.y - a.y) * (d.x - a.x));
        float distance = orientation > 0.0 ? -min(min(e0, e1), e2) : max(max(e0, e1), e2);
        distance /= max(length(b - a), 1.0);
        color = premultiply(vFill) * step(distance, 0.0);
        return;
    }

    vec2 point = vLocal - size * 0.5;
    float outer = coverage(sdRoundedBox(point, size, vParams.z));
    float borderWidth = min(max(vParams.w, 0.0), min(size.x, size.y) * 0.5);
    if (borderWidth <= 0.0) {
        color = premultiply(vFill) * outer;
        return;
    }
    vec2 innerSize = max(size - borderWidth * 2.0, vec2(0.0));
    float inner = min(innerSize.x, innerSize.y) > 0.0
        ? coverage(sdRoundedBox(point, innerSize, max(vParams.z - borderWidth, 0.0)))
        : 0.0;
    color = mix(premultiply(vBorder), premultiply(vFill), inner) * outer;
}
