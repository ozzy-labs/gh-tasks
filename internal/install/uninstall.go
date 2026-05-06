package install

// hasSharedEntry reports whether m's Shared list includes rel. The
// match is exact (no path normalization beyond the manifest's own
// forward-slash convention) so the caller's path constants flow
// through unchanged.
func hasSharedEntry(m Manifest, rel string) bool {
	for _, s := range m.Shared {
		if s == rel {
			return true
		}
	}
	return false
}

// isSharedRelReferencedByOthers reports whether any of the still-installed
// adapters in `others` lists rel under their Shared. Used by codex-cli /
// gemini-cli `PlanUninstall` to tell "I am the last reference, tear the
// marker block down" from "the other adapter still needs this aggregator,
// leave it alone." Adapters whose own uninstall is also scheduled in the
// current run are intentionally excluded by the cmd-layer caller before
// this helper is invoked.
func isSharedRelReferencedByOthers(others map[Agent]Manifest, rel string) bool {
	for _, m := range others {
		if hasSharedEntry(m, rel) {
			return true
		}
	}
	return false
}
