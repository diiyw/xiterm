#version 450
// Generated depth variant from mesh.frag.wgsl.




struct PBRUniforms {
    mat4x4 projection;
    vec4 cameraPosition;
    vec4 ambientLight;
    vec4 lightDirection;
    vec4 directionalLight;
    vec4 lightPosition[16];
    vec4 lightDirections[16];
    vec4 lightColors[16];
    vec4 lightParams[16];
    vec4 lightMeta;
    vec4 environment;
    vec4 renderParams;
    vec4 cameraForward;
    vec4 lightRight[16];
    vec4 lightUp[16];
    mat4x4 shadowMatrices[16];
    vec4 shadowRects[16];
    vec4 shadowParams[16];
    vec4 lightShadows[16];
    mat4x4 occlusionInverseProjection;
};
struct PBRInput {
    vec4 position;
    bool front;
    vec2 uv;
    vec4 baseColor;
    vec4 material;
};
const float PI = 3.1415927;
const bool SHADOW_FLIP_Y = true;

layout(set=2,binding=1,std140) uniform PBRUniforms_block_0Fragment { PBRUniforms _group_0_binding_0_fs; };

layout(set=0,binding=0) uniform sampler2D tex;

layout(location=2) smooth in vec2 _vs2fs_location2;
layout(location=3) smooth in vec4 _vs2fs_location3;
layout(location=5) flat in vec4 _vs2fs_location5;
layout(location = 0) out vec4 _fs2p_location0;

void main() {
    PBRInput input_ = PBRInput(gl_FragCoord, gl_FrontFacing, _vs2fs_location2, _vs2fs_location3, _vs2fs_location5);
    bool local = false;
    uint flags = uint((input_.material.w + 0.5));
    float _e10 = _group_0_binding_0_fs.renderParams.w;
    bool front = ((input_.front != (_e10 > 0.5)) != ((flags & 1024u) != 0u));
    if (((flags & 512u) == 0u)) {
        local = !(front);
    } else {
        local = false;
    }
    bool _e27 = local;
    if (_e27) {
        discard;
    }
    if (((flags & 3u) == 1u)) {
        vec4 _e35 = texture(tex, vec2(input_.uv));
        if (((_e35.w * input_.baseColor.w) < input_.material.z)) {
            discard;
        }
    }
    uint depth = uint((clamp(input_.position.z, 0.0, 1.0) * 16777215.0));
    _fs2p_location0 = vec4((vec3(float(((depth >> 16u) & 255u)), float(((depth >> 8u) & 255u)), float((depth & 255u))) / vec3(255.0)), 1.0);
    return;
}
