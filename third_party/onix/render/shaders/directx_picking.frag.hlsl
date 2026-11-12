struct Input {
    nointerpolation uint pickingID : LOC0;
};

struct FragmentInput_mesh_fragment {
    nointerpolation uint pickingID : LOC0;
};

uint mesh_fragment(FragmentInput_mesh_fragment fragmentinput_mesh_fragment) : SV_Target0
{
    Input input = { fragmentinput_mesh_fragment.pickingID };
    return input.pickingID;
}
