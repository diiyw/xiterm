// Package scene provides scene components and runtime systems built on ecs,
// including transforms, common cameras, 2D rendering, animation, and input.
// The ui package owns container layouts and widget constraints.
// The scene/render3d package owns the optional 3D renderer and binds its camera
// settings through RenderWorld.RegisterCameraRenderer. Applications install the systems they use.
package scene
