issues:
  - id: ISSUE-001
    title: Fix tilde expansion in working directory path
    description: The tilde expansion logic in main.go uses strings.ReplaceAll but discards the result.
    feature: FEAT-001
    sprint: SPRINT-001
    status: DONE
    priority: P1
    effort: S
    story_points: 1
    assignee: AI_AGENT
  - id: ISSUE-010
    title: Fix multi-module detection duplication
    description: Multi-module projects can be detected multiple times if multiple build files exist in the same directory.
    feature: FEAT-002
    sprint: SPRINT-001
    status: DONE
    priority: P0
    effort: S
    story_points: 2
    assignee: AI_AGENT
  - id: ISSUE-002
    title: Refactor main function into smaller components
    description: Split main function into separate functions or packages for better testability.
    feature: FEAT-001
    sprint: SPRINT-001
    status: DONE
    priority: P1
    effort: M
    story_points: 3
    assignee: AI_AGENT
  - id: ISSUE-003
    title: Detect multiple modules in subdirectories
    description: Extend detection logic to scan subdirectories for build files.
    feature: FEAT-002
    sprint: SPRINT-001
    status: DONE
    priority: P1
    effort: M
    story_points: 3
    assignee: AI_AGENT
  - id: ISSUE-004
    title: Multi-module execution flags
    description: Add CLI flags to control which module(s) to run.
    feature: FEAT-002
    sprint: SPRINT-001
    status: DONE
    priority: P2
    effort: M
    story_points: 3
    assignee: AI_AGENT
  - id: ISSUE-005
    title: Concurrent execution for 'run' command
    description: Run multiple modules concurrently.
    feature: FEAT-002
    sprint: SPRINT-002
    status: DONE
    priority: P2
    effort: L
    story_points: 5
    assignee: AI_AGENT
  - id: ISSUE-006
    title: Watch mode for automatic restart
    description: Add --watch flag to monitor file changes and restart.
    feature: FEAT-003
    sprint: SPRINT-002
    status: DONE
    priority: P2
    effort: M
    story_points: 5
    assignee: AI_AGENT
  - id: ISSUE-007
    title: Module-specific configuration
    description: Allow .sdlc.json in module directories to override global settings.
    feature: FEAT-004
    sprint: SPRINT-002
    status: DONE
    priority: P2
    effort: M
    story_points: 3
    assignee: AI_AGENT
  - id: ISSUE-008
    title: Multi-module argument passing
    description: Allow .sdlc.conf file in each module directory to define env vars and flags.
    feature: FEAT-004
    sprint: SPRINT-002
    status: DONE
    priority: P1
    effort: M
    story_points: 3
    assignee: AI_AGENT
  - id: ISSUE-011
    title: Ignore modules in multi-module projects
    description: Add --ignore flag to exclude specific modules.
    feature: FEAT-002
    sprint: SPRINT-002
    status: DONE
    priority: P2
    effort: S
    story_points: 2
    assignee: AI_AGENT
  - id: ISSUE-009
    title: Interactive module selection
    description: Prompt user to select which module(s) to run when ambiguous.
    feature: FEAT-005
    sprint: SPRINT-002
    status: DONE
    priority: P2
    effort: M
    story_points: 3
    assignee: AI_AGENT