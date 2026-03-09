package issueservice

import (
	"context"
	"log"

	"beads-lite/internal/issuestorage"
)

const autoCloseReason = "Auto-closed: all children completed"

func autoCloseAncestors(ctx context.Context, store issuestorage.IssueStore, issueID string) ([]string, error) {
	issue, err := store.Get(ctx, issueID)
	if err != nil {
		log.Printf("warning: auto-close skipped for %s: get issue failed: %v", issueID, err)
		return nil, nil
	}

	visited := make(map[string]bool)
	currentID := issue.Parent
	closedAncestors := make([]string, 0)

	for currentID != "" {
		if visited[currentID] {
			log.Printf("warning: auto-close stopped due to cycle in parent chain at %s", currentID)
			break
		}
		visited[currentID] = true

		parent, getErr := store.Get(ctx, currentID)
		if getErr != nil {
			log.Printf("warning: auto-close stopped at ancestor %s: get failed: %v", currentID, getErr)
			return closedAncestors, nil
		}
		nextParentID := parent.Parent

		if parent.Status == issuestorage.StatusClosed {
			currentID = nextParentID
			continue
		}
		if parent.Type == issuestorage.TypeGate || parent.Type == issuestorage.TypeMolecule {
			currentID = nextParentID
			continue
		}

		childIDs := parent.Children()
		if len(childIDs) == 0 {
			currentID = nextParentID
			continue
		}

		allChildrenClosed := true
		for _, childID := range childIDs {
			child, childErr := store.Get(ctx, childID)
			if childErr != nil {
				log.Printf("warning: auto-close stopped at ancestor %s: get child %s failed: %v", currentID, childID, childErr)
				return closedAncestors, nil
			}
			if child.Status != issuestorage.StatusClosed {
				allChildrenClosed = false
				break
			}
		}

		if !allChildrenClosed {
			break
		}

		modifyErr := store.Modify(ctx, currentID, func(i *issuestorage.Issue) error {
			i.Status = issuestorage.StatusClosed
			i.CloseReason = autoCloseReason
			return nil
		})
		if modifyErr != nil {
			log.Printf("warning: auto-close stopped at ancestor %s: modify failed: %v", currentID, modifyErr)
			return closedAncestors, nil
		}

		closedAncestors = append(closedAncestors, currentID)
		currentID = nextParentID
	}

	return closedAncestors, nil
}

func autoReopenAncestors(ctx context.Context, store issuestorage.IssueStore, issueID string) ([]string, error) {
	issue, err := store.Get(ctx, issueID)
	if err != nil {
		log.Printf("warning: auto-reopen skipped for %s: get issue failed: %v", issueID, err)
		return nil, nil
	}

	visited := make(map[string]bool)
	currentID := issue.Parent
	reopenedAncestors := make([]string, 0)

	for currentID != "" {
		if visited[currentID] {
			log.Printf("warning: auto-reopen stopped due to cycle in parent chain at %s", currentID)
			break
		}
		visited[currentID] = true

		parent, getErr := store.Get(ctx, currentID)
		if getErr != nil {
			log.Printf("warning: auto-reopen stopped at ancestor %s: get failed: %v", currentID, getErr)
			return reopenedAncestors, nil
		}
		nextParentID := parent.Parent

		if parent.Status == issuestorage.StatusClosed {
			modifyErr := store.Modify(ctx, currentID, func(i *issuestorage.Issue) error {
				i.Status = issuestorage.StatusOpen
				return nil
			})
			if modifyErr != nil {
				log.Printf("warning: auto-reopen stopped at ancestor %s: modify failed: %v", currentID, modifyErr)
				return reopenedAncestors, nil
			}
			reopenedAncestors = append(reopenedAncestors, currentID)
		}

		currentID = nextParentID
	}

	return reopenedAncestors, nil
}
