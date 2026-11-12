#version 300 es
layout(location=0) in vec3 position;
layout(location=1) in vec2 localCoord;
layout(location=2) in vec3 model0;
layout(location=3) in vec3 model1;
layout(location=4) in vec3 local0;
layout(location=5) in vec3 local1;
layout(location=6) in vec4 fillColor;
layout(location=7) in vec4 borderColor;
layout(location=8) in vec4 shapeParams;
layout(location=9) in uint shapeKind;
out vec2 vLocal;
flat out vec4 vFill;
flat out vec4 vBorder;
flat out vec4 vParams;
flat out uint vKind;
uniform mat4 projection;
void main() {
    vec3 uv = vec3(localCoord, 1.0);
    vLocal = vec2(dot(local0, uv), dot(local1, uv)) * shapeParams.xy;
    vFill = fillColor;
    vBorder = borderColor;
    vParams = shapeParams;
    vKind = shapeKind;
    vec3 p = vec3(position.xy, 1.0);
    gl_Position = projection * vec4(dot(model0, p), dot(model1, p), 0.0, 1.0);
}
