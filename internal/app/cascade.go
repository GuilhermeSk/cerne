package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/henriquesakata/cerne/internal/domain"
	"github.com/henriquesakata/cerne/internal/store"
)

// cascadeFrom is the state a child leaves when a parent's action carries it.
var cascadeFrom = map[domain.Action]domain.FeatureState{
	domain.ActionApprove: domain.StateCandidate,
	domain.ActionAccept:  domain.StateImplemented,
}

// cascaded is the outcome of a parent's cascade (T-0007): the lines the
// caller prints and the children it moved, for their webhooks.
type cascaded struct {
	lines []string
	moved []store.Feature
}

// cascadeApprove carries a parent's approve to its direct candidate children
// (T-0007 crit. 1), inside the parent's critical section: each child the
// approve check passes (no impediment, the --check exit) is approved like a
// manual approve — the event with its bar and cascade_from, the approved spec
// kept, the plan scaffolded; every other candidate child stays, listed with
// its impediment codes. An actor the approve's human gate refuses takes no
// child along (crit. 2): the CLI refuses the parent first, and the UI calls
// the core with no gate of its own.
func cascadeApprove(repoDir, projDir, parentID string, gauntletOn bool) (cascaded, error) {
	var out cascaded
	if CheckHumanGate(projectGates(projDir).ApproveGate()) != nil {
		return out, nil
	}
	children, err := childrenOf(repoDir, parentID)
	if err != nil {
		return out, err
	}
	for _, c := range children {
		if c.State != cascadeFrom[domain.ActionApprove] {
			continue
		}
		imps, err := approveReadiness(repoDir, c.ID, gauntletOn)
		if err != nil {
			return out, err
		}
		if len(imps) > 0 {
			var codes []string
			for _, im := range imps {
				codes = append(codes, im.Code)
			}
			out.lines = append(out.lines, fmt.Sprintf("%s stays in candidate: %s — details: 'cerne feature approve %s --check'", c.ID, strings.Join(codes, ", "), c.ID))
			continue
		}
		payload := map[string]string{"cascade_from": parentID}
		if !gauntletOn {
			payload["gauntlet"] = "off"
		}
		bar, err := AcceptedBar(repoDir, c.ID)
		if err != nil {
			return out, err
		}
		if bar != "" {
			payload["bar"] = bar
			q, _, err := AcceptedJudgeQuestion(repoDir, c.ID)
			if err != nil {
				return out, err
			}
			if q != "" {
				payload["judge_question"] = q
			}
		}
		if err := moveChild(repoDir, &c, domain.ActionApprove, payload); err != nil {
			return out, err
		}
		if err := writePlanStub(repoDir, c.ID, c.Title); err != nil {
			return out, err
		}
		if err := keepApprovedSpec(repoDir, c.ID); err != nil {
			return out, err
		}
		out.moved = append(out.moved, c)
		out.lines = append(out.lines, fmt.Sprintf("%s: %s → %s along with %s", c.ID, cascadeFrom[domain.ActionApprove], c.State, parentID))
	}
	return out, nil
}

// childrenOf is parentID's direct children, sorted by id.
func childrenOf(repoDir, parentID string) ([]store.Feature, error) {
	all, err := store.LoadAllFeatures(repoDir)
	if err != nil {
		return nil, err
	}
	return store.ChildrenByParent(all)[parentID], nil
}

// moveChild applies action to child c through the FSM, persists it and
// appends its lifecycle event.
func moveChild(repoDir string, c *store.Feature, action domain.Action, payload map[string]string) error {
	to, err := domain.Transition(c.State, action)
	if err != nil {
		return err
	}
	c.State = to
	c.UpdatedAt = time.Now().UTC()
	if err := store.SaveFeature(repoDir, *c); err != nil {
		return err
	}
	return store.AppendEvent(repoDir, domain.NewEvent(c.ID, eventTypes[action], payload, EventActor()))
}
