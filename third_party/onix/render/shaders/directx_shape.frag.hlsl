struct Input {
    float2 local : LOC0;
    nointerpolation float4 fill : LOC1;
    nointerpolation float4 border : LOC2;
    nointerpolation float4 params : LOC3;
    nointerpolation uint kind : LOC4;
};

struct FragmentInput_shape_fragment {
    float2 local_2 : LOC0;
    nointerpolation float4 fill : LOC1;
    nointerpolation float4 border : LOC2;
    nointerpolation float4 params : LOC3;
    nointerpolation uint kind : LOC4;
};

float roundedBox(float2 point_, float2 size, float radius)
{
    float r = clamp(radius, 0.0, (min(size.x, size.y) * 0.5));
    float2 q = ((abs(point_) - (size * 0.5)) + (r).xx);
    return ((length(max(q, (0.0).xx)) + min(max(q.x, q.y), 0.0)) - r);
}

float segment(float2 point_1, float2 a, float2 b)
{
    float2 pa = (point_1 - a);
    float2 ba = (b - a);
    float h = clamp((dot(pa, ba) / dot(ba, ba)), 0.0, 1.0);
    return length((pa - (ba * h)));
}

float coverage(float distance_, float derivative)
{
    return clamp((0.5 - (distance_ / max(derivative, 0.0001))), 0.0, 1.0);
}

float4 premultiply(float4 value)
{
    return float4((value.xyz * value.w), value.w);
}

float4 shape_fragment(FragmentInput_shape_fragment fragmentinput_shape_fragment) : SV_Target0
{
    Input input = { fragmentinput_shape_fragment.local_2, fragmentinput_shape_fragment.fill, fragmentinput_shape_fragment.border, fragmentinput_shape_fragment.params, fragmentinput_shape_fragment.kind };
    float2 triangleA = (float2)0;
    float2 triangleB = (float2)0;
    float2 triangleC = (float2)0;
    bool local = (bool)0;
    bool local_1 = (bool)0;

    float2 size_1 = input.params.xy;
    float2 point_2 = (input.local - (size_1 * 0.5));
    float direction = input.params.w;
    triangleA = float2((size_1.x * 0.5), (size_1.y * 0.4));
    triangleB = float2((size_1.x * 0.2), (size_1.y * 0.73));
    triangleC = float2((size_1.x * 0.8), (size_1.y * 0.73));
    if ((direction >= 0.5)) {
        local = (direction < 1.5);
    } else {
        local = false;
    }
    bool _e40 = local;
    if (_e40) {
        triangleA = float2((size_1.x * 0.2), (size_1.y * 0.2));
        triangleB = float2((size_1.x * 0.8), (size_1.y * 0.2));
        triangleC = float2((size_1.x * 0.5), (size_1.y * 0.5));
    }
    if ((direction >= 1.5)) {
        local_1 = (direction < 2.5);
    } else {
        local_1 = false;
    }
    bool _e69 = local_1;
    if (_e69) {
        triangleA = float2((size_1.x * 0.73), (size_1.y * 0.2));
        triangleB = float2((size_1.x * 0.73), (size_1.y * 0.8));
        triangleC = float2((size_1.x * 0.4), (size_1.y * 0.5));
    }
    if ((direction >= 2.5)) {
        triangleA = float2((size_1.x * 0.27), (size_1.y * 0.2));
        triangleB = float2((size_1.x * 0.27), (size_1.y * 0.8));
        triangleC = float2((size_1.x * 0.6), (size_1.y * 0.5));
    }
    float _e115 = triangleB.x;
    float _e117 = triangleA.x;
    float _e122 = triangleA.y;
    float _e126 = triangleB.y;
    float _e128 = triangleA.y;
    float _e133 = triangleA.x;
    float edge0_ = (((_e115 - _e117) * (input.local.y - _e122)) - ((_e126 - _e128) * (input.local.x - _e133)));
    float _e138 = triangleC.x;
    float _e140 = triangleB.x;
    float _e145 = triangleB.y;
    float _e149 = triangleC.y;
    float _e151 = triangleB.y;
    float _e156 = triangleB.x;
    float edge1_ = (((_e138 - _e140) * (input.local.y - _e145)) - ((_e149 - _e151) * (input.local.x - _e156)));
    float _e161 = triangleA.x;
    float _e163 = triangleC.x;
    float _e168 = triangleC.y;
    float _e172 = triangleA.y;
    float _e174 = triangleC.y;
    float _e179 = triangleC.x;
    float edge2_ = (((_e161 - _e163) * (input.local.y - _e168)) - ((_e172 - _e174) * (input.local.x - _e179)));
    float _e184 = triangleB.x;
    float _e186 = triangleA.x;
    float _e189 = triangleC.y;
    float _e191 = triangleA.y;
    float _e195 = triangleB.y;
    float _e197 = triangleA.y;
    float _e200 = triangleC.x;
    float _e202 = triangleA.x;
    float orientation = sign((((_e184 - _e186) * (_e189 - _e191)) - ((_e195 - _e197) * (_e200 - _e202))));
    float2 _e215 = triangleB;
    float2 _e216 = triangleA;
    float triangleDistance = (((orientation > 0.0) ? -(min(min(edge0_, edge1_), edge2_)) : max(max(edge0_, edge1_), edge2_)) / max(length((_e215 - _e216)), 1.0));
    float triangle_ = ((triangleDistance <= 0.0) ? 1.0 : 0.0);
    float2 checkA = (size_1 * float2(0.18, 0.56));
    float2 checkB = (size_1 * float2(0.42, 0.78));
    float2 checkC = (size_1 * float2(0.84, 0.22));
    float thickness = max((min(size_1.x, size_1.y) * 0.14), 1.4);
    const float _e247 = segment(input.local, checkA, checkB);
    const float _e249 = segment(input.local, checkB, checkC);
    float checkDistance = (min(_e247, _e249) - (thickness * 0.5));
    const float _e256 = roundedBox(point_2, size_1, input.params.z);
    float borderWidth = min(max(input.params.w, 0.0), (min(size_1.x, size_1.y) * 0.5));
    float2 innerSize = max((size_1 - ((borderWidth * 2.0)).xx), (0.0).xx);
    const float _e279 = roundedBox(point_2, innerSize, max((input.params.z - borderWidth), 0.0));
    float _e280 = fwidth(checkDistance);
    const float _e281 = coverage(checkDistance, _e280);
    float _e282 = fwidth(_e256);
    const float _e283 = coverage(_e256, _e282);
    float _e284 = fwidth(_e279);
    const float _e285 = coverage(_e279, _e284);
    if ((input.kind == 2u)) {
        const float4 _e290 = premultiply(input.fill);
        return (_e290 * _e281);
    }
    if ((input.kind == 4u)) {
        const float4 _e296 = premultiply(input.fill);
        return (_e296 * triangle_);
    }
    if ((borderWidth <= 0.0)) {
        const float4 _e301 = premultiply(input.fill);
        return (_e301 * _e283);
    }
    bool hasInner = (min(innerSize.x, innerSize.y) > 0.0);
    const float4 _e309 = premultiply(input.border);
    const float4 _e311 = premultiply(input.fill);
    return (lerp(_e309, _e311, (hasInner ? _e285 : 0.0)) * _e283);
}
