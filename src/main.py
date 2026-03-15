"""Entry point for the Python portion of the SDLC project.

Provides a simple demonstration function and a CLI entry point.
"""

def hello() -> str:
    """Return a greeting string.

    This function is deliberately simple so that the test suite can verify
    that the package layout works correctly.
    """
    return "Hello, World!"


def main() -> None:
    """CLI entry point used by the console script defined in ``pyproject.toml``.

    When the package is installed it will expose a ``sdlc-py`` command that
    prints a short greeting.
    """
    print(hello())
