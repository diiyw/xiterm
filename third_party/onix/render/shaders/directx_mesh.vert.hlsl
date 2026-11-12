struct Uniforms {
    row_major float4x4 projection;
    float4 cameraPosition;
    float4 ambientLight;
    float4 lightDirection;
    float4 directionalLight;
    float4 lightPosition[16];
    float4 lightDirections[16];
    float4 lightColors[16];
    float4 lightParams[16];
    float4 lightMeta;
};

struct Input {
    float3 position : LOC0;
    float3 normal : LOC1;
    float2 textureCoord : LOC2;
    float4 tangent : LOC15;
    float4 model0_ : LOC3;
    float4 model1_ : LOC4;
    float4 model2_ : LOC5;
    float4 model3_ : LOC6;
    float4 baseColor : LOC7;
    float4 emissiveMetallic : LOC8;
    float4 material : LOC9;
    float4 uvTransform : LOC10;
    float4 custom0_ : LOC11;
    float4 custom1_ : LOC12;
    float4 custom2_ : LOC13;
    float4 custom3_ : LOC14;
    uint vertexIndex : SV_VertexID;
};

struct Output {
    float4 position : SV_Position;
    float3 worldPosition : LOC0;
    float3 normal : LOC1;
    float2 textureCoord : LOC2;
    float4 baseColor : LOC3;
    float4 emissiveMetallic : LOC4;
    nointerpolation float4 material : LOC5;
    nointerpolation float4 custom0_ : LOC6;
    nointerpolation float4 custom1_ : LOC7;
    nointerpolation float4 custom2_ : LOC8;
    nointerpolation float4 custom3_ : LOC9;
    float4 tangent : LOC10;
};

cbuffer uniforms : register(b0) { Uniforms uniforms; }
ByteAddressBuffer deformation : register(t16);
ByteAddressBuffer staticDeformation : register(t17);

struct VertexOutput_mesh_vertex {
    float3 worldPosition : LOC0;
    float3 normal_1 : LOC1;
    float2 textureCoord : LOC2;
    float4 baseColor : LOC3;
    float4 emissiveMetallic : LOC4;
    nointerpolation float4 material : LOC5;
    nointerpolation float4 custom0_ : LOC6;
    nointerpolation float4 custom1_ : LOC7;
    nointerpolation float4 custom2_ : LOC8;
    nointerpolation float4 custom3_ : LOC9;
    float4 tangent_1 : LOC10;
    float4 position_1 : SV_Position;
};

float deformationFloat(uint value)
{
    return asfloat(value);
}

float4x4 deformationMatrix(uint offset)
{
    uint _e3 = asuint(deformation.Load(offset*4));
    const float _e4 = deformationFloat(_e3);
    uint _e9 = asuint(deformation.Load((offset + 1u)*4));
    const float _e10 = deformationFloat(_e9);
    uint _e15 = asuint(deformation.Load((offset + 2u)*4));
    const float _e16 = deformationFloat(_e15);
    uint _e21 = asuint(deformation.Load((offset + 3u)*4));
    const float _e22 = deformationFloat(_e21);
    uint _e28 = asuint(deformation.Load((offset + 4u)*4));
    const float _e29 = deformationFloat(_e28);
    uint _e34 = asuint(deformation.Load((offset + 5u)*4));
    const float _e35 = deformationFloat(_e34);
    uint _e40 = asuint(deformation.Load((offset + 6u)*4));
    const float _e41 = deformationFloat(_e40);
    uint _e46 = asuint(deformation.Load((offset + 7u)*4));
    const float _e47 = deformationFloat(_e46);
    uint _e53 = asuint(deformation.Load((offset + 8u)*4));
    const float _e54 = deformationFloat(_e53);
    uint _e59 = asuint(deformation.Load((offset + 9u)*4));
    const float _e60 = deformationFloat(_e59);
    uint _e65 = asuint(deformation.Load((offset + 10u)*4));
    const float _e66 = deformationFloat(_e65);
    uint _e71 = asuint(deformation.Load((offset + 11u)*4));
    const float _e72 = deformationFloat(_e71);
    uint _e78 = asuint(deformation.Load((offset + 12u)*4));
    const float _e79 = deformationFloat(_e78);
    uint _e84 = asuint(deformation.Load((offset + 13u)*4));
    const float _e85 = deformationFloat(_e84);
    uint _e90 = asuint(deformation.Load((offset + 14u)*4));
    const float _e91 = deformationFloat(_e90);
    uint _e96 = asuint(deformation.Load((offset + 15u)*4));
    const float _e97 = deformationFloat(_e96);
    return float4x4(float4(_e4, _e10, _e16, _e22), float4(_e29, _e35, _e41, _e47), float4(_e54, _e60, _e66, _e72), float4(_e79, _e85, _e91, _e97));
}

uint naga_f2u32(float value) {
    return uint(clamp(value, 0.0, 4294967000.0));
}

VertexOutput_mesh_vertex mesh_vertex(Input input)
{
    Output out_ = (Output)0;
    float4 base = (float4)0;
    float4 custom = (float4)0;
    float3 position = (float3)0;
    float3 normal = (float3)0;
    float3 tangent = (float3)0;
    float tangentSign = (float)0;
    bool local = (bool)0;
    uint morphTarget = 0u;
    bool local_1 = (bool)0;
    float4x4 skinMatrix = float4x4((0.0).xxxx, (0.0).xxxx, (0.0).xxxx, (0.0).xxxx);
    float skinWeight = 0.0;
    uint influence = 0u;

    bool spine = (input.material.x < -1.5);
    base = input.baseColor;
    custom = input.custom0_;
    position = input.position;
    normal = input.normal;
    tangent = input.tangent.xyz;
    tangentSign = input.tangent.w;
    float _e23 = uniforms.directionalLight.w;
    uint deformationIndex = naga_f2u32((_e23 + 0.5));
    if (!(spine)) {
        local = (deformationIndex != 0u);
    } else {
        local = false;
    }
    bool _e33 = local;
    if (_e33) {
        uint header = ((deformationIndex - 1u) * 8u);
        uint skinOffset = asuint(deformation.Load(header*4));
        uint morphOffset = asuint(deformation.Load((header + 1u)*4));
        uint jointOffset = asuint(deformation.Load((header + 2u)*4));
        uint weightOffset = asuint(deformation.Load((header + 3u)*4));
        uint targetCount = asuint(deformation.Load((header + 5u)*4));
        uint jointCount = asuint(deformation.Load((header + 6u)*4));
        if ((morphOffset != 0u)) {
            uint2 loop_bound = uint2(4294967295u, 4294967295u);
            bool loop_init = true;
            [loop] while(true) {
                if (all(loop_bound == uint2(0u, 0u))) { break; }
                loop_bound -= uint2(loop_bound.y == 0u, 1u);
                if (!loop_init) {
                    uint _e157 = morphTarget;
                    morphTarget = (_e157 + 1u);
                }
                loop_init = false;
                uint _e70 = morphTarget;
                if ((_e70 < targetCount)) {
                } else {
                    break;
                }
                {
                    uint _e72 = morphTarget;
                    uint _e77 = asuint(deformation.Load((header + 4u)*4));
                    uint delta = (morphOffset + (((_e72 * _e77) + input.vertexIndex) * 9u));
                    uint _e85 = morphTarget;
                    uint _e88 = asuint(deformation.Load((weightOffset + _e85)*4));
                    const float _e89 = deformationFloat(_e88);
                    if ((_e89 == 0.0)) {
                        continue;
                    }
                    float3 _e92 = position;
                    uint _e95 = asuint(staticDeformation.Load(delta*4));
                    const float _e96 = deformationFloat(_e95);
                    uint _e101 = asuint(staticDeformation.Load((delta + 1u)*4));
                    const float _e102 = deformationFloat(_e101);
                    uint _e107 = asuint(staticDeformation.Load((delta + 2u)*4));
                    const float _e108 = deformationFloat(_e107);
                    position = (_e92 + (float3(_e96, _e102, _e108) * _e89));
                    float3 _e112 = normal;
                    uint _e117 = asuint(staticDeformation.Load((delta + 3u)*4));
                    const float _e118 = deformationFloat(_e117);
                    uint _e123 = asuint(staticDeformation.Load((delta + 4u)*4));
                    const float _e124 = deformationFloat(_e123);
                    uint _e129 = asuint(staticDeformation.Load((delta + 5u)*4));
                    const float _e130 = deformationFloat(_e129);
                    normal = (_e112 + (float3(_e118, _e124, _e130) * _e89));
                    float3 _e134 = tangent;
                    uint _e139 = asuint(staticDeformation.Load((delta + 6u)*4));
                    const float _e140 = deformationFloat(_e139);
                    uint _e145 = asuint(staticDeformation.Load((delta + 7u)*4));
                    const float _e146 = deformationFloat(_e145);
                    uint _e151 = asuint(staticDeformation.Load((delta + 8u)*4));
                    const float _e152 = deformationFloat(_e151);
                    tangent = (_e134 + (float3(_e140, _e146, _e152) * _e89));
                }
            }
        }
        if ((skinOffset != 0u)) {
            local_1 = (jointCount != 0u);
        } else {
            local_1 = false;
        }
        bool _e166 = local_1;
        if (_e166) {
            float3 _e167 = position;
            float4 sourcePosition = float4(_e167, 1.0);
            float3 _e170 = normal;
            float4 sourceNormal = float4(_e170, 0.0);
            float3 _e173 = tangent;
            float4 sourceTangent = float4(_e173, 0.0);
            uint2 loop_bound_1 = uint2(4294967295u, 4294967295u);
            bool loop_init_1 = true;
            [loop] while(true) {
                if (all(loop_bound_1 == uint2(0u, 0u))) { break; }
                loop_bound_1 -= uint2(loop_bound_1.y == 0u, 1u);
                if (!loop_init_1) {
                    uint _e227 = influence;
                    influence = (_e227 + 1u);
                }
                loop_init_1 = false;
                uint _e190 = influence;
                if ((_e190 < 4u)) {
                } else {
                    break;
                }
                {
                    uint _e200 = influence;
                    uint _e203 = asuint(staticDeformation.Load((((skinOffset + (input.vertexIndex * 8u)) + 4u) + _e200)*4));
                    const float _e204 = deformationFloat(_e203);
                    if ((_e204 > 0.0)) {
                        uint _e212 = influence;
                        uint joint = asuint(staticDeformation.Load(((skinOffset + (input.vertexIndex * 8u)) + _e212)*4));
                        if ((joint < jointCount)) {
                            const float4x4 _e220 = deformationMatrix((jointOffset + (joint * 16u)));
                            float4x4 _e221 = skinMatrix;
                            skinMatrix = (_e221 + mul(_e204, _e220));
                            float _e224 = skinWeight;
                            skinWeight = (_e224 + _e204);
                        }
                    }
                }
            }
            float _e229 = skinWeight;
            if ((_e229 > 0.0)) {
                float4x4 _e232 = skinMatrix;
                position = mul(sourcePosition, _e232).xyz;
                float4 _e236 = skinMatrix[1];
                float4 _e239 = skinMatrix[2];
                float4 _e243 = skinMatrix[2];
                float4 _e246 = skinMatrix[0];
                float4 _e250 = skinMatrix[0];
                float4 _e253 = skinMatrix[1];
                float3x3 cofactor = float3x3(cross(_e236.xyz, _e239.xyz), cross(_e243.xyz, _e246.xyz), cross(_e250.xyz, _e253.xyz));
                float4 _e258 = skinMatrix[0];
                float flip = ((dot(_e258.xyz, cofactor[0]) >= 0.0) ? 1.0 : -1.0);
                normal = (normalize(mul(sourceNormal.xyz, cofactor)) * flip);
                float4x4 _e271 = skinMatrix;
                tangent = mul(sourceTangent, _e271).xyz;
                float _e274 = tangentSign;
                tangentSign = (_e274 * flip);
            }
        }
    }
    if (spine) {
        uint packedA = naga_f2u32(input.normal.x);
        uint packedB = naga_f2u32(input.normal.z);
        base = float4((float3(float((packedA & 255u)), float(((packedA >> 8u) & 255u)), float(((packedA >> 16u) & 255u))) / (255.0).xxx), input.normal.y);
        custom = float4((float3(float((packedB & 255u)), float(((packedB >> 8u) & 255u)), float(((packedB >> 16u) & 255u))) / (255.0).xxx), input.custom0_.w);
    }
    float4x4 model = float4x4(input.model0_, input.model1_, input.model2_, input.model3_);
    float3 _e327 = position;
    float4 world = mul(float4(_e327, 1.0), model);
    float4x4 _e334 = uniforms.projection;
    out_.position = mul(world, _e334);
    if (((naga_f2u32((input.material.w + 0.5)) & 2048u) != 0u)) {
        float _e349 = out_.position.w;
        out_.position.z = (1e-06 * _e349);
    }
    out_.worldPosition = world.xyz;
    float3x3 cofactor_1 = float3x3(cross(input.model1_.xyz, input.model2_.xyz), cross(input.model2_.xyz, input.model0_.xyz), cross(input.model0_.xyz, input.model1_.xyz));
    float determinant_ = dot(input.model0_.xyz, cofactor_1[0]);
    float3 _e375 = normal;
    out_.normal = (spine ? float3(0.0, 0.0, 1.0) : (normalize(mul(_e375, cofactor_1)) * ((determinant_ >= 0.0) ? 1.0 : -1.0)));
    float3 _e390 = tangent;
    float _e395 = tangentSign;
    out_.tangent = float4(mul(float4(_e390, 0.0), model).xyz, (_e395 * ((determinant_ >= 0.0) ? 1.0 : -1.0)));
    out_.textureCoord = ((input.textureCoord * input.uvTransform.xy) + input.uvTransform.zw);
    float4 _e412 = base;
    out_.baseColor = _e412;
    out_.emissiveMetallic = input.emissiveMetallic;
    out_.material = input.material;
    float4 _e418 = custom;
    out_.custom0_ = _e418;
    out_.custom1_ = input.custom1_;
    out_.custom2_ = input.custom2_;
    out_.custom3_ = input.custom3_;
    Output _e425 = out_;
    const Output output = _e425;
    const VertexOutput_mesh_vertex output_1 = { output.worldPosition, output.normal, output.textureCoord, output.baseColor, output.emissiveMetallic, output.material, output.custom0_, output.custom1_, output.custom2_, output.custom3_, output.tangent, output.position };
    return output_1;
}
