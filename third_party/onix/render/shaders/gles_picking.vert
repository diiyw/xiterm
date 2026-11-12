#version 300 es
layout(location=0) in vec3 position;
layout(location=3) in vec4 model0;
layout(location=4) in vec4 model1;
layout(location=5) in vec4 model2;
layout(location=6) in vec4 model3;
layout(location=14) in vec4 custom3;
uniform mat4 projection;
flat out uint pickingID;
void main() {
    pickingID = floatBitsToUint(custom3.w);
    gl_Position = projection * mat4(model0, model1, model2, model3) * vec4(position, 1.0);
    gl_Position.z = gl_Position.z * 2.0 - gl_Position.w;
}
