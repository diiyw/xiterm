#version 450

layout(location = 0) in vec3 position;
layout(location = 1) in vec3 normal;
layout(location = 2) in vec2 textureCoord;
layout(location=15) in vec4 tangent;
layout(location = 3) in vec4 model0;
layout(location = 4) in vec4 model1;
layout(location = 5) in vec4 model2;
layout(location = 6) in vec4 model3;
layout(location = 7) in vec4 baseColor;
layout(location = 8) in vec4 emissiveMetallic;
layout(location = 9) in vec4 material;
layout(location = 10) in vec4 uvTransform;
layout(location = 11) in vec4 custom0;
layout(location = 12) in vec4 custom1;
layout(location = 13) in vec4 custom2;
layout(location = 14) in vec4 custom3;

layout(location = 0) out vec3 fragWorldPosition;
layout(location = 1) out vec3 fragNormal;
layout(location = 2) out vec2 fragTexture;
layout(location = 3) out vec4 fragBaseColor;
layout(location = 4) out vec4 fragEmissiveMetallic;
layout(location = 5) flat out vec4 fragMaterial;
layout(location = 6) flat out vec4 fragCustom0;
layout(location = 7) flat out vec4 fragCustom1;
layout(location = 8) flat out vec4 fragCustom2;
layout(location = 9) flat out vec4 fragCustom3;
layout(location=10) out vec4 fragTangent;

layout(push_constant) uniform PushConstants {
	mat4 viewProjection;
	vec4 cameraPosition;
	vec4 ambientLight;
	vec4 lightDirection;
	vec4 directionalLight;
} push;

layout(set = 2, binding = 0, std430) readonly buffer DeformationBuffer {
	uint deformation[];
};

vec3 unpackRGB(float encoded) {
	uint encodedColor = uint(encoded);
	return vec3(encodedColor & 255u, (encodedColor >> 8u) & 255u, (encodedColor >> 16u) & 255u) / 255.0;
}

mat4 deformationMatrix(uint offset) {
	return mat4(
		vec4(uintBitsToFloat(deformation[offset]), uintBitsToFloat(deformation[offset + 1]), uintBitsToFloat(deformation[offset + 2]), uintBitsToFloat(deformation[offset + 3])),
		vec4(uintBitsToFloat(deformation[offset + 4]), uintBitsToFloat(deformation[offset + 5]), uintBitsToFloat(deformation[offset + 6]), uintBitsToFloat(deformation[offset + 7])),
		vec4(uintBitsToFloat(deformation[offset + 8]), uintBitsToFloat(deformation[offset + 9]), uintBitsToFloat(deformation[offset + 10]), uintBitsToFloat(deformation[offset + 11])),
		vec4(uintBitsToFloat(deformation[offset + 12]), uintBitsToFloat(deformation[offset + 13]), uintBitsToFloat(deformation[offset + 14]), uintBitsToFloat(deformation[offset + 15])));
}

layout(set=3,binding=14,std430) readonly buffer StaticDeformation { uint staticDeformation[]; };

void main() {
	bool spine = material.x < -1.5;
	vec4 vertexBaseColor = baseColor;
	vec4 vertexCustom0 = custom0;
	vec3 deformedPosition = position;
	vec3 deformedNormal = normal;
	vec3 deformedTangent = tangent.xyz;
	float tangentSign = tangent.w;
	uint deformationIndex = push.directionalLight.w < 0.0 ? uint(-push.directionalLight.w + 0.5) : 0;
	if (!spine && deformationIndex != 0) {
		uint header = (deformationIndex - 1) * 8;
		uint skinOffset = deformation[header];
		uint morphOffset = deformation[header + 1];
		uint jointOffset = deformation[header + 2];
		uint weightOffset = deformation[header + 3];
		uint vertexCount = deformation[header + 4];
		uint targetCount = deformation[header + 5];
		uint jointCount = deformation[header + 6];
		if (morphOffset != 0) {
			for (uint morphTarget = 0; morphTarget < targetCount; morphTarget++) {
				uint delta = morphOffset + (morphTarget * vertexCount + uint(gl_VertexIndex)) * 9;
				float weight = uintBitsToFloat(deformation[weightOffset + morphTarget]);
				if (weight == 0.0) { continue; }
				deformedPosition += vec3(uintBitsToFloat(staticDeformation[delta]), uintBitsToFloat(staticDeformation[delta + 1]), uintBitsToFloat(staticDeformation[delta + 2])) * weight;
				deformedNormal += vec3(uintBitsToFloat(staticDeformation[delta + 3]), uintBitsToFloat(staticDeformation[delta + 4]), uintBitsToFloat(staticDeformation[delta + 5])) * weight;
				deformedTangent += vec3(uintBitsToFloat(staticDeformation[delta+6]),uintBitsToFloat(staticDeformation[delta+7]),uintBitsToFloat(staticDeformation[delta+8]))*weight;
			}
		}
		if (skinOffset != 0 && jointCount != 0) {
			vec4 sourcePosition = vec4(deformedPosition, 1.0);
			vec4 sourceNormal = vec4(deformedNormal, 0.0);
			vec4 sourceTangent = vec4(deformedTangent,0.0);
			mat4 skinMatrix = mat4(0.0);
			float skinWeight = 0.0;
			for (uint influence = 0; influence < 4; influence++) {
				float weight = uintBitsToFloat(staticDeformation[skinOffset + uint(gl_VertexIndex) * 8 + 4 + influence]);
				if (weight > 0.0) {
					uint joint = staticDeformation[skinOffset + uint(gl_VertexIndex) * 8 + influence];
					if (joint < jointCount) {
						mat4 matrix = deformationMatrix(jointOffset + joint * 16);
						skinMatrix += matrix*weight;
						skinWeight += weight;
					}
				}
			}
			if (skinWeight > 0.0) {
				deformedPosition = (skinMatrix*sourcePosition).xyz;
				mat3 cofactor=mat3(cross(skinMatrix[1].xyz,skinMatrix[2].xyz),cross(skinMatrix[2].xyz,skinMatrix[0].xyz),cross(skinMatrix[0].xyz,skinMatrix[1].xyz));
				float flip=dot(skinMatrix[0].xyz,cofactor[0])<0.0 ? -1.0 : 1.0;
				deformedNormal=normalize(cofactor*sourceNormal.xyz)*flip;
				deformedTangent=(skinMatrix*sourceTangent).xyz;
				tangentSign*=flip;
			}
		}
	}
	if (spine) {
		vertexBaseColor = vec4(unpackRGB(normal.x), normal.y);
		vertexCustom0.rgb = unpackRGB(normal.z);
	}
	mat4 model = mat4(model0, model1, model2, model3);
	vec4 world = model * vec4(deformedPosition, 1.0);
	fragWorldPosition = world.xyz;
	mat3 cofactor = mat3(cross(model1.xyz, model2.xyz), cross(model2.xyz, model0.xyz), cross(model0.xyz, model1.xyz));
	float flip = dot(model0.xyz, cofactor[0]) < 0.0 ? -1.0 : 1.0;
	fragNormal = spine ? vec3(0.0, 0.0, 1.0) : normalize(cofactor * deformedNormal) * flip;
	fragTexture = textureCoord * uvTransform.xy + uvTransform.zw;
	fragBaseColor = vertexBaseColor;
	fragEmissiveMetallic = emissiveMetallic;
	fragMaterial = material;
	fragCustom0 = vertexCustom0;
	fragCustom1 = custom1;
	fragCustom2 = custom2;
	fragCustom3 = custom3;
	fragTangent = vec4(mat3(model)*deformedTangent,tangentSign*flip);
	gl_Position = push.viewProjection * world;
    if ((uint(material.w+0.5)&2048u)!=0u) gl_Position.z=0.000001*gl_Position.w;
	gl_Position.y = -gl_Position.y;
}
