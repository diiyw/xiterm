#version 450

layout(location = 0) in vec3 position;
layout(location = 1) in vec2 localCoord;
layout(location = 2) in vec3 model0;
layout(location = 3) in vec3 model1;
layout(location = 4) in vec3 local0;
layout(location = 5) in vec3 local1;
layout(location = 6) in vec4 fillColor;
layout(location = 7) in vec4 borderColor;
layout(location = 8) in vec4 shapeParams;
layout(location = 9) in uint shapeKind;

layout(location = 0) out vec2 fragLocal;
layout(location = 1) flat out vec4 fragFill;
layout(location = 2) flat out vec4 fragBorder;
layout(location = 3) flat out vec4 fragParams;
layout(location = 4) flat out uint fragKind;

layout(push_constant) uniform PushConstants {
    mat4 projection;
	vec4 shaderPadding[2];
} push;

void main() {
	vec3 uv = vec3(localCoord, 1.0);
	fragLocal = vec2(dot(local0, uv), dot(local1, uv)) * shapeParams.xy;
	fragFill = fillColor;
	fragBorder = borderColor;
	fragParams = shapeParams;
	fragKind = shapeKind;
	vec3 p = vec3(position.xy, 1.0);
	gl_Position = push.projection * vec4(dot(model0, p), dot(model1, p), 0.0, 1.0);
	gl_Position.y = -gl_Position.y;
}
