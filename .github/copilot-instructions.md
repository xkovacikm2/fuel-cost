# Agent rules

Application tests cover application features and bugs. Do not add tests for dev-container configuration changes.

## Test-first bug fixing (red → green)

When fixing a bug or a pipeline failure:

1. Analyze the issue and identify the root cause before changing code.
2. Write a test that reproduces the issue **before** implementing the fix.
3. Run the test and confirm it fails for the expected reason (red). If it passes, the test does not capture the issue; revise it.
4. Implement the fix.
5. Run the new test and the full suite and confirm they pass (green).
6. Report both the red and the green result to the user.

If the issue cannot reasonably be reproduced in a test (for example, Azure DevOps UI settings or permissions), say so explicitly instead of skipping silently.

## Test Driven Development

When developing a feature, convert requirements and specs to tests and only then start working on implementation (red -> green -> refactor)
