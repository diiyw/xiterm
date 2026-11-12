#version 300 es
precision mediump float;
in vec2 _vs2fs_location0;
in vec4 _vs2fs_location1;
flat in vec4 _vs2fs_location2;
flat in vec4 _vs2fs_location3;
flat in vec4 _vs2fs_location4;
flat in vec4 _vs2fs_location5;
uniform sampler2D tex;
out vec4 color;
void main() {
    color = texture(tex, _vs2fs_location0) * _vs2fs_location1;
    color.rgb *= color.a;
}
