#version 450

layout(location = 0) in vec2 fragTexture;
layout(location = 1) in vec4 fragColor;

layout(set = 0, binding = 0) uniform sampler2D tex;
layout(location = 0) out vec4 outColor;

void main() {
	outColor = texture(tex, fragTexture) * fragColor;
	outColor.rgb *= outColor.a;
}
