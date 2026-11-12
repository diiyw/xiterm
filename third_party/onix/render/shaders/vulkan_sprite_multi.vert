#version 450
layout(location=0) in vec3 position;
layout(location=1) in vec2 textureCoord;
layout(location=2) in vec3 model0;
layout(location=3) in vec3 model1;
layout(location=4) in vec3 texture0;
layout(location=5) in vec3 texture1;
layout(location=6) in vec4 instanceColor;
layout(location=7) in vec4 custom0;
layout(location=8) in vec4 custom1;
layout(location=9) in vec4 custom2;
layout(location=10) in vec4 custom3;
layout(location=11) in float textureSlot;
layout(location=0) out vec2 uv;
layout(location=1) out vec4 tint;
layout(location=2) flat out int slot;
layout(location=3) out vec2 position2D;
layout(location=4) flat out vec4 clipRect;
layout(location=5) flat out vec4 clipParams;
layout(push_constant) uniform PushConstants { mat4 projection; vec4 padding[2]; } push;
void main() {
    vec3 t = vec3(textureCoord, 1.0);
    uv = vec2(dot(texture0, t), dot(texture1, t));
    tint = instanceColor;
    slot = int(textureSlot);
    vec3 p = vec3(position.xy, 1.0);
	position2D = vec2(dot(model0, p), dot(model1, p));
	gl_Position = push.projection * vec4(position2D, 0.0, 1.0);
	gl_Position.y = -gl_Position.y;
	clipRect = custom2;
	clipParams = custom3;
}
