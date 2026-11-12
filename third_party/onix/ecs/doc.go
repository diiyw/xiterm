// Package ecs provides typed component storage, queries, structural commands,
// schedules, resources and messages. It is independent of scene, rendering,
// input and UI runtimes; applications supply their own component types.
//
// World.ComponentRevision tracks storage changes, including archetype migrations,
// and is suitable for caches retaining component references. Caches retaining
// copied values can use World.ComponentValueRevision to ignore unchanged migrations.
// World.EntityVersion tracks live entities independently of component membership.
//
// Component access, resources, revision tracking and message writes are generic
// World methods. Commands provides deferred structural operations; Bundle.Put
// builds dynamic component groups. EntitySlot methods access the slot's bound
// world and validate the entity generation on every lookup.
//
// SystemContext provides Local values and Added/Changed filters. World owns
// FrameValue, FrameScratch, FrameScratchMap, ScopeValue and MessageScratch;
// these caches retain their distinct reset, scope and message lifetimes.
//
// World.Query and Query2 through Query7 construct typed query views. World also
// provides NewSpawner1 through NewSpawner8 and NewPatch1 through NewPatch8 for
// reusable creation and mutation plans bound to that world.
package ecs
