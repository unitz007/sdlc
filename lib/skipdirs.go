// Package lib provides core types and utilities for executing software
// development lifecycle commands.
package lib

// SkippedDirs is the canonical set of directory names that should be
// excluded from project detection and file watching. Both the engine
// (project scanning) and the CLI (watcher / ignore) should reference
// this single list so they stay in sync.
var SkippedDirs = map[string]bool{
	".git":         true,
	".idea":        true,
	".planner":     true,
	"node_modules": true,
	"dist":         true,
	"build":        true,
	"target":       true,
	"bin":          true,
	"pkg":          true,
}
