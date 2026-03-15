# Unit tests for the Python workflow engine

import os
import tempfile
import unittest
from workflow.core import Workflow, Stage


class TestWorkflow(unittest.TestCase):
    def setUp(self):
        # Create a temporary file for persisting state
        self.temp_dir = tempfile.TemporaryDirectory()
        self.persist_path = os.path.join(self.temp_dir.name, "state.json")
        self.wf = Workflow(self.persist_path)

    def tearDown(self):
        self.temp_dir.cleanup()

    def test_stage_callbacks_and_persistence(self):
        entered = []
        exited = []
        # Add stages
        s1 = self.wf.add_stage("requirements")
        s2 = self.wf.add_stage("design")
        s3 = self.wf.add_stage("implementation")
        # Register callbacks
        s1.on_enter(lambda: entered.append("requirements"))
        s1.on_exit(lambda: exited.append("requirements"))
        s2.on_enter(lambda: entered.append("design"))
        s2.on_exit(lambda: exited.append("design"))
        s3.on_enter(lambda: entered.append("implementation"))
        # Add transitions
        self.wf.add_transition("requirements", "design")
        self.wf.add_transition("design", "implementation", lambda: True)
        # Initial stage should be the first added
        self.assertEqual(self.wf.current(), "requirements")
        # Move to design
        self.wf.move("design")
        self.assertEqual(self.wf.current(), "design")
        self.assertIn("requirements", exited)
        self.assertIn("design", entered)
        # Move to implementation
        self.wf.move("implementation")
        self.assertEqual(self.wf.current(), "implementation")
        self.assertIn("design", exited)
        self.assertIn("implementation", entered)
        # Verify persisted state by creating a new workflow instance
        wf2 = Workflow(self.persist_path)
        self.assertEqual(wf2.current(), "implementation")


if __name__ == "__main__":
    unittest.main()
