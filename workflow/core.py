"""workflow.core
Provides a simple Python implementation of the SDLC workflow engine.
It mirrors the Go implementation in workflow/workflow.go but is idiomatic Python.
"""

import json
import os
from threading import Lock
from typing import Callable, Dict, List, Optional

# Types
Callback = Callable[[], None]
Condition = Callable[[], bool]


class Stage:
    """A stage in the workflow.

    Callbacks can be registered for entry and exit events.
    """

    def __init__(self, name: str):
        self.name = name
        self._on_enter: List[Callback] = []
        self._on_exit: List[Callback] = []

    def on_enter(self, cb: Callback) -> None:
        self._on_enter.append(cb)

    def on_exit(self, cb: Callback) -> None:
        self._on_exit.append(cb)

    def _run_enter(self) -> None:
        for cb in self._on_enter:
            cb()

    def _run_exit(self) -> None:
        for cb in self._on_exit:
            cb()


class Transition:
    """A permitted transition between two stages.

    Optional ``condition`` must return True for the transition to be allowed.
    Optional ``action`` is executed between exit and enter callbacks.
    """

    def __init__(
        self,
        from_stage: str,
        to_stage: str,
        condition: Optional[Condition] = None,
        action: Optional[Callback] = None,
    ):
        self.from_stage = from_stage
        self.to_stage = to_stage
        self.condition = condition
        self.action = action


class Workflow:
    """Core workflow engine.

    Stages and transitions can be configured, and the workflow can move between
    stages while running registered callbacks. The current stage can be persisted
    to a JSON file.
    """

    def __init__(self, persist_path: Optional[str] = None):
        self._stages: Dict[str, Stage] = {}
        self._transitions: List[Transition] = []
        self._current: Optional[str] = None
        self._persist_path = persist_path
        self._lock = Lock()
        if persist_path:
            # Ensure directory exists
            os.makedirs(os.path.dirname(persist_path), exist_ok=True)
            # Load persisted state if file exists
            if os.path.isfile(persist_path):
                try:
                    with open(persist_path, "r", encoding="utf-8") as f:
                        data = json.load(f)
                        self._current = data.get("current")
                except Exception:
                    pass

    # Stage management
    def add_stage(self, name: str) -> Stage:
        with self._lock:
            if name in self._stages:
                raise ValueError(f"stage {name} already exists")
            stage = Stage(name)
            self._stages[name] = stage
            if self._current is None:
                self._current = name
                self._persist()
            return stage

    def get_stage(self, name: str) -> Stage:
        return self._stages[name]

    # Transition management
    def add_transition(
        self,
        from_stage: str,
        to_stage: str,
        condition: Optional[Condition] = None,
        action: Optional[Callback] = None,
    ) -> None:
        with self._lock:
            if from_stage not in self._stages:
                raise ValueError(f"unknown from stage {from_stage}")
            if to_stage not in self._stages:
                raise ValueError(f"unknown to stage {to_stage}")
            self._transitions.append(
                Transition(from_stage, to_stage, condition, action)
            )

    # Query
    def current(self) -> Optional[str]:
        with self._lock:
            return self._current

    # Transition execution
    def move(self, to_stage: str) -> None:
        with self._lock:
            if self._current is None:
                raise RuntimeError("workflow has no current stage")
            # Find matching transition
            tr = next(
                (
                    t
                    for t in self._transitions
                    if t.from_stage == self._current and t.to_stage == to_stage
                ),
                None,
            )
            if tr is None:
                raise ValueError(
                    f"no transition from {self._current} to {to_stage}"
                )
            if tr.condition is not None and not tr.condition():
                raise ValueError(
                    f"transition condition from {self._current} to {to_stage} not satisfied"
                )
            # Execute exit callbacks
            self._stages[self._current]._run_exit()
            # Execute transition action
            if tr.action:
                tr.action()
            # Update current stage
            self._current = to_stage
            # Execute enter callbacks
            self._stages[self._current]._run_enter()
            # Persist if needed
            self._persist()

    def _persist(self) -> None:
        if not self._persist_path:
            return
        data = {"current": self._current}
        try:
            with open(self._persist_path, "w", encoding="utf-8") as f:
                json.dump(data, f, indent=2)
        except Exception:
            # Non‑critical – ignore persistence errors
            pass
