package app

import "github.com/henriquesakata/cerne/internal/store"

// CreateBugWithSpec creates a B- bug report (T-0006): a feature-shaped card
// with its own id sequence. Born approved when the origin resolves to human
// (the report is the approval), candidate when an agent found it itself. With
// fixes it links the delivered feature and inherits the tags the caller left
// out. The spec is the three-field report (spectmpl.BugBodyIn).
func CreateBugWithSpec(repoDir, projDir, title, module, fixes string, facets, components []string, specBody string, force bool, owner, priority, onBehalf string, by DocAuthor) (store.Feature, error) {
	return createFeatureCore(repoDir, projDir, "B", title, module, "", facets, components, specBody, force, owner, priority, fixes, by, onBehalf)
}
