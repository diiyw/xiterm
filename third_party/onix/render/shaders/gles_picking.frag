#version 300 es
precision highp float;
precision highp int;
flat in uint pickingID;
layout(location=0) out uint outPickingID;
void main() {
    outPickingID = pickingID;
}
