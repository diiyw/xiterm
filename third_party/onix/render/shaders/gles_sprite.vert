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
out vec2 _vs2fs_location0;
out vec4 _vs2fs_location1;
flat out vec4 _vs2fs_location2;
flat out vec4 _vs2fs_location3;
flat out vec4 _vs2fs_location4;
flat out vec4 _vs2fs_location5;
uniform mat4 projection;
void main() {
    vec3 uv = vec3(textureCoord, 1.0);
    _vs2fs_location0 = vec2(dot(texture0, uv), dot(texture1, uv));
    _vs2fs_location1 = instanceColor;
    _vs2fs_location2 = custom0;
    _vs2fs_location3 = custom1;
    _vs2fs_location4 = custom2;
    _vs2fs_location5 = custom3;
    vec3 p = vec3(position.xy, 1.0);
    gl_Position = projection * vec4(dot(model0, p), dot(model1, p), 0.0, 1.0);
}
