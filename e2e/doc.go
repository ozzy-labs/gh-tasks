// Package e2e holds End-to-End tests for gh-tasks against real GitHub.
//
// E2E tests are guarded by the `e2e` build tag and run separately from the
// unit / flow tests in cmd/ and internal/ (which are mock-driven via
// internal/testfake). To execute:
//
//	mise run e2e                            # all flows (~20min)
//	mise run e2e:smoke                      # smoke only (~2min)
//	mise run e2e:run -- TestE2E_FlowA       # specific test
//	go test -tags=e2e -v ./e2e/...          # raw form
//
// Test resources are written to:
//   - org Project: ozzy-labs/3 ("gh-tasks dev test")
//   - user Project: ozzy-3/5  ("gh-tasks dev test")
//   - repo:        ozzy-labs/gh-tasks (Issues / PRs)
//
// All resources are tagged with the [E2E] prefix and closed (not deleted)
// at teardown to retain history. See docs/design/e2e-test-plan.md for the
// full test strategy and matrix.
package e2e
