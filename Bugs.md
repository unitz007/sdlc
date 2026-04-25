# Bugs Documentation

This file lists various bugs identified in the codebase along with their severity levels.

## Bug List

### 1. Duplicate Function Implementation
**Severity: High**

The `hasChanges` function in `cmd/commands.go` is incomplete and duplicated functionality with missing return statements. The function implements walking the filesystem to check for file modifications but doesn't properly return the results.

**Location:** `cmd/commands.go`

### 2. Dead Code in Reverse Dependencies
**Severity: Medium**

Several variables and functions related to reverse dependencies are declared but not properly implemented:
- `reverseDeps` map is initialized but never populated
- `resolveProject` function has a stub implementation
- `restartModule` function has a stub implementation
- Functions like `restartWithCascade` reference these but don't work properly

**Location:** `cmd/commands.go`

### 3. Unused Return Value
**Severity: Low**

In `cmd/commands.go`, the `runCommand` function returns `nil` at the end but this return value is not used in many places where it's called.

**Location:** `cmd/commands.go`

### 4. Duplicate Logic in Pre/Post Hooks
**Severity: Medium**

There appears to be duplicate implementations of pre/post hook execution logic with slightly different approaches:
- One approach using `runHook` function
- Another approach manually handling pre/post hooks

This duplication increases maintenance burden and potential for inconsistencies.

**Location:** `cmd/commands.go`

### 5. Inconsistent Color Handling
**Severity: Low**

The color constants are escaped twice (e.g., `"\\\\033[0m"` instead of `"\\033[0m"`), which might lead to incorrect color display in the terminal.

**Location:** `cmd/commands.go`

### 6. Resource Leak Potential
**Severity: Medium**

In the watch mode implementation, the context cancellation and goroutine management could potentially lead to resource leaks if not handled carefully during shutdown scenarios.

**Location:** `cmd/commands.go`

### 7. Missing Error Checks
**Severity: Medium**

Several operations don't have proper error checking:
- The banner printing in `executeTask` doesn't handle potential errors
- Some file operations in watch mode don't have comprehensive error handling

**Location:** Various files

### 8. Incomplete Configuration Validation
**Severity: Low**

The configuration validation is extensive but could be expanded to cover more edge cases, particularly around environment variable substitution limits and circular dependency detection.

**Location:** `config/config.go`

### 9. Race Condition Potential
**Severity: High**

The `states` map in the watch mode implementation is accessed from multiple goroutines without proper synchronization mechanisms, which could lead to race conditions.

**Location:** `cmd/commands.go`

### 10. Partial Implementation of Features
**Severity: Medium**

Some features like module cascading restarts and smart dependency tracking are partially implemented but not fully functional.

**Location:** `cmd/commands.go`