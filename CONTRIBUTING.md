# Contributing to SDLC

Thank you for considering contributing to **SDLC**!

## How to Contribute

1. **Fork the repository**
   - Click the **Fork** button at the top right of the repository page.
2. **Clone your fork**
   ```bash
   git clone https://github.com/<your-username>/sdlc.git
   cd sdlc
   ```
3. **Create a new branch** for your changes
   ```bash
   git checkout -b <feature-or-bug-name>
   ```
4. **Set up the development environment**
   - Ensure you have Go 1.20 or newer installed.
   - Install dependencies:
     ```bash
     go mod tidy
     ```
5. **Make your changes**
   - Follow the existing code style and conventions.
   - Add or update tests where appropriate.
6. **Run the test suite**
   ```bash
   go test ./...
   ```
   - All tests must pass before submitting a pull request.
7. **Commit your changes**
   ```bash
   git add .
   git commit -m "Brief description of your changes"
   ```
8. **Push to your fork**
   ```bash
   git push origin <feature-or-bug-name>
   ```
9. **Open a Pull Request**
   - Navigate to the original repository and click **New Pull Request**.
   - Choose your branch and fill out the PR template.
   - Provide a clear description of the changes and reference any related issues.

## Code of Conduct

Please note that this project adheres to the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## Reporting Issues

If you encounter a bug or have a feature request, please open an issue using the provided templates. Provide as much detail as possible to help us reproduce and understand the problem.

## License

By contributing, you agree that your contributions will be licensed under the same Apache 2.0 license as the project.
