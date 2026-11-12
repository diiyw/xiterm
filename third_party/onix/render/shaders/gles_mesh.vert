#version 300 es
layout(location=0) in vec3 position;
layout(location=1) in vec3 normal;
layout(location=2) in vec2 textureCoord;
layout(location=15) in vec4 tangent;
layout(location=3) in vec4 model0;
layout(location=4) in vec4 model1;
layout(location=5) in vec4 model2;
layout(location=6) in vec4 model3;
layout(location=7) in vec4 baseColor;
layout(location=8) in vec4 emissiveMetallic;
layout(location=9) in vec4 material;
layout(location=10) in vec4 uvTransform;
layout(location=11) in vec4 custom0;
layout(location=12) in vec4 custom1;
layout(location=13) in vec4 custom2;
layout(location=14) in vec4 custom3;
out vec3 _vs2fs_location0;
out vec3 _vs2fs_location1;
out vec2 _vs2fs_location2;
out vec4 _vs2fs_location3;
out vec4 _vs2fs_location4;
flat out vec4 _vs2fs_location5;
flat out vec4 _vs2fs_location6;
flat out vec4 _vs2fs_location7;
flat out vec4 _vs2fs_location8;
flat out vec4 _vs2fs_location9;
out vec4 _vs2fs_location10;
uniform mat4 projection;
vec3 unpackRGB(float encoded) {
    uint encodedColor = uint(encoded);
    return vec3(encodedColor & 255u, (encodedColor >> 8u) & 255u, (encodedColor >> 16u) & 255u) / 255.0;
}
void main() {
    bool spine = material.x < -1.5;
    vec4 vertexBaseColor = baseColor;
    vec4 vertexCustom0 = custom0;
    if (spine) {
        vertexBaseColor = vec4(unpackRGB(normal.x), normal.y);
        vertexCustom0.rgb = unpackRGB(normal.z);
    }
    mat4 model = mat4(model0, model1, model2, model3);
    vec4 world = model * vec4(position, 1.0);
    _vs2fs_location0 = world.xyz;
    mat3 cofactor = mat3(cross(model1.xyz, model2.xyz), cross(model2.xyz, model0.xyz), cross(model0.xyz, model1.xyz));
    float flip = dot(model0.xyz, cofactor[0]) < 0.0 ? -1.0 : 1.0;
    _vs2fs_location1 = spine ? vec3(0.0, 0.0, 1.0) : normalize(cofactor * normal) * flip;
    _vs2fs_location2 = textureCoord * uvTransform.xy + uvTransform.zw;
    _vs2fs_location3 = vertexBaseColor;
    _vs2fs_location4 = emissiveMetallic;
    _vs2fs_location5 = material;
    _vs2fs_location6 = vertexCustom0;
    _vs2fs_location7 = custom1;
    _vs2fs_location8 = custom2;
    _vs2fs_location9 = custom3;
    _vs2fs_location10 = vec4(mat3(model)*tangent.xyz,tangent.w*flip);
    gl_Position = projection * world;
    if ((uint(material.w+0.5)&2048u)!=0u) gl_Position.z=0.000001*gl_Position.w;
    gl_Position.z = gl_Position.z * 2.0 - gl_Position.w;
}
