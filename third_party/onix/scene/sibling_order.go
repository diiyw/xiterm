package scene

// SiblingOrder preserves authored sibling order. Runtime-created nodes fall
// back to their Entity value.
type SiblingOrder int

// PainterGroup2D keeps a subtree in one stable painter group. Groups sort
// before local UIStackIndex/RenderOrder2D values and are inherited by children.
type PainterGroup2D uint64
