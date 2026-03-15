import unittest
from src import main

class TestMain(unittest.TestCase):
    def test_hello(self):
        self.assertEqual(main.hello(), "Hello, World!")

if __name__ == "__main__":
    unittest.main()
