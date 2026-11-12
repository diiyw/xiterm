#version 300 es
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
out vec2 uv;
out vec2 position2D;
out vec4 tint;
flat out int slot;
flat out vec4 clipRect;
flat out vec4 clipParams;
uniform mat4 projection;
void main() {
    vec3 t = vec3(textureCoord, 1.0);
    uv = vec2(dot(texture0, t), dot(texture1, t));
    tint = instanceColor;
    slot = int(textureSlot);
    vec3 p = vec3(position.xy, 1.0);
    position2D = vec2(dot(model0, p), dot(model1, p));
    gl_Position = projection * vec4(position2D, 0.0, 1.0);
    clipRect = custom2;
    clipParams = custom3;
}
