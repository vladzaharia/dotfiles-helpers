package state

import "time"

// PruneCandidate represents a VM eligible for deletion.
type PruneCandidate struct {
	VM
	IdleFor time.Duration
}

// PruneCandidates returns VM records with no live owner_pids whose
// last_active is older than `idleAfter`. Caller is responsible for
// running `orb delete` and ForgetVM on each candidate.
func PruneCandidates(idleAfter time.Duration) ([]PruneCandidate, error) {
	vms, err := VMs()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var out []PruneCandidate
	for _, v := range vms {
		if len(uniqueAlive(v.OwnerPIDs)) > 0 {
			continue
		}
		idle := now.Sub(v.LastActive)
		if idle < idleAfter {
			continue
		}
		out = append(out, PruneCandidate{VM: v, IdleFor: idle})
	}
	return out, nil
}
