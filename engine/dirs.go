package engine

// defaultSkipDirs returns directory names that should be skipped during
// project detection. These are common non-project directories that would
// slow down detection without producing useful results.
func defaultSkipDirs() map[string]bool {
	return map[string]bool{
		"node_modules": true,
		"vendor":       true,
		"venv":         true,
		".git":         true,
		".svn":         true,
		".hg":          true,
		"__pycache__":  true,
		".idea":        true,
		".planner":     true,
		"target":       true, // Maven build output
		"build":        true, // Common build output
		"dist":         true, // Distribution output
		".next":        true, // Next.js build cache
		".nuxt":        true, // Nuxt.js build cache
		"coverage":     true, // Test coverage output
		".tox":         true, // Python tox environments
		".pytest_cache": true,
		".mypy_cache":  true,
		".gradle":      true, // Gradle cache
		".cache":       true, // Generic cache
		"Pods":         true, // CocoaPods
		"Carthage":     true, // Carthage
		".terraform":   true, // Terraform
	}
}
