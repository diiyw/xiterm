#version 450

layout(location = 0) in vec3 position;
layout(location = 1) in vec2 textureCoord;
layout(location = 2) in vec3 model0;
layout(location = 3) in vec3 model1;
layout(location = 4) in vec3 texture0;
layout(location = 5) in vec3 texture1;
layout(location = 6) in vec4 instanceColor;
layout(location = 7) in vec4 custom0;
layout(location = 8) in vec4 custom1;
layout(location = 9) in vec4 custom2;
layout(location = 10) in vec4 custom3;

layout(location = 0) out vec2 fragTexture;
layout(location = 1) out vec4 fragColor;
layout(location = 2) flat out vec4 fragCustom0;
layout(location = 3) flat out vec4 fragCustom1;
layout(location = 4) flat out vec4 fragCustom2;
layout(location = 5) flat out vec4 fragCustom3;

layout(push_constant) uniform PushConstants {
    mat4 projection;
	vec4 shaderPadding[2];
} push;

void main() {
	vec3 uv = vec3(textureCoord, 1.0);
	fragTexture = vec2(dot(texture0, uv), dot(texture1, uv));
	fragColor = instanceColor;
	fragCustom0 = custom0;
	fragCustom1 = custom1;
	fragCustom2 = custom2;
	fragCustom3 = custom3;
	vec3 p = vec3(position.xy, 1.0);
	gl_Position = push.projection * vec4(dot(model0, p), dot(model1, p), 0.0, 1.0);
	gl_Position.y = -gl_Position.y;
}
